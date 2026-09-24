package main

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddress  = "127.0.0.1:8080"
	defaultAuthMode       = "jwt"
	defaultAuthServerURL  = "http://localhost:9090"
	defaultShutdownPeriod = 10 * time.Second
	defaultMaxConnections = 10
	defaultRequestTimeout = 10 * time.Second
	defaultDocumentPath   = "./go_documents"
	defaultMaxUploadBytes = 100 << 20 // 100 MiB
)

// serverConfig holds all runtime configuration, loaded from environment variables.
type serverConfig struct {
	// ListenAddress is the host:port to bind (GOELAND_LISTEN_ADDRESS).
	ListenAddress string
	// DatabaseURL is DATABASE_URL, or a DSN assembled from the DB_* variables.
	// It carries the database password and must never be logged.
	DatabaseURL string
	// AuthMode is "jwt" (default) or "dev" (GOELAND_AUTH_MODE).
	AuthMode string
	// AuthServerURL is the base URL of the auth server used for PAT
	// introspection and advertised to the SPA (AUTH_SERVER_URL), without a
	// trailing slash.
	AuthServerURL string
	// DevToken is the static bearer token accepted in dev mode
	// (GOELAND_DEV_TOKEN, required then); a secret that must never be logged.
	DevToken string
	// DevUserID is the application user ID of the dev-mode user
	// (GOELAND_DEV_USER_ID, default 1).
	DevUserID int64
	// DevUserEmail is the e-mail of the dev-mode user (GOELAND_DEV_USER_EMAIL).
	DevUserEmail string
	// DevDisplayName is the display name of the dev-mode user
	// (GOELAND_DEV_USER_NAME).
	DevDisplayName string
	// LogLevel is the slog level parsed from LOG_LEVEL (default info).
	LogLevel slog.Level
	// MaxConnections caps the pgx pool size (GOELAND_DB_MAX_CONNECTIONS).
	MaxConnections int32
	// ShutdownPeriod bounds the graceful shutdown
	// (GOELAND_SHUTDOWN_TIMEOUT_SECONDS, default 10s).
	ShutdownPeriod time.Duration
	// RequestTimeout bounds each RPC through the timeout interceptor
	// (GOELAND_REQUEST_TIMEOUT_SECONDS, default 10s).
	RequestTimeout time.Duration
	// DocumentPath is the local directory where uploaded document blobs are
	// stored (referenced by documents via an internal:// storage_ref).
	DocumentPath string
	// MaxUploadBytes caps the size of a single multipart upload.
	MaxUploadBytes int64
}

