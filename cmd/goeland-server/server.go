package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"connectrpc.com/vanguard"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-common-libs/pkg/goHttpEcho"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
	documentmodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document/module"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version"
)

const maxRequestBodyBytes = 8 << 20 // 8 MiB

// authScopes are granted to every authenticated caller in this POC.
var authScopes = []string{"goeland:read", "goeland:write"}

// application holds the shared resources for the Goéland server.
type application struct {
	pool    *pgxpool.Pool
	handler http.Handler
	log     *slog.Logger
}

// newApplication opens the pool, migrates the schema, wires the core + document
// modules onto a single shared Vanguard transcoder, and returns a ready application.
func newApplication(ctx context.Context, config serverConfig, log *slog.Logger) (*application, error) {
	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	poolConfig.MaxConns = config.MaxConnections
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			pool.Close()
		}
	}()
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	// The core module owns the full schema bootstrap (core + document tables + seed).
	if err := coremodule.Migrate(ctx, pool); err != nil {
		return nil, err
	}

	verifier, err := buildTokenVerifier(config, log)
	if err != nil {
		return nil, err
	}

	coreMod, err := coremodule.New(ctx, coremodule.Config{RequestTimeout: config.RequestTimeout}, coremodule.Deps{
		Pool:     pool,
		Verifier: verifier,
		Logger:   log,
	})
	if err != nil {
		return nil, fmt.Errorf("core module: %w", err)
	}
	docMod, err := documentmodule.New(ctx, documentmodule.Config{RequestTimeout: config.RequestTimeout}, documentmodule.Deps{
		Pool:        pool,
		Verifier:    verifier,
		CoreService: coreMod.Service(),
		Logger:      log,
	})
	if err != nil {
		return nil, fmt.Errorf("document module: %w", err)
	}

	// Bundle mode: aggregate every module's Vanguard services into ONE transcoder.
	services := append(coreMod.VanguardServices(), docMod.VanguardServices()...)
	transcoder, err := vanguard.NewTranscoder(services)
	if err != nil {
		return nil, fmt.Errorf("build shared transcoder: %w", err)
	}

	mux := http.NewServeMux()
	serviceNames := append(coreMod.ServiceNames(), docMod.ServiceNames()...)
	for _, name := range serviceNames {
		mux.Handle("/"+name+"/", http.MaxBytesHandler(transcoder, maxRequestBodyBytes))
	}
	mux.Handle("GET /health", healthHandler(pool))
	mux.HandleFunc("GET /readiness", readinessHandler(pool))
	mux.HandleFunc("GET /goAppInfo", appInfoHandler)

	log.Info("registered services", "services", serviceNames)

	cleanup = false
	return &application{
		pool:    pool,
		handler: recoverMiddleware(log, requestIDMiddleware(requestLogMiddleware(log, mux))),
		log:     log,
	}, nil
}

// buildTokenVerifier selects the verifier for the configured auth mode.
func buildTokenVerifier(config serverConfig, log *slog.Logger) (authadapter.TokenVerifier, error) {
	if config.AuthMode == "dev" {
		return authadapter.NewDevTokenVerifier(config.DevToken, authadapter.AuthenticatedUser{
			AppUserID:   config.DevUserID,
			Email:       config.DevUserEmail,
			DisplayName: config.DevDisplayName,
			Scopes:      authScopes,
		})
	}
	checker, err := goHttpEcho.GetNewJwtCheckerFromConfig(version.AppName, 60, log)
	if err != nil {
		return nil, fmt.Errorf("configure JWT verifier: %w", err)
	}
	jwtVerifier, err := authadapter.NewJWTVerifier(checker, authScopes)
	if err != nil {
		return nil, err
	}
	patVerifier, err := authadapter.NewPatVerifier(config.AuthServerURL)
	if err != nil {
		return nil, fmt.Errorf("configure PAT verifier: %w", err)
	}
	return authadapter.NewCompositeVerifier(jwtVerifier, patVerifier)
}

// close releases the database connection pool.
func (a *application) close() { a.pool.Close() }

// serve starts the HTTP server and blocks until ctx is cancelled or the server fails.
func (a *application) serve(ctx context.Context, listener net.Listener, shutdownPeriod time.Duration) error {
	server := &http.Server{
		Handler:           a.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownPeriod)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{"status": "ok"}
		if pool != nil {
			stat := pool.Stat()
			resp["db"] = map[string]any{
				"acquired_conns": stat.AcquiredConns(),
				"total_conns":    stat.TotalConns(),
				"max_conns":      stat.MaxConns(),
			}
		}
		writeJSON(writer, http.StatusOK, resp)
	}
}

// readinessHandler returns 503 when the database cannot be reached within 2 seconds.
func readinessHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func appInfoHandler(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{
		"app":        version.AppName,
		"version":    version.Version,
		"revision":   version.Revision,
		"build":      version.BuildStamp,
		"repository": version.Repository,
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

type requestIDKey struct{}

// requestIDMiddleware ensures every request has an X-Request-ID and stores it in context.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id := request.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		writer.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(request.Context(), requestIDKey{}, id)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func requestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// requestLogMiddleware logs method, path and elapsed time for every request.
func requestLogMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		next.ServeHTTP(writer, request)
		userID := ""
		if u, err := authadapter.RequireUser(request.Context()); err == nil {
			userID = fmt.Sprintf("%d", u.AppUserID)
		}
		log.Info("HTTP request",
			"method", request.Method,
			"path", request.URL.Path,
			"request_id", requestIDFromContext(request.Context()),
			"user_id", userID,
			"duration", time.Since(started),
		)
	})
}

// recoverMiddleware catches panics, logs the stack, and returns a 500 JSON response.
func recoverMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("HTTP panic", "panic", recovered, "request_id", requestIDFromContext(request.Context()), "stack", string(debug.Stack()))
				writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
		}()
		next.ServeHTTP(writer, request)
	})
}
