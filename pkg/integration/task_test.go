package integration

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// newUser records an internal user as a verified token would.
func newUser(t *testing.T, env *testEnv, name string) *core.AppUser {
	t.Helper()
	u, err := env.coreRepo.RecordUser(env.ctx, core.UserProfile{UserID: "it-" + uniqueToken(), DisplayName: name})
	if err != nil {
		t.Fatalf("record user: %v", err)
	}
	return u
}

// TestTaskLifecycle covers v2 §50 steps 22-23: create a task, reassign it
// (history kept), start and complete it (SYSTEM timeline entry), reopen and
// cancel it, and the rules around the case.
func TestTaskLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Tâches "+uniqueToken())
	alice := newUser(t, env, "Alice Test")
	unit := newUnit(t, env, "SERVICE", "Service tâches "+uniqueToken(), nil)
	due := time.Now().Add(-time.Hour).UTC()

	created, ev, err := env.taskSvc.Create(ctx, task.CreateInput{
		CaseID:     c.ID,
		Content:    task.Content{TypeCode: "SITE_VISIT", Title: "Visite du chantier", DueAt: &due},
		Assignee:   task.Assignee{UserID: &alice.UserID},
		OperatorID: testOperator,
	})
	if err != nil || ev.EventType != "TASK_CREATED" || ev.SubjectID != c.ID || created.AssigneeLabel != "Alice Test" {
		t.Fatalf("create: %+v / %+v (%v)", created, ev, err)
	}
	if !created.Overdue(time.Now()) || created.Origin != task.OriginManual || len(created.Assignments) != 1 {
		t.Fatalf("a manual task past its deadline is overdue, with its first assignment: %+v", created)
	}

	reassigned, _, err := env.taskSvc.Assign(ctx, created.ID, task.Assignee{OrgUnitID: &unit.ID}, testOperator, "renfort du service")
	if err != nil || reassigned.AssigneeOrgUnitID == nil || len(reassigned.Assignments) != 2 || reassigned.Assignments[0].EndedAt == nil {
		t.Fatalf("reassignment keeps the history: %+v (%v)", reassigned, err)
	}
	if _, _, err := env.taskSvc.Assign(ctx, created.ID, task.Assignee{OrgUnitID: &unit.ID}, testOperator, ""); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("the same assignee again: want ErrInvalidInput, got %v", err)
	}
	both := task.Assignee{UserID: &alice.UserID, OrgUnitID: &unit.ID}
	if _, _, err := env.taskSvc.Assign(ctx, created.ID, both, testOperator, ""); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("a user and a unit: want ErrInvalidInput, got %v", err)
	}
	ghost := "it-ghost-" + uniqueToken()
	if _, _, err := env.taskSvc.Assign(ctx, created.ID, task.Assignee{UserID: &ghost}, testOperator, ""); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("an unknown user: want ErrInvalidInput, got %v", err)
	}

	assertTaskMoves(t, env, created.ID)
	assertTaskTimeline(t, env, c.ID)
}

// assertTaskMoves checks the state machine through the service.
func assertTaskMoves(t *testing.T, env *testEnv, id uuid.UUID) {
	t.Helper()
	ctx := env.ctx
	if _, _, err := env.taskSvc.ChangeStatus(ctx, id, task.MoveReopen, testOperator, "rien à rouvrir"); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("reopening an open task: want ErrInvalidState, got %v", err)
	}
	started, _, err := env.taskSvc.ChangeStatus(ctx, id, task.MoveStart, testOperator, "")
	if err != nil || started.Status != task.StatusInProgress || started.StartedAt == nil {
		t.Fatalf("start: %+v (%v)", started, err)
	}
	done, ev, err := env.taskSvc.ChangeStatus(ctx, id, task.MoveComplete, testOperator, "Chantier conforme")
	if err != nil || done.Status != task.StatusDone || done.CompletionNote != "Chantier conforme" || ev.EventType != "TASK_COMPLETED" {
		t.Fatalf("complete: %+v (%v)", done, err)
	}
	edit := task.UpdateInput{Content: task.Content{TypeCode: "OTHER", Title: "Réécrite"}, OperatorID: testOperator}
	if _, _, err := env.taskSvc.Update(ctx, id, edit); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("editing a done task: want ErrInvalidState, got %v", err)
	}
	reopened, _, err := env.taskSvc.ChangeStatus(ctx, id, task.MoveReopen, testOperator, "contrôle à refaire")
	if err != nil || reopened.Status != task.StatusOpen || reopened.CompletedAt != nil || reopened.CompletionNote != "" {
		t.Fatalf("reopen clears the completion stamps: %+v (%v)", reopened, err)
	}
	cancelled, _, err := env.taskSvc.ChangeStatus(ctx, id, task.MoveCancel, testOperator, "plus nécessaire")
	if err != nil || cancelled.Status != task.StatusCancelled || cancelled.CancellationReason != "plus nécessaire" {
		t.Fatalf("cancel: %+v (%v)", cancelled, err)
	}
}

