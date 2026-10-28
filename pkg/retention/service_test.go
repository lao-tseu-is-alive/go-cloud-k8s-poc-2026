package retention

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// stubRepository records whether the service reached it.
type stubRepository struct {
	Repository
	reached bool
}

func (r *stubRepository) CreatePolicy(context.Context, PolicyInput) (*Policy, *core.ReferenceChange, error) {
	r.reached = true
	return &Policy{}, nil, nil
}

func (r *stubRepository) PlaceHold(context.Context, HoldInput) (*Hold, *core.AuditEvent, error) {
	r.reached = true
	return &Hold{}, nil, nil
}

func (r *stubRepository) ListDue(context.Context, DueFilter) (DueResult, error) {
	r.reached = true
	return DueResult{}, nil
}

func (r *stubRepository) ApplyCaseType(context.Context, ApplyInput) (ApplyResult, error) {
	r.reached = true
	return ApplyResult{}, nil
}

func newStubService(t *testing.T) (*Service, *stubRepository) {
	t.Helper()
	repo := &stubRepository{}
	svc, err := NewService(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo
}

func TestCreatePolicyValidation(t *testing.T) {
	valid := PolicyInput{Code: "TEST_POLICY", Label: "Test", DurationYears: 10, FinalDisposition: DispositionArchive}
	cases := map[string]func(*PolicyInput){
		"bad code":            func(in *PolicyInput) { in.Code = "lower" },
		"no label":            func(in *PolicyInput) { in.Label = " " },
		"negative duration":   func(in *PolicyInput) { in.DurationYears = -1 },
		"too long":            func(in *PolicyInput) { in.DurationYears = MaxDurationYears + 1 },
		"unknown disposition": func(in *PolicyInput) { in.FinalDisposition = "KEEP" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newStubService(t)
			in := valid
			mutate(&in)
			if _, _, err := svc.CreatePolicy(context.Background(), in); !errors.Is(err, core.ErrInvalidInput) || repo.reached {
				t.Fatalf("want ErrInvalidInput before the repository, got %v (reached %v)", err, repo.reached)
			}
		})
	}
	svc, repo := newStubService(t)
	if _, _, err := svc.CreatePolicy(context.Background(), valid); err != nil || !repo.reached {
		t.Fatalf("a valid policy reaches the repository: %v", err)
	}
}

func TestHoldAndDueValidation(t *testing.T) {
	ctx := context.Background()
	svc, repo := newStubService(t)
	for _, in := range []HoldInput{
		{Reason: "litige"},
		{CaseID: uuid.New(), Reason: "  "},
		{CaseID: uuid.New(), Reason: strings.Repeat("é", MaxReasonLength+1)},
	} {
		if _, _, err := svc.PlaceHold(ctx, in); !errors.Is(err, core.ErrInvalidInput) || repo.reached {
			t.Fatalf("hold %+v: want ErrInvalidInput, got %v", in.CaseID, err)
		}
	}
	for _, f := range []DueFilter{{DueOn: "2026-02-30"}, {DueOn: "28.10.2026"}, {Disposition: "KEEP"}} {
		if _, err := svc.ListDue(ctx, f); !errors.Is(err, core.ErrInvalidInput) || repo.reached {
			t.Fatalf("due filter %+v: want ErrInvalidInput, got %v", f, err)
		}
	}
	if _, err := svc.ApplyCaseType(ctx, ApplyInput{CaseTypeCode: "X"}); !errors.Is(err, core.ErrInvalidInput) || repo.reached {
		t.Fatalf("apply without reason: want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.ListDue(ctx, DueFilter{DueOn: " 2026-10-28 "}); err != nil || !repo.reached {
		t.Fatalf("a valid filter reaches the repository: %v", err)
	}
}

func TestDispositionWireMapping(t *testing.T) {
	for _, d := range []Disposition{DispositionArchive, DispositionDestroy, DispositionReview} {
		if got := DispositionFromProto(DispositionToProto(d)); got != d {
			t.Fatalf("%s does not round-trip: %s", d, got)
		}
	}
	if DispositionFromProto(0) != "" {
		t.Fatal("UNSPECIFIED maps to no disposition")
	}
}
