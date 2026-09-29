package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/orgunit"
)

// newUnit creates a unit (abbreviation TST) and fails the test on error. Roots
// are siblings, so callers give roots a unique label.
func newUnit(t *testing.T, env *testEnv, typeCode, label string, parent *uuid.UUID) *orgunit.OrgUnit {
	t.Helper()
	u, ev, err := env.orgUnitSvc.Create(env.ctx, orgunit.CreateInput{
		Input: orgunit.Input{TypeCode: typeCode, Abbreviation: "TST", Label: label, ParentID: parent, OperatorID: testOperator},
	})
	if err != nil {
		t.Fatalf("create org unit %q: %v", label, err)
	}
	if ev.EventType != "ORG_UNIT_CREATED" || ev.SubjectID != u.ID {
		t.Fatalf("expected ORG_UNIT_CREATED on the unit, got %+v", ev)
	}
	return u
}

func TestOrgUnitTypesSeeded(t *testing.T) {
	env := newTestEnv(t)
	types, err := env.orgUnitSvc.ListTypes(env.ctx, true)
	if err != nil || len(types) < 7 || types[0].Code != "ENTERPRISE" {
		t.Fatalf("expected the 7 production types, ENTERPRISE first, got %d (%v)", len(types), err)
	}
}

// TestOrgUnitTree covers the tree rules: identity, path, sibling labels,
// shared abbreviations, external references, e-mail normalization, no cycle
// (service and trigger), dissolution order and the effects of a dissolved unit.
func TestOrgUnitTree(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	root := newUnit(t, env, "ENTERPRISE", "Ville "+uniqueToken(), nil)
	direction := newUnit(t, env, "DIRECTION", "Direction test", &root.ID)
	service := newUnit(t, env, "SERVICE", "Service test", &direction.ID)

	detail, err := env.orgUnitSvc.Get(ctx, service.ID)
	if err != nil || len(detail.Ancestors) != 2 || detail.Ancestors[0].ID != root.ID || detail.Ancestors[1].ID != direction.ID {
		t.Fatalf("the path must go from the root down to the parent: %+v (%v)", detail, err)
	}
	if detail.Unit.Subject.Kind != core.SubjectKindOrgUnit || detail.Unit.Subject.DisplayLabel != "Service test (TST)" {
		t.Fatalf("unexpected subject %+v", detail.Unit.Subject)
	}
	assertSiblingsAndRefs(t, env, root, direction)

	edit := orgunit.Input{TypeCode: "SERVICE", Abbreviation: "SRV", Label: "Service renommé", Email: "Service.Test@Lausanne.CH", ParentID: &direction.ID, OperatorID: testOperator}
	updated, ev, err := env.orgUnitSvc.Update(ctx, service.ID, edit)
	if err != nil || updated.Email != "Service.Test@lausanne.ch" || updated.Subject.DisplayLabel != "Service renommé (SRV)" || ev.BeforeState["label"] != "Service test" {
		t.Fatalf("update must normalize the e-mail and sync the label: %+v / %+v (%v)", updated, ev, err)
	}
	edit.Email = "pas-un-email"
	if _, _, err := env.orgUnitSvc.Update(ctx, service.ID, edit); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("invalid e-mail: want ErrInvalidInput, got %v", err)
	}

	assertNoCycle(t, env, root, service)
	assertDissolution(t, env, direction, service)
}

