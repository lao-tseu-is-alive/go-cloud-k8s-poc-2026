package actor

import (
	"context"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists actors. It reuses the transversal core primitives
// (subject_ref, record_metadata, audit_event) so an actor and its
// governance/identity are created atomically.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Actor, *core.AuditEvent, error)
	Get(ctx context.Context, id uuid.UUID) (*Actor, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Actor, *core.AuditEvent, error)
	Search(ctx context.Context, filter SearchFilter) (SearchResult, error)
	SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error)
	ListCategories(ctx context.Context, onlyActive bool) ([]*OrganizationCategory, error)
}
