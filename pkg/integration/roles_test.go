package integration

import (
	"errors"
	"slices"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// grantAdmin grants ADMIN to user as the test operator and fails on error.
func grantAdmin(t *testing.T, env *testEnv, user *core.AppUser) {
	t.Helper()
	in := core.RoleChangeInput{UserID: user.UserID, RoleCode: core.RoleAdmin, Reason: "test", OperatorID: testOperator}
	if _, ev, err := env.coreSvc.GrantUserRole(env.ctx, in); err != nil || ev.EventType != core.EventUserRoleGranted || ev.SubjectID != user.SubjectID {
		t.Fatalf("grant ADMIN to %s: %+v (%v)", user.UserID, ev, err)
	}
}

// TestApplicationRoles covers GLD-047: roles stored in Goéland, granted and
// revoked with a reason, audited on the USER subject, kept as history, and the
// derived is_admin / roles of the user.
func TestApplicationRoles(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	alice, bob := newUser(t, env, "Alice Roles"), newUser(t, env, "Bob Roles")

	roles, err := env.coreSvc.ListAppRoles(ctx)
	if err != nil || !slices.ContainsFunc(roles, func(r *core.AppRole) bool { return r.Code == core.RoleAdmin && r.IsActive }) {
		t.Fatalf("ADMIN is seeded: %+v (%v)", roles, err)
	}
	grantAdmin(t, env, alice)
	grantAdmin(t, env, bob)
	again := core.RoleChangeInput{UserID: alice.UserID, RoleCode: core.RoleAdmin, Reason: "again", OperatorID: testOperator}
	if _, _, err := env.coreSvc.GrantUserRole(ctx, again); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a held role: want ErrConflict, got %v", err)
	}
	checkRoleRefusals(t, env, alice)

	users, err := env.coreSvc.BatchGetUsers(ctx, []string{alice.UserID})
	if err != nil || len(users) != 1 || !users[0].IsAdmin || !slices.Equal(users[0].Roles, []string{core.RoleAdmin}) {
		t.Fatalf("is_admin and roles derive from the assignments: %+v (%v)", users, err)
	}
	holders, err := env.coreSvc.ListRoleHolders(ctx, core.RoleAdmin)
	if err != nil || !slices.ContainsFunc(holders, func(u *core.AppUser) bool { return u.UserID == bob.UserID }) {
		t.Fatalf("bob holds ADMIN: %v", err)
	}

	revoke := core.RoleChangeInput{UserID: alice.UserID, RoleCode: core.RoleAdmin, Reason: "changement de poste", OperatorID: testOperator}
	revoked, ev, err := env.coreSvc.RevokeUserRole(ctx, revoke)
	if err != nil || revoked.RevokedAt == nil || ev.EventType != core.EventUserRoleRevoked || ev.Reason != "changement de poste" {
		t.Fatalf("revoke: %+v %+v (%v)", revoked, ev, err)
	}
	if _, _, err := env.coreSvc.RevokeUserRole(ctx, revoke); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("revoking a role not held: want ErrNotFound, got %v", err)
	}
	history, err := env.coreSvc.ListUserRoles(ctx, alice.UserID, true)
	current, _ := env.coreSvc.ListUserRoles(ctx, alice.UserID, false)
	if err != nil || len(history) != 1 || len(current) != 0 {
		t.Fatalf("a revocation is kept as history: %d rows, %d current (%v)", len(history), len(current), err)
	}
	if n := countUserEvents(t, env, alice, core.EventUserRoleGranted) + countUserEvents(t, env, alice, core.EventUserRoleRevoked); n != 2 {
		t.Fatalf("grant and revocation are audited on the user: %d events", n)
	}
	grantAdmin(t, env, alice) // granted again after a revocation
}

// checkRoleRefusals covers the input and state refusals of a grant.
func checkRoleRefusals(t *testing.T, env *testEnv, user *core.AppUser) {
	t.Helper()
	cases := []struct {
		in   core.RoleChangeInput
		want error
	}{
		{core.RoleChangeInput{UserID: "it-never-seen-" + uniqueToken(), RoleCode: core.RoleAdmin, Reason: "x", OperatorID: testOperator}, core.ErrNotFound},
		{core.RoleChangeInput{UserID: user.UserID, RoleCode: "NO_SUCH_ROLE", Reason: "x", OperatorID: testOperator}, core.ErrNotFound},
		{core.RoleChangeInput{UserID: user.UserID, RoleCode: "lower", Reason: "x", OperatorID: testOperator}, core.ErrInvalidInput},
		{core.RoleChangeInput{UserID: user.UserID, RoleCode: core.RoleAdmin, Reason: " ", OperatorID: testOperator}, core.ErrInvalidInput},
	}
	for _, c := range cases {
		if _, _, err := env.coreSvc.GrantUserRole(env.ctx, c.in); !errors.Is(err, c.want) {
			t.Fatalf("grant %+v: want %v, got %v", c.in, c.want, err)
		}
	}
}
