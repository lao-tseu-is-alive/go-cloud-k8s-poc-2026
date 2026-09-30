package integration

import (
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TestAuditLogsAreAppendOnly checks that the database itself refuses to rewrite,
// delete or truncate the audit trail and the reference change log (migration
// 0021, GLD-044), even for a direct SQL session of the application role.
func TestAuditLogsAreAppendOnly(t *testing.T) {
	env := newTestEnv(t)
	c := openCase(t, env, "Journal inaltérable "+uniqueToken())
	// A reference change of its own, so the reference_change statements touch a row.
	code := referenceCode("IT_APPEND")
	if _, _, err := env.caseSvc.CreateCaseType(env.ctx, casefile.CaseTypeInput{Code: code, Label: "Append-only", OperatorID: testOperator, Reason: "test"}); err != nil {
		t.Fatalf("create case type: %v", err)
	}
	statements := []string{
		`UPDATE audit_event SET reason = 'réécrit' WHERE subject_id = '` + c.ID.String() + `'`,
		`DELETE FROM audit_event WHERE subject_id = '` + c.ID.String() + `'`,
		`TRUNCATE audit_event`,
		`UPDATE reference_change SET reason = 'réécrit' WHERE code = '` + code + `'`,
		`DELETE FROM reference_change WHERE code = '` + code + `'`,
		`TRUNCATE reference_change`,
	}
	for _, stmt := range statements {
		err := pgx.BeginFunc(env.ctx, env.pool, func(tx pgx.Tx) error {
			_, err := tx.Exec(env.ctx, stmt)
			return err
		})
		if _, refused := core.PgErrorWithCode(err, "23000"); !refused { // integrity_constraint_violation
			t.Fatalf("%s: want the append-only refusal, got %v", stmt, err)
		}
	}
	events, err := env.coreSvc.ListAuditEvents(env.ctx, core.AuditFilter{SubjectID: c.ID, Limit: 10})
	if err != nil || len(events.Events) == 0 {
		t.Fatalf("the case audit trail is intact: %d events (%v)", len(events.Events), err)
	}
}