// loadConfig reads and validates all environment variables.
func loadConfig() (serverConfig, error) {
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return serverConfig{}, err
	}

	listenAddress := envOrDefault("GOELAND_LISTEN_ADDRESS", defaultListenAddress)
	if _, _, err := net.SplitHostPort(listenAddress); err != nil {
		return serverConfig{}, fmt.Errorf("GOELAND_LISTEN_ADDRESS must be host:port: %w", err)
	}

	authMode := strings.ToLower(strings.TrimSpace(envOrDefault("GOELAND_AUTH_MODE", defaultAuthMode)))
	if authMode != "jwt" && authMode != "dev" {
		return serverConfig{}, fmt.Errorf("GOELAND_AUTH_MODE must be jwt or dev")
	}

	authServerURL, err := authServerURLFromEnv()
	if err != nil {
		return serverConfig{}, err
	}
	devUserID, err := envInt64("GOELAND_DEV_USER_ID", 1)
	if err != nil {
		return serverConfig{}, err
	}
	maxConnections, err := envInt64InRange("GOELAND_DB_MAX_CONNECTIONS", defaultMaxConnections, 1, 1000)
	if err != nil {
		return serverConfig{}, err
	}
	shutdownSeconds, err := envInt64InRange("GOELAND_SHUTDOWN_TIMEOUT_SECONDS", int64(defaultShutdownPeriod/time.Second), 1, 300)
	if err != nil {
		return serverConfig{}, err
	}
	requestTimeoutSeconds, err := envInt64InRange("GOELAND_REQUEST_TIMEOUT_SECONDS", int64(defaultRequestTimeout/time.Second), 1, 300)
	if err != nil {
		return serverConfig{}, err
	}
	logLevel, err := parseLogLevel(envOrDefault("LOG_LEVEL", "info"))
	if err != nil {
		return serverConfig{}, err
	}
	maxUploadBytes, err := envInt64InRange("GOELAND_MAX_UPLOAD_BYTES", defaultMaxUploadBytes, 1, math.MaxInt64)
	if err != nil {
		return serverConfig{}, err
	}

	config := serverConfig{
		ListenAddress:  listenAddress,
		DatabaseURL:    databaseURL,
		AuthMode:       authMode,
		AuthServerURL:  authServerURL,
		DevToken:       os.Getenv("GOELAND_DEV_TOKEN"),
		DevUserID:      devUserID,
		DevUserEmail:   envOrDefault("GOELAND_DEV_USER_EMAIL", "dev@localhost"),
		DevDisplayName: envOrDefault("GOELAND_DEV_USER_NAME", "Local Goeland User"),
		LogLevel:       logLevel,
		MaxConnections: int32(maxConnections),
		ShutdownPeriod: time.Duration(shutdownSeconds) * time.Second,
		RequestTimeout: time.Duration(requestTimeoutSeconds) * time.Second,
		DocumentPath:   envOrDefault("GOELAND_DOCUMENT_PATH", defaultDocumentPath),
		MaxUploadBytes: maxUploadBytes,
	}
	if config.AuthMode == "dev" && config.DevToken == "" {
		return serverConfig{}, fmt.Errorf("GOELAND_DEV_TOKEN is required when GOELAND_AUTH_MODE=dev")
	}
	return config, nil
}

// authServerURLFromEnv reads AUTH_SERVER_URL, which must be an http(s) URL; a
// trailing slash is removed.
func authServerURLFromEnv() (string, error) {
	authServerURL := strings.TrimRight(envOrDefault("AUTH_SERVER_URL", defaultAuthServerURL), "/")
	parsed, err := url.Parse(authServerURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("AUTH_SERVER_URL must be a valid http(s) URL")
	}
	return authServerURL, nil
}

// envInt64InRange reads an integer environment variable (fallback when unset)
// and requires minimum <= value <= maximum.
func envInt64InRange(name string, fallback, minimum, maximum int64) (int64, error) {
	value, err := envInt64(name, fallback)
	if err != nil || value < minimum || value > maximum {
		if maximum == math.MaxInt64 {
			return 0, fmt.Errorf("%s must be an integer of at least %d", name, minimum)
		}
		return 0, fmt.Errorf("%s must be between %d and %d", name, minimum, maximum)
	}
	return value, nil
}

// databaseURLFromEnv builds a PostgreSQL connection string from DATABASE_URL or from individual DB_* variables.
func databaseURLFromEnv() (string, error) {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
			return "", fmt.Errorf("DATABASE_URL is invalid")
		}
		return value, nil
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		return "", fmt.Errorf("DATABASE_URL or DB_PASSWORD is required")
	}
	host := envOrDefault("DB_HOST", "127.0.0.1")
	port := envOrDefault("DB_PORT", "5432")
	name := envOrDefault("DB_NAME", "goeland_poc_db")
	user := envOrDefault("DB_USER", "goeland_poc_db")
	sslMode := envOrDefault("DB_SSL_MODE", "prefer")
	result := &url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(host, port),
		Path:     name,
		RawQuery: url.Values{"sslmode": []string{sslMode}}.Encode(),
		User:     url.UserPassword(user, password),
	}
	return result.String(), nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt64(name string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return value, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
}
