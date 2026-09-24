package integration

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// newActor creates an organization actor for relationship tests.
func newActor(t *testing.T, env *testEnv) *actor.Actor {
	t.Helper()
	a, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{
		ActorKind: actor.KindOrganization, DisplayName: "Bureau " + uniqueToken(), LegalName: "Bureau SA", OperatorID: testOperator,
	})
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}
	return a
}

// link creates an edge from source as the test operator.
func link(t *testing.T, env *testEnv, source uuid.UUID, in core.LinkInput) *core.SubjectRelationship {
	t.Helper()
	in.SourceSubjectID, in.OperatorID = source, testOperator
	rel, _, err := env.coreSvc.LinkSubjects(env.ctx, in)
	if err != nil {
		t.Fatalf("link %s: %v", in.RelationshipTypeCode, err)
	}
	return rel
}

// TestEndRelationship covers ending an edge (history kept, relink allowed) as
// opposed to unlinking it, with the state and validity-order guards.
func TestEndRelationship(t *testing.T) {
	env := newTestEnv(t)
	c := openCase(t, env, "Mandat "+uniqueToken())
	mandatee := newActor(t, env)
	start := time.Now().Add(-48 * time.Hour)
	first := link(t, env, c.ID, core.LinkInput{TargetSubjectID: mandatee.ID, RelationshipTypeCode: "CASE_HAS_ACTOR_MANDATEE", ValidFrom: &start})

	beforeStart := start.Add(-time.Hour)
	if _, _, err := env.coreSvc.EndRelationship(env.ctx, core.EndInput{RelationshipID: first.ID, ValidTo: &beforeStart, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("end before valid_from: want ErrInvalidInput, got %v", err)
	}

	ended, ev, err := env.coreSvc.EndRelationship(env.ctx, core.EndInput{RelationshipID: first.ID, OperatorID: testOperator, Reason: " mandate revoked "})
	if err != nil {
		t.Fatalf("end relationship: %v", err)
	}
	if ended.ValidTo == nil || ended.RelationshipType == nil || ended.RelationshipType.Code != "CASE_HAS_ACTOR_MANDATEE" || ended.Target == nil {
		t.Fatalf("ended edge not stamped or not hydrated: %+v", ended)
	}
	if ev.EventType != "RELATIONSHIP_ENDED" || ev.Reason != "mandate revoked" || ev.SubjectID != c.ID {
		t.Fatalf("unexpected audit event %+v", ev)
	}
	if _, _, err := env.coreSvc.EndRelationship(env.ctx, core.EndInput{RelationshipID: first.ID, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("ending twice: want ErrInvalidState, got %v", err)
	}

	// The ended edge no longer blocks a new open edge of the same type, which in turn does.
	second := link(t, env, c.ID, core.LinkInput{TargetSubjectID: mandatee.ID, RelationshipTypeCode: "CASE_HAS_ACTOR_MANDATEE"})
	if _, _, err := env.coreSvc.LinkSubjects(env.ctx, core.LinkInput{
		SourceSubjectID: c.ID, TargetSubjectID: mandatee.ID, RelationshipTypeCode: "CASE_HAS_ACTOR_MANDATEE", OperatorID: testOperator,
	}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate open edge: want ErrConflict, got %v", err)
	}
	assertEndedKeptAsHistory(t, env, c.ID, first.ID, second.ID)

	// An unlinked edge cannot be ended.
	if _, _, err := env.coreSvc.UnlinkSubjects(env.ctx, second.ID, testOperator, "mistake"); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if _, _, err := env.coreSvc.EndRelationship(env.ctx, core.EndInput{RelationshipID: second.ID, OperatorID: testOperator}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("end unlinked edge: want ErrNotFound, got %v", err)
	}
}

// TestEndRelationshipDefaultBeforeFutureStart checks that the default end (the
// database time) is rejected for an edge whose validity starts in the future.
func TestEndRelationshipDefaultBeforeFutureStart(t *testing.T) {
	env := newTestEnv(t)
	c := openCase(t, env, "Futur "+uniqueToken())
	future := time.Now().Add(24 * time.Hour)
	rel := link(t, env, c.ID, core.LinkInput{TargetSubjectID: newActor(t, env).ID, RelationshipTypeCode: "CASE_HAS_ACTOR_OWNER", ValidFrom: &future})
	if _, _, err := env.coreSvc.EndRelationship(env.ctx, core.EndInput{RelationshipID: rel.ID, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("default end before future start: want ErrInvalidInput, got %v", err)
	}
	scheduled := future.Add(30 * 24 * time.Hour)
	if ended, _, err := env.coreSvc.EndRelationship(env.ctx, core.EndInput{RelationshipID: rel.ID, ValidTo: &scheduled, OperatorID: testOperator}); err != nil || ended.ValidTo == nil {
		t.Fatalf("scheduled end: %v", err)
	}
}

// assertEndedKeptAsHistory checks that both the ended and the open edge are listed.
func assertEndedKeptAsHistory(t *testing.T, env *testEnv, caseID, endedID, openID uuid.UUID) {
	t.Helper()
	res, err := env.coreSvc.ListRelationships(env.ctx, core.RelationshipFilter{SubjectID: caseID, Outgoing: true})
	if err != nil {
		t.Fatalf("list relationships: %v", err)
	}
	ended := map[uuid.UUID]bool{}
	for _, rel := range res.Relationships {
		ended[rel.ID] = rel.ValidTo != nil
	}
	if isEnded, ok := ended[endedID]; !ok || !isEnded {
		t.Fatalf("ended edge missing or still open in %v", ended)
	}
	if isEnded, ok := ended[openID]; !ok || isEnded {
		t.Fatalf("open edge missing or ended in %v", ended)
	}
}
