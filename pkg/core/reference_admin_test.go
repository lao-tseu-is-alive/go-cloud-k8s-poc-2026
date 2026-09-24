package core_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/coretest"
)

// TestReferenceAdministrationRequiresAdmin checks that writing reference data
// and reading its log need goeland:admin, not just goeland:write.
func TestReferenceAdministrationRequiresAdmin(t *testing.T) {
	server, err := core.NewConnectServer(coretest.NewService(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	writer := authadapter.ContextWithUser(t.Context(), &authadapter.AuthenticatedUser{AppUserID: 7, Scopes: []string{core.ScopeRead, core.ScopeWrite}})
	calls := map[string]func(ctx context.Context) error{
		"create": func(ctx context.Context) error {
			_, err := server.CreateRelationshipType(ctx, connect.NewRequest(&goelandv1.CreateRelationshipTypeRequest{Code: "X_Y", Label: "x"}))
			return err
		},
		"update": func(ctx context.Context) error {
			_, err := server.UpdateRelationshipType(ctx, connect.NewRequest(&goelandv1.UpdateRelationshipTypeRequest{Code: "X_Y"}))
			return err
		},
		"log": func(ctx context.Context) error {
			_, err := server.ListReferenceChanges(ctx, connect.NewRequest(&goelandv1.ListReferenceChangesRequest{}))
			return err
		},
	}
	for name, call := range calls {
		if err := call(writer); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatalf("%s as a writer: want PERMISSION_DENIED, got %v", name, err)
		}
	}
	admin := authadapter.ContextWithUser(t.Context(), &authadapter.AuthenticatedUser{AppUserID: 1, Scopes: []string{core.ScopeAdmin}})
	if err := calls["log"](admin); err != nil {
		t.Fatalf("log as an admin: %v", err)
	}
}

func TestValidateReferenceCode(t *testing.T) {
	for _, ok := range []string{"OPC_DEMANDE_PC", "AB", "X1_2"} {
		if err := core.ValidateReferenceCode(ok); err != nil {
			t.Fatalf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "A", "lower", "1ABC", "A-B", "A B"} {
		if err := core.ValidateReferenceCode(bad); !errors.Is(err, core.ErrInvalidInput) {
			t.Fatalf("%q: want ErrInvalidInput, got %v", bad, err)
		}
	}
}
