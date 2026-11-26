package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/demodata"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version"
)

// demoSeedTimeout bounds a whole generation.
const demoSeedTimeout = 30 * time.Minute

// demoSeedBaseURL is the base URL of the in-process requests: nothing listens
// on it, the transport hands each request to the handler directly.
const demoSeedBaseURL = "http://demo-seed.invalid"

// errNotBrandNew refuses a database that already holds subjects.
var errNotBrandNew = errors.New("the database already holds data: demo-seed only writes into a brand-new database")

// demoSeedMain runs `goeland-server demo-seed` and returns the exit code: it
// generates the synthetic demo data (GLD-079, docs/DEMO_SCRIPT.md) through the
// API of a server built in this process for the demo users only, which never
// listens on a network.
func demoSeedMain(args []string) int {
	flags := flag.NewFlagSet("demo-seed", flag.ContinueOnError)
	profile := flags.String("profile", demodata.ProfileSmall, "amount of data: "+demodata.ProfileSmall)
	lang := flags.String("lang", "fr", "language of the generated texts: "+strings.Join(demodata.Languages(), ", "))
	if err := flags.Parse(args); err != nil {
		return 2
	}
	config, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "goeland-server demo-seed configuration error:", err)
		return 1
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: config.LogLevel}))
	log.Info("goeland demo-seed", "app", version.AppName, "version", version.Version, "profile", *profile, "lang", *lang)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, demoSeedTimeout)
	defer cancel()
	report, err := runDemoSeed(ctx, config, demodata.Options{Profile: *profile, Language: *lang}, log)
	if report.Counts != nil {
		report.Print(os.Stdout)
	}
	if err != nil {
		log.Error("demo-seed failed", "error", err)
		return 1
	}
	return 0
}

// runDemoSeed builds the application with the demo users' verifier and their
// administrator bootstrapped, checks that the database is brand new, and
// generates the data through the in-process API. The requests are logged at
// warning level only: thousands of them would drown the report.
func runDemoSeed(ctx context.Context, config serverConfig, opts demodata.Options, log *slog.Logger) (demodata.Report, error) {
	session, err := demodata.NewSession()
	if err != nil {
		return demodata.Report{}, err
	}
	config.tokenVerifier = session.Verifier()
	config.BootstrapAdmins = []string{demodata.AdminUserID()}
	quiet := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: max(config.LogLevel, slog.LevelWarn)}))
	app, err := newApplication(ctx, config, quiet)
	if err != nil {
		return demodata.Report{}, err
	}
	defer app.close()
	if err := ensureBrandNew(ctx, app.pool); err != nil {
		return demodata.Report{}, err
	}
	clients := session.NewClients(&http.Client{Transport: handlerTransport{app.handler}}, demoSeedBaseURL)
	started := time.Now()
	report, err := demodata.Generate(ctx, clients, opts)
	log.Info("demo data written", "duration", time.Since(started).Round(time.Millisecond))
	return report, err
}

// ensureBrandNew refuses a database holding any subject: the demo never mixes
// with other data, real or not.
func ensureBrandNew(ctx context.Context, pool *pgxpool.Pool) error {
	var used bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM subject_ref)`).Scan(&used); err != nil {
		return fmt.Errorf("check that the database is brand new: %w", err)
	}
	if used {
		return errNotBrandNew
	}
	return nil
}

// handlerTransport is an http.RoundTripper serving each request with a
// handler of this process, as a server would after reading it.
type handlerTransport struct{ handler http.Handler }

// RoundTrip serves req with the handler and returns the recorded response.
func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	in := req.Clone(req.Context())
	in.RequestURI = req.URL.RequestURI()
	in.RemoteAddr = "127.0.0.1:0"
	if in.Body == nil {
		in.Body = http.NoBody
	}
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, in)
	resp := recorder.Result()
	resp.Request = req
	return resp, nil
}
