package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/access"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// grant gives a level on subject as the test operator (its creator) and fails on error.
func grant(t *testing.T, env *testEnv, subject uuid.UUID, kind core.GranteeKind, grantee string, level core.Level) *access.Grant {
	t.Helper()
	g, _, err := env.accessSvc.SetGrant(env.ctx, access.SetGrantInput{
		SubjectID: subject, GranteeKind: kind, GranteeID: grantee, Level: level, Reason: "test", OperatorID: testOperator,
	})
	if err != nil {
		t.Fatalf("grant %s %s %s: %v", kind, grantee, level, err)
	}
	return g
}

// expectAccess checks a user's effective level and its source on a subject.
func expectAccess(t *testing.T, env *testEnv, user string, subject uuid.UUID, level core.Level, source core.AccessSource) {
	t.Helper()
	a, err := env.accessSvc.MyAccess(env.ctx, user, subject)
	if err != nil || a.Level != level || a.Source != source {
		t.Fatalf("access of %s: got %s from %s (%v), want %s from %s", user, a.Level, a.Source, err, level, source)
	}
}

// member links a user to an org unit or a group as the test operator.
func member(t *testing.T, env *testEnv, user *core.AppUser, target uuid.UUID, typeCode string) {
	t.Helper()
	if _, _, err := env.coreSvc.LinkSubjects(env.ctx, core.LinkInput{
		SourceSubjectID: user.SubjectID, TargetSubjectID: target, RelationshipTypeCode: typeCode, OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("membership %s: %v", typeCode, err)
	}
}

// TestAccessPrecedence covers the effective level (GLD-048): baseline, nearest
// unit over its ancestors, groups over units, personal over everything, roles,
// and confidentiality where neither roles nor the baseline apply.
func TestAccessPrecedence(t *testing.T) {
	env := newTestEnv(t)
	c := openCase(t, env, "Accès "+uniqueToken())
	x := newUser(t, env, "Xavier Accès")
	direction := newUnit(t, env, "DIRECTION", "Direction accès "+uniqueToken(), nil)
	service := newUnit(t, env, "SERVICE", "Service accès "+uniqueToken(), &direction.ID)
	member(t, env, x, service.ID, "USER_MEMBER_OF_ORG_UNIT")

	expectAccess(t, env, testOperator, c.ID, core.LevelFullControl, core.SourcePersonal)
	expectAccess(t, env, x.UserID, c.ID, core.LevelRead, core.SourceBaseline)
	grant(t, env, c.ID, core.GranteeOrgUnit, direction.ID.String(), core.LevelManage)
	expectAccess(t, env, x.UserID, c.ID, core.LevelManage, core.SourceOrgUnit) // a unit grant covers its sub-units
	grant(t, env, c.ID, core.GranteeOrgUnit, service.ID.String(), core.LevelRead)
	expectAccess(t, env, x.UserID, c.ID, core.LevelRead, core.SourceOrgUnit) // the nearest unit wins, even lower

	group, _, err := env.accessSvc.CreateGroup(env.ctx, access.GroupInput{Name: "Commission " + uniqueToken(), OperatorID: testOperator})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, _, err := env.accessSvc.AddMember(env.ctx, access.MemberInput{GroupID: group.ID, UserID: x.UserID, OperatorID: testOperator}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	grant(t, env, c.ID, core.GranteeGroup, group.ID.String(), core.LevelContribute)
	expectAccess(t, env, x.UserID, c.ID, core.LevelContribute, core.SourceGroup) // groups before units
	personal := grant(t, env, c.ID, core.GranteeUser, x.UserID, core.LevelRead)
	expectAccess(t, env, x.UserID, c.ID, core.LevelRead, core.SourcePersonal) // personal before everything

	if _, _, err := env.accessSvc.RevokeGrant(env.ctx, personal.ID, testOperator, "fin"); err != nil {
		t.Fatalf("revoke personal grant: %v", err)
	}
	if _, _, err := env.accessSvc.ArchiveGroup(env.ctx, group.ID, testOperator, "commission dissoute"); err != nil {
		t.Fatalf("archive group: %v", err)
	}
	expectAccess(t, env, x.UserID, c.ID, core.LevelRead, core.SourceOrgUnit) // an archived group grants nothing
	checkRolesAndConfidentiality(t, env, c)
}

// checkRolesAndConfidentiality covers ADMIN and kind-wide roles, and a
// confidential case where only explicit grants count.
func checkRolesAndConfidentiality(t *testing.T, env *testEnv, c *casefile.Case) {
	t.Helper()
	admin := newUser(t, env, "Ada Admin Accès")
	grantAdmin(t, env, admin)
	expectAccess(t, env, admin.UserID, c.ID, core.LevelFullControl, core.SourceRole)

	secret, _, err := env.caseSvc.Create(env.ctx, casefile.CreateInput{
		CaseTypeCode: "GENERIC_REQUEST", Title: "Affaire confidentielle " + uniqueToken(), OperatorID: testOperator,
		Governance: core.CreateSubjectInput{ConfidentialityLevel: core.ConfidentialLevel},
	})
	if err != nil {
		t.Fatalf("create confidential case: %v", err)
	}
	expectAccess(t, env, admin.UserID, secret.ID, core.LevelNone, core.SourceBaseline) // no administrator bypass
	expectAccess(t, env, testOperator, secret.ID, core.LevelFullControl, core.SourcePersonal)

	manager := newUser(t, env, "Marc Gestionnaire")
	if _, _, err := env.coreSvc.GrantUserRole(env.ctx, core.RoleChangeInput{UserID: manager.UserID, RoleCode: "ACTOR_MANAGER", Reason: "test", OperatorID: testOperator}); err != nil {
		t.Fatalf("grant ACTOR_MANAGER: %v", err)
	}
	actor := newActor(t, env)
	expectAccess(t, env, manager.UserID, actor.ID, core.LevelManage, core.SourceRole)
	expectAccess(t, env, manager.UserID, c.ID, core.LevelRead, core.SourceBaseline) // the role covers actors only
}

// TestGrantRules covers who may grant, the history of a changed grant and the
// last FULL_CONTROL guard.
func TestGrantRules(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Règles d'accès "+uniqueToken())
	x := newUser(t, env, "Xénia Règles")

	if _, _, err := env.accessSvc.SetGrant(ctx, access.SetGrantInput{SubjectID: c.ID, GranteeKind: core.GranteeUser, GranteeID: x.UserID, Level: core.LevelFullControl, Reason: "moi", OperatorID: x.UserID}); !errors.Is(err, core.ErrPermissionDenied) {
		t.Fatalf("granting without FULL_CONTROL: want ErrPermissionDenied, got %v", err)
	}
	if _, _, err := env.accessSvc.SetGrant(ctx, access.SetGrantInput{SubjectID: c.ID, GranteeKind: core.GranteeUser, GranteeID: "it-never-seen", Level: core.LevelRead, Reason: "x", OperatorID: testOperator}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("a user who never signed in: want ErrNotFound, got %v", err)
	}
	grant(t, env, c.ID, core.GranteeUser, x.UserID, core.LevelRead)
	if _, _, err := env.accessSvc.SetGrant(ctx, access.SetGrantInput{SubjectID: c.ID, GranteeKind: core.GranteeUser, GranteeID: x.UserID, Level: core.LevelRead, Reason: "encore", OperatorID: testOperator}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("the same level again: want ErrConflict, got %v", err)
	}
	changed, ev, err := env.accessSvc.SetGrant(ctx, access.SetGrantInput{SubjectID: c.ID, GranteeKind: core.GranteeUser, GranteeID: x.UserID, Level: core.LevelFullControl, Reason: "co-responsable", OperatorID: testOperator})
	if err != nil || ev.EventType != access.EventAccessChanged || changed.Level != core.LevelFullControl {
		t.Fatalf("change a level: %+v %+v (%v)", changed, ev, err)
	}
	history, err := env.accessSvc.ListGrants(ctx, testOperator, c.ID, true)
	if err != nil || countGrantsOf(history, x.UserID) != 2 {
		t.Fatalf("a changed grant keeps the old row: %d rows (%v)", countGrantsOf(history, x.UserID), err)
	}

	creator := currentGrantOf(t, env, c.ID, testOperator)
	if _, _, err := env.accessSvc.RevokeGrant(ctx, creator.ID, testOperator, "je passe la main"); err != nil {
		t.Fatalf("revoking a FULL_CONTROL while another remains: %v", err)
	}
	if _, _, err := env.accessSvc.RevokeGrant(ctx, changed.ID, x.UserID, "plus personne"); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("revoking the last FULL_CONTROL: want ErrInvalidState, got %v", err)
	}
}

// countGrantsOf counts the grants (current and history) of a user.
func countGrantsOf(grants []*access.Grant, userID string) int {
	n := 0
	for _, g := range grants {
		if g.GranteeUserID != nil && *g.GranteeUserID == userID {
			n++
		}
	}
	return n
}

// currentGrantOf returns the current grant of a user on a subject.
func currentGrantOf(t *testing.T, env *testEnv, subject uuid.UUID, userID string) *access.Grant {
	t.Helper()
	grants, err := env.accessSvc.ListGrants(env.ctx, testOperator, subject, false)
	if err != nil {
		t.Fatalf("list grants: %v", err)
	}
	for _, g := range grants {
		if g.GranteeUserID != nil && *g.GranteeUserID == userID {
			return g
		}
	}
	t.Fatalf("no current grant of %s", userID)
	return nil
}
