// Package module provides an importable, bundleable Module for the Goéland thing domain.
//
// The thing schema (thing_type, thing, thing_parcel, thing_building) is bootstrapped by
// the core module migrations because the thing tables have foreign keys into the core
// tables. This module therefore has no migrations of its own; it wires the thing
// repository/service/handler on top of a shared pool and the core service.
package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/thing"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds thing-module-specific configuration.
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

// Deps holds cross-cutting dependencies. CoreService is required because the
// thing domain reuses core primitives (subjects, relationships, audit).
type Deps struct {
	// Pool is the shared PostgreSQL pool; required.
	Pool *pgxpool.Pool
	// Verifier authenticates bearer tokens for the auth interceptor; required.
	Verifier authadapter.TokenVerifier
	// CoreService is the core domain service used to read relationships and
	// audit; required.
	CoreService *core.Service
	// Logger receives module logs; nil falls back to slog.Default.
	Logger *slog.Logger
}

// Module encapsulates the thing domain: repository, service and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	service *thing.Service
	connect *thing.ConnectServer
}

// New creates a fully wired thing Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("thing module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("thing module: token verifier is required")
	}
	if deps.CoreService == nil {
		return nil, fmt.Errorf("thing module: core service is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	repo, err := thing.NewPostgresRepository(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("thing module: storage init: %w", err)
	}
	svc, err := thing.NewService(repo, deps.CoreService, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("thing module: service init: %w", err)
	}
	cs, err := thing.NewConnectServer(svc, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("thing module: connect server init: %w", err)
	}

	return &Module{cfg: cfg, deps: deps, service: svc, connect: cs}, nil
}

// Service exposes the thing service (used by tests and cross-module composition).
func (m *Module) Service() *thing.Service { return m.service }

// Start is a placeholder for future background workers.
func (m *Module) Start(_ context.Context) error { return nil }

// Stop is a placeholder for graceful shutdown of future background workers.
func (m *Module) Stop(_ context.Context) error { return nil }
