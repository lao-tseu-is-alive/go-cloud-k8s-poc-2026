package document

import (
	"context"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists documents. It reuses the transversal core primitives
// (subject_ref, record_metadata, audit_event, relationships) so that a document
// and its governance/identity are created atomically.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Document, *core.AuditEvent, *core.SubjectRelationship, error)
	Get(ctx context.Context, id uuid.UUID) (*Document, error)
	UpdateMetadata(ctx context.Context, id uuid.UUID, in UpdateInput) (*Document, *core.AuditEvent, error)
	Finalize(ctx context.Context, id uuid.UUID, actorUserID, reason string, alsoLock bool) (*Document, *core.AuditEvent, error)
	Verify(ctx context.Context, id uuid.UUID, expectedSHA256 string) (*Document, bool, error)
	Search(ctx context.Context, filter SearchFilter) (SearchResult, error)
	Link(ctx context.Context, in core.LinkInput) (*core.SubjectRelationship, *core.AuditEvent, error)
	SoftDelete(ctx context.Context, id uuid.UUID, actorUserID, reason string) (*core.AuditEvent, error)
	ListTypes(ctx context.Context, onlyActive bool) ([]*DocumentType, error)
}
