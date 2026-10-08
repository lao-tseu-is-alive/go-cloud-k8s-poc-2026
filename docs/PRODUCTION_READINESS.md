# Production Readiness

This page is the deployment contract for the Goéland POC server (`cmd/goeland-server`):
what the database must provide, how migrations behave, how blobs persist, how auth is
configured, which probes to wire, and which values are secrets.

> **Status: POC.** The server basics are production-shaped (timeouts, graceful shutdown,
> health/readiness, structured logs, request IDs, non-root scratch image) and access is
> enforced per subject, filtered in every search and list. What is **not** production-grade
> yet is called out explicitly below — chiefly **no audit of sensitive reads**, **migrations
> run by the runtime role** and **node-local blob storage**. Read
> [Known limitations](#known-limitations) before exposing this to real data.

## 1. Database

PostgreSQL is required. The schema is created by embedded migrations at startup (see §2).

### Required extensions

Migration `0001_subject_core.sql` runs `CREATE EXTENSION IF NOT EXISTS` for:

| Extension  | Purpose                                              |
|------------|------------------------------------------------------|
| `pgcrypto` | `gen_random_uuid()` for subject/document identifiers |
| `pg_trgm`  | trigram indexing for search                          |
| `unaccent` | accent-insensitive full-text search (migration 0005) |
| `postgis`  | LV95 geometry of things (`thing.geometry`, GIST index) |

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

The server sets two session parameters on its own connections (nothing to configure on the
instance), both measured at production volume (GLD-053): `jit=off` (JIT compilation added
100–150 ms to searches that run in tens of milliseconds) and `plan_cache_mode=force_custom_plan`
(a generic plan of the optional-filter searches took a text search from 0.2 s to 2.3 s). After a
bulk load (`cmd/goeland-import` does it), run `ANALYZE`: without statistics the planner picks very
poor plans. Search totals are counted up to 10 000 matches (`totalSizeCapped` beyond); the
unscoped document search counts within the 20 000 newest documents. The read filter's worst case
is a table where the caller can read very few rows (it must scan far to fill a page): measured at
~1.6 s on ~2.3M documents with 81% unreadable; an access-aware search index would remove it.

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

Uploaded document bytes are written to a **local filesystem** directory and registered as
a `content_blob` (unique SHA-256, so identical content is stored once) referenced through an
`internal://…` storage ref:

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
  fails without it) and the `GOELAND_DEV_USER_*` identity fields (`GOELAND_DEV_USER_ADMIN=true`
  bootstraps the dev user as administrator). **Never enable in production.**

Every verified caller is recorded in `app_user` (operator id, display name, e-mail, first/last
seen) so governance and audit can show names: employee personal data, mirrored from the auth
service and never edited here; the audit log records name changes, not e-mail addresses.

**Administrators are decided in Goéland (GLD-047).** The token only authenticates: the
`goeland:admin` scope comes from the ADMIN application role (`app_user_role`, granted and revoked
with a reason, audited on the user, kept as history), and the auth server's `IsAdmin` flag is
ignored. The first administrators come from `GOELAND_BOOTSTRAP_ADMINS` (comma-separated user ids,
granted ADMIN on their next request as `system:bootstrap`); it is also the recovery path, since the
last administrator cannot be revoked. Role changes apply at once on the replica that made them and
within 30 s on the others (role cache).

`AUTH_SERVER_URL` must be a valid `http(s)` URL (PAT introspection + login redirect), and HTTPS
unless its host is loopback: personal access tokens would otherwise travel in clear.
`GOELAND_ALLOW_INSECURE_AUTH_URL=true` accepts plain http elsewhere (for example inside a cluster
whose service mesh encrypts the traffic). The auth service must list the SPA's public origin in
its redirect allowlist and CORS origins (README, "Running locally with SSO").

