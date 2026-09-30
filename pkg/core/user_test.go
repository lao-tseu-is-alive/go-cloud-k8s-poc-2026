package core

import (
	"context"
	"errors"
	"slices"
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

// fakeRecorder counts recordings and can fail; it also keeps roles per user.
type fakeRecorder struct {
	profiles  []UserProfile
	err       error
	roles     map[string][]string
	roleReads int
	rolesErr  error
	grants    []RoleChangeInput
}

func (f *fakeRecorder) RecordUser(_ context.Context, p UserProfile) (*AppUser, error) {
	f.profiles = append(f.profiles, p)
	if f.err != nil {
		return nil, f.err
	}
	return &AppUser{UserID: p.UserID, DisplayName: p.DisplayName}, nil
}

func (f *fakeRecorder) ActiveRoles(_ context.Context, userID string) ([]string, error) {
	f.roleReads++
	if f.rolesErr != nil {
		return nil, f.rolesErr
	}
	return f.roles[userID], nil
}

func (f *fakeRecorder) GrantUserRole(_ context.Context, in RoleChangeInput) (*UserRole, *AuditEvent, error) {
	f.grants = append(f.grants, in)
	if f.roles == nil {
		f.roles = map[string][]string{}
	}
	f.roles[in.UserID] = append(f.roles[in.UserID], in.RoleCode)
	return &UserRole{UserID: in.UserID, RoleCode: in.RoleCode}, &AuditEvent{}, nil
}

func TestProfileFromUser(t *testing.T) {
	p := ProfileFromUser(&authadapter.AuthenticatedUser{AppUserID: 7, DisplayName: " Ada ", Email: "ada@example.org", Scopes: []string{ScopeRead, ScopeAdmin}})
	if p != (UserProfile{UserID: "7", DisplayName: "Ada", Email: "ada@example.org"}) {
		t.Fatalf("unexpected profile %+v", p)
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
	v, err := NewRecordingVerifier(&fakeVerifier{user: user}, rec, nil, nil)
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
	v, _ := NewRecordingVerifier(&fakeVerifier{user: &authadapter.AuthenticatedUser{AppUserID: 7}}, rec, nil, nil)
	for range 2 {
		if user, err := v.VerifyBearerToken(context.Background(), "t"); err != nil || user == nil {
			t.Fatalf("a recording failure must not fail authentication: %v", err)
		}
	}
	if len(rec.profiles) != 2 {
		t.Fatalf("a failed recording must be retried, got %d attempts", len(rec.profiles))
	}

	bad := &fakeRecorder{}
	v, _ = NewRecordingVerifier(&fakeVerifier{err: authadapter.ErrInvalidToken}, bad, nil, nil)
	if _, err := v.VerifyBearerToken(context.Background(), "t"); !errors.Is(err, authadapter.ErrInvalidToken) || len(bad.profiles) != 0 {
		t.Fatalf("an invalid token must fail without recording: %v", err)
	}
	if _, err := NewRecordingVerifier(nil, bad, nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil verifier: want ErrInvalidInput, got %v", err)
	}
}

func TestRecordingVerifierScopesComeFromStoredRoles(t *testing.T) {
	ctx := context.Background()
	tokenAdmin := &authadapter.AuthenticatedUser{AppUserID: 7, Scopes: []string{ScopeRead, ScopeWrite, ScopeAdmin}}
	rec := &fakeRecorder{}
	v, _ := NewRecordingVerifier(&fakeVerifier{user: tokenAdmin}, rec, nil, nil)
	user, err := v.VerifyBearerToken(ctx, "t")
	if err != nil || slices.Contains(user.Scopes, ScopeAdmin) || !slices.Contains(user.Scopes, ScopeWrite) {
		t.Fatalf("the token's admin scope is ignored, the others kept: %v (%v)", user.Scopes, err)
	}

	rec = &fakeRecorder{roles: map[string][]string{"7": {RoleAdmin}}}
	v, _ = NewRecordingVerifier(&fakeVerifier{user: &authadapter.AuthenticatedUser{AppUserID: 7, Scopes: []string{ScopeRead}}}, rec, nil, nil)
	if user, _ := v.VerifyBearerToken(ctx, "t"); !slices.Contains(user.Scopes, ScopeAdmin) {
		t.Fatalf("a holder of ADMIN gets goeland:admin: %v", user.Scopes)
	}
}

func TestRecordingVerifierCachesRolesUntilForgotten(t *testing.T) {
	ctx := context.Background()
	rec := &fakeRecorder{roles: map[string][]string{}}
	v, _ := NewRecordingVerifier(&fakeVerifier{user: &authadapter.AuthenticatedUser{AppUserID: 7}}, rec, nil, nil)
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	v.now = func() time.Time { return now }
	_, _ = v.VerifyBearerToken(ctx, "t")
	rec.roles["7"] = []string{RoleAdmin}
	if user, _ := v.VerifyBearerToken(ctx, "t"); slices.Contains(user.Scopes, ScopeAdmin) || rec.roleReads != 1 {
		t.Fatalf("roles are cached for rolesTTL: reads=%d scopes=%v", rec.roleReads, user.Scopes)
	}
	v.ForgetRoles("7")
	if user, _ := v.VerifyBearerToken(ctx, "t"); !slices.Contains(user.Scopes, ScopeAdmin) {
		t.Fatalf("a forgotten user reads its roles again: %v", user.Scopes)
	}
	delete(rec.roles, "7")
	now = now.Add(rolesTTL)
	if user, _ := v.VerifyBearerToken(ctx, "t"); slices.Contains(user.Scopes, ScopeAdmin) {
		t.Fatalf("after rolesTTL a revocation made elsewhere applies: %v", user.Scopes)
	}
}

func TestRecordingVerifierBootstrapsAdministrators(t *testing.T) {
	ctx := context.Background()
	rec := &fakeRecorder{}
	v, _ := NewRecordingVerifier(&fakeVerifier{user: &authadapter.AuthenticatedUser{AppUserID: 7}}, rec, []string{" 7 ", ""}, nil)
	for range 2 {
		v.ForgetRoles("7")
		if user, _ := v.VerifyBearerToken(ctx, "t"); !slices.Contains(user.Scopes, ScopeAdmin) {
			t.Fatalf("a bootstrap user is an administrator: %v", user.Scopes)
		}
	}
	if len(rec.grants) != 1 || rec.grants[0].OperatorID != OperatorBootstrap || rec.grants[0].RoleCode != RoleAdmin {
		t.Fatalf("ADMIN is granted once, by system:bootstrap: %+v", rec.grants)
	}

	other, _ := NewRecordingVerifier(&fakeVerifier{user: &authadapter.AuthenticatedUser{AppUserID: 8}}, &fakeRecorder{}, []string{"7"}, nil)
	if user, _ := other.VerifyBearerToken(ctx, "t"); slices.Contains(user.Scopes, ScopeAdmin) {
		t.Fatal("a user outside the bootstrap list gets nothing")
	}
}

func TestRecordingVerifierFailsClosedOnRoles(t *testing.T) {
	rec := &fakeRecorder{roles: map[string][]string{"7": {RoleAdmin}}, rolesErr: errors.New("db down")}
	v, _ := NewRecordingVerifier(&fakeVerifier{user: &authadapter.AuthenticatedUser{AppUserID: 7, Scopes: []string{ScopeAdmin}}}, rec, nil, nil)
	user, err := v.VerifyBearerToken(context.Background(), "t")
	if err != nil || slices.Contains(user.Scopes, ScopeAdmin) {
		t.Fatalf("unreadable roles give no admin scope but keep the authentication: %v (%v)", user.Scopes, err)
	}
}
