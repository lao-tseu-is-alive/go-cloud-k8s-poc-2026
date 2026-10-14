// Package module provides an importable, bundleable Module for the Goéland case export.
//
// The case_export table is bootstrapped by the core module migrations. The
// module wires the export service over the read services of the other domains
// and runs the background worker that builds and expires the archives.
package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/caseexport"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds case-export-module-specific configuration.
type Config struct {
	// RequestTimeout is the per-RPC deadline enforced by the timeout interceptor. Defaults to 10s.
	RequestTimeout time.Duration
	// Worker tunes the archive worker (time to live, poll interval).
	Worker caseexport.Config
}

func (c Config) requestTimeout() time.Duration {
	if c.RequestTimeout <= 0 {
		return defaultRequestTimeout
	}
	return c.RequestTimeout
}

// Deps holds cross-cutting dependencies.
type Deps struct {
	// Pool is the shared PostgreSQL pool; required.
	Pool *pgxpool.Pool
	// Verifier authenticates bearer tokens for the auth interceptor; required.
	Verifier authadapter.TokenVerifier
	// BlobStore holds the document bytes and the built archives; required.
	BlobStore blobstore.Store
	// Sources are the read services an archive is assembled from; all required.
	Sources caseexport.Sources
	// Logger receives module logs; nil falls back to slog.Default.
	Logger *slog.Logger
}

// Module encapsulates the case export: service, worker and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	service *caseexport.Service
	connect *caseexport.ConnectServer
}

// New creates a fully wired case export Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("case export module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("case export module: token verifier is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	svc, err := caseexport.NewService(deps.Pool, deps.BlobStore, deps.Sources, cfg.Worker, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("case export module: service init: %w", err)
	}
	cs, err := caseexport.NewConnectServer(svc, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("case export module: connect server init: %w", err)
	}
	return &Module{cfg: cfg, deps: deps, service: svc, connect: cs}, nil
}

// Service exposes the case export service (download handler, tests).
func (m *Module) Service() *caseexport.Service { return m.service }

// Run runs the archive worker until ctx is cancelled.
func (m *Module) Run(ctx context.Context) { m.service.Run(ctx) }
