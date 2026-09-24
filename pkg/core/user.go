package core

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
)

// ScopeAdmin is the administrator scope; authadapter treats it as a wildcard
// satisfying every other scope check.
const ScopeAdmin = "goeland:admin"

// MaxBatchUsers bounds BatchGetUsers.
const MaxBatchUsers = 200

// userSeenRefresh is how long an unchanged profile is not written again.
const userSeenRefresh = 15 * time.Minute

// AppUser is an internal, authenticated user (an employee) as last described by
// its verified token. It is a USER subject, never an ACTOR.
type AppUser struct {
	// UserID is the operator id recorded in governance and audit columns.
	UserID string `db:"user_id"`
	// SubjectID is the user's USER subject_ref, the target for future task links.
	SubjectID uuid.UUID `db:"subject_id"`
	// DisplayName is the human name from the token; empty when the token has none.
	DisplayName string `db:"display_name"`
	// Email is the e-mail address from the token; empty when the token has none.
	Email string `db:"email"`
	// IsAdmin reports whether the last token carried the admin scope.
	IsAdmin bool `db:"is_admin"`
	// FirstSeenAt is when the user was first recorded.
	FirstSeenAt time.Time `db:"first_seen_at"`
	// LastSeenAt is when the user was last recorded (refreshed at most every
	// userSeenRefresh while the profile is unchanged).
	LastSeenAt time.Time `db:"last_seen_at"`
}

// UserProfile is what a verified token says about its user.
type UserProfile struct {
	// UserID is the operator id (see OperatorID).
	UserID string
	// DisplayName is the token's human name.
	DisplayName string
	// Email is the token's e-mail address.
	Email string
	// IsAdmin reports whether the token grants ScopeAdmin.
	IsAdmin bool
}

// ProfileFromUser derives the profile of an authenticated user.
func ProfileFromUser(user *authadapter.AuthenticatedUser) UserProfile {
	if user == nil {
		return UserProfile{}
	}
	return UserProfile{
		UserID:      OperatorID(user),
		DisplayName: strings.TrimSpace(user.DisplayName),
		Email:       strings.TrimSpace(user.Email),
		IsAdmin:     slices.Contains(user.Scopes, ScopeAdmin),
	}
}

// label is the USER subject's display label: the name, else the e-mail, else the id.
func (p UserProfile) label() string {
	switch {
	case p.DisplayName != "":
		return p.DisplayName
	case p.Email != "":
		return p.Email
	default:
		return "User " + p.UserID
	}
}

// sameAs reports whether u already holds profile p.
func (p UserProfile) sameAs(u *AppUser) bool {
	return u.DisplayName == p.DisplayName && u.Email == p.Email && u.IsAdmin == p.IsAdmin
}

// UserRecorder persists user profiles; PostgresRepository implements it.
type UserRecorder interface {
	RecordUser(ctx context.Context, profile UserProfile) (*AppUser, error)
}

// RecordingVerifier decorates a TokenVerifier: every successfully verified user
// is recorded (created on first sight, updated when its profile changes). It
// keeps the interceptor chain unchanged and covers the out-of-proto HTTP
// endpoints too. Recording is best effort: a failure is logged and never fails
// the authentication.
type RecordingVerifier struct {
	next     authadapter.TokenVerifier
	recorder UserRecorder
	log      *slog.Logger
	now      func() time.Time

	mu   sync.Mutex
	seen map[string]seenUser
}

type seenUser struct {
	profile UserProfile
	at      time.Time
}

// NewRecordingVerifier wraps next so verified users are recorded by recorder.
func NewRecordingVerifier(next authadapter.TokenVerifier, recorder UserRecorder, log *slog.Logger) (*RecordingVerifier, error) {
	if next == nil || recorder == nil {
		return nil, fmt.Errorf("%w: a token verifier and a user recorder are required", ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &RecordingVerifier{next: next, recorder: recorder, log: log, now: time.Now, seen: map[string]seenUser{}}, nil
}

// VerifyBearerToken verifies the token with the wrapped verifier, then records
// the user unless the same profile was recorded recently.
func (v *RecordingVerifier) VerifyBearerToken(ctx context.Context, token string) (*authadapter.AuthenticatedUser, error) {
	user, err := v.next.VerifyBearerToken(ctx, token)
	if err != nil || user == nil || user.AppUserID <= 0 {
		return user, err
	}
	profile := ProfileFromUser(user)
	if v.fresh(profile) {
		return user, nil
	}
	if _, err := v.recorder.RecordUser(ctx, profile); err != nil {
		v.log.Warn("record authenticated user", "user_id", profile.UserID, "error", err)
		return user, nil
	}
	v.mu.Lock()
	v.seen[profile.UserID] = seenUser{profile: profile, at: v.now()}
	v.mu.Unlock()
	return user, nil
}

// fresh reports whether profile was recorded unchanged within userSeenRefresh.
func (v *RecordingVerifier) fresh(profile UserProfile) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	last, ok := v.seen[profile.UserID]
	return ok && last.profile == profile && v.now().Sub(last.at) < userSeenRefresh
}
