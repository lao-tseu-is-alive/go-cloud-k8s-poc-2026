package task

import (
	"context"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists case tasks. Every change locks the case's governance
// row, rejects a closed, locked or deleted case and writes its audit event on
// the CASE subject in the same transaction.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Task, *core.AuditEvent, error)
	Get(ctx context.Context, id uuid.UUID) (*Task, error)
	ListCase(ctx context.Context, filter CaseFilter) (ListResult, error)
	ListMine(ctx context.Context, filter MineFilter) (ListResult, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Task, *core.AuditEvent, error)
	Assign(ctx context.Context, id uuid.UUID, to Assignee, operatorID, reason string) (*Task, *core.AuditEvent, error)
	ChangeStatus(ctx context.Context, id uuid.UUID, move Move, operatorID, note string) (*Task, *core.AuditEvent, error)
	ListTypes(ctx context.Context, onlyActive bool) ([]*TaskType, error)
	CreateType(ctx context.Context, in TypeInput) (*TaskType, *core.ReferenceChange, error)
	UpdateType(ctx context.Context, code string, in TypeUpdate) (*TaskType, *core.ReferenceChange, error)
}
