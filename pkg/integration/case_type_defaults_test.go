package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/access"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// templateLine builds a default grant line and fails on a shape error.
func templateLine(t *testing.T, kind core.GranteeKind, id string, level core.Level) core.DefaultGrant {
	t.Helper()
	g, err := core.NewDefaultGrant(kind, id, level)
	if err != nil {
		t.Fatalf("default grant %s %s: %v", kind, id, err)
	}
	return g
}

// TestCaseTypeDefaults covers GLD-050: a case type's minimum confidentiality
// and default grants are applied once to its new cases — creator units
// resolved at creation, the highest level kept for a grantee already granted —
// while a later template change leaves existing cases unchanged.
func TestCaseTypeDefaults(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	code := referenceCode("ITD")
	if _, _, err := env.caseSvc.CreateCaseType(ctx, casefile.CaseTypeInput{
		Code: code, Label: "Contentieux test", DefaultConfidentialityLevel: core.ConfidentialLevel, OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("create case type: %v", err)
	}
	creator, reader := newUser(t, env, "Céline Créatrice"), newUser(t, env, "Rémi Lecteur")
	unit := newUnit(t, env, "SERVICE", "Service contentieux "+uniqueToken(), nil)
	member(t, env, creator, unit.ID, "USER_MEMBER_OF_ORG_UNIT")
	group, _, err := env.accessSvc.CreateGroup(ctx, access.GroupInput{Name: "Juristes " + uniqueToken(), OperatorID: testOperator})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	_, _, err = env.caseSvc.SetDefaultGrants(ctx, code, casefile.DefaultGrantsInput{OperatorID: testOperator, Grants: []core.DefaultGrant{
		templateLine(t, core.GranteeUser, "it-unknown-"+uniqueToken(), core.LevelRead),
	}})
	if !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("an unknown user in a template: want ErrInvalidInput, got %v", err)
	}
	entry, change, err := env.caseSvc.SetDefaultGrants(ctx, code, casefile.DefaultGrantsInput{OperatorID: testOperator, Reason: "modèle", Grants: []core.DefaultGrant{
		templateLine(t, core.GranteeCreatorUnits, "", core.LevelContribute),
		templateLine(t, core.GranteeGroup, group.ID.String(), core.LevelRead),
		templateLine(t, core.GranteeUser, reader.UserID, core.LevelRead),
		templateLine(t, core.GranteeUser, creator.UserID, core.LevelRead), // below the creator's FULL_CONTROL
	}})
	if err != nil || len(entry.DefaultGrants) != 4 || change.EventType != core.ReferenceUpdated || change.AfterState["default_grants"] == nil {
		t.Fatalf("set default grants: %+v %+v (%v)", entry, change, err)
	}

	c, _, err := env.caseSvc.Create(ctx, casefile.CreateInput{CaseTypeCode: code, Title: "Litige " + uniqueToken(), OperatorID: creator.UserID})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if c.RecordMetadata.ConfidentialityLevel != core.ConfidentialLevel {
		t.Fatalf("the type's minimum confidentiality applies: %d", c.RecordMetadata.ConfidentialityLevel)
	}
	expectAccess(t, env, creator.UserID, c.ID, core.LevelFullControl, core.SourcePersonal) // the highest level is kept
	expectAccess(t, env, reader.UserID, c.ID, core.LevelRead, core.SourcePersonal)
	colleague := newUser(t, env, "Corinne Collègue")
	member(t, env, colleague, unit.ID, "USER_MEMBER_OF_ORG_UNIT")
	expectAccess(t, env, colleague.UserID, c.ID, core.LevelContribute, core.SourceOrgUnit) // the creator's unit

	if _, _, err := env.caseSvc.SetDefaultGrants(ctx, code, casefile.DefaultGrantsInput{OperatorID: testOperator, Reason: "vidé"}); err != nil {
		t.Fatalf("clear default grants: %v", err)
	}
	expectAccess(t, env, reader.UserID, c.ID, core.LevelRead, core.SourcePersonal) // copied once, kept
	later, _, err := env.caseSvc.Create(ctx, casefile.CreateInput{CaseTypeCode: code, Title: "Litige suivant " + uniqueToken(), OperatorID: creator.UserID})
	if err != nil {
		t.Fatalf("create a later case: %v", err)
	}
	if a, err := env.accessSvc.MyAccess(ctx, reader.UserID, later.ID); err != nil || a.Level != core.LevelNone {
		t.Fatalf("a later case no longer gets the cleared template: %+v (%v)", a, err)
	}
	if _, _, err := env.caseSvc.SetDefaultGrants(ctx, "UNKNOWN_"+uuid.NewString()[:8], casefile.DefaultGrantsInput{OperatorID: testOperator}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("an unknown case type: want ErrNotFound, got %v", err)
	}
}
