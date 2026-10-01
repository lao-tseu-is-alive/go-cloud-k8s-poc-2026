package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

func TestWaitForDatabaseRetriesUntilReachable(t *testing.T) {
	calls := 0
	ping := func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}
		return nil
	}
	if err := waitForDatabase(context.Background(), ping, 10*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil || calls != 3 {
		t.Fatalf("want success after 3 attempts, got %v after %d", err, calls)
	}
}

func TestWaitForDatabaseGivesUp(t *testing.T) {
	calls := 0
	down := func(context.Context) error { calls++; return errors.New("connection refused") }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := waitForDatabase(context.Background(), down, 0, log); err == nil || calls != 1 {
		t.Fatalf("a zero timeout makes one attempt and fails, got %v after %d", err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForDatabase(ctx, down, time.Minute, log); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled context stops the wait, got %v", err)
	}
}

func TestRequestIDMiddlewareKeepsOnlySafeClientIDs(t *testing.T) {
	var seen string
	handler := requestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = core.RequestIDFromContext(r.Context())
	}))
	for _, c := range []struct {
		header string
		kept   bool
	}{
		{"3f2a9c1e-5b7d-4e8a-9c21-7d4e5f6a8b90", true},
		{"trace:abc.def_1", true},
		{"", false},
		{strings.Repeat("a", maxRequestIDLength+1), false},
		{"bad id\nX-Injected: 1", false},
		{"<script>", false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("X-Request-ID", c.header)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if (seen == c.header) != c.kept || seen == "" || rec.Header().Get("X-Request-ID") != seen {
			t.Fatalf("X-Request-ID %q: context %q, response %q, want kept=%v", c.header, seen, rec.Header().Get("X-Request-ID"), c.kept)
		}
	}
}

// TestEmbeddedFrontendIsComplete checks that every file of the built SPA is
// embedded: a plain //go:embed dir/* pattern silently drops the files whose
// names start with "_" or ".", such as Vite's _plugin-vue_export-helper chunk.
func TestEmbeddedFrontendIsComplete(t *testing.T) {
	embedded, err := fs.Sub(frontendFiles, "goeland-front/dist")
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(os.DirFS("goeland-front/dist"), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if _, statErr := fs.Stat(embedded, path); statErr != nil {
			t.Errorf("%s is built but not embedded: %v", path, statErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