// assertSiblingsAndRefs checks that live siblings never share a label (any
// case) while homonyms elsewhere and shared abbreviations are fine, and that
// an external reference is unique.
func assertSiblingsAndRefs(t *testing.T, env *testEnv, root, direction *orgunit.OrgUnit) {
	t.Helper()
	ctx := env.ctx
	twin := orgunit.CreateInput{Input: orgunit.Input{TypeCode: "SERVICE", Label: "SERVICE TEST", ParentID: &direction.ID, OperatorID: testOperator}}
	if _, _, err := env.orgUnitSvc.Create(ctx, twin); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a sibling with the same label (other case): want ErrConflict, got %v", err)
	}
	ref := "test:" + uniqueToken()
	elsewhere := orgunit.CreateInput{ExternalRef: ref, Input: orgunit.Input{TypeCode: "SERVICE", Abbreviation: "TST", Label: "Service test", ParentID: &root.ID, OperatorID: testOperator}}
	homonym, _, err := env.orgUnitSvc.Create(ctx, elsewhere)
	if err != nil || homonym.ExternalRef != ref {
		t.Fatalf("the same label and abbreviation under another parent are fine: %+v (%v)", homonym, err)
	}
	elsewhere.Label = "Autre service"
	if _, _, err := env.orgUnitSvc.Create(ctx, elsewhere); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a taken external reference: want ErrConflict, got %v", err)
	}
	found, err := env.orgUnitSvc.Search(ctx, orgunit.SearchFilter{Query: ref})
	if err != nil || found.TotalSize != 1 || found.Units[0].ID != homonym.ID {
		t.Fatalf("search by exact external reference: %+v (%v)", found, err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE org_unit SET external_ref = 'changed' WHERE id = $1`, homonym.ID); err == nil {
		t.Fatal("the database must refuse an external reference change")
	}
}

// assertNoCycle checks that a unit cannot move under its own descendant and
// that the database refuses a cycle too.
func assertNoCycle(t *testing.T, env *testEnv, root, descendant *orgunit.OrgUnit) {
	t.Helper()
	move := orgunit.Input{TypeCode: "ENTERPRISE", Label: root.Label, ParentID: &descendant.ID, OperatorID: testOperator}
	if _, _, err := env.orgUnitSvc.Update(env.ctx, root.ID, move); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("moving a unit under its descendant: want ErrInvalidInput, got %v", err)
	}
	if _, err := env.pool.Exec(env.ctx, `UPDATE org_unit SET parent_id = $1 WHERE id = $2`, descendant.ID, root.ID); err == nil {
		t.Fatal("the database must refuse a cycle")
	}
}

// assertDissolution checks the dissolution order and what a dissolved unit refuses.
func assertDissolution(t *testing.T, env *testEnv, parent, child *orgunit.OrgUnit) {
	t.Helper()
	ctx := env.ctx
	if _, _, err := env.orgUnitSvc.Dissolve(ctx, parent.ID, testOperator, "réorganisation"); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("dissolving a unit with live sub-units: want ErrInvalidState, got %v", err)
	}
	if _, _, err := env.orgUnitSvc.Dissolve(ctx, child.ID, testOperator, " "); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("dissolving without a reason: want ErrInvalidInput, got %v", err)
	}
	dissolved, ev, err := env.orgUnitSvc.Dissolve(ctx, child.ID, testOperator, "fusion")
	if err != nil || !dissolved.Dissolved() || dissolved.DissolutionReason != "fusion" || ev.EventType != "ORG_UNIT_DISSOLVED" {
		t.Fatalf("dissolve: %+v (%v)", dissolved, err)
	}
	edit := orgunit.Input{TypeCode: "SERVICE", Label: "Trop tard", OperatorID: testOperator}
	if _, _, err := env.orgUnitSvc.Update(ctx, child.ID, edit); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("editing a dissolved unit: want ErrInvalidState, got %v", err)
	}
	under := orgunit.CreateInput{Input: orgunit.Input{TypeCode: "UNIT", Label: "Orphelin", ParentID: &child.ID, OperatorID: testOperator}}
	if _, _, err := env.orgUnitSvc.Create(ctx, under); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("a child of a dissolved unit: want ErrInvalidInput, got %v", err)
	}
	live, err := env.orgUnitSvc.List(ctx, false)
	if err != nil || containsNode(live, child.ID) || !containsNode(live, parent.ID) {
		t.Fatalf("the tree hides dissolved units by default (%v)", err)
	}
	all, err := env.orgUnitSvc.List(ctx, true)
	if err != nil || !containsNode(all, child.ID) {
		t.Fatalf("include_dissolved returns them (%v)", err)
	}
	found, err := env.orgUnitSvc.Search(ctx, orgunit.SearchFilter{Query: "tst", Limit: 200})
	if err != nil || !containsUnit(found.Units, parent.ID) || containsUnit(found.Units, child.ID) {
		t.Fatalf("search by exact abbreviation (any case) returns live units only: %+v (%v)", found, err)
	}
}

// TestOrgUnitGovernanceAndRoles covers the owning unit of a subject and the
// case ↔ unit roles, both refused for a dissolved unit.
func TestOrgUnitGovernanceAndRoles(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	live := newUnit(t, env, "SERVICE", "Service pilote "+uniqueToken(), nil)
	gone := newUnit(t, env, "SERVICE", "Service fermé "+uniqueToken(), nil)
	if _, _, err := env.orgUnitSvc.Dissolve(ctx, gone.ID, testOperator, "fermeture"); err != nil {
		t.Fatalf("dissolve: %v", err)
	}

	owned, _, err := env.caseSvc.Create(ctx, casefile.CreateInput{
		CaseTypeCode: "GENERIC_REQUEST", Title: "Affaire du service " + uniqueToken(), OperatorID: testOperator,
		Governance: core.CreateSubjectInput{OwnerOrgID: &live.ID},
	})
	if err != nil || owned.RecordMetadata.OwnerOrgID == nil || *owned.RecordMetadata.OwnerOrgID != live.ID {
		t.Fatalf("a subject can be owned by a live unit: %+v (%v)", owned, err)
	}
	orphan := casefile.CreateInput{
		CaseTypeCode: "GENERIC_REQUEST", Title: "Affaire orpheline", OperatorID: testOperator,
		Governance: core.CreateSubjectInput{OwnerOrgID: &gone.ID},
	}
	if _, _, err := env.caseSvc.Create(ctx, orphan); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("owning unit dissolved: want ErrInvalidInput, got %v", err)
	}

	leader := core.LinkInput{SourceSubjectID: owned.ID, TargetSubjectID: live.ID, RelationshipTypeCode: "CASE_HAS_ORG_UNIT_LEADER", OperatorID: testOperator}
	if _, _, err := env.coreSvc.LinkSubjects(ctx, leader); err != nil {
		t.Fatalf("a case can be led by a unit: %v", err)
	}
	leader.TargetSubjectID = gone.ID
	if _, _, err := env.coreSvc.LinkSubjects(ctx, leader); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("linking a dissolved unit: want ErrInvalidInput, got %v", err)
	}
}

func containsUnit(units []*orgunit.OrgUnit, id uuid.UUID) bool {
	for _, u := range units {
		if u.ID == id {
			return true
		}
	}
	return false
}

func containsNode(nodes []*orgunit.Node, id uuid.UUID) bool {
	for _, n := range nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}
