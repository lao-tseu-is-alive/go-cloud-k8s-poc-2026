// Command goeland-import loads the POC from the legacy Goéland database, a
// read-only replica of production (GLD-051, GLD-052; docs/IMPORT_MAPPING.md).
//
// The target must be a brand-new database: the command migrates it, then
// imports every wave 1 stage in one transaction (pkg/legacyimport). Without
// -apply it is a dry run: everything is written, checked and rolled back. It
// prints counts only, never values. Use scripts/import_rebuild.sh, which
// recreates the target first:
//
//	GOELAND_IMPORT_SOURCE_URL='postgres://goeland_read:<password>@localhost/goeland' \
//	GOELAND_IMPORT_TARGET_URL='postgres://<user>:<password>@localhost/goeland_import' \
//	    go run ./cmd/goeland-import [-apply]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/legacyimport"
)

// importTimeout bounds the whole run (a full import takes minutes).
const importTimeout = 2 * time.Hour

func main() {
	apply := flag.Bool("apply", false, "commit the import (default: dry run, rolled back)")
	flag.Parse()
	if err := run(*apply); err != nil {
		fmt.Fprintln(os.Stderr, "goeland-import:", err)
		os.Exit(1)
	}
}

// run connects to both databases, migrates the target and imports.
func run(apply bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), importTimeout)
	defer cancel()
	source, err := connect(ctx, "GOELAND_IMPORT_SOURCE_URL")
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := connect(ctx, "GOELAND_IMPORT_TARGET_URL")
	if err != nil {
		return err
	}
	defer target.Close()
	if err := coremodule.Migrate(ctx, target); err != nil {
		return fmt.Errorf("migrate the target: %w", err)
	}
	started := time.Now()
	report, err := legacyimport.Run(ctx, source, target, legacyimport.Options{Apply: apply, StartedBy: startedBy()})
	if report != nil {
		report.Print(os.Stdout)
	}
	if err != nil {
		return err
	}
	mode := "dry run, rolled back"
	if apply {
		mode = "committed"
	}
	fmt.Printf("import %s in %s\n", mode, time.Since(started).Round(time.Second))
	return nil
}

// connect opens a pool on the DSN held by the environment variable env.
func connect(ctx context.Context, env string) (*pgxpool.Pool, error) {
	dsn := strings.TrimSpace(os.Getenv(env))
	if dsn == "" {
		return nil, errors.New(env + " is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", env, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping %s: %w", env, err)
	}
	return pool, nil
}

// startedBy names the operating-system user running the import.
func startedBy() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return "import:" + u.Username
	}
	return legacyimport.OperatorID
}
