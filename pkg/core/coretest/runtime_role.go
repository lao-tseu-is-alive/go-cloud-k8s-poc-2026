package coretest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
)

// RuntimeRoleURL creates a throwaway login role with a random password,
// grants it the runtime privileges as the schema owner behind owner
// (coremodule.GrantRuntimeRole, GLD-046), and returns ownerDSN rewritten to
// connect as that role. The role is dropped when the test ends.
func RuntimeRoleURL(t *testing.T, owner *pgxpool.Pool, ownerDSN string) string {
	t.Helper()
	ctx := context.Background()
	role := "goeland_rt_" + randomHex(t, 6)
	password := randomHex(t, 16)
	ident := pgx.Identifier{role}.Sanitize()
	if _, err := owner.Exec(ctx, "CREATE ROLE "+ident+" LOGIN PASSWORD '"+password+"'"); err != nil {
		t.Fatalf("create runtime role: %v", err)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, "DROP OWNED BY "+ident)
		_, _ = owner.Exec(ctx, "DROP ROLE IF EXISTS "+ident)
	})
	if err := coremodule.GrantRuntimeRole(ctx, owner, role); err != nil {
		t.Fatalf("grant runtime role: %v", err)
	}
	u, err := url.Parse(ownerDSN)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatalf("the test DSN must be a postgres:// URL to derive the runtime role's (%v)", err)
	}
	u.User = url.UserPassword(role, password)
	return u.String()
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
