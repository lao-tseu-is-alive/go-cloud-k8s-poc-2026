package circulation

import (
	"context"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists case circulations. Every change locks the case's
// governance row, rejects a closed, locked or deleted case and writes its audit
// event on the CASE subject in the same transaction as the tasks and timeline
// entries it produces.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Circulation, *core.AuditEvent, error)
	Get(ctx context.Context, id uuid.UUID) (*Circulation, error)
	ListCase(ctx context.Context, caseID uuid.UUID) ([]*Circulation, error)
	Respond(ctx context.Context, in RespondInput) (*Circulation, *core.AuditEvent, error)
	Cancel(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Circulation, *core.AuditEvent, error)
}
