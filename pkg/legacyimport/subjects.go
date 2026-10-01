package legacyimport

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// subject is the identity and governance of one imported subject.
type subject struct {
	id              uuid.UUID
	kind            core.SubjectKind
	label           string
	createdAt       *time.Time
	createdBy       string
	updatedAt       *time.Time
	confidentiality int32
	businessRef     string
	businessRefNS   string
	sourceTable     string
	sourceID        int64
}

// newSubject builds the subject of legacy row sourceID of sourceTable.
func newSubject(kind core.SubjectKind, sourceTable string, sourceID int64, label string) subject {
	return subject{id: ID(string(kind), sourceID), kind: kind, label: label, sourceTable: sourceTable, sourceID: sourceID}
}

var (
	subjectRefColumns     = []string{"id", "kind", "display_label", "created_at", "business_ref", "business_ref_namespace"}
	recordMetadataColumns = []string{"subject_id", "created_at", "created_by", "updated_at", "confidentiality_level"}
	provenanceColumns     = []string{"subject_id", "source_system", "source_table", "source_id", "import_batch_id"}
)

// writeSubjects writes the subject_ref, record_metadata and provenance rows of
// subjects and remembers their ids. A missing creation date becomes the
// import time (the columns are never NULL).
func (imp *Importer) writeSubjects(ctx context.Context, subjects []subject) error {
	refs := make([][]any, len(subjects))
	metadata := make([][]any, len(subjects))
	provenance := make([][]any, len(subjects))
	for i, s := range subjects {
		refs[i], metadata[i], provenance[i] = imp.subjectRefRow(s), imp.metadataRow(s), imp.provenanceRow(s)
		imp.subjects[s.id] = s.kind
	}
	if err := imp.copyRows(ctx, "subject_ref", subjectRefColumns, refs); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "record_metadata", recordMetadataColumns, metadata); err != nil {
		return err
	}
	return imp.copyRows(ctx, "subject_provenance", provenanceColumns, provenance)
}

// subjectRefRow, metadataRow and provenanceRow are the rows of one subject.
func (imp *Importer) subjectRefRow(s subject) []any {
	return []any{s.id, string(s.kind), s.label, *firstTime(s.createdAt, &imp.now), s.businessRef, s.businessRefNS}
}

func (imp *Importer) metadataRow(s subject) []any {
	createdBy := s.createdBy
	if createdBy == "" {
		createdBy = OperatorID
	}
	return []any{s.id, *firstTime(s.createdAt, &imp.now), createdBy, s.updatedAt, s.confidentiality}
}

func (imp *Importer) provenanceRow(s subject) []any {
	return []any{s.id, SourceSystem, s.sourceTable, strconv.FormatInt(s.sourceID, 10), imp.batchID}
}
