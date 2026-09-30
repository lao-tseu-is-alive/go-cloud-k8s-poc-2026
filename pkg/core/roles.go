package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Application roles are stored in Goéland (GLD-047): the token only
// authenticates, and goeland:admin comes from the ADMIN role.

// RoleAdmin is the application role behind the goeland:admin scope.
const RoleAdmin = "ADMIN"

// Operators recorded when the system, not a person, grants a role.
const (
	// OperatorBootstrap grants ADMIN to the users named by GOELAND_BOOTSTRAP_ADMINS.
	OperatorBootstrap = "system:bootstrap"
)

// Audit event types of role assignments, written on the user's USER subject.
const (
	// EventUserRoleGranted records a role granted to a user.
	EventUserRoleGranted = "USER_ROLE_GRANTED"
	// EventUserRoleRevoked records a role revoked from a user.
	EventUserRoleRevoked = "USER_ROLE_REVOKED"
)

// roleCodePattern is the format of a role code (as the app_role CHECK).
var roleCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// AppRole is an application role of the catalogue.
type AppRole struct {
	// Code is the stable identifier (e.g. ADMIN).
	Code string `db:"code"`
	// Label is the human name.
	Label string `db:"label"`
	// Description tells what the role allows.
	Description string `db:"description"`
	// IsActive is false for a role that can no longer be granted.
	IsActive bool `db:"is_active"`
}

// UserRole is one assignment of a role to a user; a revocation keeps the row.
type UserRole struct {
	// ID identifies the assignment.
	ID uuid.UUID `db:"id"`
	// UserID is the internal user (AppUser.UserID).
	UserID string `db:"user_id"`
	// RoleCode is the assigned role.
	RoleCode string `db:"role_code"`
	// GrantedAt is when the role was granted.
	GrantedAt time.Time `db:"granted_at"`
	// GrantedBy is the operator who granted it (or a system:* operator).
	GrantedBy string `db:"granted_by"`
	// GrantReason explains the grant.
	GrantReason string `db:"grant_reason"`
	// RevokedAt is when the role was revoked; nil while it is held.
	RevokedAt *time.Time `db:"revoked_at"`
	// RevokedBy is the operator who revoked it; nil while it is held.
	RevokedBy *string `db:"revoked_by"`
	// RevokeReason explains the revocation.
	RevokeReason string `db:"revoke_reason"`
}

// RoleChangeInput grants or revokes one role of one user.
type RoleChangeInput struct {
	// UserID is the internal user.
	UserID string
	// RoleCode is the role.
	RoleCode string
	// Reason explains the change; it is audited.
	Reason string
	// OperatorID is who makes the change (server-derived, or a system:* operator).
	OperatorID string
}

// normalize trims the input and checks it is complete.
func (in RoleChangeInput) normalize() (RoleChangeInput, error) {
	in.UserID = strings.TrimSpace(in.UserID)
	in.RoleCode = strings.TrimSpace(in.RoleCode)
	in.Reason = strings.TrimSpace(in.Reason)
	switch {
	case in.UserID == "":
		return in, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	case !roleCodePattern.MatchString(in.RoleCode):
		return in, fmt.Errorf("%w: role code %q is not a valid code", ErrInvalidInput, in.RoleCode)
	case in.Reason == "":
		return in, fmt.Errorf("%w: a reason is required", ErrInvalidInput)
	case strings.TrimSpace(in.OperatorID) == "":
		return in, fmt.Errorf("%w: operator id is required", ErrInvalidInput)
	}
	return in, nil
}

// ListAppRoles returns the application role catalogue.
func (s *Service) ListAppRoles(ctx context.Context) ([]*AppRole, error) {
	return s.repo.ListAppRoles(ctx)
}

// ListRoleHolders returns the users currently holding roleCode.
func (s *Service) ListRoleHolders(ctx context.Context, roleCode string) ([]*AppUser, error) {
	return s.repo.ListRoleHolders(ctx, strings.TrimSpace(roleCode))
}

// ListUserRoles returns a user's assignments, newest first, with the revoked
// ones when includeRevoked.
func (s *Service) ListUserRoles(ctx context.Context, userID string, includeRevoked bool) ([]*UserRole, error) {
	return s.repo.ListUserRoles(ctx, strings.TrimSpace(userID), includeRevoked)
}

// GrantUserRole grants a role to a known user; the role must be active and not
// already held (ErrConflict).
func (s *Service) GrantUserRole(ctx context.Context, in RoleChangeInput) (*UserRole, *AuditEvent, error) {
	in, err := in.normalize()
	if err != nil {
		return nil, nil, err
	}
	role, ev, err := s.repo.GrantUserRole(ctx, in)
	if err == nil {
		s.rolesChanged(in.UserID)
	}
	return role, ev, err
}

// RevokeUserRole revokes a held role, kept as history; revoking the last
// administrator is refused (ErrInvalidState).
func (s *Service) RevokeUserRole(ctx context.Context, in RoleChangeInput) (*UserRole, *AuditEvent, error) {
	in, err := in.normalize()
	if err != nil {
		return nil, nil, err
	}
	role, ev, err := s.repo.RevokeUserRole(ctx, in)
	if err == nil {
		s.rolesChanged(in.UserID)
	}
	return role, ev, err
}

// OnRolesChanged registers a callback told the id of a user whose roles
// changed, so a cache of effective roles (RecordingVerifier) forgets it at once.
func (s *Service) OnRolesChanged(fn func(userID string)) {
	s.rolesListener = fn
}

// rolesChanged tells the registered listener, if any.
func (s *Service) rolesChanged(userID string) {
	if s.rolesListener != nil {
		s.rolesListener(userID)
	}
}
