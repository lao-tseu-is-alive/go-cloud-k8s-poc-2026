package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL SQLSTATE codes the domains translate into domain errors.
const (
	// PgUniqueViolation is unique_violation (a unique index or constraint).
	PgUniqueViolation = "23505"
	// PgForeignKeyViolation is foreign_key_violation (an unknown referenced row).
	PgForeignKeyViolation = "23503"
	// PgCheckViolation is check_violation (a CHECK constraint or a guard trigger).
	PgCheckViolation = "23514"
)

// TxBeginner starts a transaction; *pgxpool.Pool and pgx.Tx satisfy it.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// InTx runs fn in a transaction and commits it when fn succeeds; otherwise the
// transaction is rolled back and fn's error returned unchanged. op names the
// operation in begin/commit errors ("create task").
func InTx(ctx context.Context, db TxBeginner, op string, fn func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", op, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", op, err)
	}
	return nil
}

// PgErrorWithCode returns the PostgreSQL error wrapped in err when its SQLSTATE
// is code.
func PgErrorWithCode(err error, code string) (*pgconn.PgError, bool) {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok || pgErr.Code != code {
		return nil, false
	}
	return pgErr, true
}

// MapDBError is the shared base of the domains' error translation: nil stays
// nil, pgx.ErrNoRows becomes ErrNotFound, and a PostgreSQL error is handed to
// translate, whose non-nil result replaces it (each domain keeps its own
// messages). Any other error, or a nil translation, is returned unchanged.
func MapDBError(err error, translate func(*pgconn.PgError) error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && translate != nil {
		if mapped := translate(pgErr); mapped != nil {
			return mapped
		}
	}
	return err
}
