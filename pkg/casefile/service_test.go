package casefile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/coretest"
)

func TestCanTransition(t *testing.T) {
	allowed := map[[2]Status]bool{
		{StatusOpen, StatusInProgress}: true, {StatusOpen, StatusSuspended}: true, {StatusOpen, StatusClosed}: true,
		{StatusInProgress, StatusOpen}: true, {StatusInProgress, StatusSuspended}: true, {StatusInProgress, StatusClosed}: true,
		{StatusSuspended, StatusOpen}: true, {StatusSuspended, StatusInProgress}: true, {StatusSuspended, StatusClosed}: true,
		{StatusClosed, StatusOpen}: true,
	}
	all := []Status{StatusUnspecified, StatusOpen, StatusInProgress, StatusSuspended, StatusClosed}
	for _, from := range all {
		for _, to := range all {
			if got := CanTransition(from, to); got != allowed[[2]Status{from, to}] {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, !got)
			}
		}
	}
}

func TestRequiresReason(t *testing.T) {
	tests := []struct {
		from, to Status
		want     bool
	}{
		{StatusOpen, StatusClosed, true},
		{StatusSuspended, StatusClosed, true},
		{StatusClosed, StatusOpen, true},
		{StatusOpen, StatusInProgress, false},
		{StatusInProgress, StatusSuspended, false},
	}
	for _, tt := range tests {
		if got := RequiresReason(tt.from, tt.to); got != tt.want {
			t.Errorf("RequiresReason(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestStatusString(t *testing.T) {
	if StatusInProgress.String() != "IN_PROGRESS" || Status(9).String() != "Status(9)" {
		t.Fatalf("unexpected names %q / %q", StatusInProgress.String(), Status(9).String())
	}
}

// fakeRepo records the create input and fails any call it is not meant for.
type fakeRepo struct {
	Repository
	lastCreate  CreateInput
	createCalls int
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput) (*Case, *core.AuditEvent, error) {
	f.lastCreate = in
	f.createCalls++
	return &Case{ID: uuid.New(), Title: in.Title}, &core.AuditEvent{}, nil
}

func newTestService(t *testing.T, repo Repository) *Service {
	t.Helper()
	svc, err := NewService(repo, coretest.NewService(t), nil)
	if err != nil {
		t.Fatalf("case service: %v", err)
	}
	return svc
}

func TestCreateValidation(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(t, repo)
	for name, in := range map[string]CreateInput{
		"missing type":       {Title: "x"},
		"blank title":        {CaseTypeCode: "OPC_DEMANDE_PC", Title: "  "},
		"title too long":     {CaseTypeCode: "OPC_DEMANDE_PC", Title: strings.Repeat("a", MaxTitleLength+1)},
		"invalid namespace":  {CaseTypeCode: "OPC_DEMANDE_PC", Title: "x", BusinessRef: core.BusinessRefRequest{Namespace: "opc", Allocate: true}},
		"description length": {CaseTypeCode: "OPC_DEMANDE_PC", Title: "x", Description: strings.Repeat("é", MaxDescriptionLength+1)},
	} {
		if _, _, err := svc.Create(context.Background(), in); !errors.Is(err, core.ErrInvalidInput) {
			t.Errorf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
	if repo.createCalls != 0 {
		t.Fatalf("repository.Create must not be called for invalid input, got %d calls", repo.createCalls)
	}
}

func TestCreatePopulatesGovernance(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(t, repo)
	if _, _, err := svc.Create(context.Background(), CreateInput{CaseTypeCode: " OPC_DEMANDE_PC ", Title: "  Permis  ", OperatorID: "42"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	got := repo.lastCreate
	if got.CaseTypeCode != "OPC_DEMANDE_PC" || got.Title != "Permis" || got.Governance.Kind != core.SubjectKindCase ||
		got.Governance.DisplayLabel != "Permis" || got.Governance.OwnerUserID != "42" || !got.BusinessRef.IsZero() {
		t.Fatalf("unexpected create input %+v", got)
	}
}

func TestTransitionValidation(t *testing.T) {
	svc := newTestService(t, &fakeRepo{})
	if _, _, err := svc.Transition(context.Background(), uuid.Nil, TransitionInput{Target: StatusOpen}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("nil id: want ErrInvalidInput, got %v", err)
	}
	if _, _, err := svc.Transition(context.Background(), uuid.New(), TransitionInput{Target: Status(7)}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("unknown status: want ErrInvalidInput, got %v", err)
	}
}

func TestBusinessRefFor(t *testing.T) {
	typed := &CaseType{BusinessRefNamespace: "OPC"}
	if got := businessRefFor(core.BusinessRefRequest{}, typed); got != (core.BusinessRefRequest{Namespace: "OPC", Allocate: true}) {
		t.Fatalf("default must allocate in the type namespace, got %+v", got)
	}
	explicit := core.BusinessRefRequest{Value: "LEG-1"}
	if got := businessRefFor(explicit, typed); got != explicit {
		t.Fatalf("an explicit reference must win, got %+v", got)
	}
	if got := businessRefFor(core.BusinessRefRequest{}, &CaseType{}); !got.IsZero() {
		t.Fatalf("a type without namespace assigns no reference, got %+v", got)
	}
}
