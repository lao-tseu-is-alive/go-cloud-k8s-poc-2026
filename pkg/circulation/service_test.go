package circulation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
)

// fakeRepo counts the calls it receives and fails any call it is not meant for.
type fakeRepo struct {
	Repository
	responds int
}

func (f *fakeRepo) Respond(_ context.Context, in RespondInput) (*Circulation, *core.AuditEvent, error) {
	f.responds++
	return &Circulation{ID: uuid.New()}, &core.AuditEvent{}, nil
}

func user(id string) task.Assignee { return task.Assignee{UserID: &id} }

func TestNormalizeRecipientsRenumbersSteps(t *testing.T) {
	unit := uuid.New()
	got, err := NormalizeRecipients([]RecipientInput{
		{Step: 3, Assignee: user("a")},
		{Step: 0, Assignee: user("b")},
		{Step: 7, Assignee: task.Assignee{OrgUnitID: &unit}},
		{Step: 3, Assignee: user("c")},
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	want := []int32{2, 1, 3, 2}
	for i, rec := range got {
		if rec.Step != want[i] {
			t.Fatalf("steps = %v, want %v", []int32{got[0].Step, got[1].Step, got[2].Step, got[3].Step}, want)
		}
	}
	if stepCount(got) != 3 {
		t.Fatalf("stepCount = %d, want 3", stepCount(got))
	}
}

func TestNormalizeRecipientsRejects(t *testing.T) {
	unit := uuid.New()
	blank := " "
	someone := "u1"
	many := make([]RecipientInput, MaxRecipients+1)
	for i := range many {
		many[i] = RecipientInput{Assignee: user(uuid.NewString())}
	}
	for name, in := range map[string][]RecipientInput{
		"none":         nil,
		"too many":     many,
		"both":         {{Assignee: task.Assignee{UserID: &someone, OrgUnitID: &unit}}},
		"nobody":       {{Assignee: task.Assignee{UserID: &blank}}},
		"twice (user)": {{Assignee: user("a")}, {Step: 2, Assignee: user(" a ")}},
		"twice (unit)": {{Assignee: task.Assignee{OrgUnitID: &unit}}, {Assignee: task.Assignee{OrgUnitID: &unit}}},
	} {
		if _, err := NormalizeRecipients(in); !errors.Is(err, core.ErrInvalidInput) {
			t.Errorf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
}

func TestRespondValidation(t *testing.T) {
	repo := &fakeRepo{}
	svc, err := NewService(repo, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	ctx := context.Background()
	id := uuid.New()
	for name, in := range map[string]RespondInput{
		"no recipient":      {Response: ResponseFavorable},
		"unknown response":  {RecipientID: id, Response: Response(9)},
		"none":              {RecipientID: id},
		"comment no text":   {RecipientID: id, Response: ResponseComment, Text: "  "},
		"more info no text": {RecipientID: id, Response: ResponseNeedMoreInfo},
		"text too long":     {RecipientID: id, Response: ResponseFavorable, Text: strings.Repeat("é", MaxTextLength+1)},
	} {
		if _, _, err := svc.Respond(ctx, in); !errors.Is(err, core.ErrInvalidInput) {
			t.Errorf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
	if repo.responds != 0 {
		t.Fatal("the repository must not be called for rejected answers")
	}
	if _, _, err := svc.Respond(ctx, RespondInput{RecipientID: id, Response: ResponseNotConcerned}); err != nil || repo.responds != 1 {
		t.Fatalf("a NOT_CONCERNED answer needs no text: %v", err)
	}
}

func TestResponseNames(t *testing.T) {
	if ResponseNeedMoreInfo.String() != "NEED_MORE_INFO" || Response(9).String() != "Response(9)" || labelsFR[ResponseUnfavorable] != "Défavorable" {
		t.Fatalf("unexpected names")
	}
}
