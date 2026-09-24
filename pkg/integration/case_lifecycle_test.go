package integration

import (
	"errors"
	"strings"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

// TestCaseTypesSeeded asserts the seeded case types and their namespaces.
func TestCaseTypesSeeded(t *testing.T) {
	env := newTestEnv(t)
	types, err := env.caseSvc.ListTypes(env.ctx, true)
	if err != nil {
		t.Fatalf("list case types: %v", err)
	}
	namespaces := map[string]string{}
	for _, ct := range types {
		namespaces[ct.Code] = ct.BusinessRefNamespace
	}
	if namespaces["OPC_DEMANDE_PC"] != "OPC" || namespaces["GENERIC_REQUEST"] != "GEN" {
		t.Fatalf("unexpected seeded case types %v", namespaces)
	}
}

// openCase opens an OPC case and returns it.
func openCase(t *testing.T, env *testEnv, title string) *casefile.Case {
	t.Helper()
	c, ev, err := env.caseSvc.Create(env.ctx, casefile.CreateInput{
		CaseTypeCode: "OPC_DEMANDE_PC",
		Title:        title,
		OperatorID:   testOperator,
		Governance:   core.CreateSubjectInput{ConfidentialityLevel: 1},
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if ev.EventType != "CASE_CREATED" {
		t.Fatalf("expected CASE_CREATED, got %+v", ev)
	}
	return c
}

// TestCaseLifecycle covers the spec §3.1 / v2 §50 case steps: open with an
// allocated business reference, link participants and a document, move
// through the lifecycle, freeze a closed case, reopen, search and audit.
func TestCaseLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	token := uniqueToken()

	c := openCase(t, env, "Permis de construire château "+token)
	if c.Status != casefile.StatusOpen || c.ClosedAt != nil {
		t.Fatalf("a new case is OPEN, got %s", c.Status)
	}
	if c.Subject.BusinessRefNamespace != "OPC" || !strings.HasPrefix(c.Subject.BusinessRef, currentPeriod(t)+"-") {
		t.Fatalf("the OPC type must allocate an OPC reference, got %q in %q", c.Subject.BusinessRef, c.Subject.BusinessRefNamespace)
	}
	linkParticipants(t, env, c)

	rels, err := env.caseSvc.Relationships(ctx, c.ID)
	if err != nil || len(rels) != 4 {
		t.Fatalf("expected 4 relationships (requester, mandatee, architect, document), got %d (%v)", len(rels), err)
	}

	if _, _, err := env.caseSvc.Transition(ctx, c.ID, casefile.TransitionInput{Target: casefile.StatusInProgress, OperatorID: testOperator}); err != nil {
		t.Fatalf("OPEN → IN_PROGRESS: %v", err)
	}
	if _, _, err := env.caseSvc.Transition(ctx, c.ID, casefile.TransitionInput{Target: casefile.StatusClosed, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("closing without a reason: want ErrInvalidInput, got %v", err)
	}
	closed, ev, err := env.caseSvc.Transition(ctx, c.ID, casefile.TransitionInput{Target: casefile.StatusClosed, OperatorID: testOperator, Reason: "permis délivré"})
	if err != nil || closed.Status != casefile.StatusClosed || closed.ClosedAt == nil || closed.ClosureReason != "permis délivré" {
		t.Fatalf("close: %+v (%v)", closed, err)
	}
	if ev.BeforeState["status"] != "IN_PROGRESS" || ev.AfterState["status"] != "CLOSED" {
		t.Fatalf("CASE_STATUS_CHANGED must record before/after, got %+v / %+v", ev.BeforeState, ev.AfterState)
	}
	assertClosedCaseFrozen(t, env, c)

	reopened, _, err := env.caseSvc.Transition(ctx, c.ID, casefile.TransitionInput{Target: casefile.StatusOpen, OperatorID: testOperator, Reason: "recours"})
	if err != nil || reopened.Status != casefile.StatusOpen || reopened.ClosedAt != nil || reopened.ClosureReason != "" {
		t.Fatalf("reopen must clear the closure stamps: %+v (%v)", reopened, err)
	}
	updated, _, err := env.caseSvc.Update(ctx, c.ID, casefile.UpdateInput{Title: "Permis modifié " + token, OperatorID: testOperator})
	if err != nil || updated.Subject.DisplayLabel != updated.Title {
		t.Fatalf("update after reopen must succeed and sync the label: %+v (%v)", updated, err)
	}
	assertCaseSearch(t, env, c, token)
	assertAuditTrail(t, env, c)
}

// linkParticipants links a requester, a mandatee and a document to the case.
func linkParticipants(t *testing.T, env *testEnv, c *casefile.Case) {
	t.Helper()
	requester, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{ActorKind: actor.KindPerson, DisplayName: "Requester " + uniqueToken(), OperatorID: testOperator})
	if err != nil {
		t.Fatalf("create requester: %v", err)
	}
	mandatee, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{
		ActorKind: actor.KindOrganization, DisplayName: "Bureau " + uniqueToken(), LegalName: "Bureau SA", OperatorID: testOperator,
	})
	if err != nil {
		t.Fatalf("create mandatee: %v", err)
	}
	for code, target := range map[string]*actor.Actor{
		"CASE_HAS_ACTOR_REQUESTER": requester, "CASE_HAS_ACTOR_MANDATEE": mandatee, "CASE_HAS_ACTOR_ARCHITECT": mandatee,
	} {
		if _, _, err := env.coreSvc.LinkSubjects(env.ctx, core.LinkInput{
			SourceSubjectID: c.ID, TargetSubjectID: target.ID, RelationshipTypeCode: code, OperatorID: testOperator,
		}); err != nil {
			t.Fatalf("link %s: %v", code, err)
		}
	}
	if _, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Plan " + uniqueToken(), OperatorID: testOperator, LinkToCaseID: &c.ID,
	}); err != nil {
		t.Fatalf("create document linked to the case: %v", err)
	}
}

