# go-cloud-k8s-poc-2026 — Goéland POC

A modern, proto-first POC rebuilding the conceptual core of **Goéland** (territorial
administrative case management) as a clean, durable domain model:

> a graph of durable business **subjects**, linked by **typed relationships**,
> animated by chronological history, protected by rights, and made trustworthy by
> **auditability**.

This first slice implements the **transversal core** (subjects, governance,
relationships, audit) and the **Document** component (a modern GED entity), keeping
the proto-first Go / gRPC / ConnectRPC / PostgreSQL approach and the structural
conventions of `go-cloud-k8s-thing` + `go-mcp-markdown-notes`.

## Architecture at a glance

- **Proto-first** API defined in `proto/goeland/v1/` (`core.proto`, `document.proto`),
  generated with **buf** into `gen/`.
- **ConnectRPC + Vanguard**: each service is a Connect handler wrapped in a Vanguard
  transcoder, reachable over Connect, gRPC and gRPC-Web on the standard RPC path.
- **protovalidate** enforces request validation at the edge (declarative rules in the proto).
- **pgx (raw SQL)** with `db:"..."` struct tags and named row scanning — no ORM.
- **Bundleable module pattern** (`pkg/<domain>/module`): each domain exposes
  `VanguardServices()` so it can run standalone or be composed into one shared
  `http.Server` / DB pool / transcoder / auth verifier.
- **Embedded, dbmate-compatible migrations** (numbered, commented).
- **PostGIS-ready** from the first migration (extension enabled up-front for the
  future THING component; no geometry columns yet).
- **Auth** reuses the ecosystem's `authadapter` (JWT from `go-cloud-k8s-auth` +
  personal access token introspection, plus a `dev` mode).

## Domain model (this slice)

Transversal core (`pkg/core`):

| Table                  | Purpose |
|------------------------|---------|
| `subject_kind`         | Controlled list: CASE, DOCUMENT, THING, ACTOR, USER, ORG_UNIT |
| `subject_ref`          | Canonical identity of every subject (one row per Case/Document/Thing/Actor) |
| `record_metadata`      | 1:1 governance: ownership, confidentiality, soft-delete, locking, versioning |
| `audit_event`          | Append-only probative history (written on every mutation) |
| `relationship_type`    | Catalogue of allowed typed edges (with source/target kind constraints) |
| `subject_relationship` | Actual typed, validated, soft-deletable graph edges |

Document component (`pkg/document`), a first-class subject (`document.id` **is** a
`subject_ref.id` of kind DOCUMENT, pinned by a composite FK):

- cryptographic integrity (`sha256` + `sha256_verified_at`);
- no-duplication external references (`external_system` / `external_id` / `external_url` / `storage_ref`);
- versioning (`version` + `previous_version_id`);
- records-management prep (`is_final`, `is_record`, `status`, governance locking);
- full-text search via a generated `search_vector` (`tsvector`) + GIN index;
- controlled classification (`document_type`).

Every mutation is **non-destructive** (logical delete via `record_metadata.deleted_at`)
and writes an `audit_event`. Finalizing/locking a document makes it immutable.

## Project structure

```
proto/goeland/v1/        core.proto, document.proto        (API contract)
gen/goeland/v1/          generated Go + ConnectRPC          (do not edit)
api/openapi/             generated OpenAPI (goeland.swagger.yaml)
pkg/version/             build/version metadata
pkg/authadapter/         JWT + PAT + dev token verification (shared)
pkg/core/                transversal domain: model, sql, storage, service, mappers, connect_server
  └── module/            bundleable module + embedded migrations (owns schema bootstrap)
      └── db/migrations/  0001..0004 (dbmate format)
pkg/document/            document domain (reuses core primitives)
  └── module/            bundleable module (schema owned by core)
cmd/goeland-server/      server: pool, migrate, wire modules onto one shared transcoder
```

## Prerequisites

- Go 1.26+
- PostgreSQL 14+ with **PostGIS**, **pgcrypto** and **pg_trgm** available
  (e.g. the `postgis/postgis` image, or `apt install postgresql-16-postgis-3`)
