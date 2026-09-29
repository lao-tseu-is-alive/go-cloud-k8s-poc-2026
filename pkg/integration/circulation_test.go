package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/circulation"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// recipientOf returns the recipient of a circulation assigned to the user or unit.
func recipientOf(t *testing.T, c *circulation.Circulation, userID *string, unitID *uuid.UUID) *circulation.Recipient {
	t.Helper()
	for _, rec := range c.Recipients {
		if (userID != nil && rec.AssigneeUserID != nil && *rec.AssigneeUserID == *userID) ||
			(unitID != nil && rec.AssigneeOrgUnitID != nil && *rec.AssigneeOrgUnitID == *unitID) {
			return rec
		}
	}
	t.Fatalf("recipient not found in %+v", c.Recipients)
	return nil
}

// respond records an answer and fails the test on error.
func respond(t *testing.T, env *testEnv, rec *circulation.Recipient, r circulation.Response, text string) *circulation.Circulation {
	t.Helper()
	c, ev, err := env.circulationSvc.Respond(env.ctx, circulation.RespondInput{RecipientID: rec.ID, Response: r, Text: text, OperatorID: testOperator})
	if err != nil {
		t.Fatalf("respond %s: %v", r, err)
	}
	if ev.EventType != "CIRCULATION_RESPONDED" {
		t.Fatalf("expected CIRCULATION_RESPONDED, got %+v", ev)
	}
	return c
}

// TestCirculationSteps covers v2 §50 steps 24-25: a two-step circulation to
// two units and users, tasks per open step, answers as RESPONSE entries, the
// next step opening, completion with a summary, and the guards.
func TestCirculationSteps(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Circulation "+uniqueToken())
	alice, bob := newUser(t, env, "Alice Circ"), newUser(t, env, "Bob Circ")
	unit := newUnit(t, env, "SERVICE", "Service préavis "+uniqueToken(), nil)

	created, ev, err := env.circulationSvc.Create(ctx, circulation.CreateInput{
		CaseID: c.ID, Title: "Préavis des services", Message: "Merci de donner votre préavis.",
		Recipients: []circulation.RecipientInput{
			{Step: 1, Assignee: task.Assignee{UserID: &alice.UserID}},
			{Step: 1, Assignee: task.Assignee{OrgUnitID: &unit.ID}},
			{Step: 5, Assignee: task.Assignee{UserID: &bob.UserID}},
		},
		OperatorID: testOperator,
	})
	if err != nil || ev.EventType != "CIRCULATION_CREATED" || created.StepCount != 2 || created.CurrentStep != 1 {
		t.Fatalf("create: %+v / %+v (%v)", created, ev, err)
	}
	recA, recU, recB := recipientOf(t, created, &alice.UserID, nil), recipientOf(t, created, nil, &unit.ID), recipientOf(t, created, &bob.UserID, nil)
	if recA.TaskID == nil || recU.TaskID == nil || recB.TaskID != nil || recB.Step != 2 {
		t.Fatalf("only the first step gets tasks; steps renumbered: %+v %+v %+v", recA, recU, recB)
	}
	assertManagedTask(t, env, *recA.TaskID)
	if _, _, err := env.circulationSvc.Respond(ctx, circulation.RespondInput{RecipientID: recB.ID, Response: circulation.ResponseFavorable, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("answering before one's step opens: want ErrInvalidState, got %v", err)
	}

	afterA := respond(t, env, recA, circulation.ResponseFavorable, "")
	if afterA.Status != circulation.StatusOpen || afterA.CurrentStep != 1 {
		t.Fatalf("the step waits for every recipient: %+v", afterA)
	}
	doneTask, err := env.taskSvc.Get(ctx, *recA.TaskID)
	if err != nil || doneTask.Status != task.StatusDone {
		t.Fatalf("an answer completes the recipient's task: %+v (%v)", doneTask, err)
	}
	if _, _, err := env.circulationSvc.Respond(ctx, circulation.RespondInput{RecipientID: recA.ID, Response: circulation.ResponseUnfavorable, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("answering twice: want ErrInvalidState, got %v", err)
	}

	afterU := respond(t, env, recU, circulation.ResponseComment, "Réserve sur l'accès pompiers")
	recB = recipientOf(t, afterU, &bob.UserID, nil)
	if afterU.CurrentStep != 2 || recB.TaskID == nil {
		t.Fatalf("a fully answered step opens the next one: %+v", afterU)
	}
	mine, err := env.taskSvc.ListMine(ctx, task.MineFilter{UserID: bob.UserID})
	if err != nil || mine.TotalSize != 1 || mine.Tasks[0].Origin != task.OriginCirculation {
		t.Fatalf("the next recipient finds the task in \"my tasks\": %+v (%v)", mine, err)
	}
	closing := casefile.TransitionInput{Target: casefile.StatusClosed, OperatorID: testOperator, Reason: "fin"}
	if _, _, err := env.caseSvc.Transition(ctx, c.ID, closing); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("closing with an open circulation: want ErrInvalidState, got %v", err)
	}

	final := respond(t, env, recB, circulation.ResponseUnfavorable, "Hauteur non conforme")
	if final.Status != circulation.StatusCompleted || final.CompletedAt == nil {
		t.Fatalf("the last answer completes the circulation: %+v", final)
	}
	assertCirculationTimeline(t, env, c.ID)
}

// assertManagedTask checks that a circulation task cannot be completed,
// cancelled, reassigned or edited directly (starting it is fine).
func assertManagedTask(t *testing.T, env *testEnv, id uuid.UUID) {
	t.Helper()
	ctx := env.ctx
	if _, _, err := env.taskSvc.ChangeStatus(ctx, id, task.MoveComplete, testOperator, ""); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("completing a circulation task directly: want ErrInvalidState, got %v", err)
	}
	if _, _, err := env.taskSvc.Assign(ctx, id, task.Assignee{}, testOperator, ""); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("reassigning a circulation task: want ErrInvalidState, got %v", err)
	}
	if _, _, err := env.taskSvc.ChangeStatus(ctx, id, task.MoveStart, testOperator, ""); err != nil {
		t.Fatalf("starting a circulation task is fine: %v", err)
	}
}

