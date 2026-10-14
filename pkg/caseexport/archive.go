package caseexport

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/circulation"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// assembly is an archive ready to be written: its manifest, the content
// blobs to copy (one per digest) and the exported documents.
type assembly struct {
	manifest  *goelandv1.CaseExportManifest
	blobs     []*document.ContentBlob
	documents []uuid.UUID
}

// assemble reads the case as the requester (viewer, with level on the case)
// and builds the manifest.
func (s *Service) assemble(ctx context.Context, exp *Export, viewer core.Viewer, level core.Level) (*assembly, error) {
	c, err := s.sources.Cases.Get(ctx, exp.CaseID)
	if err != nil {
		return nil, fmt.Errorf("read case: %w", err)
	}
	now := time.Now()
	m := &goelandv1.CaseExportManifest{
		FormatVersion: FormatVersion, ExportId: exp.ID.String(), ExportedAt: timestamppb.New(now),
		ExportedBy: exp.RequestedBy, Case: casefile.DomainToProto(c),
	}
	if err := s.addCaseContents(ctx, m, exp, level, now); err != nil {
		return nil, err
	}
	rels, err := collectPages(func(offset int) ([]*core.SubjectRelationship, error) {
		res, err := s.sources.Core.ListRelationships(ctx, core.RelationshipFilter{
			Viewer: viewer, SubjectID: exp.CaseID, BothDirections: true, Limit: core.MaxPageSize, Offset: offset,
		})
		return res.Relationships, err
	})
	if err != nil {
		return nil, fmt.Errorf("list relationships: %w", err)
	}
	m.Relationships = core.DomainRelationshipsToProto(rels)
	a := &assembly{manifest: m}
	if err := s.addDocuments(ctx, a, exp.CaseID, rels); err != nil {
		return nil, err
	}
	if err := s.addHistory(ctx, a, exp.CaseID, rels); err != nil {
		return nil, err
	}
	return a, s.addWithheld(ctx, m, exp.CaseID)
}

// addCaseContents adds the timeline, tasks and circulations of the case.
func (s *Service) addCaseContents(ctx context.Context, m *goelandv1.CaseExportManifest, exp *Export, level core.Level, now time.Time) error {
	entries, err := collectPages(func(offset int) ([]*timeline.Entry, error) {
		res, err := s.sources.Timeline.List(ctx, timeline.ListFilter{
			CaseID: exp.CaseID, IncludeWithdrawn: true, ViewerID: exp.RequestedBy,
			MaxVisibility: timeline.MaxVisibility(level), Limit: core.MaxPageSize, Offset: offset,
		})
		return res.Entries, err
	})
	if err != nil {
		return fmt.Errorf("list timeline: %w", err)
	}
	tasks, err := collectPages(func(offset int) ([]*task.Task, error) {
		res, err := s.sources.Tasks.ListCase(ctx, task.CaseFilter{CaseID: exp.CaseID, Limit: core.MaxPageSize, Offset: offset})
		return res.Tasks, err
	})
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}
	circulations, err := s.sources.Circulations.ListCase(ctx, exp.CaseID)
	if err != nil {
		return fmt.Errorf("list circulations: %w", err)
	}
	m.Timeline = timeline.DomainsToProto(entries)
	m.Tasks = task.DomainsToProto(tasks, now)
	m.Circulations = circulation.DomainsToProto(circulations, now)
	return nil
}

// addDocuments adds every readable document related to the case with all its
// versions, and collects their content blobs once per digest.
func (s *Service) addDocuments(ctx context.Context, a *assembly, caseID uuid.UUID, rels []*core.SubjectRelationship) error {
	seenBlobs := map[string]bool{}
	for _, id := range otherEnds(rels, caseID, core.SubjectKindDocument) {
		doc, err := s.sources.Documents.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("read document %s: %w", id, err)
		}
		versions, err := s.sources.Documents.ListVersions(ctx, id)
		if err != nil {
			return fmt.Errorf("list versions of %s: %w", id, err)
		}
		exported := &goelandv1.ExportedDocument{Document: document.DomainToProto(doc)}
		hideStorageRef(exported.Document.GetCurrentVersion())
		for _, v := range versions {
			if b := v.Content; b != nil && b.StorageRef != "" && !seenBlobs[b.SHA256] {
				seenBlobs[b.SHA256] = true
				a.blobs = append(a.blobs, b)
				a.manifest.Blobs = append(a.manifest.Blobs, &goelandv1.ExportedBlob{
					Sha256: b.SHA256, SizeBytes: b.FileSizeBytes, MimeType: b.MimeType, Path: blobDir + b.SHA256,
				})
			}
			pv := document.VersionToProto(v)
			hideStorageRef(pv)
			exported.Versions = append(exported.Versions, pv)
		}
		a.manifest.Documents = append(a.manifest.Documents, exported)
		a.documents = append(a.documents, id)
	}
	return nil
}