// assertClosedCaseFrozen checks that a closed case rejects edits and invalid moves.
func assertClosedCaseFrozen(t *testing.T, env *testEnv, c *casefile.Case) {
	t.Helper()
	if _, _, err := env.caseSvc.Update(env.ctx, c.ID, casefile.UpdateInput{Title: "edit while closed", OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("editing a closed case: want ErrInvalidState, got %v", err)
	}
	if _, _, err := env.caseSvc.Transition(env.ctx, c.ID, casefile.TransitionInput{Target: casefile.StatusSuspended, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("CLOSED → SUSPENDED: want ErrInvalidState, got %v", err)
	}
}

// assertCaseSearch finds the case by accent-insensitive text, business reference and status.
func assertCaseSearch(t *testing.T, env *testEnv, c *casefile.Case, token string) {
	t.Helper()
	for name, filter := range map[string]casefile.SearchFilter{
		"text":         {Query: token},
		"business ref": {Query: c.Subject.BusinessRef},
		"status":       {Query: token, Status: casefile.StatusOpen, CaseTypeCode: "OPC_DEMANDE_PC"},
	} {
		res, err := env.caseSvc.Search(env.ctx, filter)
		if err != nil || !containsCase(res.Cases, c) {
			t.Fatalf("search by %s must find the case: %d results (%v)", name, len(res.Cases), err)
		}
	}
	res, err := env.caseSvc.Search(env.ctx, casefile.SearchFilter{Query: token, Status: casefile.StatusClosed})
	if err != nil || containsCase(res.Cases, c) {
		t.Fatalf("an OPEN case must not match a CLOSED filter (%v)", err)
	}
}

// assertAuditTrail checks the case's audit history.
func assertAuditTrail(t *testing.T, env *testEnv, c *casefile.Case) {
	t.Helper()
	events, err := env.caseSvc.RecentAudit(env.ctx, c.ID)
	if err != nil {
		t.Fatalf("recent audit: %v", err)
	}
	seen := map[string]int{}
	for _, ev := range events {
		seen[ev.EventType]++
	}
	if seen["CASE_CREATED"] != 1 || seen["CASE_STATUS_CHANGED"] != 3 || seen["CASE_UPDATED"] != 1 || seen["RELATIONSHIP_LINKED"] < 2 {
		t.Fatalf("unexpected audit trail %v", seen)
	}
}

func containsCase(cases []*casefile.Case, c *casefile.Case) bool {
	for _, got := range cases {
		if got.ID == c.ID {
			return true
		}
	}
	return false
}

// TestCaseExplicitReferenceAndDeletion covers an explicit business reference
// overriding the type's allocation, and soft deletion.
func TestCaseExplicitReferenceAndDeletion(t *testing.T) {
	env := newTestEnv(t)
	legacy := "LEG-" + uniqueToken()
	c, _, err := env.caseSvc.Create(env.ctx, casefile.CreateInput{
		CaseTypeCode: "GENERIC_REQUEST", Title: "Imported", OperatorID: testOperator,
		BusinessRef: core.BusinessRefRequest{Value: legacy},
	})
	if err != nil || c.Subject.BusinessRef != legacy || c.Subject.BusinessRefNamespace != "" {
		t.Fatalf("explicit reference must win over the type namespace: %+v (%v)", c.Subject, err)
	}
	if _, err := env.caseSvc.SoftDelete(env.ctx, c.ID, testOperator, "duplicate"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, _, err := env.caseSvc.Transition(env.ctx, c.ID, casefile.TransitionInput{Target: casefile.StatusInProgress, OperatorID: testOperator}); !errors.Is(err, core.ErrDeleted) {
		t.Fatalf("moving a deleted case: want ErrDeleted, got %v", err)
	}
	res, err := env.caseSvc.Search(env.ctx, casefile.SearchFilter{Query: legacy})
	if err != nil || containsCase(res.Cases, c) {
		t.Fatalf("a deleted case must be hidden from the default search (%v)", err)
	}
	res, err = env.caseSvc.Search(env.ctx, casefile.SearchFilter{Query: legacy, IncludeDeleted: true})
	if err != nil || !containsCase(res.Cases, c) {
		t.Fatalf("include_deleted must return the deleted case (%v)", err)
	}
}
