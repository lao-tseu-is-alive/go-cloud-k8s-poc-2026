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
	Create(ctx context.Context, in CreateInput) (CreateResult, error)
	Get(ctx context.Context, id uuid.UUID) (*Document, error)
	AddVersion(ctx context.Context, documentID uuid.UUID, in VersionInput) (*Document, *Version, *core.AuditEvent, error)
	ListVersions(ctx context.Context, documentID uuid.UUID) ([]*Version, error)
	RegisterBlob(ctx context.Context, blob ContentBlob) (*ContentBlob, bool, error)
	FindBlobBySHA256(ctx context.Context, sha256 string) (*ContentBlob, error)
	UpdateMetadata(ctx context.Context, id uuid.UUID, in UpdateInput) (*Document, *core.AuditEvent, error)
	Finalize(ctx context.Context, id uuid.UUID, operatorID, reason string, alsoLock bool) (*Document, *core.AuditEvent, error)
	Verify(ctx context.Context, id uuid.UUID, expectedSHA256 string) (*Document, bool, error)
	Search(ctx context.Context, filter SearchFilter) (SearchResult, error)
	Link(ctx context.Context, in core.LinkInput) (*core.SubjectRelationship, *core.AuditEvent, error)
	SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error)
	ListTypes(ctx context.Context, onlyActive bool) ([]*DocumentType, error)
	CreateDocumentType(ctx context.Context, in DocumentTypeInput) (*DocumentType, *core.ReferenceChange, error)
	UpdateDocumentType(ctx context.Context, code string, in DocumentTypeUpdate) (*DocumentType, *core.ReferenceChange, error)
}
