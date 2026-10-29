// Command goeland-server runs the Goéland POC as a single binary: it loads the
// configuration from the environment, connects to PostgreSQL, applies the
// embedded migrations under an advisory lock (or, with GOELAND_DB_MIGRATE=verify,
// only checks that none is pending), wires the core, document, actor, case,
// thing, org unit, timeline, task and circulation modules onto one shared Vanguard transcoder (REST under /api/ plus Connect,
// gRPC and gRPC-Web), and serves the embedded Vue SPA at /. SIGINT and SIGTERM
// trigger a graceful shutdown bounded by GOELAND_SHUTDOWN_TIMEOUT_SECONDS.
//
// Usage:
//
//	goeland-server            run the server (configured by environment variables)
//	goeland-server migrate    apply the migrations as the schema owner, grant the
//	                          runtime role GOELAND_DB_APP_ROLE, and exit (GLD-046)
//	goeland-server --version  print "<app> v<Version> (revision <rev>, built <stamp>)" and exit
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version"
)

// startupWorkBudget bounds the startup work after the database answered:
// migrations, module wiring and the first queries.
const startupWorkBudget = 60 * time.Second

// traceFlushTimeout bounds the export of the last spans at shutdown.
const traceFlushTimeout = 5 * time.Second

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(versionLine())
		return
	}
	switch args := flag.Args(); {
	case len(args) == 1 && args[0] == "migrate":
		os.Exit(migrateMain())
	case len(args) > 0:
		fmt.Fprintf(os.Stderr, "goeland-server: unknown command %q (usage: goeland-server [migrate] [--version])\n", strings.Join(args, " "))
		os.Exit(2)
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "goeland-server configuration error:", err)
		os.Exit(1)
	}
	log := slog.New(traceLogHandler{slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: config.LogLevel})})
	slog.SetDefault(log)
	log.Info("starting goeland server",
		"app", version.AppName,
		"version", version.Version,
		"revision", version.Revision,
		"build", version.BuildStamp,
		"listen", config.ListenAddress,
		"auth_mode", config.AuthMode,
	)

	// SIGINT / SIGTERM stop the server, and also a startup still waiting for its
	// database. Startup may take the database wait plus the migration budget.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := setupTracing(ctx, log)
	if err != nil {
		log.Error("failed to set up tracing", "error", err)
		os.Exit(1)
	}
	defer flushTraces(ctx, shutdownTracing, log)
	startupCtx, startupCancel := context.WithTimeout(ctx, config.DBConnectTimeout+startupWorkBudget)
	app, err := newApplication(startupCtx, config, log)
	startupCancel()
	if err != nil {
		log.Error("failed to initialize goeland server", "error", err)
		os.Exit(1)
	}
	defer app.close()

	listener, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		log.Error("failed to listen", "error", err)
		os.Exit(1)
	}
	log.Info("goeland server listening", "address", listener.Addr().String())
	metricsListener, err := listenMetrics(config.MetricsAddress)
	if err != nil {
		log.Error("failed to listen for metrics", "error", err)
		os.Exit(1)
	}
	if metricsListener != nil {
		log.Info("metrics listening", "address", metricsListener.Addr().String())
	}

	if err := app.serve(ctx, listener, metricsListener, config.ShutdownPeriod); err != nil {
		log.Error("goeland server stopped with error", "error", err)
		os.Exit(1)
	}
	log.Info("goeland server stopped")
}

// migrateMain runs `goeland-server migrate` and returns the exit code.
func migrateMain() int {
	cfg, err := loadMigrateConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "goeland-server migrate configuration error:", err)
		return 1
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	log.Info("goeland migrate", "app", version.AppName, "version", version.Version, "revision", version.Revision)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runMigrate(ctx, cfg, log); err != nil {
		log.Error("migration failed", "error", err)
		return 1
	}
	return 0
}

// versionLine renders the build identity checked by make release-check.
// flushTraces sends the spans still buffered before the process exits.
func flushTraces(ctx context.Context, shutdown func(context.Context) error, log *slog.Logger) {
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), traceFlushTimeout)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		log.Warn("failed to flush traces", "error", err)
	}
}

// listenMetrics opens the metrics listener, or returns nil when the metrics
// are off (GOELAND_METRICS_ADDRESS empty).
func listenMetrics(address string) (net.Listener, error) {
	if address == "" {
		return nil, nil
	}
	return net.Listen("tcp", address)
}

func versionLine() string {
	return fmt.Sprintf("%s v%s (revision %s, built %s)", version.AppNameKebab, version.Version, version.Revision, version.BuildStamp)
}
