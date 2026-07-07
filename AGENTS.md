# go-cloud-k8s-poc-2026 (Goéland POC) — Agent Instructions

Read this before making changes. It captures the conventions, the layers that
must stay in sync, and the non-obvious gotchas discovered while building the POC.

## Security

- Never copy, print, log, or commit real credentials, passwords, tokens, keys,
  cookies, or connection strings from `.env`, local config, or command output.
- Use obvious placeholders in examples: `<db-password>`, `<jwt-secret>`,
  `<dev-token>`, `pat_<redacted>`.
- Do not read or display secret-bearing environment files unless the task
  requires inspecting their structure, and redact all values.
- Values in `.env` must not be quoted: the Makefile includes and exports the
  file, so quotes become part of the value.
- `.env` and coverage files are Git-ignored, but that does not make their
  contents safe to expose.

## What this project is

A proto-first POC rebuilding the conceptual core of **Goéland** (territorial
administrative case management) from `../goeland_poc_domain_model_agent.md`.
It is **not** a re-code of legacy Goéland — it is a clean, durable core:

> a graph of durable business **subjects**, linked by **typed relationships**,
> animated by chronological history, protected by rights, made trustworthy by
> **auditability**.

Stack: proto-first (buf) · ConnectRPC + Vanguard · pgx raw SQL with `db:"..."`
tags · bundleable `pkg/<domain>/module` pattern · embedded dbmate migrations ·
PostGIS-ready from migration 0001.

### Implemented so far

- **core** (`pkg/core`) — transversal: `subject_ref`, `record_metadata`,
  `audit_event`, `relationship_type`, `subject_relationship` → `CoreService`.
- **document** (`pkg/document`) — modern GED entity → `DocumentService`.

### Not yet built (same foundation)

Case (`case_file` + timeline + circulation), Thing (parcelle/bâtiment with
PostGIS geometry), Actor, a real permission/confidentiality engine, storage
(MinIO), search (Meilisearch), workflow. Design new domains as first-class
subjects that reuse the core primitives.

## Key paths

```text
proto/goeland/v1/            core.proto, document.proto        (API contract, source of truth)
gen/goeland/v1/              generated Go + ConnectRPC          (never hand-edit)
api/openapi/                 generated OpenAPI (goeland.swagger.yaml, never hand-edit)
pkg/version/                 build/version metadata
pkg/authadapter/             JWT + PAT + dev token verification (shared, ecosystem-wide)
pkg/core/                    transversal domain
  ├── tx.go                  exported tx-scoped helpers reused by sibling domains
  ├── module/                bundleable module + OWNS the full schema bootstrap
  │   └── db/migrations/     0001..0005 (dbmate format)
pkg/document/                document domain (reuses core primitives)
  └── module/                bundleable module (NO migrations; core owns schema)
cmd/goeland-server/          server: pool → migrate → wire both modules → one shared transcoder
```

## Commands

- `make run` — `buf generate` + run the server from source (default `127.0.0.1:8080`).
- `make build` — test + build `bin/goeland-server`.
- `make test` — all Go tests with the race detector + `coverage.out`.
- `make lint` — `go vet ./...` + `buf lint`.
- `make fmt` — `gofmt -w .` (repo-wide; prefer `gofmt -w` on touched files only).
- `make generate` — lint protos, update buf deps, regenerate Go/Connect/OpenAPI.
- `make db-status | db-up | db-down` — dbmate against `.env`.
- `make db-new name=add_case` — scaffold a new migration.

There is no frontend in this project.

Helper scripts live in `scripts/` (all run from the repo root; documented in the
README's Scripts table). Notable ones: `createLocalDBAndUser.sh` (creates the
role/db and enables pgcrypto/pg_trgm/unaccent/postgis as admin so the app role's
`CREATE EXTENSION IF NOT EXISTS` is a privilege-free no-op),
`install_go_protobuf_tools.sh`, `getAppInfo.sh` (sourced by build/release scripts
to read `pkg/version/version.go`), `GoRunWithEnv.sh` / `GoTestWithEnv.sh` /
`execWithEnv.sh` (dotenv-loading wrappers), `get_jwt_token.sh`,
`01_build_image_locally.sh`, `02_tag_new_release_github.sh`. Keep scripts POSIX-
bash, `set -euo pipefail` where practical, and never echo secret values.

## Authentication

