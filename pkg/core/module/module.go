package module

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const defaultRequestTimeout = 10 * time.Second

// Config holds core-module-specific configuration.
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

// Deps holds cross-cutting dependencies injected by the main binary or a bundle.
type Deps struct {
	Pool     *pgxpool.Pool
	Verifier authadapter.TokenVerifier
	Logger   *slog.Logger
}

// Module encapsulates the transversal core domain: repository, service and Connect handler.
type Module struct {
	cfg     Config
	deps    Deps
	repo    *core.PostgresRepository
	service *core.Service
	connect *core.ConnectServer
}

// New creates a fully wired core Module ready to register routes.
func New(_ context.Context, cfg Config, deps Deps) (*Module, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("core module: database pool is required")
	}
	if deps.Verifier == nil {
		return nil, fmt.Errorf("core module: token verifier is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	repo, err := core.NewPostgresRepository(deps.Pool, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("core module: storage init: %w", err)
	}
	svc, err := core.NewService(repo, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("core module: service init: %w", err)
	}
	cs, err := core.NewConnectServer(svc, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("core module: connect server init: %w", err)
	}

	return &Module{cfg: cfg, deps: deps, repo: repo, service: svc, connect: cs}, nil
}

// Service exposes the core service so sibling modules (Document) can reuse core primitives.
func (m *Module) Service() *core.Service { return m.service }

// Repo exposes the core repository for sibling modules that need transaction composition.
func (m *Module) Repo() *core.PostgresRepository { return m.repo }

// Start is a placeholder for future background workers.
func (m *Module) Start(_ context.Context) error { return nil }

// Stop is a placeholder for graceful shutdown of future background workers.
func (m *Module) Stop(_ context.Context) error { return nil }
