package integration

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

// uniqueNamespace returns a fresh business_ref namespace so each run starts its
// counters at 1 without touching other rows.
func uniqueNamespace() string {
	return "T" + strings.ToUpper(uniqueToken()[1:])
}

// currentPeriod is the calendar year the allocator uses (Europe/Zurich).
func currentPeriod(t *testing.T) string {
	t.Helper()
	zone, err := time.LoadLocation(core.BusinessRefPeriodTimeZone)
	if err != nil {
		t.Fatalf("load %s: %v", core.BusinessRefPeriodTimeZone, err)
	}
	return fmt.Sprintf("%d", time.Now().In(zone).Year())
}

// createCase creates a bare CASE subject, optionally with a business reference.
func createCase(t *testing.T, env *testEnv, label string, req core.BusinessRefRequest) (*core.SubjectRef, *core.AuditEvent, error) {
	t.Helper()
	ref, _, ev, err := env.coreSvc.CreateSubjectRef(env.ctx, core.CreateSubjectInput{
		Kind:         core.SubjectKindCase,
		DisplayLabel: label,
		OperatorID:   testOperator,
		BusinessRef:  req,
	})
	return ref, ev, err
}

// TestBusinessRefAllocationAndUniqueness covers allocation at creation, explicit
// references with and without namespace, and lookup.
func TestBusinessRefAllocationAndUniqueness(t *testing.T) {
	env := newTestEnv(t)
	ns := uniqueNamespace()
	period := currentPeriod(t)

	first, ev, err := createCase(t, env, "Case A", core.BusinessRefRequest{Namespace: ns, Allocate: true})
	if err != nil {
		t.Fatalf("create with allocation: %v", err)
	}
	if want := period + "-000001"; first.BusinessRef != want || first.BusinessRefNamespace != ns {
		t.Fatalf("got %q in %q, want %q in %q", first.BusinessRef, first.BusinessRefNamespace, want, ns)
	}
	if ev.AfterState["business_ref"] != first.BusinessRef {
		t.Fatalf("SUBJECT_CREATED audit must record the reference, got %+v", ev.AfterState)
	}
	second, _, err := createCase(t, env, "Case B", core.BusinessRefRequest{Namespace: ns, Allocate: true})
	if err != nil {
		t.Fatalf("second allocation: %v", err)
	}
	if want := period + "-000002"; second.BusinessRef != want {
		t.Fatalf("second allocation got %q, want %q", second.BusinessRef, want)
	}

	// An explicit value colliding in the same namespace is a conflict and rolls
	// back the whole creation.
	label := "Duplicate " + uniqueToken()
	if _, _, err := createCase(t, env, label, core.BusinessRefRequest{Namespace: ns, Value: first.BusinessRef}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate namespaced reference: want ErrConflict, got %v", err)
	}
	var leaked int
	if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM subject_ref WHERE display_label = $1`, label).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("a failed creation must not leave a subject (count=%d, err=%v)", leaked, err)
	}

	// Without a namespace a reference is free text and may repeat.
	legacy := "LEG-" + uniqueToken()
	for _, l := range []string{"Legacy 1", "Legacy 2"} {
		if _, _, err := createCase(t, env, l, core.BusinessRefRequest{Value: legacy}); err != nil {
			t.Fatalf("free reference %q: %v", legacy, err)
		}
	}

	found, err := env.coreSvc.LookupSubjects(env.ctx, core.LookupFilter{BusinessRef: first.BusinessRef, Namespace: ns})
	if err != nil || len(found) != 1 || found[0].ID != first.ID {
		t.Fatalf("namespaced lookup: got %+v, %v", found, err)
	}
	found, err = env.coreSvc.LookupSubjects(env.ctx, core.LookupFilter{BusinessRef: legacy, Kind: core.SubjectKindCase})
	if err != nil || len(found) != 2 {
		t.Fatalf("free-reference lookup: want 2 matches, got %d (%v)", len(found), err)
	}
	found, err = env.coreSvc.LookupSubjects(env.ctx, core.LookupFilter{BusinessRef: legacy, Kind: core.SubjectKindDocument})
	if err != nil || len(found) != 0 {
		t.Fatalf("kind filter: want 0 documents, got %d (%v)", len(found), err)
	}
}

// TestAssignBusinessRef covers assignment after creation, its audit event, the
// one-reference-per-subject rule and the soft-deleted guard.
func TestAssignBusinessRef(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	ns := uniqueNamespace()

	subject, _, err := createCase(t, env, "Case to reference", core.BusinessRefRequest{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if subject.BusinessRef != "" {
		t.Fatalf("no reference requested, got %q", subject.BusinessRef)
	}
	ref, ev, err := env.coreSvc.AssignBusinessRef(ctx, subject.ID, core.BusinessRefRequest{Namespace: ns, Allocate: true}, testOperator, "registered")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if ref.BusinessRef != currentPeriod(t)+"-000001" || ev.EventType != "BUSINESS_REF_ASSIGNED" || ev.Reason != "registered" {
		t.Fatalf("unexpected assignment %q / %+v", ref.BusinessRef, ev)
	}
	if _, _, err := env.coreSvc.AssignBusinessRef(ctx, subject.ID, core.BusinessRefRequest{Value: "OTHER"}, testOperator, ""); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("second assignment: want ErrConflict, got %v", err)
	}

	res, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "PLAN",
		Title:            "Deleted " + uniqueToken(),
		OperatorID:       testOperator,
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	doc := res.Document
	if _, err := env.docSvc.SoftDelete(ctx, doc.ID, testOperator, "test"); err != nil {
		t.Fatalf("soft delete document: %v", err)
	}
	if _, _, err := env.coreSvc.AssignBusinessRef(ctx, doc.ID, core.BusinessRefRequest{Namespace: ns, Allocate: true}, testOperator, ""); !errors.Is(err, core.ErrDeleted) {
		t.Fatalf("assign on a deleted subject: want ErrDeleted, got %v", err)
	}
	// The failed allocation above was rolled back, so the next number is 2.
	next, _, err := createCase(t, env, "Next", core.BusinessRefRequest{Namespace: ns, Allocate: true})
	if err != nil || next.BusinessRef != currentPeriod(t)+"-000002" {
		t.Fatalf("rolled-back allocation must give its number back: got %q (%v)", next.BusinessRef, err)
	}
}

// TestBusinessRefConcurrentAllocation checks that parallel allocations in one
// namespace produce distinct, contiguous numbers.
func TestBusinessRefConcurrentAllocation(t *testing.T) {
	env := newTestEnv(t)
	ns := uniqueNamespace()
	const workers = 12

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		refs []string
		errs []error
	)
	for i := range workers {
		wg.Go(func() {
			ref, _, err := createCase(t, env, fmt.Sprintf("Parallel %d", i), core.BusinessRefRequest{Namespace: ns, Allocate: true})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			refs = append(refs, ref.BusinessRef)
		})
	}
	wg.Wait()
	if len(errs) != 0 {
		t.Fatalf("parallel allocations failed: %v", errs)
	}
	sort.Strings(refs)
	period := currentPeriod(t)
	for i, got := range refs {
		if want := core.FormatAllocatedBusinessRef(period, int64(i+1)); got != want {
			t.Fatalf("allocation %d: got %q, want %q (all: %v)", i, got, want, refs)
		}
	}
}
