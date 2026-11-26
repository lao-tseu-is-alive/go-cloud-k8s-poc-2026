package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/coretest"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/demodata"
)

// TestDemoSeed generates the small demo profile in every language, each in a
// brand-new database, as the runtime role (GLD-079): every request of the
// generator must pass the API's validation and authorization. A second run on
// the same database is refused.
func TestDemoSeed(t *testing.T) {
	adminDSN := coretest.TestDatabaseURL(t, "demo-seed test")
	for _, lang := range demodata.Languages() {
		t.Run(lang, func(t *testing.T) {
			ownerDSN := freshDatabase(t, adminDSN)
			cfg := e2eBlobStore(t)
			cfg.DatabaseURL, cfg.DBMigrate = runtimeDatabaseURL(t, ownerDSN), migrateVerify
			cfg.demoOwnerURL = ownerDSN
			cfg.MaxConnections, cfg.RequestTimeout, cfg.MaxUploadBytes, cfg.ShutdownPeriod = 4, 10*time.Second, 1<<20, time.Second
			quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
			demoCfg := cfg
			demoCfg.AuthMode = authModeDemo
			if _, err := newApplication(t.Context(), demoCfg, quiet); !errors.Is(err, errNotDemoDatabase) {
				t.Fatalf("the demo mode must refuse a database demo-seed did not generate, got %v", err)
			}

			report, err := runDemoSeed(t.Context(), cfg, demodata.Options{Profile: demodata.ProfileSmall, Language: lang}, quiet)
			if err != nil {
				t.Fatalf("demo-seed: %v", err)
			}
			if report.Counts["cases"] != 3 || report.Counts["users"] != len(demodata.Users()) {
				t.Errorf("counts: %v", report.Counts)
			}
			checkDemoDatabase(t, ownerDSN, lang)
			checkStoryDates(t, ownerDSN)
			checkDemoMode(t, demoCfg, lang)

			if _, err := runDemoSeed(t.Context(), cfg, demodata.Options{Profile: demodata.ProfileSmall, Language: lang}, quiet); !errors.Is(err, errNotBrandNew) {
				t.Errorf("a second run must be refused, got %v", err)
			}
		})
	}
}