// addHistory adds the audit of the case and of the exported documents, and
// the provenance of every exported subject.
func (s *Service) addHistory(ctx context.Context, a *assembly, caseID uuid.UUID, rels []*core.SubjectRelationship) error {
	for _, id := range append([]uuid.UUID{caseID}, a.documents...) {
		events, err := collectPages(func(offset int) ([]*core.AuditEvent, error) {
			res, err := s.sources.Core.ListAuditEvents(ctx, core.AuditFilter{SubjectID: id, Limit: core.MaxPageSize, Offset: offset})
			return res.Events, err
		})
		if err != nil {
			return fmt.Errorf("list audit of %s: %w", id, err)
		}
		a.manifest.Audit = append(a.manifest.Audit, core.DomainAuditEventsToProto(events)...)
	}
	ids := append([]uuid.UUID{caseID}, otherEnds(rels, caseID, "")...)
	rows, err := s.pool.Query(ctx, listProvenanceSQL, pgx.NamedArgs{"ids": ids})
	if err != nil {
		return fmt.Errorf("list provenance: %w", err)
	}
	provenance, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*goelandv1.SubjectProvenance, error) {
		var subjectID, batchID uuid.UUID
		p := &goelandv1.SubjectProvenance{}
		err := row.Scan(&subjectID, &p.SourceSystem, &p.SourceTable, &p.SourceId, &batchID)
		p.SubjectId, p.ImportBatchId = subjectID.String(), batchID.String()
		return p, err
	})
	if err != nil {
		return fmt.Errorf("read provenance: %w", err)
	}
	a.manifest.Provenance = provenance
	return nil
}

// addWithheld counts what the requester could not read: the relationships
// and timeline entries of the case missing from the manifest.
func (s *Service) addWithheld(ctx context.Context, m *goelandv1.CaseExportManifest, caseID uuid.UUID) error {
	var rels, entries int32
	args := pgx.NamedArgs{"case_id": caseID}
	if err := s.pool.QueryRow(ctx, countRelationshipsSQL, args).Scan(&rels); err != nil {
		return fmt.Errorf("count relationships: %w", err)
	}
	if err := s.pool.QueryRow(ctx, countTimelineSQL, args).Scan(&entries); err != nil {
		return fmt.Errorf("count timeline: %w", err)
	}
	m.Withheld = &goelandv1.ExportWithheld{
		Relationships:   max(rels-int32(len(m.Relationships)), 0),
		TimelineEntries: max(entries-int32(len(m.Timeline)), 0),
	}
	return nil
}

// writeArchive writes the manifest then every blob into w as a ZIP, dated at
// the export time, checking each blob's digest while copying it.
func (s *Service) writeArchive(ctx context.Context, w io.Writer, a *assembly) error {
	zw := zip.NewWriter(w)
	manifest, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(a.manifest)
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	at := a.manifest.GetExportedAt().AsTime()
	f, err := zw.CreateHeader(&zip.FileHeader{Name: ManifestName, Method: zip.Deflate, Modified: at})
	if err != nil {
		return err
	}
	if _, err := f.Write(manifest); err != nil {
		return err
	}
	for _, b := range a.blobs {
		if err := s.copyBlob(ctx, zw, b, at); err != nil {
			return err
		}
	}
	return zw.Close()
}

// copyBlob copies one content blob into the archive and fails when its bytes
// no longer match the recorded digest.
func (s *Service) copyBlob(ctx context.Context, zw *zip.Writer, b *document.ContentBlob, at time.Time) error {
	obj, err := s.store.Get(ctx, b.StorageRef)
	if err != nil {
		return fmt.Errorf("open blob %s: %w", b.SHA256, err)
	}
	defer func() { _ = obj.Close() }()
	f, err := zw.CreateHeader(&zip.FileHeader{Name: blobDir + b.SHA256, Method: zip.Deflate, Modified: at})
	if err != nil {
		return err
	}
	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, digest), obj); err != nil {
		return fmt.Errorf("copy blob %s: %w", b.SHA256, err)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != b.SHA256 {
		return fmt.Errorf("blob %s: stored bytes have digest %s", b.SHA256, got)
	}
	return nil
}

// storeArchive streams the archive of a into the blob store.
func (s *Service) storeArchive(ctx context.Context, exp *Export, a *assembly) (blobstore.Stored, error) {
	pr, pw := io.Pipe()
	written := make(chan error, 1)
	go func() {
		err := s.writeArchive(ctx, pw, a)
		_ = pw.CloseWithError(err)
		written <- err
	}()
	stored, err := s.store.Put(ctx, pr, blobstore.Metadata{Filename: "case-export-" + exp.ID.String() + ".zip", ContentType: "application/zip"})
	_ = pr.CloseWithError(io.ErrClosedPipe) // unblocks the writer when Put stopped early
	writeErr := <-written
	if err != nil { // a write error reaches Put through the pipe
		return blobstore.Stored{}, fmt.Errorf("store archive: %w", err)
	}
	return stored, writeErr
}

// otherEnds returns the distinct other ends of rels seen from subjectID, of
// kind (any kind when empty), in first-seen order.
func otherEnds(rels []*core.SubjectRelationship, subjectID uuid.UUID, kind core.SubjectKind) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	var ids []uuid.UUID
	for _, rel := range rels {
		other := rel.Target
		if rel.TargetSubjectID == subjectID {
			other = rel.Source
		}
		if other == nil || seen[other.ID] || (kind != "" && other.Kind != kind) {
			continue
		}
		seen[other.ID] = true
		ids = append(ids, other.ID)
	}
	return ids
}

// hideStorageRef blanks the internal storage reference of a version's content:
// the archive names its bytes by digest.
func hideStorageRef(v *goelandv1.DocumentVersion) {
	if v.GetContent() != nil {
		v.Content.StorageRef = ""
	}
}

// collectPages calls fetch with growing offsets until a short page.
func collectPages[T any](fetch func(offset int) ([]T, error)) ([]T, error) {
	var all []T
	for offset := 0; ; offset += core.MaxPageSize {
		page, err := fetch(offset)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < core.MaxPageSize {
			return all, nil
		}
	}
}
