package integration

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/caseexport"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// caseDocument creates a document with content and attaches it to a case.
func caseDocument(t *testing.T, env *testEnv, caseID uuid.UUID, content string, confidentiality int32) *document.Document {
	t.Helper()
	blob := ingest(t, env, content).Blob
	created, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Pièce " + uniqueToken(), ContentBlobID: &blob.ID, OperatorID: testOperator,
		Governance: core.CreateSubjectInput{ConfidentialityLevel: confidentiality},
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	link(t, env, caseID, core.LinkInput{TargetSubjectID: created.Document.ID, RelationshipTypeCode: "CASE_HAS_DOCUMENT"})
	return created.Document
}

// readArchive downloads an export as its requester and returns its files.
func readArchive(t *testing.T, env *testEnv, id uuid.UUID, requester string) map[string][]byte {
	t.Helper()
	archive, err := env.exportSvc.OpenArchive(env.ctx, id, requester, false)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer func() { _ = archive.Object.Close() }()
	raw, err := io.ReadAll(archive.Object)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("archive is not a zip: %v", err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		files[f.Name], _ = io.ReadAll(rc)
		_ = rc.Close()
	}
	return files
}

// exportRows counts the EXPORT rows of a subject per outcome.
func exportRows(t *testing.T, env *testEnv, subject uuid.UUID) map[core.AccessOutcome]int {
	t.Helper()
	counts := map[core.AccessOutcome]int{}
	for _, e := range subjectAccessLog(t, env, subject, viewerOf(t, env, testOperator)) {
		if e.Action == core.AccessExport {
			counts[e.Outcome]++
		}
	}
	return counts
}

// TestCaseExport covers GLD-029 end to end: MANAGE to request (a refusal is
// recorded), the archive built by the worker as the requester (contents
// filtered, withheld counted, every version's bytes once per digest), the
// EXPORT rows, the download reserved to the requester and the expiry.
func TestCaseExport(t *testing.T) {
	env := newTestEnv(t)
	manager, reader := newUser(t, env, "Marie Export"), newUser(t, env, "Rémi Lecture")
	c, _, err := env.caseSvc.Create(env.ctx, caseInput("Dossier à exporter "+uniqueToken(), 0))
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	managerGrant := grant(t, env, c.ID, core.GranteeUser, manager.UserID, core.LevelManage)
	grant(t, env, c.ID, core.GranteeUser, reader.UserID, core.LevelRead)

	plan := caseDocument(t, env, c.ID, "plan "+uniqueToken(), 0)
	shared := ingest(t, env, "contenu partagé "+uniqueToken()).Blob
	if _, _, _, err := env.docSvc.AddVersion(env.ctx, plan.ID, document.VersionInput{ContentBlobID: &shared.ID, OperatorID: testOperator}); err != nil {
		t.Fatalf("add version: %v", err)
	}
	annex := caseDocument(t, env, c.ID, "annexe "+uniqueToken(), 0)
	// the same bytes as the plan's second version: one blob in the archive
	if _, _, _, err := env.docSvc.AddVersion(env.ctx, annex.ID, document.VersionInput{ContentBlobID: &shared.ID, OperatorID: testOperator}); err != nil {
		t.Fatalf("add shared version: %v", err)
	}
	visibleSecret := caseDocument(t, env, c.ID, "secret lisible "+uniqueToken(), core.ConfidentialLevel)
	grant(t, env, visibleSecret.ID, core.GranteeUser, manager.UserID, core.LevelRead)
	caseDocument(t, env, c.ID, "secret caché "+uniqueToken(), core.ConfidentialLevel) // withheld
	if _, _, err := env.timelineSvc.Create(env.ctx, timeline.CreateInput{
		CaseID: c.ID, Type: timeline.TypeComment, Title: "Note", Body: "Pièces transmises.", Visibility: timeline.VisibilityRestricted, OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("create timeline entry: %v", err)
	}

	_, err = env.exportSvc.RequestExport(env.ctx, caseexport.RequestInput{CaseID: c.ID, OperatorID: reader.UserID})
	denied(t, "exporting with READ only", err)
	exp, err := env.exportSvc.RequestExport(env.ctx, caseexport.RequestInput{CaseID: c.ID, Reason: "transmission au tribunal", OperatorID: manager.UserID})
	if err != nil || exp.Status != caseexport.StatusQueued {
		t.Fatalf("request export: %+v (%v)", exp, err)
	}
	if processed, err := env.exportSvc.ProcessNext(env.ctx); err != nil || !processed {
		t.Fatalf("the worker builds the export: %v (%v)", processed, err)
	}
	if exp, err = env.exportSvc.Get(env.ctx, exp.ID, manager.UserID, false); err != nil || exp.Status != caseexport.StatusDone || exp.SizeBytes == 0 {
		t.Fatalf("the export is built: %+v (%v)", exp, err)
	}

	files := readArchive(t, env, exp.ID, manager.UserID)
	manifest := &goelandv1.CaseExportManifest{}
	if err := protojson.Unmarshal(files[caseexport.ManifestName], manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	assertManifest(t, manifest, c.ID, plan.ID, files)
	if got := exportRows(t, env, c.ID); got[core.AccessGranted] != 1 || got[core.AccessDenied] != 1 {
		t.Fatalf("every export request is recorded, granted or refused: %v", got)
	}
	if got := exportRows(t, env, visibleSecret.ID); got[core.AccessGranted] != 1 {
		t.Fatalf("a confidential document placed in an archive is recorded: %v", got)
	}

	if _, err := env.exportSvc.OpenArchive(env.ctx, exp.ID, reader.UserID, false); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("only the requester downloads: %v", err)
	}
	if _, err := env.pool.Exec(env.ctx, `UPDATE case_export SET expires_at = now() - interval '1 second' WHERE id = $1`, exp.ID); err != nil {
		t.Fatalf("age the archive: %v", err)
	}
	if n, err := env.exportSvc.ExpireArchives(env.ctx); err != nil || n != 1 {
		t.Fatalf("the expired archive is removed: %d (%v)", n, err)
	}
	if _, err := env.exportSvc.OpenArchive(env.ctx, exp.ID, manager.UserID, false); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("an expired export cannot be downloaded: %v", err)
	}

	late, err := env.exportSvc.RequestExport(env.ctx, caseexport.RequestInput{CaseID: c.ID, OperatorID: manager.UserID})
	if err != nil {
		t.Fatalf("request a second export: %v", err)
	}
	if _, _, err := env.accessSvc.RevokeGrant(env.ctx, managerGrant.ID, testOperator, "fin du mandat"); err != nil {
		t.Fatalf("revoke MANAGE: %v", err)
	}
	if _, err := env.exportSvc.ProcessNext(env.ctx); err != nil {
		t.Fatalf("process the second export: %v", err)
	}
	if late, err = env.exportSvc.Get(env.ctx, late.ID, manager.UserID, false); err != nil || late.Status != caseexport.StatusFailed || late.Error == "" {
		t.Fatalf("an export whose requester lost MANAGE fails with a reason: %+v (%v)", late, err)
	}
}

