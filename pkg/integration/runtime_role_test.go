package integration

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/coretest"
	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
)

// TestRuntimeRoleIsLimitedToRows covers GLD-046: the role granted by
// GrantRuntimeRole reads and writes rows, deletes only from the listed tables,
// and can neither change the schema, nor touch the immutability triggers, nor
// record a migration; the schema check sees it up to date and not an owner.
func TestRuntimeRoleIsLimitedToRows(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	dsn := coretest.TestDatabaseURL(t, "DB integration tests")
	runtime, err := pgxpool.New(ctx, coretest.RuntimeRoleURL(t, env.pool, dsn))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)

	allowed := []string{
		`SELECT count(*) FROM audit_event`,
		`UPDATE case_file SET title = title WHERE false`,
		`DELETE FROM outbox_event WHERE false`,
		`DELETE FROM case_type_default_grant WHERE false`,
		`SELECT pg_notify('goeland_test', '')`,
	}
	for _, sql := range allowed {
		if _, err := runtime.Exec(ctx, sql); err != nil {
			t.Errorf("runtime role should run %q: %v", sql, err)
		}
	}
	refused := []string{
		`DELETE FROM case_file WHERE false`,
		`DELETE FROM audit_event WHERE false`,
		`TRUNCATE outbox_event`,
		`ALTER TABLE audit_event DISABLE TRIGGER ALL`,
		`DROP TRIGGER trg_audit_event_append_only ON audit_event`,
		`CREATE TABLE goeland_rt_probe (i int)`,
		`INSERT INTO schema_migrations (version) VALUES ('9999')`,
	}
	for _, sql := range refused {
		_, err := runtime.Exec(ctx, sql)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("runtime role must be refused %q (insufficient_privilege), got %v", sql, err)
		}
	}

	if pending, err := coremodule.PendingMigrations(ctx, runtime); err != nil || len(pending) != 0 {
		t.Fatalf("the runtime role sees an up-to-date schema: %v (%v)", pending, err)
	}
	if can, err := coremodule.CanAlterSchema(ctx, runtime); err != nil || can {
		t.Fatalf("the runtime role cannot alter the schema: %v (%v)", can, err)
	}
	if can, err := coremodule.CanAlterSchema(ctx, env.pool); err != nil || !can {
		t.Fatalf("the test owner can: %v (%v)", can, err)
	}
	if err := coremodule.GrantRuntimeRole(ctx, env.pool, "goeland_rt_missing_role"); err == nil {
		t.Fatal("granting a role that does not exist fails")
	}
}
