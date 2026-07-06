// Package module provides an importable, bundleable Module for the Goéland document domain.
//
// The document schema (document, document_type) is bootstrapped by the core module
// migrations because the document tables have foreign keys into the core tables.
// This module therefore has no migrations of its own; it wires the document
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
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds document-module-specific configuration.
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
// document domain reuses core primitives (subjects, relationships, audit).
type Deps struct {
	Pool        *pgxpool.Pool
	Verifier    authadapter.TokenVerifier
	CoreService *core.Service
	Logger      *slog.Logger
}

// Module encapsulates the document domain: repository, service and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	service *document.Service
	connect *document.ConnectServer
}

// New creates a fully wired document Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("document module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("document module: token verifier is required")
	}
	if deps.CoreService == nil {
		return nil, fmt.Errorf("document module: core service is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	repo, err := document.NewPostgresRepository(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("document module: storage init: %w", err)
	}
	svc, err := document.NewService(repo, deps.CoreService, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("document module: service init: %w", err)
	}
	cs, err := document.NewConnectServer(svc, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("document module: connect server init: %w", err)
	}

	return &Module{cfg: cfg, deps: deps, service: svc, connect: cs}, nil
}

// Start is a placeholder for future background workers.
func (m *Module) Start(_ context.Context) error { return nil }

// Stop is a placeholder for graceful shutdown of future background workers.
func (m *Module) Stop(_ context.Context) error { return nil }