- [`buf`](https://buf.build) (regenerate code), [`dbmate`](https://github.com/amacneil/dbmate) (optional, for CLI migrations)

## Setup

```bash
cp .env_sample .env          # then edit DB_PASSWORD / DATABASE_URL
make db-up                   # apply migrations with dbmate (optional; the server also self-migrates)
make run                     # buf generate + go run ./cmd/goeland-server
```

The server **migrates on startup** (embedded migrations, guarded by a PG advisory
lock), so `make db-up` is optional. Both paths share the same `schema_migrations`
version keys (`0001`…), so they are interchangeable.

## Running locally (dev auth)

```bash
GOELAND_AUTH_MODE=dev GOELAND_DEV_TOKEN=devtoken GOELAND_DEV_USER_ID=42 \
GOELAND_LISTEN_ADDRESS=127.0.0.1:8088 \
go run ./cmd/goeland-server
```

Health: `curl http://127.0.0.1:8088/health` · info: `/goAppInfo` · readiness: `/readiness`.

## Calling the API

Services (fully-qualified names, path = `/<service>/<Method>`):

- `goeland.v1.CoreService` — `CreateSubjectRef`, `GetSubjectRef`, `LinkSubjects`,
  `UnlinkSubjects`, `ListRelationships`, `ListRelationshipTypes`, `ListAuditEvents`
- `goeland.v1.DocumentService` — `CreateDocument`, `GetDocument`,
  `UpdateDocumentMetadata`, `FinalizeDocument`, `VerifyDocumentIntegrity`,
  `SearchDocuments`, `LinkDocument`, `DeleteDocument`, `ListDocumentTypes`

> **Connect over curl:** unary Connect JSON requests must send the
> `Connect-Protocol-Version: 1` header, otherwise Vanguard treats the
> `application/json` POST as a (non-existent) REST route and returns 404.

```bash
BASE=http://127.0.0.1:8088
H=(-H 'Authorization: Bearer devtoken' -H 'Content-Type: application/json' -H 'Connect-Protocol-Version: 1')

# List seeded document types
curl -s "${H[@]}" -d '{"onlyActive":true}' $BASE/goeland.v1.DocumentService/ListDocumentTypes

# Create a document (creates subject_ref + record_metadata + audit atomically)
curl -s "${H[@]}" -d '{
  "documentTypeCode":"PLAN","title":"Plan de masse v1",
  "storageRef":"minio://plans/plan-1234.pdf","mimeType":"application/pdf",
  "sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "initialGovernance":{"confidentialityLevel":1,"ownerOrgId":"OPC"}
}' $BASE/goeland.v1.DocumentService/CreateDocument

# Full-text search
curl -s "${H[@]}" -d '{"query":"masse"}' $BASE/goeland.v1.DocumentService/SearchDocuments

# Finalize + lock (document becomes immutable; later updates return failed_precondition)
curl -s "${H[@]}" -d '{"id":"<DOC_ID>","reason":"signed","alsoLockGovernance":true}' \
  $BASE/goeland.v1.DocumentService/FinalizeDocument
```

## Migrations

Numbered, commented dbmate files in `pkg/core/module/db/migrations/`:

```
0001_subject_core.sql        extensions + subject_kind/subject_ref/record_metadata/audit_event
0002_relationships.sql       relationship_type + subject_relationship
0003_document.sql            document_type + document (+ generated search_vector, trigger)
0004_seed_reference_data.sql seed subject kinds, relationship types, document types
```

The **core module owns the full schema bootstrap** for this POC because the document
tables have foreign keys into the core tables. As the POC grows (Case, Thing, Actor),
migrations can be split per module.

```bash
make db-status   # dbmate status
make db-up       # apply pending
make db-down     # roll back latest
make db-new name=add_case   # scaffold a new migration
```

## Common make targets

```
make run        generate + run the server
make generate   buf lint + generate (Go, ConnectRPC, OpenAPI)
make build      test + compile bin/goeland-server
make test       go test -race with coverage
make lint       go vet + buf lint
make fmt        gofmt -w .
make db-up      apply migrations (dbmate)
```

## Scripts (`scripts/`)

Helper scripts for the dev loop and ops (all run from the repo root):

| Script | Purpose |
|--------|---------|
| `createLocalDBAndUser.sh <name>` | Create a local role + database, enable pgcrypto/pg_trgm/postgis (as admin), and write `.env` |
| `install_go_protobuf_tools.sh`   | Install `buf`, `protoc-gen-go`, `protoc-gen-connect-go` |
| `buf_generate.sh`                | `buf lint` + `buf dep update` + `buf generate` (used by `make generate`) |
| `getAppInfo.sh`                  | Export `APP_NAME`/`APP_VERSION`/… from `pkg/version/version.go` (sourced by other scripts) |
| `GoRunWithEnv.sh [main] [env]`   | `go run` the server with version ldflags + a dotenv loaded |
| `GoTestWithEnv.sh [env]`         | `go test -race` with coverage + a dotenv loaded |
| `execWithEnv.sh <bin> [env]`     | Run a compiled binary with a dotenv loaded |
| `get_jwt_token.sh [env]`         | Fetch a JWT from go-cloud-k8s-auth (jwt mode testing) |
| `01_build_image_locally.sh`      | Build the container image, tagged from `version.go` (optional trivy scan) |
| `02_tag_new_release_github.sh`   | Tag + push `v<version>` (refuses a dirty tree) |
| `create_k8s_configmap_from_env.sh` | Render a k8s ConfigMap from `.env` (dry-run) |

## Design rules honoured

Explicit domain model (no generic EAV); JSONB only for secondary data; critical
fields as typed columns; non-destructive deletes; every mutation writes an audit
event; every relationship is typed and validated; boring, explicit, testable API.

## Roadmap (out of scope for this slice)

Case (`case_file` + timeline + circulation), Thing (parcelle/bâtiment with PostGIS
geometry), Actor, a real permission/confidentiality engine, MinIO storage, Meilisearch,
and workflow integration — all designed to sit on top of the same subject/relationship/
audit foundation.
