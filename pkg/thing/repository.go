package thing

import (
	"context"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists things, their detail blocks and the thing type catalogue.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Thing, *core.AuditEvent, error)
	Get(ctx context.Context, id uuid.UUID) (*Thing, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Thing, *core.AuditEvent, error)
	Search(ctx context.Context, filter SearchFilter) (SearchResult, error)
	SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error)
	ListTypes(ctx context.Context, onlyActive bool) ([]*ThingType, error)
	CreateThingType(ctx context.Context, in ThingTypeInput) (*ThingType, *core.ReferenceChange, error)
	UpdateThingType(ctx context.Context, code string, in ThingTypeUpdate) (*ThingType, *core.ReferenceChange, error)
}
