package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// fakeRepo records the inputs it receives and fails any call it is not meant for.
type fakeRepo struct {
	Repository
	lastCreate CreateInput
	lastMine   MineFilter
	lastNote   string
	moves      int
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput) (*Task, *core.AuditEvent, error) {
	f.lastCreate = in
	return &Task{ID: uuid.New(), CaseID: in.CaseID}, &core.AuditEvent{}, nil
}

func (f *fakeRepo) ListMine(_ context.Context, filter MineFilter) (ListResult, error) {
	f.lastMine = filter
	return ListResult{}, nil
}

func (f *fakeRepo) ChangeStatus(_ context.Context, id uuid.UUID, _ Move, _, note string) (*Task, *core.AuditEvent, error) {
	f.moves++
	f.lastNote = note
	return &Task{ID: id}, &core.AuditEvent{}, nil
}

func newTestService(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{}
	svc, err := NewService(repo, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc, repo
}

func TestCreateNormalizes(t *testing.T) {
	svc, repo := newTestService(t)
	blank := "  "
	unit := uuid.Nil
	_, _, err := svc.Create(context.Background(), CreateInput{
		CaseID:   uuid.New(),
		Content:  Content{TypeCode: " SITE_VISIT ", Title: "  Visite  ", Description: " d "},
		Assignee: Assignee{UserID: &blank, OrgUnitID: &unit},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got := repo.lastCreate
	if got.TypeCode != "SITE_VISIT" || got.Title != "Visite" || got.Description != "d" || !got.Assignee.None() {
		t.Fatalf("unexpected normalization %+v", got)
	}
}

func TestCreateRejectsInvalidContent(t *testing.T) {
	svc, _ := newTestService(t)
	for name, c := range map[string]Content{
		"missing type":     {Title: "x"},
		"missing title":    {TypeCode: "OTHER"},
		"title too long":   {TypeCode: "OTHER", Title: strings.Repeat("é", MaxTitleLength+1)},
		"description long": {TypeCode: "OTHER", Title: "x", Description: strings.Repeat("a", MaxDescriptionLength+1)},
	} {
		if _, _, err := svc.Create(context.Background(), CreateInput{CaseID: uuid.New(), Content: c}); !errors.Is(err, core.ErrInvalidInput) {
			t.Errorf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
	if _, _, err := svc.Create(context.Background(), CreateInput{Content: Content{TypeCode: "OTHER", Title: "x"}}); !errors.Is(err, core.ErrInvalidInput) {
		t.Errorf("missing case: want ErrInvalidInput, got %v", err)
	}
}

func TestChangeStatusReasons(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	for _, move := range []Move{MoveCancel, MoveReopen} {
		if _, _, err := svc.ChangeStatus(ctx, uuid.New(), move, "op", "  "); !errors.Is(err, core.ErrInvalidInput) {
			t.Errorf("move %d without reason: want ErrInvalidInput, got %v", move, err)
		}
	}
	if _, _, err := svc.ChangeStatus(ctx, uuid.New(), Move(99), "op", ""); !errors.Is(err, core.ErrInvalidInput) {
		t.Errorf("unknown move: want ErrInvalidInput, got %v", err)
	}
	if repo.moves != 0 {
		t.Fatal("the repository must not be called for rejected moves")
	}
	if _, _, err := svc.ChangeStatus(ctx, uuid.New(), MoveComplete, "op", " fait "); err != nil || repo.lastNote != "fait" {
		t.Fatalf("complete with a note: %q (%v)", repo.lastNote, err)
	}
}

func TestListMineDefaultsToPending(t *testing.T) {
	svc, repo := newTestService(t)
	if _, err := svc.ListMine(context.Background(), MineFilter{UserID: "7"}); err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if len(repo.lastMine.Statuses) != 2 || repo.lastMine.Statuses[0] != StatusOpen || repo.lastMine.Limit != core.DefaultPageSize {
		t.Fatalf("unexpected filter %+v", repo.lastMine)
	}
	if _, err := svc.ListMine(context.Background(), MineFilter{UserID: "7", Statuses: []Status{StatusUnspecified}}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("unknown status: want ErrInvalidInput, got %v", err)
	}
}

func TestStateMachine(t *testing.T) {
	allowed := map[Move][]Status{
		MoveStart:    {StatusOpen},
		MoveComplete: {StatusOpen, StatusInProgress},
		MoveCancel:   {StatusOpen, StatusInProgress},
		MoveReopen:   {StatusDone, StatusCancelled},
	}
	for move, from := range allowed {
		for _, s := range []Status{StatusOpen, StatusInProgress, StatusDone, StatusCancelled} {
			want := false
			for _, f := range from {
				want = want || f == s
			}
			if got := moves[move].allowed(s); got != want {
				t.Errorf("move %d from %s: allowed=%v, want %v", move, s, got, want)
			}
		}
	}
}

func TestOverdue(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	cases := []struct {
		task Task
		want bool
	}{
		{Task{Status: StatusOpen, DueAt: &past}, true},
		{Task{Status: StatusInProgress, DueAt: &past}, true},
		{Task{Status: StatusDone, DueAt: &past}, false},
		{Task{Status: StatusOpen, DueAt: &future}, false},
		{Task{Status: StatusOpen}, false},
	}
	for _, c := range cases {
		if got := c.task.Overdue(now); got != c.want {
			t.Errorf("Overdue(%s, %v) = %v", c.task.Status, c.task.DueAt, got)
		}
	}
}