// assertTaskTimeline checks the SYSTEM entries written on completion and cancellation.
func assertTaskTimeline(t *testing.T, env *testEnv, caseID uuid.UUID) {
	t.Helper()
	res, err := env.timelineSvc.List(env.ctx, timeline.ListFilter{CaseID: caseID, Types: []timeline.EntryType{timeline.TypeSystem}})
	if err != nil || len(res.Entries) != 2 {
		t.Fatalf("completion and cancellation write two SYSTEM entries, got %d (%v)", len(res.Entries), err)
	}
	events := map[any]bool{}
	for _, e := range res.Entries {
		events[e.Metadata["event"]] = true
	}
	if !events["TASK_COMPLETED"] || !events["TASK_CANCELLED"] {
		t.Fatalf("unexpected SYSTEM events %v", events)
	}
}

// TestMyTasksAndClosure covers "my tasks" (mine and my units' through
// USER_MEMBER_OF_ORG_UNIT) and the case closure rules.
func TestMyTasksAndClosure(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Mes tâches "+uniqueToken())
	bob := newUser(t, env, "Bob Test")
	unit := newUnit(t, env, "SERVICE", "Service de Bob "+uniqueToken(), nil)
	member := core.LinkInput{SourceSubjectID: bob.SubjectID, TargetSubjectID: unit.ID, RelationshipTypeCode: "USER_MEMBER_OF_ORG_UNIT", OperatorID: testOperator}
	if _, _, err := env.coreSvc.LinkSubjects(ctx, member); err != nil {
		t.Fatalf("bob joins the unit: %v", err)
	}
	add := func(title string, a task.Assignee) *task.Task {
		tk, _, err := env.taskSvc.Create(ctx, task.CreateInput{CaseID: c.ID, Content: task.Content{TypeCode: "CALLBACK", Title: title}, Assignee: a, OperatorID: testOperator})
		if err != nil {
			t.Fatalf("create %q: %v", title, err)
		}
		return tk
	}
	mine := add("Rappeler le requérant", task.Assignee{UserID: &bob.UserID})
	ours := add("Préparer la séance", task.Assignee{OrgUnitID: &unit.ID})
	add("Sans attribution", task.Assignee{})

	own, err := env.taskSvc.ListMine(ctx, task.MineFilter{UserID: bob.UserID})
	if err != nil || own.TotalSize != 1 || own.Tasks[0].ID != mine.ID {
		t.Fatalf("my tasks without units: %+v (%v)", own, err)
	}
	withUnits, err := env.taskSvc.ListMine(ctx, task.MineFilter{UserID: bob.UserID, IncludeUnits: true})
	if err != nil || withUnits.TotalSize != 2 || !containsTask(withUnits.Tasks, ours) {
		t.Fatalf("my tasks with my units': %+v (%v)", withUnits, err)
	}

	closing := casefile.TransitionInput{Target: casefile.StatusClosed, OperatorID: testOperator, Reason: "terminé"}
	if _, _, err := env.caseSvc.Transition(ctx, c.ID, closing); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("closing with open tasks: want ErrInvalidState, got %v", err)
	}
	list, err := env.taskSvc.ListCase(ctx, task.CaseFilter{CaseID: c.ID})
	if err != nil || list.OpenCount != 3 {
		t.Fatalf("open count: %+v (%v)", list, err)
	}
	for _, tk := range list.Tasks {
		if _, _, err := env.taskSvc.ChangeStatus(ctx, tk.ID, task.MoveCancel, testOperator, "clôture"); err != nil {
			t.Fatalf("cancel %s: %v", tk.ID, err)
		}
	}
	if _, _, err := env.caseSvc.Transition(ctx, c.ID, closing); err != nil {
		t.Fatalf("close once no task is open: %v", err)
	}
	late := task.CreateInput{CaseID: c.ID, Content: task.Content{TypeCode: "OTHER", Title: "Trop tard"}, OperatorID: testOperator}
	if _, _, err := env.taskSvc.Create(ctx, late); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("a closed case accepts no task: got %v", err)
	}
	if _, _, err := env.taskSvc.ChangeStatus(ctx, mine.ID, task.MoveReopen, testOperator, "oups"); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("a closed case freezes its tasks: got %v", err)
	}
}

func containsTask(tasks []*task.Task, target *task.Task) bool {
	for _, tk := range tasks {
		if tk.ID == target.ID {
			return true
		}
	}
	return false
}
