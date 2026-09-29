// Package module provides an importable, bundleable Module for the Goéland circulation domain.
//
// The circulation schema (case_circulation, case_circulation_recipient) is
// bootstrapped by the core module migrations because the circulation tables have
// foreign keys into the case, task and timeline tables. This module therefore has no
// migrations of its own; it wires the circulation repository/service/handler on
// top of a shared pool.
package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/circulation"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds circulation-module-specific configuration.
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

// Deps holds cross-cutting dependencies. The circulation domain reuses the core
// primitives through the exported transaction helpers, so no core service is needed.
type Deps struct {
	// Pool is the shared PostgreSQL pool; required.
	Pool *pgxpool.Pool
	// Verifier authenticates bearer tokens for the auth interceptor; required.
	Verifier authadapter.TokenVerifier
	// Logger receives module logs; nil falls back to slog.Default.
	Logger *slog.Logger
}

// Module encapsulates the circulation domain: repository, service and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	service *circulation.Service
	connect *circulation.ConnectServer
}

// New creates a fully wired circulation Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("circulation module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("circulation module: token verifier is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	repo, err := circulation.NewPostgresRepository(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("circulation module: storage init: %w", err)
	}
	svc, err := circulation.NewService(repo, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("circulation module: service init: %w", err)
	}
	cs, err := circulation.NewConnectServer(svc, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("circulation module: connect server init: %w", err)
	}

	return &Module{cfg: cfg, deps: deps, service: svc, connect: cs}, nil
}

// Service exposes the circulation service (used by tests and cross-module composition).
func (m *Module) Service() *circulation.Service { return m.service }

// Start is a placeholder for future background workers.
func (m *Module) Start(_ context.Context) error { return nil }

// Stop is a placeholder for graceful shutdown of future background workers.
func (m *Module) Stop(_ context.Context) error { return nil }
