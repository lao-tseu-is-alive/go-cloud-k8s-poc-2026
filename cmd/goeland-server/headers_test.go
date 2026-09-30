package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContentSecurityPolicyAllowsTheAuthServerOnlyInJWTMode(t *testing.T) {
	jwt := contentSecurityPolicy(serverConfig{AuthMode: "jwt", AuthServerURL: "http://localhost:9090/some/path"})
	if !strings.Contains(jwt, "connect-src 'self' http://localhost:9090;") {
		t.Fatalf("jwt mode must allow the auth origin in connect-src: %s", jwt)
	}
	dev := contentSecurityPolicy(serverConfig{AuthMode: "dev", AuthServerURL: "http://localhost:9090"})
	if !strings.Contains(dev, "connect-src 'self';") {
		t.Fatalf("dev mode never calls the auth server: %s", dev)
	}
	if bad := contentSecurityPolicy(serverConfig{AuthMode: "jwt", AuthServerURL: "not a url"}); !strings.Contains(bad, "connect-src 'self';") {
		t.Fatalf("a relative auth URL adds no origin: %s", bad)
	}
	for _, directive := range []string{"script-src 'self'", "object-src 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(jwt, directive) {
			t.Fatalf("missing %q in %s", directive, jwt)
		}
	}
}

func TestSecurityHeadersMiddlewareSetsEveryHeader(t *testing.T) {
	handler := securityHeadersMiddleware("default-src 'self'", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	want := map[string]string{
		"Content-Security-Policy":    "default-src 'self'",
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "strict-origin-when-cross-origin",
		"Cross-Origin-Opener-Policy": "same-origin",
	}
	for name, value := range want {
		if got := rec.Header().Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
	if rec.Header().Get("Permissions-Policy") == "" || rec.Code != http.StatusTeapot {
		t.Fatalf("headers set before the handler writes: %v %d", rec.Header(), rec.Code)
	}
}
