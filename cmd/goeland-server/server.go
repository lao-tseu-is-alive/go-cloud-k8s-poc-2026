package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document/filestore"
	documentmodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document/module"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version"
)

const maxRequestBodyBytes = 8 << 20 // 8 MiB

//go:embed goeland-front/dist/*
var frontendFiles embed.FS

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

	// Local blob store for uploaded document files (referenced by documents via
	// an internal:// storage_ref). The directory is created if missing.
	blobStore, err := filestore.New(config.DocumentPath)
	if err != nil {
		return nil, fmt.Errorf("document blob store: %w", err)
	}
	log.Info("document blob store ready", "path", blobStore.Root(), "max_upload_bytes", config.MaxUploadBytes)

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

	serviceNames := append(coreMod.ServiceNames(), docMod.ServiceNames()...)

	mux := http.NewServeMux()
	mux.Handle("GET /health", healthHandler(pool))
	mux.HandleFunc("GET /readiness", readinessHandler(pool))
	mux.HandleFunc("GET /goAppInfo", appInfoHandler)
	mux.HandleFunc("GET /config", frontendConfigHandler(config))

	// Binary upload/download live OUTSIDE the proto contract (metadata-first):
	// the client uploads bytes here, then calls CreateDocument with the returned
	// storage_ref. These literal paths are more specific than the "/api/"
	// transcoder subtree, so http.ServeMux routes them here first. They carry
	// their own bearer-token check since they bypass the Connect interceptor,
	// and their own (larger) body cap for file payloads.
	mux.Handle("POST /api/documents/upload",
		httpAuthMiddleware(verifier, log, http.MaxBytesHandler(uploadHandler(blobStore, log), config.MaxUploadBytes)))
	mux.Handle("GET /api/documents/download",
		httpAuthMiddleware(verifier, log, downloadHandler(blobStore, log)))

	// The Vanguard transcoder serves BOTH the Connect/gRPC RPC paths
	// (/goeland.v1.<Service>/<Method>) and the REST bindings declared via
	// google.api.http (/api/...). Mount it on those explicit prefixes so the
	// embedded SPA can own "/" as the catch-all fallback below.
	transcoderHandler := http.MaxBytesHandler(transcoder, maxRequestBodyBytes)
	mux.Handle("/api/", transcoderHandler)
	for _, name := range serviceNames {
		mux.Handle("/"+name+"/", transcoderHandler)
	}

	// Serve the embedded Vuetify frontend (SPA fallback to index.html).
	frontendFS, err := fs.Sub(frontendFiles, "goeland-front/dist")
	if err != nil {
		return nil, fmt.Errorf("sub-filesystem for frontend: %w", err)
	}
	mux.Handle("/", spaHandler(http.FileServer(http.FS(frontendFS)), frontendFS))

	log.Info("registered services", "services", serviceNames)

	cleanup = false
	return &application{
		pool:    pool,
		handler: recoverMiddleware(log, requestIDMiddleware(requestLogMiddleware(log, mux))),
		log:     log,
	}, nil
}

// spaHandler serves static assets from the embedded frontend FS and falls back to
// index.html for any path that does not match an embedded file, enabling client-side
// routing in the Vuetify SPA.
func spaHandler(fileServer http.Handler, frontendFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Serve the file when it exists in the embedded FS; otherwise fall back
		// to index.html so the SPA router can handle the route client-side.
		if _, err := fs.Stat(frontendFS, r.URL.Path[1:]); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
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

// requestIDMiddleware ensures every request has an X-Request-ID and stores it in
// the context (via the shared core key) so the value reaches the service layer and
// is stamped onto every audit_event.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id := request.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		writer.Header().Set("X-Request-ID", id)
		ctx := core.WithRequestID(request.Context(), id)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// statusRecorder captures the response status and byte count for access logging.
// It forwards Flush so Connect/gRPC-Web streaming still works.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// requestLogMiddleware logs method, path, status, bytes and elapsed time per request.
// Note: the authenticated user is intentionally not logged here — it is established
// inside the Connect auth interceptor and is not visible to this outer middleware.
// The operator identity is captured on the audit_event instead.
func requestLogMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		rec := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(rec, request)
		log.Info("HTTP request",
			"method", request.Method,
			"path", request.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"request_id", core.RequestIDFromContext(request.Context()),
			"duration", time.Since(started),
		)
	})
}

// recoverMiddleware catches panics, logs the stack, and returns a 500 JSON response.
func recoverMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("HTTP panic", "panic", recovered, "request_id", core.RequestIDFromContext(request.Context()), "stack", string(debug.Stack()))
				writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
		}()
		next.ServeHTTP(writer, request)
	})
}