// checkDemoMode serves the generated demo in demo mode: GET /config lists the
// demo users with their public tokens, each token signs in as its user, and
// only Alice is an administrator (GLD-085).
func checkDemoMode(t *testing.T, cfg serverConfig, lang string) {
	t.Helper()
	app, err := newApplication(t.Context(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("demo mode on the generated demo: %v", err)
	}
	srv := httptest.NewServer(app.handler)
	defer func() { srv.Close(); app.close() }()
	e := &e2e{t: t}
	config := e.call(srv, "", "GET", "/config", nil)
	if str(config.body, "authMode") != authModeDemo || str(config.body, "demo", "language") != lang {
		t.Fatalf("/config in demo mode: %v", config.body)
	}
	users := list(config.body, "demo", "users")
	if len(users) != len(demodata.Users()) {
		t.Fatalf("/config lists %d demo users", len(users))
	}
	for _, u := range users {
		me := e.call(srv, str(u, "token"), "GET", "/api/me", nil)
		admin, _ := me.body["user"].(map[string]any)["isAdmin"].(bool)
		if me.status != 200 || str(me.body, "user", "id") != str(u, "id") || admin != (str(u, "key") == "alice") {
			t.Errorf("sign in as %s: HTTP %d %v", str(u, "key"), me.status, me.body)
		}
	}
	e.expect(e.call(srv, "not-a-demo-token", "GET", "/api/me", nil), codeUnauthenticated, "an unknown token")
}

// checkStoryDates checks the re-dated history: each case opens and closes on
// its story days with the reference its story gives, the audit follows, the
// reference counters continue after them, and the audit is immutable again.
func checkStoryDates(t *testing.T, dsn string) {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	rows, err := pool.Query(t.Context(), `
		SELECT s.business_ref_namespace || ' ' || s.business_ref,
		       to_char(c.opened_at AT TIME ZONE 'Europe/Zurich', 'YYYY-MM-DD'),
		       to_char(c.closed_at AT TIME ZONE 'Europe/Zurich', 'YYYY-MM-DD'),
		       (SELECT to_char(min(a.occurred_at) AT TIME ZONE 'Europe/Zurich', 'YYYY-MM-DD') FROM audit_event a WHERE a.subject_id = c.id)
		  FROM case_file c JOIN subject_ref s ON s.id = c.id ORDER BY 1`)
	if err != nil {
		t.Fatalf("read the cases: %v", err)
	}
	got, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var ref, opened, closed, audited string
		err := row.Scan(&ref, &opened, &closed, &audited)
		return ref + " " + opened + " " + closed + " " + audited, err
	})
	want := []string{
		"OPC 2025-000031 2025-03-03 2025-05-21 2025-03-03",
		"OPP 2025-000007 2025-04-11 2025-05-21 2025-04-11",
		"SIG 2026-000014 2026-02-09 2026-02-13 2026-02-09",
	}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("story dates: %v (%v), want %v", got, err, want)
	}
	var next int64
	if err := pool.QueryRow(t.Context(), `SELECT last_value FROM business_ref_counter WHERE namespace = 'OPC' AND period = '2025'`).Scan(&next); err != nil || next != 31 {
		t.Errorf("the OPC 2025 counter continues after the stories: %d (%v)", next, err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE audit_event SET occurred_at = now()`); err == nil {
		t.Error("the audit must be immutable again after the re-dating")
	}
}

// checkDemoDatabase reads back what the generator wrote.
func checkDemoDatabase(t *testing.T, dsn, lang string) {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	var closed, versions, admins, records int
	var namespaces string
	row := pool.QueryRow(t.Context(), `
		SELECT (SELECT count(*) FROM case_file WHERE closed_at IS NOT NULL),
		       (SELECT count(*) FROM document_version),
		       (SELECT count(*) FROM app_user_role WHERE role_code = 'ADMIN' AND revoked_at IS NULL),
		       (SELECT count(*) FROM app_user_role WHERE role_code = 'RECORDS_MANAGER' AND revoked_at IS NULL),
		       (SELECT string_agg(DISTINCT s.business_ref_namespace, ',' ORDER BY s.business_ref_namespace)
		          FROM subject_ref s WHERE s.kind = 'CASE')`)
	if err := row.Scan(&closed, &versions, &admins, &records, &namespaces); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if closed != 3 || admins != 1 || records != 1 || namespaces != "OPC,OPP,SIG" {
		t.Errorf("closed=%d admins=%d records=%d namespaces=%q", closed, admins, records, namespaces)
	}
	if versions != 13 { // 12 documents, the plan in two versions
		t.Errorf("document versions: %d", versions)
	}
	var label string
	if err := pool.QueryRow(t.Context(), `SELECT label FROM case_type WHERE code = 'CITIZEN_REPORT'`).Scan(&label); err != nil {
		t.Fatalf("read case type: %v", err)
	}
	if want := map[string]string{"fr": "Signalement citoyen", "en": "Citizen report"}[lang]; label != want {
		t.Errorf("case type label in %s: %q", lang, label)
	}
}

// freshDatabase creates an empty database next to the test database (the test
// role needs CREATEDB, as the disposable test servers have) and drops it after
// the test; it returns its DSN.
func freshDatabase(t *testing.T, adminDSN string) string {
	t.Helper()
	name := "goeland_demo_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	admin, err := pgx.Connect(t.Context(), adminDSN)
	if err != nil {
		t.Fatalf("connect to the test database: %v", err)
	}
	if _, err := admin.Exec(t.Context(), "CREATE DATABASE "+name); err != nil {
		_ = admin.Close(context.Background())
		t.Fatalf("create a fresh database (the test role needs CREATEDB): %v", err)
	}
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
		_ = admin.Close(ctx)
	})
	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse the test DSN: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}
