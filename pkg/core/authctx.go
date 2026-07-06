package core

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
)

const (
	// ScopeRead is the OAuth scope required for read-only Goéland operations.
	ScopeRead = "goeland:read"
	// ScopeWrite is the OAuth scope required for mutating Goéland operations.
	ScopeWrite = "goeland:write"
)

// RequireCaller extracts the authenticated user from context and verifies the required scope.
func RequireCaller(ctx context.Context, scope string) (*authadapter.AuthenticatedUser, error) {
	user, err := authadapter.RequireUser(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if !user.HasScope(scope) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("required scope is missing"))
	}
	return user, nil
}

// ActorID returns the actor id to record for audit/ownership: the explicit
// request value when provided, otherwise the authenticated user's app id.
func ActorID(user *authadapter.AuthenticatedUser, requestActorID string) string {
	if requestActorID != "" {
		return requestActorID
	}
	if user != nil {
		return strconv.FormatInt(user.AppUserID, 10)
	}
	return ""
}

// NewTimeoutInterceptor returns a Connect interceptor enforcing a hard per-RPC deadline.
func NewTimeoutInterceptor(d time.Duration) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			return next(ctx, req)
		}
	})
}

// MapError converts domain errors to Connect status codes. Unexpected errors are
// logged by the caller and surfaced as a generic internal error.
func MapError(err error) *connect.Error {
	switch {
	case errors.Is(err, ErrInvalidInput):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrUnauthenticated):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, ErrConflict):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, ErrKindMismatch):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, ErrLocked):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return nil
	}
}