Reuses `pkg/authadapter`. `GOELAND_AUTH_MODE`:

- `jwt` (default): non-`pat_` bearer tokens verified locally (needs `JWT_SECRET`,
  `JWT_ISSUER_ID`, `JWT_CONTEXT_KEY`); `pat_` tokens introspected against
  `<AUTH_SERVER_URL>/goapi/v1/auth/introspect` (cached ~60s).
- `dev`: accepts `GOELAND_DEV_TOKEN` (required in dev mode) for one user
  (`GOELAND_DEV_USER_ID` / `_EMAIL` / `_NAME`).

Scopes: `goeland:read` (read RPCs), `goeland:write` (mutations). The audit
actor is the request's `actor_user_id` when set, else the authenticated app
user id. Env vars are `GOELAND_*`; DB vars are `DB_*` / `DATABASE_URL`.

## API routing & the Connect-over-curl gotcha

RPC paths are `/<fully-qualified-service>/<Method>`:

```text
/goeland.v1.CoreService/<Method>
/goeland.v1.DocumentService/<Method>
```

Plus `/health`, `/readiness`, `/goAppInfo`.

⚠️ **Unary Connect JSON requests MUST send `Connect-Protocol-Version: 1`.**
There are no `google.api.http` annotations, so Vanguard exposes no REST routes;
without that header it treats an `application/json` POST as a (non-existent)
REST route and returns **404 "Not Found"** (10-byte body — that is Vanguard, not
the Go mux, whose 404 is "404 page not found"). Real Connect clients send the
header automatically; only hand-rolled curl needs it added.

Example:

```bash
curl -s \
  -H 'Authorization: Bearer <dev-token>' \
  -H 'Content-Type: application/json' \
  -H 'Connect-Protocol-Version: 1' \
  -d '{"onlyActive":true}' \
  http://127.0.0.1:8088/goeland.v1.DocumentService/ListDocumentTypes
```

## Generated code and protobuf

- `proto/` is the source of truth. Never hand-edit `gen/` or `api/openapi/`.
- After changing protos, run `make generate` then `make lint`.
- `make generate` may change `buf.lock`, `gen/`, `api/openapi/` — review and
  commit intended generated changes together.
- Requires `buf`, local `protoc-gen-go` + `protoc-gen-connect-go`, and the
  remote OpenAPI plugin.
- protovalidate's `ignore` enum in the pinned buf schema is
  **`IGNORE_IF_ZERO_VALUE`** (not `IGNORE_IF_UNPOPULATED`). It is used on
  `RecordMetadata.subject_id` so that message doubles as an input (initial
  governance, server-assigned id) without tripping the `uuid` rule. Request
  validation runs via `connectvalidate.NewInterceptor()`; responses are not
  validated.

## Keeping layers in sync

The proto generates client/server types only. These layers are **not**
generated and must be updated by hand when the contract changes:

- DB schema: `pkg/core/module/db/migrations/*.sql` (add a new migration; never
  rewrite an applied one).
- Raw SQL + column projections: `pkg/<domain>/sql.go` (the `db.`-prefixed
  projections; see the INSERT-alias note below).
- Domain model: `pkg/<domain>/model.go` — the `db:"..."` tags drive pgx named
  scanning (`RowTo*ByNameLax`). Nullable columns use pointer fields.
- Mappers: `pkg/<domain>/mappers.go` (domain ↔ proto).
- Business rules/normalization: `pkg/<domain>/service.go`.
- Wire adapters: `pkg/<domain>/connect_server.go`.

### Checklist: adding a field to Document (or similar)

1. Edit `proto/goeland/v1/document.proto` (message + validation).
2. `make generate` + `make lint`.
3. Add a new migration under `pkg/core/module/db/migrations/`.
4. Update the column projection + affected DML in `pkg/document/sql.go`.
5. Add the field + `db` tag to the struct in `pkg/document/model.go`
   (and to `CreateInput`/`UpdateInput` if user-supplied).
6. Update `DomainToProto` (and any input parsing) in `mappers.go`.
7. Update normalization/defaults in `service.go`.
8. Wire it in `connect_server.go`.
9. Add/adjust tests; run `go test ./pkg/... -count=1` then `make lint`.

Named scanning means you usually do **not** touch individual `Scan(...)` calls —
only the struct + SQL projection.