// assertCirculationTimeline checks the RESPONSE entries and the SYSTEM entries
// of the circulation (sent, completed with its summary).
func assertCirculationTimeline(t *testing.T, env *testEnv, caseID uuid.UUID) {
	t.Helper()
	responses, err := env.timelineSvc.List(env.ctx, timeline.ListFilter{CaseID: caseID, Types: []timeline.EntryType{timeline.TypeResponse}})
	if err != nil || responses.TotalSize != 3 {
		t.Fatalf("each answer is a RESPONSE entry, got %d (%v)", responses.TotalSize, err)
	}
	for _, e := range responses.Entries {
		if e.Status != timeline.StatusLocked || e.Metadata["event"] != "CIRCULATION_RESPONSE" {
			t.Fatalf("a response entry is locked and structured: %+v", e)
		}
	}
	system, err := env.timelineSvc.List(env.ctx, timeline.ListFilter{CaseID: caseID, Types: []timeline.EntryType{timeline.TypeSystem}})
	if err != nil {
		t.Fatalf("list system entries: %v", err)
	}
	events := map[any]any{}
	for _, e := range system.Entries {
		events[e.Metadata["event"]] = e.Metadata["counts"]
	}
	counts, ok := events["CIRCULATION_COMPLETED"].(map[string]any)
	if _, sent := events["CIRCULATION_CREATED"]; !sent || !ok || counts["FAVORABLE"] != float64(1) || counts["UNFAVORABLE"] != float64(1) || counts["COMMENT"] != float64(1) {
		t.Fatalf("sent and completed entries with the summary, got %v", events)
	}
}

// TestCirculationCancel covers cancelling: open tasks cancelled, no more answers.
func TestCirculationCancel(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Circulation annulée "+uniqueToken())
	carol := newUser(t, env, "Carol Circ")
	created, _, err := env.circulationSvc.Create(ctx, circulation.CreateInput{
		CaseID: c.ID, Title: "Consultation", OperatorID: testOperator,
		Recipients: []circulation.RecipientInput{{Assignee: task.Assignee{UserID: &carol.UserID}}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ghost := "it-ghost-" + uniqueToken()
	bad := circulation.CreateInput{CaseID: c.ID, Title: "Inconnu", OperatorID: testOperator,
		Recipients: []circulation.RecipientInput{{Assignee: task.Assignee{UserID: &ghost}}}}
	if _, _, err := env.circulationSvc.Create(ctx, bad); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("an unknown recipient: want ErrInvalidInput, got %v", err)
	}
	if _, _, err := env.circulationSvc.Cancel(ctx, created.ID, testOperator, " "); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("cancelling without a reason: want ErrInvalidInput, got %v", err)
	}
	cancelled, ev, err := env.circulationSvc.Cancel(ctx, created.ID, testOperator, "dossier retiré")
	if err != nil || cancelled.Status != circulation.StatusCancelled || ev.EventType != "CIRCULATION_CANCELLED" {
		t.Fatalf("cancel: %+v (%v)", cancelled, err)
	}
	tk, err := env.taskSvc.Get(ctx, *cancelled.Recipients[0].TaskID)
	if err != nil || tk.Status != task.StatusCancelled || tk.CancellationReason != "dossier retiré" {
		t.Fatalf("cancelling cancels the open tasks: %+v (%v)", tk, err)
	}
	answer := circulation.RespondInput{RecipientID: cancelled.Recipients[0].ID, Response: circulation.ResponseFavorable, OperatorID: testOperator}
	if _, _, err := env.circulationSvc.Respond(ctx, answer); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("answering a cancelled circulation: want ErrInvalidState, got %v", err)
	}
	closing := casefile.TransitionInput{Target: casefile.StatusClosed, OperatorID: testOperator, Reason: "retiré"}
	if _, _, err := env.caseSvc.Transition(ctx, c.ID, closing); err != nil {
		t.Fatalf("a case closes once its circulation is cancelled: %v", err)
	}
}
