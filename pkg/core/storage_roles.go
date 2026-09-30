package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListAppRoles returns the application role catalogue, by code.
func (r *PostgresRepository) ListAppRoles(ctx context.Context) ([]*AppRole, error) {
	rows, err := r.pool.Query(ctx, listAppRolesSQL)
	if err != nil {
		return nil, fmt.Errorf("list app roles: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[AppRole])
}

// ListRoleHolders returns the users currently holding roleCode, by name.
func (r *PostgresRepository) ListRoleHolders(ctx context.Context, roleCode string) ([]*AppUser, error) {
	rows, err := r.pool.Query(ctx, listRoleHoldersSQL, pgx.NamedArgs{"role_code": roleCode})
	if err != nil {
		return nil, fmt.Errorf("list role holders: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[AppUser])
}

// ListUserRoles returns a user's assignments, newest first.
func (r *PostgresRepository) ListUserRoles(ctx context.Context, userID string, includeRevoked bool) ([]*UserRole, error) {
	rows, err := r.pool.Query(ctx, listUserRolesSQL, pgx.NamedArgs{"user_id": userID, "include_revoked": includeRevoked})
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[UserRole])
}

// ActiveRoles returns the codes of the roles userID currently holds, sorted.
func (r *PostgresRepository) ActiveRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, activeRoleCodesSQL, pgx.NamedArgs{"user_id": userID})
	if err != nil {
		return nil, fmt.Errorf("active roles: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// GrantUserRole records a new assignment and USER_ROLE_GRANTED on the user's
// subject, in one transaction.
func (r *PostgresRepository) GrantUserRole(ctx context.Context, in RoleChangeInput) (*UserRole, *AuditEvent, error) {
	var role *UserRole
	var ev *AuditEvent
	err := InTx(ctx, r.pool, "grant user role", func(tx pgx.Tx) error {
		user, err := lockRoleChangeTx(ctx, tx, in)
		if err != nil {
			return err
		}
		catalogued, err := CollectReferenceRow[AppRole](tx.Query(ctx, getAppRoleSQL, pgx.NamedArgs{"code": in.RoleCode}))
		if err != nil {
			return fmt.Errorf("role %s: %w", in.RoleCode, err)
		}
		if !catalogued.IsActive {
			return fmt.Errorf("%w: role %s is inactive", ErrInvalidState, in.RoleCode)
		}
		role, err = collectUserRole(tx.Query(ctx, insertUserRoleSQL, pgx.NamedArgs{
			"user_id": in.UserID, "role_code": in.RoleCode, "granted_by": in.OperatorID, "grant_reason": in.Reason,
		}))
		if _, dup := PgErrorWithCode(err, PgUniqueViolation); dup {
			return fmt.Errorf("%w: %s already holds %s", ErrConflict, in.UserID, in.RoleCode)
		}
		if err != nil {
			return fmt.Errorf("insert user role: %w", err)
		}
		ev, err = roleAuditTx(ctx, tx, user, in, EventUserRoleGranted)
		return err
	})
	return role, ev, err
}

// RevokeUserRole stamps the revocation and writes USER_ROLE_REVOKED; revoking
// the last holder of ADMIN is refused (ErrInvalidState).
func (r *PostgresRepository) RevokeUserRole(ctx context.Context, in RoleChangeInput) (*UserRole, *AuditEvent, error) {
	var role *UserRole
	var ev *AuditEvent
	err := InTx(ctx, r.pool, "revoke user role", func(tx pgx.Tx) error {
		user, err := lockRoleChangeTx(ctx, tx, in)
		if err != nil {
			return err
		}
		role, err = collectUserRole(tx.Query(ctx, revokeUserRoleSQL, pgx.NamedArgs{
			"user_id": in.UserID, "role_code": in.RoleCode, "revoked_by": in.OperatorID, "revoke_reason": in.Reason,
		}))
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %s does not hold %s", ErrNotFound, in.UserID, in.RoleCode)
		}
		if err != nil {
			return fmt.Errorf("revoke user role: %w", err)
		}
		if err := ensureAdministratorLeftTx(ctx, tx, in.RoleCode); err != nil {
			return err
		}
		ev, err = roleAuditTx(ctx, tx, user, in, EventUserRoleRevoked)
		return err
	})
	return role, ev, err
}

// ensureAdministratorLeftTx refuses (and so rolls back) a revocation of ADMIN
// that leaves no administrator; the role lock serializes concurrent revocations.
func ensureAdministratorLeftTx(ctx context.Context, tx pgx.Tx, roleCode string) error {
	if roleCode != RoleAdmin {
		return nil
	}
	var holders int
	if err := tx.QueryRow(ctx, countRoleHoldersSQL, pgx.NamedArgs{"role_code": RoleAdmin}).Scan(&holders); err != nil {
		return fmt.Errorf("count administrators: %w", err)
	}
	if holders == 0 {
		return fmt.Errorf("%w: the last administrator cannot be revoked; grant ADMIN to another user first", ErrInvalidState)
	}
	return nil
}

// lockRoleChangeTx serializes the changes of the role and locks the user,
// which must be known (ErrNotFound).
func lockRoleChangeTx(ctx context.Context, tx pgx.Tx, in RoleChangeInput) (*AppUser, error) {
	if _, err := tx.Exec(ctx, lockRoleSQL, pgx.NamedArgs{"role_code": in.RoleCode}); err != nil {
		return nil, fmt.Errorf("lock role: %w", err)
	}
	user, err := collectAppUser(tx.Query(ctx, getAppUserForUpdateSQL, pgx.NamedArgs{"user_id": in.UserID}))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: user %s has never signed in", ErrNotFound, in.UserID)
	}
	if err != nil {
		return nil, fmt.Errorf("get app_user: %w", err)
	}
	return user, nil
}

// roleAuditTx writes the role event on the user's USER subject.
func roleAuditTx(ctx context.Context, tx pgx.Tx, user *AppUser, in RoleChangeInput, eventType string) (*AuditEvent, error) {
	state := map[string]any{"role": in.RoleCode, "held": eventType == EventUserRoleGranted}
	ev, err := InsertAuditEventTx(ctx, tx, AuditEvent{
		SubjectID:   user.SubjectID,
		EventType:   eventType,
		ActorUserID: in.OperatorID,
		AfterState:  state,
		Reason:      in.Reason,
	})
	if err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	return ev, nil
}

// collectUserRole reads exactly one assignment row.
func collectUserRole(rows pgx.Rows, err error) (*UserRole, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[UserRole])
}
