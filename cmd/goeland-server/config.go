package main

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/url"
	"os"
	"slices"
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

// defaultDBConnectTimeout bounds how long startup waits for the database (a pod
// may start before its database accepts connections).
const defaultDBConnectTimeout = 60 * time.Second

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
	// DevUserAdmin grants the dev-mode user the goeland:admin scope
	// (GOELAND_DEV_USER_ADMIN=true, default false).
	DevUserAdmin bool
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
	// DBConnectTimeout bounds how long startup retries an unreachable database
	// (GOELAND_DB_CONNECT_TIMEOUT_SECONDS, default 60s; 0 means a single attempt).
	DBConnectTimeout time.Duration
	// DocumentPath is the local directory where uploaded document blobs are
	// stored (referenced by documents via an internal:// storage_ref).
	DocumentPath string
	// MaxUploadBytes caps the size of a single multipart upload.
	MaxUploadBytes int64
	// BootstrapAdmins are the user ids granted the ADMIN role on their next
	// request (GOELAND_BOOTSTRAP_ADMINS, comma-separated); the recovery path
	// when no administrator is left.
	BootstrapAdmins []string
}

// maxBootstrapAdmins bounds GOELAND_BOOTSTRAP_ADMINS.
const maxBootstrapAdmins = 20

// bootstrapAdmins returns the configured bootstrap ids, plus the dev user when
// GOELAND_DEV_USER_ADMIN=true in dev mode.
func (c serverConfig) bootstrapAdmins() []string {
	ids := slices.Clone(c.BootstrapAdmins)
	if c.AuthMode == "dev" && c.DevUserAdmin {
		ids = append(ids, strconv.FormatInt(c.DevUserID, 10))
	}
	return ids
}

// bootstrapAdminsFromEnv reads GOELAND_BOOTSTRAP_ADMINS: comma-separated user
// ids, blanks ignored, at most maxBootstrapAdmins.
func bootstrapAdminsFromEnv() ([]string, error) {
	var ids []string
	for _, id := range strings.Split(os.Getenv("GOELAND_BOOTSTRAP_ADMINS"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) > maxBootstrapAdmins {
		return nil, fmt.Errorf("GOELAND_BOOTSTRAP_ADMINS lists more than %d user ids", maxBootstrapAdmins)
	}
	return ids, nil
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
	devUserAdmin, err := envBool("GOELAND_DEV_USER_ADMIN", false)
	if err != nil {
		return serverConfig{}, err
	}
	logLevel, err := parseLogLevel(envOrDefault("LOG_LEVEL", "info"))
	if err != nil {
		return serverConfig{}, err
	}
	bootstrapAdmins, err := bootstrapAdminsFromEnv()
	if err != nil {
		return serverConfig{}, err
	}

	config := serverConfig{
		ListenAddress:   listenAddress,
		DatabaseURL:     databaseURL,
		AuthMode:        authMode,
		AuthServerURL:   authServerURL,
		DevToken:        os.Getenv("GOELAND_DEV_TOKEN"),
		DevUserID:       devUserID,
		DevUserEmail:    envOrDefault("GOELAND_DEV_USER_EMAIL", "dev@localhost"),
		DevDisplayName:  envOrDefault("GOELAND_DEV_USER_NAME", "Local Goeland User"),
		DevUserAdmin:    devUserAdmin,
		LogLevel:        logLevel,
		DocumentPath:    envOrDefault("GOELAND_DOCUMENT_PATH", defaultDocumentPath),
		BootstrapAdmins: bootstrapAdmins,
	}
	if err := config.readLimits(); err != nil {
		return serverConfig{}, err
	}
	if config.AuthMode == "dev" && config.DevToken == "" {
		return serverConfig{}, fmt.Errorf("GOELAND_DEV_TOKEN is required when GOELAND_AUTH_MODE=dev")
	}
	return config, nil
}

// readLimits reads the bounded numeric settings: pool size, timeouts and the
// upload size.
func (c *serverConfig) readLimits() error {
	maxConnections, err := envInt64InRange("GOELAND_DB_MAX_CONNECTIONS", defaultMaxConnections, 1, 1000)
	if err != nil {
		return err
	}
	shutdownSeconds, err := envInt64InRange("GOELAND_SHUTDOWN_TIMEOUT_SECONDS", int64(defaultShutdownPeriod/time.Second), 1, 300)
	if err != nil {
		return err
	}
	requestTimeoutSeconds, err := envInt64InRange("GOELAND_REQUEST_TIMEOUT_SECONDS", int64(defaultRequestTimeout/time.Second), 1, 300)
	if err != nil {
		return err
	}
	dbConnectSeconds, err := envInt64InRange("GOELAND_DB_CONNECT_TIMEOUT_SECONDS", int64(defaultDBConnectTimeout/time.Second), 0, 600)
	if err != nil {
		return err
	}
	maxUploadBytes, err := envInt64InRange("GOELAND_MAX_UPLOAD_BYTES", defaultMaxUploadBytes, 1, math.MaxInt64)
	if err != nil {
		return err
	}
	c.MaxConnections = int32(maxConnections)
	c.ShutdownPeriod = time.Duration(shutdownSeconds) * time.Second
	c.RequestTimeout = time.Duration(requestTimeoutSeconds) * time.Second
	c.DBConnectTimeout = time.Duration(dbConnectSeconds) * time.Second
	c.MaxUploadBytes = maxUploadBytes
	return nil
}

// authServerURLFromEnv reads AUTH_SERVER_URL, which must be an http(s) URL; a
// trailing slash is removed. Personal access tokens are sent there in clear, so
// plain http is accepted only for a loopback host, or anywhere when
// GOELAND_ALLOW_INSECURE_AUTH_URL=true (e.g. a service-mesh encrypted cluster).
func authServerURLFromEnv() (string, error) {
	authServerURL := strings.TrimRight(envOrDefault("AUTH_SERVER_URL", defaultAuthServerURL), "/")
	parsed, err := url.Parse(authServerURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("AUTH_SERVER_URL must be a valid http(s) URL")
	}
	if parsed.Scheme == "https" || isLoopbackHost(parsed.Hostname()) {
		return authServerURL, nil
	}
	allowInsecure, err := envBool("GOELAND_ALLOW_INSECURE_AUTH_URL", false)
	if err != nil {
		return "", err
	}
	if !allowInsecure {
		return "", fmt.Errorf("AUTH_SERVER_URL must use https outside loopback (tokens are sent to it); set GOELAND_ALLOW_INSECURE_AUTH_URL=true to accept http")
	}
	return authServerURL, nil
}

// isLoopbackHost reports whether host is localhost or a loopback IP address.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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

// envBool reads a boolean (strconv.ParseBool syntax), fallback when unset.
func envBool(name string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
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
