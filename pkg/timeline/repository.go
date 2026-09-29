package timeline

import (
	"context"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists case timelines. Every mutation locks the case's
// governance row, rejects a closed, locked or deleted case and writes its audit
// event on the CASE subject in the same transaction.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Entry, *core.AuditEvent, error)
	Get(ctx context.Context, id uuid.UUID) (*Entry, error)
	List(ctx context.Context, filter ListFilter) (ListResult, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Entry, *core.AuditEvent, error)
	Validate(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error)
	Lock(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error)
	Withdraw(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error)
	LinkDocument(ctx context.Context, entryID, documentID uuid.UUID, operatorID string) (*Entry, *core.AuditEvent, error)
	UnlinkDocument(ctx context.Context, entryID, documentID uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error)
}
