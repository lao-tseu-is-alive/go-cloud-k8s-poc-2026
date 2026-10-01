// Package module provides an importable, bundleable Module for the Goéland access domain.
//
// The access schema (access_grant, security_group) is bootstrapped by the core
// module migrations because the access tables have foreign keys into the core
// tables. This module therefore has no migrations of its own; it wires the
// access repository/service/handler on top of a shared pool.
package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/access"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds access-module-specific configuration.
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

// Deps holds cross-cutting dependencies. The access domain reuses the core
// primitives through the exported transaction helpers, so no core service is needed.
type Deps struct {
	// Pool is the shared PostgreSQL pool; required.
	Pool *pgxpool.Pool
	// Verifier authenticates bearer tokens for the auth interceptor; required.
	Verifier authadapter.TokenVerifier
	// Logger receives module logs; nil falls back to slog.Default.
	Logger *slog.Logger
}

// Module encapsulates the access domain: repository, service and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	service *access.Service
	connect *access.ConnectServer
}

// New creates a fully wired access Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("access module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("access module: token verifier is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	repo, err := access.NewPostgresRepository(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("access module: storage init: %w", err)
	}
	svc, err := access.NewService(repo, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("access module: service init: %w", err)
	}
	cs, err := access.NewConnectServer(svc, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("access module: connect server init: %w", err)
	}

	return &Module{cfg: cfg, deps: deps, service: svc, connect: cs}, nil
}

// Service exposes the access service (used by tests and cross-module composition).
func (m *Module) Service() *access.Service { return m.service }

// Start is a placeholder for future background workers.
func (m *Module) Start(_ context.Context) error { return nil }

// Stop is a placeholder for graceful shutdown of future background workers.
func (m *Module) Stop(_ context.Context) error { return nil }
