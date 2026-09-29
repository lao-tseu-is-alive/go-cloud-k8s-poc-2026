package timeline

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
	lastCreate   CreateInput
	lastUpdate   UpdateInput
	lastFilter   ListFilter
	lastReason   string
	withdrawCall int
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput) (*Entry, *core.AuditEvent, error) {
	f.lastCreate = in
	return &Entry{ID: uuid.New(), CaseID: in.CaseID, Type: in.Type, Status: StatusDraft}, &core.AuditEvent{}, nil
}

func (f *fakeRepo) Update(_ context.Context, id uuid.UUID, in UpdateInput) (*Entry, *core.AuditEvent, error) {
	f.lastUpdate = in
	return &Entry{ID: id, Type: in.Type, Status: StatusDraft}, &core.AuditEvent{}, nil
}

func (f *fakeRepo) List(_ context.Context, filter ListFilter) (ListResult, error) {
	f.lastFilter = filter
	return ListResult{}, nil
}

func (f *fakeRepo) Withdraw(_ context.Context, id uuid.UUID, _, reason string) (*Entry, *core.AuditEvent, error) {
	f.withdrawCall++
	f.lastReason = reason
	return &Entry{ID: id, Status: StatusWithdrawn}, &core.AuditEvent{}, nil
}

func newTestService(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{}
	svc, err := NewService(repo, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.now = func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }
	return svc, repo
}

