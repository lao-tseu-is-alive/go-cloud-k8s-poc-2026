package orgunit

import (
	"context"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists organizational units and their type catalogue. Unit
// mutations are serialized on the tree and write their audit event in the
// same transaction; type changes write a reference_change entry.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*OrgUnit, *core.AuditEvent, error)
	Get(ctx context.Context, id uuid.UUID) (*Detail, error)
	List(ctx context.Context, includeDissolved bool) ([]*Node, error)
	Search(ctx context.Context, filter SearchFilter) (SearchResult, error)
	Update(ctx context.Context, id uuid.UUID, in Input) (*OrgUnit, *core.AuditEvent, error)
	Dissolve(ctx context.Context, id uuid.UUID, operatorID, reason string) (*OrgUnit, *core.AuditEvent, error)
	ListTypes(ctx context.Context, onlyActive bool) ([]*OrgUnitType, error)
	CreateType(ctx context.Context, in TypeInput) (*OrgUnitType, *core.ReferenceChange, error)
	UpdateType(ctx context.Context, code string, in TypeUpdate) (*OrgUnitType, *core.ReferenceChange, error)
}
