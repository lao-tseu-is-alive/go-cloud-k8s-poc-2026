package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
)

// fakeVerifier returns a copy of user, or err.
type fakeVerifier struct {
	user *authadapter.AuthenticatedUser
	err  error
}

func (f *fakeVerifier) VerifyBearerToken(context.Context, string) (*authadapter.AuthenticatedUser, error) {
	if f.err != nil {
		return nil, f.err
	}
	u := *f.user
	return &u, nil
}

// fakeRecorder counts recordings and can fail.
type fakeRecorder struct {
	profiles []UserProfile
	err      error
}

func (f *fakeRecorder) RecordUser(_ context.Context, p UserProfile) (*AppUser, error) {
	f.profiles = append(f.profiles, p)
	if f.err != nil {
		return nil, f.err
	}
	return &AppUser{UserID: p.UserID, DisplayName: p.DisplayName}, nil
}

func TestProfileFromUser(t *testing.T) {
	p := ProfileFromUser(&authadapter.AuthenticatedUser{AppUserID: 7, DisplayName: " Ada ", Email: "ada@example.org", Scopes: []string{ScopeRead, ScopeAdmin}})
	if p.UserID != "7" || p.DisplayName != "Ada" || !p.IsAdmin {
		t.Fatalf("unexpected profile %+v", p)
	}
	if ProfileFromUser(&authadapter.AuthenticatedUser{AppUserID: 7, Scopes: []string{ScopeWrite}}).IsAdmin {
		t.Fatal("write scope must not make an admin")
	}
	for want, p := range map[string]UserProfile{
		"Ada":             {UserID: "7", DisplayName: "Ada", Email: "ada@example.org"},
		"ada@example.org": {UserID: "7", Email: "ada@example.org"},
		"User 7":          {UserID: "7"},
	} {
		if got := p.label(); got != want {
			t.Fatalf("label(%+v) = %q, want %q", p, got, want)
		}
	}
}

func TestRecordingVerifierRecordsOnChangeOnly(t *testing.T) {
	user := &authadapter.AuthenticatedUser{AppUserID: 7, DisplayName: "Ada", Scopes: []string{ScopeRead}}
	rec := &fakeRecorder{}
	v, err := NewRecordingVerifier(&fakeVerifier{user: user}, rec, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	v.now = func() time.Time { return now }
	ctx := context.Background()

	for range 3 {
		if _, err := v.VerifyBearerToken(ctx, "t"); err != nil {
			t.Fatal(err)
		}
	}
	if len(rec.profiles) != 1 {
		t.Fatalf("unchanged profile recorded %d times, want 1", len(rec.profiles))
	}
	user.DisplayName = "Ada Lovelace"
	_, _ = v.VerifyBearerToken(ctx, "t")
	now = now.Add(userSeenRefresh)
	user.DisplayName = "Ada Lovelace"
	_, _ = v.VerifyBearerToken(ctx, "t")
	if len(rec.profiles) != 3 || rec.profiles[1].DisplayName != "Ada Lovelace" {
		t.Fatalf("want a recording on change and after the refresh period, got %+v", rec.profiles)
	}
}

func TestRecordingVerifierIsBestEffort(t *testing.T) {
	rec := &fakeRecorder{err: errors.New("db down")}
	v, _ := NewRecordingVerifier(&fakeVerifier{user: &authadapter.AuthenticatedUser{AppUserID: 7}}, rec, nil)
	for range 2 {
		if user, err := v.VerifyBearerToken(context.Background(), "t"); err != nil || user == nil {
			t.Fatalf("a recording failure must not fail authentication: %v", err)
		}
	}
	if len(rec.profiles) != 2 {
		t.Fatalf("a failed recording must be retried, got %d attempts", len(rec.profiles))
	}

	bad := &fakeRecorder{}
	v, _ = NewRecordingVerifier(&fakeVerifier{err: authadapter.ErrInvalidToken}, bad, nil)
	if _, err := v.VerifyBearerToken(context.Background(), "t"); !errors.Is(err, authadapter.ErrInvalidToken) || len(bad.profiles) != 0 {
		t.Fatalf("an invalid token must fail without recording: %v", err)
	}
	if _, err := NewRecordingVerifier(nil, bad, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil verifier: want ErrInvalidInput, got %v", err)
	}
}
