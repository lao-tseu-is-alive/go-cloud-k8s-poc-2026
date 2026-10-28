package integration

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/retention"
)

// newRetentionCaseType creates a case type with the given default policy ("" for none).
func newRetentionCaseType(t *testing.T, env *testEnv, policy string) string {
	t.Helper()
	code := referenceCode("ITR")
	if _, _, err := env.caseSvc.CreateCaseType(env.ctx, casefile.CaseTypeInput{
		Code: code, Label: "Conservation test", DefaultRetentionPolicyCode: policy, OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("create case type with policy %q: %v", policy, err)
	}
	return code
}

// closedCase creates a case of a type and closes it.
func closedCase(t *testing.T, env *testEnv, typeCode string) *casefile.Case {
	t.Helper()
	c, _, err := env.caseSvc.Create(env.ctx, casefile.CreateInput{CaseTypeCode: typeCode, Title: "Dossier " + uniqueToken(), OperatorID: testOperator})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	transition(t, env, c.ID, casefile.StatusClosed, "travail terminé")
	return c
}

func transition(t *testing.T, env *testEnv, id uuid.UUID, target casefile.Status, reason string) {
	t.Helper()
	if _, _, err := env.caseSvc.Transition(env.ctx, id, casefile.TransitionInput{Target: target, OperatorID: testOperator, Reason: reason}); err != nil {
		t.Fatalf("transition to %s: %v", target, err)
	}
}

// TestRetentionFollowsTheCaseLifecycle covers GLD-032 step A: closing a case
// snapshots its type's policy (start on the closing day, end the duration
// later), reopening ends it, closing again starts a new one; a later policy
// change leaves the running retention unchanged.
func TestRetentionFollowsTheCaseLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	policy := referenceCode("ITP")
	if _, _, err := env.retentionSvc.CreatePolicy(ctx, retention.PolicyInput{
		Code: policy, Label: "Test 5 ans", DurationYears: 5, FinalDisposition: retention.DispositionDestroy, OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("create policy: %v", err)
	}
	c := closedCase(t, env, newRetentionCaseType(t, env, policy))

	got, err := env.retentionSvc.CaseRetention(ctx, c.ID)
	if err != nil || got.Current == nil || got.DefaultPolicy == nil || got.DefaultPolicy.Code != policy {
		t.Fatalf("a closed case has a running retention: %+v (%v)", got, err)
	}
	today := time.Now().In(zurich(t)).Format(time.DateOnly)
	if got.Current.StartsOn.Format(time.DateOnly) != today || got.Current.UntilOn.Year() != got.Current.StartsOn.Year()+5 ||
		got.Current.FinalDisposition != retention.DispositionDestroy || got.Current.PolicyLabel != "Test 5 ans" {
		t.Fatalf("retention snapshot: %+v", got.Current)
	}

	ten := int32(10)
	if _, _, err := env.retentionSvc.UpdatePolicy(ctx, policy, retention.PolicyUpdate{DurationYears: &ten, OperatorID: testOperator}); err != nil {
		t.Fatalf("update policy: %v", err)
	}
	if again, _ := env.retentionSvc.CaseRetention(ctx, c.ID); again.Current.DurationYears != 5 {
		t.Fatalf("a running retention keeps its snapshot: %+v", again.Current)
	}

	transition(t, env, c.ID, casefile.StatusOpen, "recours")
	if got, _ = env.retentionSvc.CaseRetention(ctx, c.ID); got.Current != nil || len(got.Retentions) != 1 || got.Retentions[0].EndReason != "recours" {
		t.Fatalf("reopening ends the retention: %+v", got)
	}
	transition(t, env, c.ID, casefile.StatusClosed, "recours traité")
	if got, _ = env.retentionSvc.CaseRetention(ctx, c.ID); got.Current == nil || got.Current.DurationYears != 10 || len(got.Retentions) != 2 {
		t.Fatalf("closing again starts a new retention from the current policy: %+v", got)
	}
	audit, err := env.coreSvc.ListAuditEvents(ctx, core.AuditFilter{SubjectID: c.ID, Limit: 50})
	if err != nil || countEvents(audit.Events, retention.EventRetentionStarted) != 2 || countEvents(audit.Events, retention.EventRetentionEnded) != 1 {
		t.Fatalf("retention changes are audited on the case: %v", err)
	}

	noPolicy := closedCase(t, env, newRetentionCaseType(t, env, ""))
	if got, err := env.retentionSvc.CaseRetention(ctx, noPolicy.ID); err != nil || got.Current != nil || got.DefaultPolicy != nil {
		t.Fatalf("a type without policy gives no retention: %+v (%v)", got, err)
	}
}

// TestRetentionHoldsAndDueList covers the legal hold (one at a time, a release
// needs one) and the due list (held cases only on request).
func TestRetentionHoldsAndDueList(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := closedCase(t, env, newRetentionCaseType(t, env, "EXAMPLE_DESTROY_5"))
	future := time.Now().AddDate(6, 0, 0).Format(time.DateOnly)

	due := dueIDs(t, env, retention.DueFilter{DueOn: future, Disposition: retention.DispositionDestroy, Limit: 200})
	if !due[c.ID] {
		t.Fatalf("a retention ending before the day is due")
	}
	if dueIDs(t, env, retention.DueFilter{Limit: 200})[c.ID] {
		t.Fatalf("a retention ending in 5 years is not due today")
	}
	if dueIDs(t, env, retention.DueFilter{DueOn: future, Disposition: retention.DispositionArchive, Limit: 200})[c.ID] {
		t.Fatalf("the disposition filter applies")
	}

	hold := retention.HoldInput{CaseID: c.ID, Reason: "procédure pendante", OperatorID: testOperator}
	if _, _, err := env.retentionSvc.ReleaseHold(ctx, hold); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("releasing without a hold: want ErrInvalidState, got %v", err)
	}
	if h, ev, err := env.retentionSvc.PlaceHold(ctx, hold); err != nil || h.ReleasedAt != nil || ev.EventType != retention.EventHoldPlaced {
		t.Fatalf("place hold: %+v %+v (%v)", h, ev, err)
	}
	if _, _, err := env.retentionSvc.PlaceHold(ctx, hold); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a second hold: want ErrConflict, got %v", err)
	}
	if dueIDs(t, env, retention.DueFilter{DueOn: future, Limit: 200})[c.ID] {
		t.Fatalf("a held case leaves the due list")
	}
	if !dueIDs(t, env, retention.DueFilter{DueOn: future, IncludeHeld: true, Limit: 200})[c.ID] {
		t.Fatalf("a held case is listed on request")
	}
	if h, _, err := env.retentionSvc.ReleaseHold(ctx, retention.HoldInput{CaseID: c.ID, Reason: "procédure close", OperatorID: testOperator}); err != nil || h.ReleasedAt == nil {
		t.Fatalf("release hold: %+v (%v)", h, err)
	}
	if got, _ := env.retentionSvc.CaseRetention(ctx, c.ID); got.Hold != nil || len(got.Holds) != 1 {
		t.Fatalf("the released hold stays as history: %+v", got)
	}
	if _, _, err := env.retentionSvc.PlaceHold(ctx, retention.HoldInput{CaseID: uuid.New(), Reason: "x", OperatorID: testOperator}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("a hold on an unknown case: want ErrNotFound, got %v", err)
	}
}

