package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
)

// Separation of migrations and runtime (GLD-046). By default the server
// migrates at startup (GOELAND_DB_MIGRATE=auto), convenient on a workstation.
// In production the schema owner runs `goeland-server migrate` (a Kubernetes
// init container or job) and the server, started with GOELAND_DB_MIGRATE=verify
// as a role limited to rows, only checks that the schema is up to date.

const (
	// migrateAuto applies the pending migrations at startup.
	migrateAuto = "auto"
	// migrateVerify only checks that no migration is pending.
	migrateVerify = "verify"
)

// dbMigrateFromEnv reads GOELAND_DB_MIGRATE: auto (default) or verify.
func dbMigrateFromEnv() (string, error) {
	mode := strings.ToLower(strings.TrimSpace(envOrDefault("GOELAND_DB_MIGRATE", migrateAuto)))
	if mode != migrateAuto && mode != migrateVerify {
		return "", fmt.Errorf("GOELAND_DB_MIGRATE must be auto or verify")
	}
	return mode, nil
}

// openPool opens the pgx pool of the server or of the migrate command.
func openPool(ctx context.Context, databaseURL string, maxConns int32) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	poolConfig.MaxConns = maxConns
	// JIT compilation costs ~100-150 ms on the capped search queries, whose
	// cost estimates cross jit_above_cost while they run in tens of
	// milliseconds (GLD-053): interactive requests never gain from it.
	poolConfig.ConnConfig.RuntimeParams["jit"] = "off"
	// The searches use optional filters (@query = '' OR ...): after five runs of
	// a prepared statement PostgreSQL may switch to a generic plan, blind to the
	// values, which took a text search from 0.2 s to 2.3 s (GLD-053).
	poolConfig.ConnConfig.RuntimeParams["plan_cache_mode"] = "force_custom_plan"
	poolConfig.ConnConfig.Tracer = newQueryTracer() // nil when tracing is off (GLD-010)
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	return pool, nil
}

// prepareSchema migrates (auto) or checks that nothing is pending (verify);
// in verify mode it also warns when the runtime role could alter the schema.
func prepareSchema(ctx context.Context, mode string, pool *pgxpool.Pool, log *slog.Logger) error {
	if mode != migrateVerify {
		// The core module owns the full schema bootstrap (core + document tables + seed).
		return coremodule.Migrate(ctx, pool)
	}
	pending, err := coremodule.PendingMigrations(ctx, pool)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return fmt.Errorf("database schema is behind this binary: %d migration(s) pending, from %s; run `goeland-server migrate` as the schema owner", len(pending), pending[0])
	}
	canAlter, err := coremodule.CanAlterSchema(ctx, pool)
	if err != nil {
		return err
	}
	if canAlter {
		log.Warn("the runtime database role owns the schema or is a superuser: it could alter it or drop the audit triggers; run the server as a role granted by `goeland-server migrate` (GOELAND_DB_APP_ROLE)")
	}
	log.Info("database schema up to date (GOELAND_DB_MIGRATE=verify)")
	return nil
}

// migrateConfig is what `goeland-server migrate` reads from the environment.
type migrateConfig struct {
	// DatabaseURL connects as the schema owner; it carries a password and must never be logged.
	DatabaseURL string
	// AppRole is the runtime role granted after the migrations (GOELAND_DB_APP_ROLE); empty grants nothing.
	AppRole string
	// LogLevel is the slog level parsed from LOG_LEVEL.
	LogLevel slog.Level
	// limits carries the database wait (GOELAND_DB_CONNECT_TIMEOUT_SECONDS).
	limits serverConfig
}

// loadMigrateConfig reads the database settings of the migrate command; it
// needs no authentication or listener settings.
func loadMigrateConfig() (migrateConfig, error) {
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return migrateConfig{}, err
	}
	logLevel, err := parseLogLevel(envOrDefault("LOG_LEVEL", "info"))
	if err != nil {
		return migrateConfig{}, err
	}
	appRole := strings.TrimSpace(os.Getenv("GOELAND_DB_APP_ROLE"))
	if appRole != "" && !coremodule.ValidRoleName(appRole) {
		return migrateConfig{}, fmt.Errorf("GOELAND_DB_APP_ROLE must be a lower-case PostgreSQL role name")
	}
	cfg := migrateConfig{DatabaseURL: databaseURL, AppRole: appRole, LogLevel: logLevel}
	if err := cfg.limits.readLimits(); err != nil {
		return migrateConfig{}, err
	}
	return cfg, nil
}

// runMigrate applies the pending migrations as the schema owner, then grants
// the runtime role its privileges, and returns; the server is not started.
func runMigrate(ctx context.Context, cfg migrateConfig, log *slog.Logger) error {
	pool, err := openPool(ctx, cfg.DatabaseURL, 2)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := waitForDatabase(ctx, pool.Ping, cfg.limits.DBConnectTimeout, log); err != nil {
		return err
	}
	if err := coremodule.Migrate(ctx, pool); err != nil {
		return err
	}
	log.Info("migrations applied")
	if cfg.AppRole == "" {
		log.Warn("GOELAND_DB_APP_ROLE is not set: no runtime role was granted")
		return nil
	}
	if err := coremodule.GrantRuntimeRole(ctx, pool, cfg.AppRole); err != nil {
		return err
	}
	log.Info("runtime role granted", "role", cfg.AppRole, "deletable_tables", coremodule.RuntimeDeletableTables)
	return nil
}
