// Package module provides an importable, bundleable Module for the Goéland task domain.
//
// The task schema (task_type, case_task, case_task_assignment) is
// bootstrapped by the core module migrations because the task tables have
// foreign keys into the case, user and org unit tables. This module therefore has no
// migrations of its own; it wires the task repository/service/handler on
// top of a shared pool.
package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds task-module-specific configuration.
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

// Deps holds cross-cutting dependencies. The task domain reuses the core
// primitives through the exported transaction helpers, so no core service is needed.
type Deps struct {
	// Pool is the shared PostgreSQL pool; required.
	Pool *pgxpool.Pool
	// Verifier authenticates bearer tokens for the auth interceptor; required.
	Verifier authadapter.TokenVerifier
	// Logger receives module logs; nil falls back to slog.Default.
	Logger *slog.Logger
}

// Module encapsulates the task domain: repository, service and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	service *task.Service
	connect *task.ConnectServer
}

// New creates a fully wired task Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("task module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("task module: token verifier is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	repo, err := task.NewPostgresRepository(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("task module: storage init: %w", err)
	}
	svc, err := task.NewService(repo, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("task module: service init: %w", err)
	}
	authz, err := core.NewAuthorizer(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("task module: authorizer init: %w", err)
	}
	cs, err := task.NewConnectServer(svc, authz, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("task module: connect server init: %w", err)
	}

	return &Module{cfg: cfg, deps: deps, service: svc, connect: cs}, nil
}

// Service exposes the task service (used by tests and cross-module composition).
func (m *Module) Service() *task.Service { return m.service }

// Start is a placeholder for future background workers.
func (m *Module) Start(_ context.Context) error { return nil }

// Stop is a placeholder for graceful shutdown of future background workers.
func (m *Module) Stop(_ context.Context) error { return nil }
