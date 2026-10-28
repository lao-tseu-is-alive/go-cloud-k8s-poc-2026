// Package module provides an importable, bundleable Module for the Goéland retention domain.
//
// The retention schema (retention_policy, subject_retention, retention_hold) is
// bootstrapped by the core module migrations because the retention tables have
// foreign keys into the subject and case tables. This module therefore has no
// migrations of its own; it wires the retention repository/service/handler on
// top of a shared pool.
package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/retention"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds retention-module-specific configuration.
type Config struct {
	// RequestTimeout is the per-RPC deadline enforced by the timeout interceptor. Defaults to 10s.
	RequestTimeout time.Duration
}

func (c Config) requestTimeout() time.Duration {
	if c.RequestTimeout <= 0 {
		return defaultRequestTimeout
	}
	return c.RequestTimeout
}

// Deps holds cross-cutting dependencies. The retention domain reuses the core
// primitives through the exported transaction helpers, so no core service is needed.
type Deps struct {
	// Pool is the shared PostgreSQL pool; required.
	Pool *pgxpool.Pool
	// Verifier authenticates bearer tokens for the auth interceptor; required.
	Verifier authadapter.TokenVerifier
	// Logger receives module logs; nil falls back to slog.Default.
	Logger *slog.Logger
	// BlobStore holds the document bytes an elimination destroys (GLD-077);
	// nil refuses the execution of elimination bordereaux.
	BlobStore blobstore.Store
}

// Module encapsulates the retention domain: repository, service and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	service *retention.Service
	connect *retention.ConnectServer
}

// New creates a fully wired retention Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("retention module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("retention module: token verifier is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	repo, err := retention.NewPostgresRepository(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("retention module: storage init: %w", err)
	}
	repo.WithBlobStore(deps.BlobStore)
	svc, err := retention.NewService(repo, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("retention module: service init: %w", err)
	}
	authz, err := core.NewAuthorizer(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("retention module: authorizer init: %w", err)
	}
	cs, err := retention.NewConnectServer(svc, authz, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("retention module: connect server init: %w", err)
	}

	return &Module{cfg: cfg, deps: deps, service: svc, connect: cs}, nil
}

// Service exposes the retention service (used by tests and cross-module composition).
func (m *Module) Service() *retention.Service { return m.service }

// Start is a placeholder for future background workers.
func (m *Module) Start(_ context.Context) error { return nil }

// Stop is a placeholder for graceful shutdown of future background workers.
func (m *Module) Stop(_ context.Context) error { return nil }
