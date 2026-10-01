package core

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
)

// Authorizer checks, in the Connect adapters, the caller's level on the subject
// an RPC addresses (GLD-048). Entities owned by a case (timeline entries,
// tasks, circulations) are checked inside their transaction instead
// (EnsureAccessTx on the case).
type Authorizer struct {
	q   Querier
	log *slog.Logger
}

// NewAuthorizer builds an Authorizer on q (the shared pool). A nil logger
// falls back to slog.Default.
func NewAuthorizer(q Querier, log *slog.Logger) (*Authorizer, error) {
	if q == nil {
		return nil, errors.New("authorizer: a querier is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Authorizer{q: q, log: log}, nil
}

// Caller authenticates the caller with scope, then requires need on subjectID;
// failures are Connect errors (Unauthenticated, PermissionDenied, NotFound).
func (a *Authorizer) Caller(ctx context.Context, scope string, subjectID uuid.UUID, need Level) (*authadapter.AuthenticatedUser, error) {
	user, err := RequireCaller(ctx, scope)
	if err != nil {
		return nil, err
	}
	if err := a.Require(ctx, user, subjectID, need); err != nil {
		return nil, err
	}
	return user, nil
}

// Require requires need for user on subjectID; failures are Connect errors.
func (a *Authorizer) Require(ctx context.Context, user *authadapter.AuthenticatedUser, subjectID uuid.UUID, need Level) error {
	if err := EnsureAccessTx(ctx, a.q, OperatorID(user), subjectID, need); err != nil {
		return ToConnectError(a.log, "access", err)
	}
	return nil
}
