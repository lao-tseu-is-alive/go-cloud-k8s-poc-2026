// Package module provides an importable, bundleable Module for the Goéland actor domain.
//
// The actor schema (actor, actor_contact, organization_category) is bootstrapped by
// the core module migrations because the actor tables have foreign keys into the core
// tables. This module therefore has no migrations of its own; it wires the actor
// repository/service/handler on top of a shared pool and the core service.
package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds actor-module-specific configuration.
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
// actor domain reuses core primitives (subjects, relationships, audit).
type Deps struct {
	Pool        *pgxpool.Pool
	Verifier    authadapter.TokenVerifier
	CoreService *core.Service
	Logger      *slog.Logger
}

// Module encapsulates the actor domain: repository, service and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	service *actor.Service
	connect *actor.ConnectServer
}

// New creates a fully wired actor Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("actor module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("actor module: token verifier is required")
	}
	if deps.CoreService == nil {
		return nil, fmt.Errorf("actor module: core service is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	repo, err := actor.NewPostgresRepository(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("actor module: storage init: %w", err)
	}
	svc, err := actor.NewService(repo, deps.CoreService, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("actor module: service init: %w", err)
	}
	cs, err := actor.NewConnectServer(svc, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("actor module: connect server init: %w", err)
	}

	return &Module{cfg: cfg, deps: deps, service: svc, connect: cs}, nil
}

// Service exposes the actor service (used by tests and cross-module composition).
func (m *Module) Service() *actor.Service { return m.service }

// Start is a placeholder for future background workers.
func (m *Module) Start(_ context.Context) error { return nil }

// Stop is a placeholder for graceful shutdown of future background workers.
func (m *Module) Stop(_ context.Context) error { return nil }
