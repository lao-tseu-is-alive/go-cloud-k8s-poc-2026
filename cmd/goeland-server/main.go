// Command goeland-server runs the Goéland POC as a single binary: it loads the
// configuration from the environment, connects to PostgreSQL, applies the
// embedded migrations under an advisory lock, wires the core, document, actor, case,
// thing, org unit, timeline, task and circulation modules onto one shared Vanguard transcoder (REST under /api/ plus Connect,
// gRPC and gRPC-Web), and serves the embedded Vue SPA at /. SIGINT and SIGTERM
// trigger a graceful shutdown bounded by GOELAND_SHUTDOWN_TIMEOUT_SECONDS.
//
// Usage:
//
//	goeland-server            run the server (configured by environment variables)
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
	"syscall"
	"time"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version"
)

// startupWorkBudget bounds the startup work after the database answered:
// migrations, module wiring and the first queries.
const startupWorkBudget = 60 * time.Second

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(versionLine())
		return
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "goeland-server configuration error:", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: config.LogLevel}))
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

	if err := app.serve(ctx, listener, config.ShutdownPeriod); err != nil {
		log.Error("goeland server stopped with error", "error", err)
		os.Exit(1)
	}
	log.Info("goeland server stopped")
}

// versionLine renders the build identity checked by make release-check.
func versionLine() string {
	return fmt.Sprintf("%s v%s (revision %s, built %s)", version.AppNameKebab, version.Version, version.Revision, version.BuildStamp)
}
