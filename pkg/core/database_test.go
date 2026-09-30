package core

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapDBError(t *testing.T) {
	unique := fmt.Errorf("insert: %w", &pgconn.PgError{Code: PgUniqueViolation, ConstraintName: "idx_x"})
	translate := func(pgErr *pgconn.PgError) error {
		if pgErr.Code == PgUniqueViolation {
			return fmt.Errorf("%w: duplicate (%s)", ErrConflict, pgErr.ConstraintName)
		}
		return nil
	}
	other := errors.New("network down")
	check := &pgconn.PgError{Code: PgCheckViolation}
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"nil stays nil", nil, nil},
		{"no rows is not found", fmt.Errorf("get: %w", pgx.ErrNoRows), ErrNotFound},
		{"translated code", unique, ErrConflict},
		{"untranslated code unchanged", check, check},
		{"other error unchanged", other, other},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MapDBError(c.err, translate)
			if (c.want == nil) != (got == nil) || (c.want != nil && !errors.Is(got, c.want)) {
				t.Fatalf("MapDBError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
	if got := MapDBError(unique, nil); !errors.Is(got, unique) {
		t.Fatalf("a nil translation keeps the PostgreSQL error, got %v", got)
	}
}

func TestPgErrorWithCode(t *testing.T) {
	wrapped := fmt.Errorf("save: %w", &pgconn.PgError{Code: PgForeignKeyViolation, ConstraintName: "fk_x"})
	if pgErr, ok := PgErrorWithCode(wrapped, PgForeignKeyViolation); !ok || pgErr.ConstraintName != "fk_x" {
		t.Fatalf("a wrapped matching error is found: %v %v", pgErr, ok)
	}
	if _, ok := PgErrorWithCode(wrapped, PgUniqueViolation); ok {
		t.Fatal("another code does not match")
	}
	if _, ok := PgErrorWithCode(errors.New("plain"), PgUniqueViolation); ok {
		t.Fatal("a non-PostgreSQL error does not match")
	}
}

// fakeTx records how a transaction ended; the embedded interface panics on
// any other call, which the tests never make.
type fakeTx struct {
	pgx.Tx
	committed, rolledBack bool
	commitErr             error
}

func (f *fakeTx) Commit(context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

type fakeBeginner struct {
	tx  *fakeTx
	err error
}

func (b fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	if b.err != nil {
		return nil, b.err
	}
	return b.tx, nil
}

func TestInTx(t *testing.T) {
	ctx := context.Background()
	ok := &fakeTx{}
	if err := InTx(ctx, fakeBeginner{tx: ok}, "op", func(pgx.Tx) error { return nil }); err != nil || !ok.committed {
		t.Fatalf("success commits: %v %+v", err, ok)
	}

	failed, boom := &fakeTx{}, errors.New("boom")
	if err := InTx(ctx, fakeBeginner{tx: failed}, "op", func(pgx.Tx) error { return boom }); !errors.Is(err, boom) || failed.committed || !failed.rolledBack {
		t.Fatalf("a failing fn rolls back and returns its error unchanged: %v %+v", err, failed)
	}

	commitFails := &fakeTx{commitErr: errors.New("serialization")}
	if err := InTx(ctx, fakeBeginner{tx: commitFails}, "save task", func(pgx.Tx) error { return nil }); err == nil || err.Error() != "commit save task: serialization" {
		t.Fatalf("a commit error names the operation: %v", err)
	}

	if err := InTx(ctx, fakeBeginner{err: errors.New("pool closed")}, "save task", func(pgx.Tx) error { return nil }); err == nil || err.Error() != "begin save task: pool closed" {
		t.Fatalf("a begin error names the operation: %v", err)
	}
}