Every response carries browser security headers (`cmd/goeland-server/headers.go`): a
Content-Security-Policy (`'self'` only, inline styles for Vuetify, the `AUTH_SERVER_URL` origin
added to `connect-src` in `jwt` mode), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy`, `Cross-Origin-Opener-Policy` and `Permissions-Policy`. HSTS is left to the TLS
terminator (ingress); set it there.

## 5. Probes (Kubernetes)

| Probe      | Endpoint      | Behavior                                                        |
|------------|---------------|----------------------------------------------------------------|
| Liveness   | `GET /health` | Returns `{"status":"ok"}` and DB pool stats; process is up.    |
| Readiness  | `GET /readiness` | Pings the database; fails when the DB is unreachable.       |

The scratch image has no shell, so there is no container `HEALTHCHECK` — configure the
probes at the orchestration layer against the endpoints above. There is no separate
startup probe; because migrations run before the listener binds, size the readiness
`initialDelay`/`failureThreshold` to allow for migration time on first boot. At startup the
server waits for the database (`GOELAND_DB_CONNECT_TIMEOUT_SECONDS`, §6) instead of exiting,
so a pod started before its database does not crash-loop.

## 6. Server tuning

| Variable                            | Default          | Notes                              |
|-------------------------------------|------------------|------------------------------------|
| `GOELAND_LISTEN_ADDRESS`            | `127.0.0.1:8080` | Use `0.0.0.0:8080` in a container. |
| `GOELAND_REQUEST_TIMEOUT_SECONDS`   | `10`             | Per-request timeout (1–300).       |
| `GOELAND_SHUTDOWN_TIMEOUT_SECONDS`  | `10`             | Graceful drain window (1–300).     |
| `GOELAND_DB_CONNECT_TIMEOUT_SECONDS` | `60`            | Startup retries an unreachable database this long (0–600; 0 = one attempt); migrations and wiring then get 60 s more. SIGTERM stops a waiting startup. |
| `LOG_LEVEL`                         | `info`           | `debug` / `info` / `warn` / `error`. |
| `GOELAND_ALLOW_INSECURE_AUTH_URL`   | `false`          | Accept a plain-http `AUTH_SERVER_URL` outside loopback (§4). |
| `GOELAND_BOOTSTRAP_ADMINS`          | —                | User ids granted the ADMIN role on their next request (§4); at most 20. |

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

- **Sensitive reads are not audited.** Access is enforced on every mutation and read (GLD-048:
  levels per subject for users, groups and org units; confidential subjects need an explicit
  grant), searches and lists return only readable subjects, and document bytes are downloaded
  through `GET /api/documents/{id}/content` with READ on the document (GLD-049). Mutations are
  audited, but reading or downloading a confidential subject leaves no trace yet (GLD-033).
  Every authenticated caller still holds `goeland:read` and `goeland:write` as scopes.
- **Migrations run with the runtime role** (§2): the application role owns the DDL, so it could
  alter the schema or drop the append-only triggers, and a faulty migration blocks every new pod
  (GLD-046: a migration job with its own role).
- **Integrity verification is not probative.** `VerifyDocumentIntegrity` compares a digest with
  the recorded one without rereading the stored bytes (GLD-021).
- **Blob storage is node-local** (§3): not safe for multi-replica or ephemeral deployments
  without a shared/persistent volume, and bytes uploaded but never attached to a document are
  not collected (GLD-020, GLD-045).
- **Observability is logs only.** No metrics or tracing endpoints yet.
- **No production chart.** `deployments/k8s/` holds smoke-test manifests only (disposable
  PostGIS, `dev` auth, one replica), exercised by `scripts/k8s_smoke_test.sh`.
- **No rate limiting in the server.** Uploads are bounded in size (`GOELAND_MAX_UPLOAD_BYTES`),
  but request rates are not limited: an in-process limiter would be per pod, so this belongs to
  the ingress or API gateway in front of the replicas.

See [requirements/IMPLEMENTATION_STATUS.md](../requirements/IMPLEMENTATION_STATUS.md) for
the full implemented-vs-pending tracker.

## Verifying a deployment

`scripts/k8s_smoke_test.sh` deploys the published image with a disposable PostGIS on a local
cluster (Rancher Desktop / k3s) and checks the rollout, both probes, the version, the SPA and a
few authenticated API calls (see [deployments/k8s/README.md](../deployments/k8s/README.md)).

Run the database integration tests against a disposable PostGIS database to prove the
schema and the full document and actor lifecycles work end-to-end:

```bash
GOELAND_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5432/goeland_test?sslmode=disable' \
    go test ./pkg/integration/...
```

They are skipped when `GOELAND_TEST_DATABASE_URL` is unset, so the default `go test ./...`
run needs no database.
