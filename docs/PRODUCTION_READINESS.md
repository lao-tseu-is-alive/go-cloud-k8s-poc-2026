# Production Readiness

This page is the deployment contract for the Goéland POC server (`cmd/goeland-server`):
what the database must provide, how migrations behave, how blobs persist, how auth is
configured, which probes to wire, and which values are secrets.

> **Status: POC.** The server basics are production-shaped (timeouts, graceful shutdown,
> health/readiness, structured logs, request IDs, non-root scratch image). Two things are
> **not** production-grade yet and are called out explicitly below: **authorization is
> coarse** and **blob storage is node-local**. Read [Known limitations](#known-limitations)
> before exposing this to real data.

## 1. Database

PostgreSQL is required. The schema is created by embedded migrations at startup (see §2).

### Required extensions

Migration `0001_subject_core.sql` runs `CREATE EXTENSION IF NOT EXISTS` for:

| Extension  | Purpose                                              |
|------------|------------------------------------------------------|
| `pgcrypto` | `gen_random_uuid()` for subject/document identifiers |
| `pg_trgm`  | trigram indexing for search                          |
| `unaccent` | accent-insensitive full-text search (migration 0005) |
| `postgis`  | geometry columns for future territorial objects      |

`CREATE EXTENSION` requires a role with sufficient privileges (superuser for PostGIS), so
**the extensions must be installable on the target instance** — use a PostGIS-enabled image
(e.g. `postgis/postgis`), not plain `postgres`. A managed instance must have these
extensions allow-listed. Once created, the application role only needs DML + DDL on the
application schema.

### Connection

Provide **either** a full DSN or the individual parts (the DSN wins):

- `DATABASE_URL` — e.g. `postgres://user:pass@host:5432/goeland_poc_db?sslmode=require`
- or `DB_HOST` / `DB_PORT` / `DB_NAME` / `DB_USER` / `DB_PASSWORD` / `DB_SSL_MODE`
  (`DB_PASSWORD` is required when `DATABASE_URL` is unset).

Pool size is capped by `GOELAND_DB_MAX_CONNECTIONS` (default 10, range 1–1000). Size it
against `max_connections` × replica count.

## 2. Migrations

Migrations are embedded (`pkg/core/module/db/migrations`) and applied automatically on
startup by `coremodule.Migrate`:

- Applied under a **PostgreSQL advisory lock**, so rolling deployments and multiple
  replicas starting concurrently are safe — only one instance migrates at a time.
- Bookkeeping is in a `schema_migrations` table; already-applied versions are skipped,
  so re-running is a **no-op** (proven by `pkg/integration.TestMigrationsIdempotentAndSeeded`).
- Migration `0004` seeds reference data (subject kinds, relationship types, document types);
  migration `0006` seeds the 33 organization categories used by the Actor component.

There is **no automated down-migration / rollback** path for data-bearing schema changes.
Treat schema changes as forward-only and take a backup before deploying a new version that
adds migrations.

## 3. Blob storage

Uploaded document bytes are written to a **local filesystem** directory, referenced by
documents through an `internal://…` `storage_ref`:

- `GOELAND_DOCUMENT_PATH` — blob directory (default `./go_documents`).
- `GOELAND_MAX_UPLOAD_BYTES` — per-upload cap (default 100 MiB).

**This is node-local.** For more than one replica, or on ephemeral containers, this path
**must** be a shared/persistent volume with a single writer, or the deployment must be
pinned to one replica. Object storage (MinIO/S3) is the intended replacement and is a
roadmap item — see [IMPLEMENTATION_STATUS.md](../requirements/IMPLEMENTATION_STATUS.md).

## 4. Authentication

`GOELAND_AUTH_MODE` selects the verifier:

- `jwt` (default, production): validates short-lived JWTs from `go-cloud-k8s-auth` and
  accepts PATs introspected against `AUTH_SERVER_URL`. Requires the JWT settings:
  `JWT_SECRET`, `JWT_ISSUER_ID`, `JWT_CONTEXT_KEY`, `JWT_DURATION_MINUTES`.
- `dev` (local only): accepts one static token. Requires `GOELAND_DEV_TOKEN` (startup
  fails without it) and the `GOELAND_DEV_USER_*` identity fields. **Never enable in
  production.**

`AUTH_SERVER_URL` must be a valid `http(s)` URL (PAT introspection + login redirect).

## 5. Probes (Kubernetes)

| Probe      | Endpoint      | Behavior                                                        |
|------------|---------------|----------------------------------------------------------------|
| Liveness   | `GET /health` | Returns `{"status":"ok"}` and DB pool stats; process is up.    |
| Readiness  | `GET /readiness` | Pings the database; fails when the DB is unreachable.       |

The scratch image has no shell, so there is no container `HEALTHCHECK` — configure the
probes at the orchestration layer against the endpoints above. There is no separate
startup probe; because migrations run before the listener binds, size the readiness
`initialDelay`/`failureThreshold` to allow for migration time on first boot.

## 6. Server tuning

| Variable                            | Default          | Notes                              |
|-------------------------------------|------------------|------------------------------------|
| `GOELAND_LISTEN_ADDRESS`            | `127.0.0.1:8080` | Use `0.0.0.0:8080` in a container. |
| `GOELAND_REQUEST_TIMEOUT_SECONDS`   | `10`             | Per-request timeout (1–300).       |
| `GOELAND_SHUTDOWN_TIMEOUT_SECONDS`  | `10`             | Graceful drain window (1–300).     |
| `LOG_LEVEL`                         | `info`           | `debug` / `info` / `warn` / `error`. |

## 7. Secrets

Never bake these into images or ConfigMaps — deliver them via a Kubernetes `Secret` (or
your secret manager):

- `DATABASE_URL` or `DB_PASSWORD`
- `JWT_SECRET`
- `GOELAND_DEV_TOKEN` (dev mode only)

Non-secret settings can go in a ConfigMap. `scripts/create_k8s_configmap_from_env.sh`
renders only non-secret keys and never prints secret values; Secret creation is handled
separately.

## Known limitations

These are the gaps that make this a POC rather than a production service:

- **Authorization is coarse.** Every authenticated caller is granted both `goeland:read`
  and `goeland:write` (`cmd/goeland-server/server.go`). Per-subject access grants and
  deny-by-default confidentiality enforcement are **not implemented**.
- **Blob storage is node-local** (§3): not safe for multi-replica or ephemeral deployments
  without a shared/persistent volume.
- **Observability is logs only.** No metrics or tracing endpoints yet.
- **No deployment manifests / Helm chart** are shipped.

See [requirements/IMPLEMENTATION_STATUS.md](../requirements/IMPLEMENTATION_STATUS.md) for
the full implemented-vs-pending tracker.

## Verifying a deployment

Run the database integration tests against a disposable PostGIS database to prove the
schema and the full document and actor lifecycles work end-to-end:

```bash
GOELAND_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5432/goeland_test?sslmode=disable' \
    go test ./pkg/integration/...
```

They are skipped when `GOELAND_TEST_DATABASE_URL` is unset, so the default `go test ./...`
run needs no database.
