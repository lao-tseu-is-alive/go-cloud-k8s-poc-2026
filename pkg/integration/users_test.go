package integration

import (
	"sync"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// countUserEvents counts the audit events of one type on a USER subject.
func countUserEvents(t *testing.T, env *testEnv, user *core.AppUser, eventType string) int {
	t.Helper()
	res, err := env.coreSvc.ListAuditEvents(env.ctx, core.AuditFilter{SubjectID: user.SubjectID, EventType: eventType})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	return len(res.Events)
}

// TestRecordUserLifecycle covers first sight, an unchanged refresh and a
// profile change of an internal user.
func TestRecordUserLifecycle(t *testing.T) {
	env := newTestEnv(t)
	profile := core.UserProfile{UserID: "it-" + uniqueToken(), DisplayName: "Ada", Email: "ada@example.org"}

	first, err := env.coreRepo.RecordUser(env.ctx, profile)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	ref, err := env.coreRepo.GetSubject(env.ctx, first.SubjectID)
	if err != nil || ref.Kind != core.SubjectKindUser || ref.DisplayLabel != "Ada" {
		t.Fatalf("USER subject not created as expected: %+v (%v)", ref, err)
	}
	if n := countUserEvents(t, env, first, "USER_REGISTERED"); n != 1 {
		t.Fatalf("USER_REGISTERED events = %d, want 1", n)
	}

	again, err := env.coreRepo.RecordUser(env.ctx, profile)
	if err != nil || again.SubjectID != first.SubjectID || again.LastSeenAt.Before(first.LastSeenAt) {
		t.Fatalf("unchanged refresh: %+v (%v)", again, err)
	}
	if n := countUserEvents(t, env, first, "USER_PROFILE_UPDATED"); n != 0 {
		t.Fatalf("an unchanged profile must not be audited, got %d events", n)
	}

	profile.DisplayName, profile.IsAdmin = "Ada Lovelace", true
	changed, err := env.coreRepo.RecordUser(env.ctx, profile)
	if err != nil || changed.DisplayName != "Ada Lovelace" || !changed.IsAdmin {
		t.Fatalf("profile change: %+v (%v)", changed, err)
	}
	if ref, _ := env.coreRepo.GetSubject(env.ctx, first.SubjectID); ref.DisplayLabel != "Ada Lovelace" {
		t.Fatalf("USER subject not renamed: %q", ref.DisplayLabel)
	}
	if n := countUserEvents(t, env, first, "USER_PROFILE_UPDATED"); n != 1 {
		t.Fatalf("USER_PROFILE_UPDATED events = %d, want 1", n)
	}

	users, err := env.coreSvc.BatchGetUsers(env.ctx, []string{profile.UserID, "it-unknown-" + uniqueToken()})
	if err != nil || len(users) != 1 || users[0].DisplayName != "Ada Lovelace" {
		t.Fatalf("batch get: %+v (%v)", users, err)
	}
}

// TestRecordUserConcurrentFirstSight checks that parallel first requests of a
// new user create exactly one user and one USER subject.
func TestRecordUserConcurrentFirstSight(t *testing.T) {
	env := newTestEnv(t)
	profile := core.UserProfile{UserID: "it-" + uniqueToken(), DisplayName: "Grace"}
	const workers = 8
	subjects := make(chan string, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			user, err := env.coreRepo.RecordUser(env.ctx, profile)
			if err != nil {
				t.Errorf("record user: %v", err)
				return
			}
			subjects <- user.SubjectID.String()
		})
	}
	wg.Wait()
	close(subjects)
	distinct := map[string]bool{}
	for id := range subjects {
		distinct[id] = true
	}
	if len(distinct) != 1 {
		t.Fatalf("concurrent first sight created %d USER subjects, want 1", len(distinct))
	}
	// An orphan USER subject would be the symptom of a lost race.
	var count int
	if err := env.pool.QueryRow(env.ctx, `
SELECT count(*) FROM subject_ref sr JOIN record_metadata rm ON rm.subject_id = sr.id
WHERE sr.kind = 'USER' AND rm.created_by = $1`, profile.UserID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("USER subjects created for the user = %d, want 1", count)
	}
}
