# Stage 1 – Frontend build
# Builds the embedded Vue/Vuetify SPA so the image is reproducible from a clean
# checkout (dist/ is git-ignored). Mirrors the `make front-build` step.
FROM oven/bun:1-alpine AS frontend
WORKDIR /app/cmd/goeland-server/goeland-front
# Install deps first (cached until the manifest/lockfile change).
COPY cmd/goeland-server/goeland-front/package.json cmd/goeland-server/goeland-front/bun.lock ./
RUN bun install --frozen-lockfile
COPY cmd/goeland-server/goeland-front/ ./
RUN bun run build

# Stage 2 – Go binary
# Mirrors the go build step from `make build`, without the test step (which needs a live DB).
# Pin the builder to the module's Go minor version for reproducibility.
FROM golang:1.26-alpine AS builder
LABEL maintainer="cgil"
# git: for module VCS metadata; ca-certificates: copied into the scratch runtime.
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Bring in the freshly built frontend so //go:embed goeland-front/dist/* resolves
# (this overrides any stale/absent dist from the build context).
COPY --from=frontend /app/cmd/goeland-server/goeland-front/dist ./cmd/goeland-server/goeland-front/dist
# Inject version provenance so the container's /goAppInfo reports real values
# (pass with: --build-arg APP_REVISION=$(git describe --always) --build-arg BUILD_STAMP=$(date -u +%FT%TZ)).
ARG APP_REVISION=unknown
ARG BUILD_STAMP=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags "-X github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version.Revision=${APP_REVISION} -X github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version.BuildStamp=${BUILD_STAMP}" \
    -o bin/goeland-server ./cmd/goeland-server

# Stage 3 – Minimal runtime image
FROM scratch
# CA roots so outbound HTTPS (PAT introspection, certificate-verifying PostgreSQL TLS) works.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 1221:1221
WORKDIR /goapp
COPY --from=builder /app/bin/goeland-server .

# --- Database ---------------------------------------------------------------
# Provide either DATABASE_URL (full DSN, takes precedence) or the individual
# DB_* variables. DB_PASSWORD is required when DATABASE_URL is not set.
# The database must have the PostGIS, pgcrypto, pg_trgm and unaccent extensions available.
ENV DATABASE_URL=""
ENV DB_HOST="127.0.0.1"
ENV DB_PORT="5432"
ENV DB_NAME="goeland_poc_db"
ENV DB_USER="goeland_poc_db"
ENV DB_PASSWORD=""
ENV DB_SSL_MODE="prefer"

# --- Authentication ----------------------------------------------------------
# GOELAND_AUTH_MODE: "jwt" (default, production) or "dev" (local dev only).
ENV GOELAND_AUTH_MODE="jwt"
# AUTH_SERVER_URL: base URL of go-cloud-k8s-auth (PAT introspection + login flow).
ENV AUTH_SERVER_URL="http://localhost:9090"
# JWT settings — required when GOELAND_AUTH_MODE=jwt.
ENV JWT_SECRET=""
ENV JWT_ISSUER_ID=""
ENV JWT_CONTEXT_KEY=""
ENV JWT_DURATION_MINUTES=""

# --- Dev-mode auth (GOELAND_AUTH_MODE=dev only) ------------------------------
ENV GOELAND_DEV_TOKEN=""
ENV GOELAND_DEV_USER_ID="1"
ENV GOELAND_DEV_USER_EMAIL="dev@localhost"
ENV GOELAND_DEV_USER_NAME="Local Goeland User"

# --- Server tuning -----------------------------------------------------------
ENV GOELAND_LISTEN_ADDRESS="0.0.0.0:8080"
ENV GOELAND_DB_MAX_CONNECTIONS="10"
ENV GOELAND_SHUTDOWN_TIMEOUT_SECONDS="10"
ENV GOELAND_REQUEST_TIMEOUT_SECONDS="10"
ENV LOG_LEVEL="info"

EXPOSE 8080

# /health returns {"status":"ok"} — use it for liveness/readiness probes at the
# orchestration layer. No HEALTHCHECK here because scratch has no shell/curl.
CMD ["./goeland-server"]
