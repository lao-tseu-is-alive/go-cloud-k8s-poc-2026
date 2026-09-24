package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RecordUser creates the user (USER subject, governance, app_user row and a
// USER_REGISTERED audit event) on first sight, updates it with a
// USER_PROFILE_UPDATED event when the profile changed, and otherwise only
// refreshes last_seen_at — all in one transaction.
func (r *PostgresRepository) RecordUser(ctx context.Context, profile UserProfile) (*AppUser, error) {
	if profile.UserID == "" {
		return nil, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin record user: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, lockAppUserSQL, pgx.NamedArgs{"user_id": profile.UserID}); err != nil {
		return nil, fmt.Errorf("lock app_user: %w", err)
	}
	current, err := collectAppUser(tx.Query(ctx, getAppUserForUpdateSQL, pgx.NamedArgs{"user_id": profile.UserID}))
	var user *AppUser
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		user, err = registerUserTx(ctx, tx, profile)
	case err != nil:
		return nil, fmt.Errorf("get app_user: %w", err)
	default:
		user, err = refreshUserTx(ctx, tx, current, profile)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit record user: %w", err)
	}
	return user, nil
}

// registerUserTx creates the USER subject, its governance and the app_user row.
func registerUserTx(ctx context.Context, q Querier, profile UserProfile) (*AppUser, error) {
	ref, err := InsertSubjectRefTx(ctx, q, SubjectKindUser, profile.label(), "")
	if err != nil {
		return nil, fmt.Errorf("insert user subject: %w", err)
	}
	gov := CreateSubjectInput{Kind: SubjectKindUser, DisplayLabel: ref.DisplayLabel, OperatorID: profile.UserID, OwnerUserID: profile.UserID}
	if _, err := InsertRecordMetadataTx(ctx, q, gov, ref.ID); err != nil {
		return nil, fmt.Errorf("insert user record_metadata: %w", err)
	}
	user, err := collectAppUser(q.Query(ctx, insertAppUserSQL, pgx.NamedArgs{
		"user_id":      profile.UserID,
		"subject_id":   ref.ID,
		"display_name": profile.DisplayName,
		"email":        profile.Email,
		"is_admin":     profile.IsAdmin,
	}))
	if err != nil {
		return nil, fmt.Errorf("insert app_user: %w", err)
	}
	if _, err := InsertAuditEventTx(ctx, q, AuditEvent{
		SubjectID:   ref.ID,
		EventType:   "USER_REGISTERED",
		ActorUserID: profile.UserID,
		AfterState:  userAuditState(user.DisplayName, user.IsAdmin),
	}); err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	return user, nil
}

// refreshUserTx updates a known user; a changed profile is audited and renames
// the USER subject, an unchanged one only refreshes last_seen_at.
func refreshUserTx(ctx context.Context, q Querier, current *AppUser, profile UserProfile) (*AppUser, error) {
	user, err := collectAppUser(q.Query(ctx, updateAppUserSQL, pgx.NamedArgs{
		"user_id":      profile.UserID,
		"display_name": profile.DisplayName,
		"email":        profile.Email,
		"is_admin":     profile.IsAdmin,
	}))
	if err != nil {
		return nil, fmt.Errorf("update app_user: %w", err)
	}
	if profile.sameAs(current) {
		return user, nil
	}
	if err := UpdateSubjectLabelTx(ctx, q, user.SubjectID, profile.label()); err != nil {
		return nil, fmt.Errorf("rename user subject: %w", err)
	}
	after := userAuditState(user.DisplayName, user.IsAdmin)
	after["email_changed"] = current.Email != user.Email
	if _, err := InsertAuditEventTx(ctx, q, AuditEvent{
		SubjectID:   user.SubjectID,
		EventType:   "USER_PROFILE_UPDATED",
		ActorUserID: profile.UserID,
		BeforeState: userAuditState(current.DisplayName, current.IsAdmin),
		AfterState:  after,
	}); err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	return user, nil
}

// userAuditState is the audited part of a profile; the e-mail address itself
// is kept out of the append-only log.
func userAuditState(displayName string, isAdmin bool) map[string]any {
	return map[string]any{"display_name": displayName, "is_admin": isAdmin}
}

// GetUsers returns the known users among userIDs, ordered by id; unknown ids
// are simply absent.
func (r *PostgresRepository) GetUsers(ctx context.Context, userIDs []string) ([]*AppUser, error) {
	rows, err := r.pool.Query(ctx, getAppUsersSQL, pgx.NamedArgs{"user_ids": userIDs})
	if err != nil {
		return nil, fmt.Errorf("get app users: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[AppUser])
}

// collectAppUser reads exactly one app_user row from a Query result.
func collectAppUser(rows pgx.Rows, err error) (*AppUser, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[AppUser])
}