// TestRetentionAppliedToClosedCases covers ApplyCaseType: the closed cases of a
// type given a policy afterwards receive a retention, once.
func TestRetentionAppliedToClosedCases(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	code := newRetentionCaseType(t, env, "")
	c := closedCase(t, env, code)
	in := retention.ApplyInput{CaseTypeCode: code, Reason: "reprise du calendrier", OperatorID: testOperator}
	if _, err := env.retentionSvc.ApplyCaseType(ctx, in); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("a type without policy: want ErrInvalidState, got %v", err)
	}
	policy := "EXAMPLE_ARCHIVE_10"
	if _, _, err := env.caseSvc.UpdateCaseType(ctx, code, casefile.CaseTypeUpdate{DefaultRetentionPolicyCode: &policy, OperatorID: testOperator}); err != nil {
		t.Fatalf("give the type a policy: %v", err)
	}
	if res, err := env.retentionSvc.ApplyCaseType(ctx, in); err != nil || res.Applied != 1 || res.Remaining != 0 {
		t.Fatalf("apply: %+v (%v)", res, err)
	}
	if res, err := env.retentionSvc.ApplyCaseType(ctx, in); err != nil || res.Applied != 0 {
		t.Fatalf("applying again changes nothing: %+v (%v)", res, err)
	}
	if got, _ := env.retentionSvc.CaseRetention(ctx, c.ID); got.Current == nil || got.Current.FinalDisposition != retention.DispositionArchive {
		t.Fatalf("the closed case has the type's policy: %+v", got)
	}

	inactive := referenceCode("ITP")
	if _, _, err := env.retentionSvc.CreatePolicy(ctx, retention.PolicyInput{Code: inactive, Label: "Inactive", DurationYears: 1, FinalDisposition: retention.DispositionReview, OperatorID: testOperator}); err != nil {
		t.Fatalf("create policy: %v", err)
	}
	off := false
	if _, _, err := env.retentionSvc.UpdatePolicy(ctx, inactive, retention.PolicyUpdate{IsActive: &off, OperatorID: testOperator}); err != nil {
		t.Fatalf("deactivate policy: %v", err)
	}
	if _, _, err := env.caseSvc.UpdateCaseType(ctx, code, casefile.CaseTypeUpdate{DefaultRetentionPolicyCode: &inactive, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("an inactive policy: want ErrInvalidInput, got %v", err)
	}
}

func dueIDs(t *testing.T, env *testEnv, filter retention.DueFilter) map[uuid.UUID]bool {
	t.Helper()
	res, err := env.retentionSvc.ListDue(env.ctx, filter)
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	ids := map[uuid.UUID]bool{}
	for _, r := range res.Retentions {
		ids[r.SubjectID] = true
	}
	return ids
}

func countEvents(events []*core.AuditEvent, eventType string) int {
	n := 0
	for _, ev := range events {
		if ev.EventType == eventType {
			n++
		}
	}
	return n
}

func zurich(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
