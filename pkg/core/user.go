package core

import (
	"context"
	"errors"
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
	// IsAdmin reports whether the user currently holds the ADMIN role.
	IsAdmin bool `db:"is_admin"`
	// Roles are the codes of the roles the user currently holds, sorted.
	Roles []string `db:"roles"`
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
	return u.DisplayName == p.DisplayName && u.Email == p.Email
}

// UserDirectory records users and reads their roles; PostgresRepository
// implements it.
type UserDirectory interface {
	RecordUser(ctx context.Context, profile UserProfile) (*AppUser, error)
	ActiveRoles(ctx context.Context, userID string) ([]string, error)
	GrantUserRole(ctx context.Context, in RoleChangeInput) (*UserRole, *AuditEvent, error)
}

// rolesTTL is how long the effective roles of a user are cached; a change made
// through this process's core service is applied at once (ForgetRoles), one
// made by another replica within rolesTTL.
const rolesTTL = 30 * time.Second

// RecordingVerifier decorates a TokenVerifier. Every successfully verified user
// is recorded (created on first sight, updated when its profile changes), and
// its scopes are corrected from the roles stored in Goéland: goeland:admin is
// removed from what the token says and added back only for a holder of the
// ADMIN role. Users named in the bootstrap list receive ADMIN on their next
// request (audited as system:bootstrap). It keeps the interceptor chain
// unchanged and covers the out-of-proto HTTP endpoints too. Recording is best
// effort; a failure to read the roles fails closed (no admin scope).
type RecordingVerifier struct {
	next      authadapter.TokenVerifier
	users     UserDirectory
	bootstrap map[string]bool
	log       *slog.Logger
	now       func() time.Time

	mu    sync.Mutex
	seen  map[string]seenUser
	roles map[string]cachedRoles
}

type seenUser struct {
	profile UserProfile
	at      time.Time
}

type cachedRoles struct {
	codes []string
	at    time.Time
}

// NewRecordingVerifier wraps next so verified users are recorded in users and
// get the scopes of their stored roles; bootstrapAdmins are user ids granted
// ADMIN on their next request.
func NewRecordingVerifier(next authadapter.TokenVerifier, users UserDirectory, bootstrapAdmins []string, log *slog.Logger) (*RecordingVerifier, error) {
	if next == nil || users == nil {
		return nil, fmt.Errorf("%w: a token verifier and a user directory are required", ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	bootstrap := map[string]bool{}
	for _, id := range bootstrapAdmins {
		if id = strings.TrimSpace(id); id != "" {
			bootstrap[id] = true
		}
	}
	return &RecordingVerifier{
		next: next, users: users, bootstrap: bootstrap, log: log, now: time.Now,
		seen: map[string]seenUser{}, roles: map[string]cachedRoles{},
	}, nil
}

// VerifyBearerToken verifies the token with the wrapped verifier, records the
// user unless the same profile was recorded recently, and sets its scopes from
// its stored roles.
func (v *RecordingVerifier) VerifyBearerToken(ctx context.Context, token string) (*authadapter.AuthenticatedUser, error) {
	user, err := v.next.VerifyBearerToken(ctx, token)
	if err != nil || user == nil || user.AppUserID <= 0 {
		return user, err
	}
	user.Scopes = slices.DeleteFunc(slices.Clone(user.Scopes), func(s string) bool { return s == ScopeAdmin })
	profile := ProfileFromUser(user)
	v.record(ctx, profile)
	if slices.Contains(v.effectiveRoles(ctx, profile.UserID), RoleAdmin) {
		user.Scopes = append(user.Scopes, ScopeAdmin)
	}
	return user, nil
}

// record stores the profile unless it was recorded unchanged recently.
func (v *RecordingVerifier) record(ctx context.Context, profile UserProfile) {
	if v.fresh(profile) {
		return
	}
	if _, err := v.users.RecordUser(ctx, profile); err != nil {
		v.log.Warn("record authenticated user", "user_id", profile.UserID, "error", err)
		return
	}
	v.mu.Lock()
	v.seen[profile.UserID] = seenUser{profile: profile, at: v.now()}
	v.mu.Unlock()
}

// fresh reports whether profile was recorded unchanged within userSeenRefresh.
func (v *RecordingVerifier) fresh(profile UserProfile) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	last, ok := v.seen[profile.UserID]
	return ok && last.profile == profile && v.now().Sub(last.at) < userSeenRefresh
}

// effectiveRoles returns the user's current roles, from the cache when fresh;
// a bootstrap administrator without ADMIN receives it first. A read failure is
// logged and yields no role.
func (v *RecordingVerifier) effectiveRoles(ctx context.Context, userID string) []string {
	v.mu.Lock()
	cached, ok := v.roles[userID]
	v.mu.Unlock()
	if ok && v.now().Sub(cached.at) < rolesTTL {
		return cached.codes
	}
	codes, err := v.users.ActiveRoles(ctx, userID)
	if err != nil {
		v.log.Warn("read user roles", "user_id", userID, "error", err)
		return nil
	}
	if v.bootstrap[userID] && !slices.Contains(codes, RoleAdmin) {
		codes = v.bootstrapAdmin(ctx, userID, codes)
	}
	v.mu.Lock()
	v.roles[userID] = cachedRoles{codes: codes, at: v.now()}
	v.mu.Unlock()
	return codes
}

// bootstrapAdmin grants ADMIN to a bootstrap user and returns its roles.
func (v *RecordingVerifier) bootstrapAdmin(ctx context.Context, userID string, codes []string) []string {
	_, _, err := v.users.GrantUserRole(ctx, RoleChangeInput{
		UserID: userID, RoleCode: RoleAdmin, OperatorID: OperatorBootstrap,
		Reason: "listed in GOELAND_BOOTSTRAP_ADMINS",
	})
	if err != nil && !errors.Is(err, ErrConflict) {
		v.log.Warn("bootstrap administrator", "user_id", userID, "error", err)
		return codes
	}
	v.log.Info("bootstrap administrator granted", "user_id", userID)
	return append(slices.Clone(codes), RoleAdmin)
}

// ForgetRoles drops the cached roles of userID, so its next request reads them
// again (wired to core.Service.OnRolesChanged).
func (v *RecordingVerifier) ForgetRoles(userID string) {
	v.mu.Lock()
	delete(v.roles, userID)
	v.mu.Unlock()
}