## SQL conventions (learned the hard way)

- Column projections are `d.`/table-alias-prefixed so they can be shared between
  single-table queries and JOINs. Because of this, **`INSERT` statements must
  alias the target** (`INSERT INTO document AS d (...) ... RETURNING <d.cols>`),
  otherwise `RETURNING d.col` fails with `missing FROM-clause entry for table "d"`.
- Non-destructive by rule: never physically delete domain records — use
  `record_metadata.deleted_at` / soft-delete relationships. Every mutation must
  write an `audit_event` (in the same transaction as the mutation).
- Relationships are typed and validated: source/target kinds must match the
  `relationship_type`; an active-edge partial unique index enforces uniqueness.
- `document.search_vector` is a Postgres **`GENERATED ALWAYS AS (...) STORED`**
  column — the app must NOT insert/scan it; Postgres computes it on every
  write. It is accent-folded via `immutable_unaccent()` (migration 0005), an
  IMMUTABLE wrapper around `unaccent()` (the raw `unaccent()` is only STABLE and
  cannot be used in a generated column/index). `searchDocumentsSQL` folds the
  query term through the same wrapper so search is accent-insensitive.

## Database migrations

- All migration SQL lives in **one** place: `pkg/core/module/db/migrations/`,
  embedded in the core module and applied at startup via `coremodule.Migrate`
  under a PG advisory lock keyed to `'go-cloud-k8s-poc-2026:migrations'`.
- **The core module owns the full schema bootstrap** (core *and* document
  tables + seed) because document tables have foreign keys into core tables.
  The document module has no migrations. As the POC grows this can be split.
- dbmate and the embedded migrator share the same `schema_migrations` version
  keys (`0001`…, the prefix before the first `_`), so the two paths are
  interchangeable. `make db-up` is optional; the server self-migrates.
- Zero-padded sequential names with `-- migrate:up` / `-- migrate:down`.
  Wrap PL/pgSQL (functions/triggers) in `-- migrate:statementbegin/end`.
- Never rewrite an applied migration — add a new one.
- `make db-down` rolls back the latest migration; the `0004` seed-down `DELETE`
  will fail if documents still reference a `document_type`, so a clean rollback
  assumes no domain data — expected for a reset, not a bug.
- Requires PostGIS, pgcrypto, pg_trgm, unaccent available on the server.
- **Never** run migrations or a migrating server startup against an unknown,
  shared, staging, or production database without explicit approval and verified
  config.

## Module & bundle architecture

Each `pkg/<domain>/module` is an importable Go package with two modes:

- **Standalone**: `RegisterRoutes(mux)` builds a transcoder for that module.
- **Bundle** (what `cmd/goeland-server` does): collect `VanguardServices()` from
  every module, build **one** shared `vanguard.NewTranscoder`, mount per service
  name. Modules share one `*pgxpool.Pool`, `TokenVerifier`, and `*slog.Logger`.

Rules:

- `New` validates deps at construction (`Pool`, `Verifier` required; the document
  module also requires `CoreService`; a nil `Logger` falls back to `slog.Default`).
- The interceptor chain (timeout → auth → protovalidate) is assembled once in
  `connectOption()`. Do not duplicate or reorder it.
- The document domain composes atomic transactions using the exported core
  helpers in `pkg/core/tx.go` (`InsertSubjectRefTx`, `InsertRecordMetadataTx`,
  `InsertAuditEventTx`, `LinkSubjectsTx`, …) so identity + governance + entity +
  audit are created in one tx.
- Dependency flow is one-way: `cmd` → `pkg`. Never import `cmd` from `pkg`.

## Testing & change discipline

- Keep changes scoped; follow existing package boundaries and patterns.
- Add/update tests for behavioral changes; prefer focused `go test ./path/...`.
  DB-touching integration tests need a live PostgreSQL (there are none yet;
  current tests are pure unit tests: pagination, kind validation, migration
  parser).
- Run `make lint` for Go or protobuf changes.
- Do not hand-edit generated files to make tests pass.
- Do not revert unrelated work in a dirty worktree.

## Design rules (from the domain spec §17 — keep honouring them)

Explicit domain model (no generic EAV); JSONB only for secondary data; critical
fields as typed columns; non-destructive deletes; every mutation audited; every
relationship typed and validated; boring, explicit, testable API; prefer clear
domain names over technical abstractions.