func TestCreateNormalizesContent(t *testing.T) {
	svc, repo := newTestService(t)
	docID := uuid.New()
	_, _, err := svc.Create(context.Background(), CreateInput{
		CaseID:      uuid.New(),
		Type:        TypeOpinion,
		Title:       "  Préavis  ",
		Body:        "\n favorable \n",
		DocumentIDs: []uuid.UUID{docID, docID},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got := repo.lastCreate
	if got.Title != "Préavis" || got.Body != "favorable" {
		t.Fatalf("title/body must be trimmed, got %q / %q", got.Title, got.Body)
	}
	if got.Visibility != VisibilityCaseParticipants {
		t.Fatalf("visibility must default to CASE_PARTICIPANTS, got %d", got.Visibility)
	}
	if len(got.DocumentIDs) != 1 || got.DocumentIDs[0] != docID {
		t.Fatalf("duplicate document ids must collapse, got %v", got.DocumentIDs)
	}
}

func TestCreateRejectsInvalidContent(t *testing.T) {
	future := time.Date(2026, 9, 29, 12, 5, 0, 0, time.UTC)
	withinSkew := time.Date(2026, 9, 29, 12, 0, 30, 0, time.UTC)
	tests := []struct {
		name    string
		in      CreateInput
		wantErr bool
	}{
		{"system type", CreateInput{Type: TypeSystem, Body: "x"}, true},
		{"ai proposal", CreateInput{Type: TypeAIProposal, Body: "x"}, true},
		{"unspecified type", CreateInput{Type: TypeUnspecified, Body: "x"}, true},
		{"unknown type", CreateInput{Type: EntryType(42), Body: "x"}, true},
		{"blank body", CreateInput{Type: TypeComment, Body: "   "}, true},
		{"body too long", CreateInput{Type: TypeComment, Body: strings.Repeat("é", MaxBodyLength+1)}, true},
		{"title too long", CreateInput{Type: TypeComment, Body: "x", Title: strings.Repeat("a", MaxTitleLength+1)}, true},
		{"unknown visibility", CreateInput{Type: TypeComment, Body: "x", Visibility: Visibility(9)}, true},
		{"future business date", CreateInput{Type: TypeComment, Body: "x", OccurredAt: &future}, true},
		{"nil document", CreateInput{Type: TypeComment, Body: "x", DocumentIDs: []uuid.UUID{uuid.Nil}}, true},
		{"clock skew tolerated", CreateInput{Type: TypeComment, Body: "x", OccurredAt: &withinSkew}, false},
		{"max body", CreateInput{Type: TypeDecision, Body: strings.Repeat("é", MaxBodyLength)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService(t)
			tt.in.CaseID = uuid.New()
			_, _, err := svc.Create(context.Background(), tt.in)
			if tt.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v, got %v", tt.wantErr, err)
			}
			if tt.wantErr && !errors.Is(err, core.ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestCreateRequiresCaseAndCapsDocuments(t *testing.T) {
	svc, _ := newTestService(t)
	if _, _, err := svc.Create(context.Background(), CreateInput{Type: TypeComment, Body: "x"}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("missing case: want ErrInvalidInput, got %v", err)
	}
	docs := make([]uuid.UUID, MaxDocumentsPerRequest+1)
	for i := range docs {
		docs[i] = uuid.New()
	}
	if _, _, err := svc.Create(context.Background(), CreateInput{CaseID: uuid.New(), Type: TypeComment, Body: "x", DocumentIDs: docs}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("too many documents: want ErrInvalidInput, got %v", err)
	}
}

func TestUpdateRejectsServerTypes(t *testing.T) {
	svc, repo := newTestService(t)
	if _, _, err := svc.Update(context.Background(), uuid.New(), UpdateInput{Type: TypeSystem, Body: "x"}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("a draft cannot become SYSTEM: got %v", err)
	}
	if _, _, err := svc.Update(context.Background(), uuid.New(), UpdateInput{Type: TypeRequest, Body: " demande ", Reason: " typo "}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if repo.lastUpdate.Body != "demande" || repo.lastUpdate.Reason != "typo" || repo.lastUpdate.Visibility != VisibilityCaseParticipants {
		t.Fatalf("update must be normalized, got %+v", repo.lastUpdate)
	}
}

func TestWithdrawRequiresReason(t *testing.T) {
	svc, repo := newTestService(t)
	if _, _, err := svc.Withdraw(context.Background(), uuid.New(), "op", "  "); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("blank reason: want ErrInvalidInput, got %v", err)
	}
	if repo.withdrawCall != 0 {
		t.Fatal("the repository must not be called without a reason")
	}
	if _, _, err := svc.Withdraw(context.Background(), uuid.New(), "op", " doublon "); err != nil || repo.lastReason != "doublon" {
		t.Fatalf("withdraw: reason %q (%v)", repo.lastReason, err)
	}
}

func TestListNormalizesFilter(t *testing.T) {
	svc, repo := newTestService(t)
	if _, err := svc.List(context.Background(), ListFilter{CaseID: uuid.New(), Offset: -3}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if repo.lastFilter.Limit != core.DefaultPageSize || repo.lastFilter.Offset != 0 {
		t.Fatalf("filter must be normalized, got %+v", repo.lastFilter)
	}
	if _, err := svc.List(context.Background(), ListFilter{CaseID: uuid.New(), Types: []EntryType{TypeUnspecified}}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("unspecified type filter: want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.List(context.Background(), ListFilter{}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("missing case: want ErrInvalidInput, got %v", err)
	}
}

func TestTypeAndStatusRules(t *testing.T) {
	for _, tt := range []struct {
		t    EntryType
		want bool
	}{
		{TypeComment, true}, {TypeOpinion, true}, {TypeDecision, true}, {TypeRequest, true},
		{TypeResponse, true}, {TypeValidation, true}, {TypeSystem, false}, {TypeAIProposal, false},
		{TypeUnspecified, false}, {EntryType(9), false},
	} {
		if got := tt.t.OperatorCreatable(); got != tt.want {
			t.Errorf("%s.OperatorCreatable() = %v, want %v", tt.t, got, tt.want)
		}
	}
	for _, tt := range []struct {
		s    Status
		want bool
	}{
		{StatusDraft, false}, {StatusValidated, true}, {StatusLocked, true}, {StatusWithdrawn, false},
	} {
		if got := tt.s.Correctable(); got != tt.want {
			t.Errorf("%s.Correctable() = %v, want %v", tt.s, got, tt.want)
		}
	}
	if TypeAIProposal.String() != "AI_PROPOSAL" || EntryType(9).String() != "EntryType(9)" || StatusLocked.String() != "LOCKED" {
		t.Fatalf("unexpected names %q / %q / %q", TypeAIProposal, EntryType(9), StatusLocked)
	}
}
