package casefile

import (
	"context"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists cases. It reuses the transversal core primitives
// (subject_ref, record_metadata, business reference, audit_event) so a case and
// its governance/identity are created atomically.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Case, *core.AuditEvent, error)
	Get(ctx context.Context, id uuid.UUID) (*Case, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Case, *core.AuditEvent, error)
	Transition(ctx context.Context, id uuid.UUID, in TransitionInput) (*Case, *core.AuditEvent, error)
	Search(ctx context.Context, filter SearchFilter) (SearchResult, error)
	SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error)
	ListTypes(ctx context.Context, onlyActive bool) ([]*CaseType, error)
}
