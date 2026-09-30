package main

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

// configEnv lists every variable loadConfig reads. setConfigEnv blanks them all
// (an empty value counts as unset) before applying a case, so a developer's
// exported .env never leaks into these tests.
var configEnv = []string{
	"AUTH_SERVER_URL", "DATABASE_URL", "DB_HOST", "DB_NAME", "DB_PASSWORD", "DB_PORT", "DB_SSL_MODE", "DB_USER",
	"GOELAND_ALLOW_INSECURE_AUTH_URL", "GOELAND_AUTH_MODE", "GOELAND_DB_CONNECT_TIMEOUT_SECONDS",
	"GOELAND_DB_MAX_CONNECTIONS", "GOELAND_DEV_TOKEN", "GOELAND_DEV_USER_ADMIN", "GOELAND_DEV_USER_EMAIL",
	"GOELAND_DEV_USER_ID", "GOELAND_DEV_USER_NAME", "GOELAND_DOCUMENT_PATH", "GOELAND_LISTEN_ADDRESS",
	"GOELAND_MAX_UPLOAD_BYTES", "GOELAND_REQUEST_TIMEOUT_SECONDS", "GOELAND_SHUTDOWN_TIMEOUT_SECONDS", "LOG_LEVEL",
}

// setConfigEnv blanks the configuration environment, then sets values; a
// database password placeholder is always present unless overridden.
func setConfigEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for _, name := range configEnv {
		t.Setenv(name, "")
	}
	t.Setenv("DB_PASSWORD", "<db-password>")
	for name, value := range values {
		t.Setenv(name, value)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	setConfigEnv(t, nil)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if cfg.ListenAddress != defaultListenAddress || cfg.AuthMode != "jwt" || cfg.AuthServerURL != defaultAuthServerURL ||
		cfg.MaxConnections != defaultMaxConnections || cfg.RequestTimeout != defaultRequestTimeout ||
		cfg.ShutdownPeriod != defaultShutdownPeriod || cfg.DBConnectTimeout != defaultDBConnectTimeout ||
		cfg.MaxUploadBytes != defaultMaxUploadBytes || cfg.DocumentPath != defaultDocumentPath ||
		cfg.LogLevel != slog.LevelInfo || cfg.DevUserID != 1 || cfg.DevUserAdmin {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "postgres://goeland_poc_db:") || !strings.Contains(cfg.DatabaseURL, "@127.0.0.1:5432/goeland_poc_db?sslmode=prefer") {
		t.Fatalf("DSN assembled from the DB_* defaults, got a different shape")
	}
}

func TestLoadConfigAcceptsValidSettings(t *testing.T) {
	setConfigEnv(t, map[string]string{
		"DATABASE_URL":                       "postgresql://app@db.internal:5433/goeland?sslmode=require",
		"GOELAND_AUTH_MODE":                  "DEV",
		"GOELAND_DEV_TOKEN":                  "<dev-token>",
		"GOELAND_DEV_USER_ID":                "42",
		"GOELAND_DEV_USER_ADMIN":             "true",
		"GOELAND_LISTEN_ADDRESS":             "0.0.0.0:8080",
		"GOELAND_DB_CONNECT_TIMEOUT_SECONDS": "0",
		"GOELAND_REQUEST_TIMEOUT_SECONDS":    "30",
		"LOG_LEVEL":                          "debug",
		"AUTH_SERVER_URL":                    "https://auth.example.ch/",
	})
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("valid settings: %v", err)
	}
	if cfg.AuthMode != "dev" || cfg.DevUserID != 42 || !cfg.DevUserAdmin || cfg.DBConnectTimeout != 0 ||
		cfg.RequestTimeout != 30*time.Second || cfg.LogLevel != slog.LevelDebug ||
		cfg.AuthServerURL != "https://auth.example.ch" || cfg.DatabaseURL != "postgresql://app@db.internal:5433/goeland?sslmode=require" {
		t.Fatalf("settings not applied: %+v", cfg)
	}
}

func TestLoadConfigRejectsInvalidSettings(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{"no database", map[string]string{"DB_PASSWORD": ""}, "DATABASE_URL or DB_PASSWORD is required"},
		{"bad database url", map[string]string{"DATABASE_URL": "mysql://x/y"}, "DATABASE_URL is invalid"},
		{"bad listen address", map[string]string{"GOELAND_LISTEN_ADDRESS": "8080"}, "GOELAND_LISTEN_ADDRESS must be host:port"},
		{"unknown auth mode", map[string]string{"GOELAND_AUTH_MODE": "basic"}, "GOELAND_AUTH_MODE must be jwt or dev"},
		{"dev without token", map[string]string{"GOELAND_AUTH_MODE": "dev"}, "GOELAND_DEV_TOKEN is required"},
		{"non-integer dev user", map[string]string{"GOELAND_DEV_USER_ID": "u-1"}, "GOELAND_DEV_USER_ID must be an integer"},
		{"pool too large", map[string]string{"GOELAND_DB_MAX_CONNECTIONS": "1001"}, "GOELAND_DB_MAX_CONNECTIONS must be between 1 and 1000"},
		{"db wait too long", map[string]string{"GOELAND_DB_CONNECT_TIMEOUT_SECONDS": "601"}, "between 0 and 600"},
		{"zero request timeout", map[string]string{"GOELAND_REQUEST_TIMEOUT_SECONDS": "0"}, "between 1 and 300"},
		{"zero upload size", map[string]string{"GOELAND_MAX_UPLOAD_BYTES": "0"}, "at least 1"},
		{"unknown log level", map[string]string{"LOG_LEVEL": "loud"}, "LOG_LEVEL"},
		{"not a boolean", map[string]string{"GOELAND_DEV_USER_ADMIN": "yes please"}, "must be a boolean"},
		{"auth url scheme", map[string]string{"AUTH_SERVER_URL": "ftp://auth.example.ch"}, "valid http(s) URL"},
		{"plain http auth url", map[string]string{"AUTH_SERVER_URL": "http://auth.example.ch"}, "must use https outside loopback"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setConfigEnv(t, c.values)
			if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestAuthServerURLHTTPRules(t *testing.T) {
	cases := []struct {
		url, allow string
		ok         bool
	}{
		{"https://auth.example.ch", "", true},
		{"http://localhost:9090", "", true},
		{"http://127.0.0.1:9090", "", true},
		{"http://[::1]:9090", "", true},
		{"http://auth.internal:9090", "", false},
		{"http://auth.internal:9090", "true", true},
		{"http://localhost.example.ch", "", false},
	}
	for _, c := range cases {
		setConfigEnv(t, map[string]string{"AUTH_SERVER_URL": c.url, "GOELAND_ALLOW_INSECURE_AUTH_URL": c.allow})
		if _, err := authServerURLFromEnv(); (err == nil) != c.ok {
			t.Fatalf("AUTH_SERVER_URL=%s allow=%q: ok=%v, err=%v", c.url, c.allow, c.ok, err)
		}
	}
}
