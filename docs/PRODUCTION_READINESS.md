# Production Readiness

This page is the deployment contract for the Goéland POC server (`cmd/goeland-server`):
what the database must provide, how migrations behave, how blobs persist, how auth is
configured, which probes to wire, and which values are secrets.

> **Status: POC.** The server basics are production-shaped (timeouts, graceful shutdown,
> health/readiness, structured logs, request IDs, non-root scratch image) and access is
> enforced per subject, filtered in every search and list. What is **not** production-grade
> yet is called out explicitly below — chiefly **migrations
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

The case export archives (GLD-029) are written to the same store and removed when they expire
(`GOELAND_EXPORT_TTL_HOURS`, default 24, 1–720); the worker that builds them runs in every
replica (`GOELAND_EXPORT_POLL_SECONDS`, default 5, 1–3600), sharing the queue through the
database, so any replica may build an archive that another one serves: one more reason for a
shared volume.

The transactional outbox (GLD-028) needs no volume: its events live in `outbox_event`, and the
worker of every replica shares them through the database (`FOR UPDATE SKIP LOCKED` with a 5-minute
lease, so a replica that dies mid-delivery leaves its events to the others). Each worker keeps one
extra database connection, outside the pool, to `LISTEN` for new events. Watch the given-up events in
Administration › Outbox: they stay until replayed.

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
added to `connect-src` in `jwt` mode, and the City of Lausanne's map tiles
`https://tilesmn95.lausanne.ch` in `img-src` and `connect-src` for the object maps, GLD-069), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy`, `Cross-Origin-Opener-Policy` and `Permissions-Policy`. HSTS is left to the TLS
terminator (ingress); set it there.

## 5. Probes (Kubernetes)

| Probe      | Endpoint      | Behavior                                                        |
|------------|---------------|----------------------------------------------------------------|
| Liveness   | `GET /health` | Returns `{"status":"ok"}`; process is up (the pool statistics are metrics, §9). |
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
| `GOELAND_EXPORT_TTL_HOURS`          | `24`             | How long a case export archive stays downloadable (1–720, §3). |
| `GOELAND_EXPORT_POLL_SECONDS`       | `5`              | How often the export worker looks for queued exports and expired archives (1–3600). |
| `GOELAND_OUTBOX_RETENTION_DAYS`     | `7`              | How long delivered outbox events are kept before the purge (1–365, GLD-028). |
| `GOELAND_OUTBOX_POLL_SECONDS`       | `5`              | How often the outbox worker looks for due events besides its notifications (1–3600). |
| `GOELAND_OUTBOX_LOG`                | `false`          | Log every delivered outbox event (references only), to watch the outbox work. |
| `GOELAND_METRICS_ADDRESS`           | — (off)          | `host:port` of the Prometheus metrics listener (`GET /metrics`, GLD-010); never the public port. |

## 7. Secrets

Never bake these into images or ConfigMaps — deliver them via a Kubernetes `Secret` (or
your secret manager):

- `DATABASE_URL` or `DB_PASSWORD`
- `JWT_SECRET`
- `GOELAND_DEV_TOKEN` (dev mode only)

Non-secret settings can go in a ConfigMap. `scripts/create_k8s_configmap_from_env.sh`
renders only non-secret keys and never prints secret values; Secret creation is handled
separately.

## 8. Supply chain

- **Pinned build inputs (GLD-059):** the Dockerfile base images (`oven/bun`, `golang`) and the
  `postgis/postgis` image of CI and the smoke manifests are pinned by digest, third-party actions
  by commit SHA; `make build-inputs-check` (part of `make check`) refuses a bare tag and a Go
  builder older than `go.mod`. Dependabot (`.github/dependabot.yml`) proposes weekly grouped
  updates of the base images, actions, Go modules and SPA packages; a new Go minor stays a
  deliberate change. The `postgis/postgis` digest is moved by hand: resolve it with
  `docker buildx imagetools inspect postgis/postgis:<tag>` and update `ci.yml` and
  `deployments/k8s/10-postgis.yaml` together.
- **SBOM and licenses:** each GitHub release carries `goeland-server-sbom.cdx.json` (CycloneDX,
  with licenses) and `goeland-server-licenses.txt`, made by `scripts/sbom.sh` (`make sbom`
  locally) from `go.mod` and the SPA's runtime packages; the published image carries BuildKit
  SBOM and provenance attestations
  (`docker buildx imagetools inspect <image> --format '{{ json .SBOM }}'`).
- **Vulnerabilities:** `make vuln-check` (govulncheck, reachable code) gates every release and
  docker-publish scans the image with Trivy before pushing it (fixable HIGH/CRITICAL fail).

## 9. Metrics (Prometheus)

`GOELAND_METRICS_ADDRESS` (e.g. `0.0.0.0:9464`) opens a second listener serving `GET /metrics`;
empty, the metrics are off. It must use another port than the public listener: scrape it from
inside the cluster (the smoke manifests name the container port `metrics` and keep it out of the
Service). Series, all prefixed `goeland_`:

- `http_requests_total` and `http_request_duration_seconds` by `handler` (the RPC, e.g.
  `goeland.v1.CaseService/GetCase`, whether reached over REST or Connect; otherwise the route
  pattern such as `GET /health` or `/` for the SPA; `unmatched` for anything else), `method` and
  `code`: the labels never carry a URL, an id or a user;
- `db_pool_*`: connections in use, idle, open and maximum, acquire counts and time;
- `outbox_events{state}`, `outbox_oldest_pending_age_seconds`, `outbox_consumer_failed_events`,
  `case_export_jobs{status}`: read from the database at scrape time (2 s bound), the same on
  every replica; `backlog_scrape_error` turns 1 when such a read fails;
- `build_info{version,revision}`, plus the Go runtime and process collectors.

## Known limitations

These are the gaps that make this a POC rather than a production service:

- **The sensitive read audit has no retention policy.** Access is enforced on every mutation and
  read (GLD-048), searches and lists return only readable subjects, and document bytes are
  downloaded through `GET /api/documents/{id}/content` with READ on the document (GLD-049).
  Reading or downloading a confidential subject, or being refused it, is recorded in the
  append-only `access_audit_event` (GLD-033), but the table grows without limit: its legal
  retention period (and partitioning to purge it) must be decided with the data protection
  officer and the archivist before production. Ordinary reads and list results are not logged.
  Every authenticated caller still holds `goeland:read` and `goeland:write` as scopes.
- **Migrations run with the runtime role** (§2): the application role owns the DDL, so it could
  alter the schema or drop the append-only triggers, and a faulty migration blocks every new pod
  (GLD-046: a migration job with its own role).
- **Integrity verification is not probative.** `VerifyDocumentIntegrity` compares a digest with
  the recorded one without rereading the stored bytes (GLD-021).
- **Blob storage is node-local** (§3): not safe for multi-replica or ephemeral deployments
  without a shared/persistent volume, and bytes uploaded but never attached to a document are
  not collected (GLD-020, GLD-045).
- **No tracing yet.** Prometheus metrics are served on their own listener (§9); OpenTelemetry
  traces are the second step of GLD-010.
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