// assertManifest checks the contents of the archive built in TestCaseExport.
func assertManifest(t *testing.T, m *goelandv1.CaseExportManifest, caseID, planID uuid.UUID, files map[string][]byte) {
	t.Helper()
	if m.GetFormatVersion() != caseexport.FormatVersion || m.GetCase().GetSubjectRef().GetId() != caseID.String() || len(m.GetTimeline()) != 1 {
		t.Fatalf("manifest header, case and timeline: %+v", m)
	}
	if len(m.GetDocuments()) != 3 || m.GetWithheld().GetRelationships() != 1 {
		t.Fatalf("three readable documents, the hidden one withheld: %d documents, withheld %+v", len(m.GetDocuments()), m.GetWithheld())
	}
	i := slices.IndexFunc(m.GetDocuments(), func(d *goelandv1.ExportedDocument) bool {
		return d.GetDocument().GetSubjectRef().GetId() == planID.String()
	})
	if i < 0 || len(m.GetDocuments()[i].GetVersions()) != 2 {
		t.Fatalf("every version of a document is listed")
	}
	if len(m.GetBlobs()) != 4 || len(files) != 5 {
		t.Fatalf("four distinct blobs, each once, plus the manifest: %d blobs, %d files", len(m.GetBlobs()), len(files))
	}
	for _, b := range m.GetBlobs() {
		if int64(len(files[b.GetPath()])) != b.GetSizeBytes() {
			t.Fatalf("blob %s is in the archive with its size", b.GetPath())
		}
	}
	for _, d := range m.GetDocuments() {
		for _, v := range d.GetVersions() {
			if v.GetContent().GetStorageRef() != "" {
				t.Fatal("the archive never discloses internal storage references")
			}
		}
	}
	if len(m.GetAudit()) == 0 {
		t.Fatal("the audit of the case and documents is included")
	}
}
