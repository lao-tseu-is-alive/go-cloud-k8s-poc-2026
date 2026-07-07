package core

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
)

func TestOperatorID(t *testing.T) {
	// The operator is ALWAYS the authenticated principal — never a client-supplied
	// value — so audit attribution cannot be forged.
	user := &authadapter.AuthenticatedUser{AppUserID: 42}
	if got := OperatorID(user); got != "42" {
		t.Fatalf("OperatorID(user 42) = %q, want \"42\"", got)
	}
	if got := OperatorID(nil); got != "" {
		t.Fatalf("OperatorID(nil) = %q, want \"\"", got)
	}
}

func TestMapError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"invalid", ErrInvalidInput, connect.CodeInvalidArgument},
		{"not found", ErrNotFound, connect.CodeNotFound},
		{"unauthenticated", ErrUnauthenticated, connect.CodeUnauthenticated},
		{"conflict", ErrConflict, connect.CodeAlreadyExists},
		{"kind mismatch", ErrKindMismatch, connect.CodeFailedPrecondition},
		{"locked", ErrLocked, connect.CodeFailedPrecondition},
		{"deleted", ErrDeleted, connect.CodeFailedPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := MapError(tt.err)
			if mapped == nil {
				t.Fatalf("MapError(%v) = nil, want code %v", tt.err, tt.want)
			}
			if connect.CodeOf(mapped) != tt.want {
				t.Fatalf("MapError(%v) code = %v, want %v", tt.err, connect.CodeOf(mapped), tt.want)
			}
		})
	}
	// Wrapped domain errors must still map (errors.Is semantics).
	wrapped := errors.New("boom")
	if MapError(wrapped) != nil {
		t.Fatalf("MapError(unknown) should return nil so the caller logs + returns internal")
	}
}

func TestRequestIDContextRoundTrip(t *testing.T) {
	ctx := WithRequestID(t.Context(), "req-123")
	if got := RequestIDFromContext(ctx); got != "req-123" {
		t.Fatalf("RequestIDFromContext = %q, want req-123", got)
	}
	// Empty id must not be stored.
	if got := RequestIDFromContext(WithRequestID(t.Context(), "")); got != "" {
		t.Fatalf("empty request id should not round-trip, got %q", got)
	}
}
