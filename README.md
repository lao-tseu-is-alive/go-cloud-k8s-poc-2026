# go-cloud-k8s-poc-2026 — Goéland POC

[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=lao-tseu-is-alive_go-cloud-k8s-poc-2026&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=lao-tseu-is-alive_go-cloud-k8s-poc-2026)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=lao-tseu-is-alive_go-cloud-k8s-poc-2026&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=lao-tseu-is-alive_go-cloud-k8s-poc-2026)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=lao-tseu-is-alive_go-cloud-k8s-poc-2026&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=lao-tseu-is-alive_go-cloud-k8s-poc-2026)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=lao-tseu-is-alive_go-cloud-k8s-poc-2026&metric=vulnerabilities)](https://sonarcloud.io/summary/new_code?id=lao-tseu-is-alive_go-cloud-k8s-poc-2026)
[![cve-trivy-scan](https://github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/actions/workflows/cve-trivy-scan.yml/badge.svg)](https://github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/actions/workflows/cve-trivy-scan.yml)
[![CI](https://github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/actions/workflows/ci.yml/badge.svg)](https://github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/actions/workflows/ci.yml)

Current version: **v0.6.0** — pre-1.0 POC.


A modern, proto-first POC rebuilding the conceptual core of **Goéland** (territorial
administrative case management) as a clean, durable domain model:

> a graph of durable business **subjects**, linked by **typed relationships**,
> animated by chronological history, protected by rights, and made trustworthy by
> **auditability**.

This first slice implements the **transversal core** (subjects, governance,
relationships, audit), the **Document** component (a modern GED entity) and the
**Actor** component (external persons & organizations), keeping the proto-first Go /
gRPC / ConnectRPC / PostgreSQL approach and the structural conventions of
`go-cloud-k8s-thing` + `go-mcp-markdown-notes`.

The [documentation quality contract](docs/DOCUMENTATION.md) applies to human and agent
contributors alike: GoDoc and Protobuf contract comments, a file-by-file repository atlas
and executable claims, all enforced by `make docs-check` inside `make check`, CI and the
release gate.

## Architecture at a glance

- **Proto-first** API defined in `proto/goeland/v1/` (`core.proto`, `document.proto`,
  `actor.proto`), generated with **buf** into `gen/`.
- **ConnectRPC + Vanguard**: each service is a Connect handler wrapped in a Vanguard
  transcoder — reachable over Connect, gRPC and gRPC-Web on the RPC path, **and** as
  REST/JSON via `google.api.http` annotations (documented in the generated OpenAPI).
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
- **Embedded web UI**: a **Vue 3 + Vuetify 4** SPA (`cmd/goeland-server/goeland-front`,
  Vite/bun) `//go:embed`ded into the binary and served at `/` — the Document and
  Actor modules are exercisable end-to-end from the browser (search/create/detail/edit
  + lifecycle actions, plus read-only governance & audit).

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

- **Document / DocumentVersion / ContentBlob** (spec v2 §15-22): the document is the logical
  business object, `document_version` its dated, append-only states (final/record versions are
  immutable, enforced by a trigger), `content_blob` the bytes identified by a **unique SHA-256** —
  identical content is stored once, and creating a document for content already held by a live
  document **reuses that document** (the new context is a relationship);
- cryptographic integrity metadata (server-computed SHA-256; `VerifyDocumentIntegrity` is a
  non-mutating, non-probative stored-hash comparison — real streamed hashing is GLD-021);
- no-duplication external references (`external_system` / `external_id` / `external_url`);
- records-management prep (per-version `is_final` / `is_record`, `status`, governance locking);
- accent-insensitive full-text search via a generated `search_vector` (`tsvector`,
  accent-folded with `unaccent`) + GIN index — "chateau" matches "château";
- controlled classification (`document_type`).

Actor component (`pkg/actor`), also a first-class subject (`actor.id` **is** a
`subject_ref.id` of kind ACTOR), modelled from the real production `Acteur` schema:

- `actor_kind` discriminates a physical **PERSON** from a moral **ORGANIZATION**
  (flattened `ActMoral` / `ActPhys*` specialization, guarded by DB CHECK constraints);
- organization fields (`legal_name`, `organization_category` classification, complement);
- a **minimal identity** for persons — salutation, last and first name (the display name is
  derived as "<first> <last>" when blank) plus an `is_ch_register` flag and opaque
  `ch_register_ref`; no birth date, AVS number or other civil-registry data;
- typed `actor_contact` channels + business identifiers (IDE fédéral, TVA, débiteur
  ABACUS, registre du commerce), kept queryable rather than in a JSON blob;
- accent-insensitive name search via a generated `search_vector`;
- business `is_active` deactivation, distinct from soft-delete.

Roles are **not** columns on the actor: actors attach to cases/documents/things only
through typed `relationship_type` edges (`CASE_HAS_ACTOR_*`, `DOCUMENT_*_ACTOR`), so the
production role vocabulary grows with the Case/Thing slices.

Every mutation is **non-destructive** (logical delete via `record_metadata.deleted_at`)
and writes an `audit_event`. Finalizing/locking a document makes it immutable.

## Project structure

```
proto/goeland/v1/        core.proto, document.proto, actor.proto, case.proto  (API contract)
gen/goeland/v1/          generated Go + ConnectRPC          (do not edit)
api/openapi/             generated OpenAPI (goeland.swagger.yaml, from google.api.http)
pkg/version/             build/version metadata
pkg/authadapter/         JWT + PAT + dev token verification (shared)
pkg/core/                transversal domain: model, sql, storage, service, mappers, connect_server
  └── module/            bundleable module + embedded migrations (owns schema bootstrap)
      └── db/migrations/  0001..0012 (dbmate format)
pkg/document/            document domain (reuses core primitives)
  └── module/            bundleable module (schema owned by core)
pkg/blobstore/           content-bytes contract (Put/Get/Delete); filestore/ = local implementation,
                         blobstoretest/ = conformance suite for any implementation (S3 later)
pkg/actor/               actor domain: persons & organizations (reuses core primitives)
  └── module/            bundleable module (schema owned by core)
pkg/casefile/            case (affaire) domain: case types, status lifecycle, search
  └── module/            bundleable module (schema owned by core)
pkg/integration/         env-gated PostgreSQL integration tests (migrations + document/actor lifecycle)
cmd/goeland-server/      server: pool, migrate, wire modules onto one shared transcoder
  ├── upload.go          out-of-proto POST /upload + GET /download endpoints
  └── goeland-front/     Vue 3 + Vuetify 4 SPA (Vite/bun); dist/ is //go:embed'd (gitignored)
cmd/doccheck/            documentation checker (GoDoc coverage + exact atlas inventory)
.github/workflows/       CI gate, Trivy CVE scan, image build/scan/publish, binary release
docs/                    DOCUMENTATION.md (doc contract), PRODUCTION_READINESS.md (deployment contract)
```

## Prerequisites

- Go 1.27+
- PostgreSQL 14+ with **PostGIS**, **pgcrypto**, **pg_trgm** and **unaccent** available
  (e.g. the `postgis/postgis` image, or `apt install postgresql-16-postgis-3`)
- [`buf`](https://buf.build) (regenerate code), [`dbmate`](https://github.com/amacneil/dbmate) (optional, for CLI migrations)
- [`bun`](https://bun.sh) to build the embedded frontend (`make front-build`); a
  clean `go build`/`go test` needs `goeland-front/dist/` present (it is gitignored)

## Setup

```bash
cp .env_sample .env          # then edit DB_PASSWORD / DATABASE_URL
make db-up                   # apply migrations with dbmate (optional; the server also self-migrates)
make run                     # buf generate + build the embedded frontend + go run ./cmd/goeland-server
```

`make run` (and `make build`) depend on `make front-build` (`bun install && bun run
build`), which produces the `goeland-front/dist/` bundle embedded via `//go:embed`.
Once the server is up, open <http://127.0.0.1:8088/> for the web UI.

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
Add `GOELAND_DEV_USER_NAME='Jane Doe' GOELAND_DEV_USER_ADMIN=true` to sign in as a named
administrator; governance and audit then show that name (users are recorded from their token).

## Running locally with SSO (jwt auth)

In `jwt` mode the SPA signs in through [go-cloud-k8s-auth](https://github.com/lao-tseu-is-alive/go-cloud-k8s-auth)
(default `http://localhost:9090`): **Sign in** redirects to `<AUTH_SERVER_URL>/auth/login`, the
auth service sets its SSO session cookie and redirects back, then the SPA silently mints short-lived
JWTs from `<AUTH_SERVER_URL>/auth/token` (cookie sent with `credentials: include`).

1. Run go-cloud-k8s-auth (`make run` in its repository). On its side:
   - `ALLOWED_REDIRECT_URIS` must contain the SPA origin, e.g. `http://localhost:8080`
     (a prefix match on a path boundary; otherwise it logs `login: redirect_uri not in allowlist`);
   - `ALLOWED_ORIGINS` (CORS) must contain the same origin, for the token and logout calls.
2. Configure this server (`.env`): `GOELAND_AUTH_MODE=jwt`, `AUTH_SERVER_URL=http://localhost:9090`,
   and `JWT_SECRET`, `JWT_ISSUER_ID`, `JWT_CONTEXT_KEY` with **the same values as the auth
   service**, so the tokens it signs verify here (placeholders only in examples: `<jwt-secret>`).
3. Open the SPA on **`http://localhost:8080`**, not `http://127.0.0.1:8080`, even though the server
   listens on `127.0.0.1:8080` by default: `localhost` and `127.0.0.1` are different origins for the
   allowlist, CORS and cookies. The sign-in panel detects this mismatch and offers the right link.

Scopes: reads need `goeland:read`, mutations `goeland:write` (granted by the auth service).

## Web UI

Open <http://127.0.0.1:8088/> for the embedded **Vue 3 + Vuetify 4** SPA
(`cmd/goeland-server/goeland-front`). It exposes vertical slices of three modules:
the **Case** module — search/list, open (reference allocated in the type namespace), detail,
edit, status transitions with reasons, link/end/unlink subjects, soft-delete — the **Document** module — search/list, create (with file upload), detail, edit metadata,
finalize, verify integrity, link/unlink subjects, soft-delete — and the **Actor** module
— search/list, create (person/organization with typed contacts), detail, edit,
activate/deactivate, soft-delete, plus read-only incoming relationships. All add
read-only governance and audit panels. Bilingual (fr-CH default, en).

- The SPA reads `GET /config` → `{authMode, authBaseUrl}` and drives either `dev`
  (manual static token) or `jwt` (silent-mint from the `go-cloud-k8s-auth` SSO
  session) auth.
- It talks to the server over the **REST/JSON** `/api/...` bindings (plain `fetch`,
  no generated client).
- **File upload is metadata-first.** Binary bytes never travel through the proto
  contract: two out-of-proto HTTP endpoints carry their own bearer **and scope** check.
  `POST /api/documents/upload` (multipart, field `file`, `goeland:write`) stores the bytes,
  computes SHA-256/size/mime server-side and registers a **deduplicated** `content_blob`
  (identical content returns the existing blob, `reused: true`, and the new bytes are
  discarded); the UI passes the returned `contentBlobId` to `CreateDocument` or
  `AddDocumentVersion`. `GET /api/documents/download?ref=…` (`goeland:read`) streams a blob back. Blobs live under `GOELAND_DOCUMENT_PATH` (default
  `./go_documents`, gitignored); a single upload is capped by `GOELAND_MAX_UPLOAD_BYTES`
  (default 100 MiB).

Dev workflow: run the Vite dev server with `cd cmd/goeland-server/goeland-front &&
bun run dev` (HMR) while the Go server runs separately, or `make run` to rebuild the
embedded bundle and serve it from the binary.

## Calling the API

Every RPC is reachable two ways via the Vanguard transcoder:

1. **REST/JSON** (from `google.api.http` annotations) — plain HTTP, documented in
   `api/openapi/goeland.swagger.yaml`. **No special header needed.**
2. **Connect / gRPC / gRPC-Web** on `/<fully-qualified-service>/<Method>`.

Services:

- `goeland.v1.CoreService` — `CreateSubjectRef`, `GetSubjectRef`, `LinkSubjects`,
  `UnlinkSubjects`, `ListRelationships`, `ListRelationshipTypes`, `ListAuditEvents`
- `goeland.v1.DocumentService` — `CreateDocument`, `GetDocument`,
  `UpdateDocumentMetadata`, `FinalizeDocument`, `VerifyDocumentIntegrity`,
  `SearchDocuments`, `LinkDocument`, `DeleteDocument`, `ListDocumentTypes`
- `goeland.v1.ActorService` — `CreateActor`, `GetActor`, `UpdateActor`,
  `SearchActors`, `DeleteActor`, `ListOrganizationCategories`

### REST (recommended for curl / browsers)

All three services are annotated (see `api/openapi/goeland.swagger.yaml` for the full contract).

DocumentService: `GET /api/document-types` · `POST /api/documents` · `GET /api/documents/{id}` ·
`PATCH /api/documents/{id}` · `POST /api/documents/{id}/finalize` ·
`GET /api/documents/{id}/integrity` · `GET /api/documents/search` ·
`POST /api/documents/{id}/links` · `DELETE /api/documents/{id}` ·
`POST /api/documents/{documentId}/versions` · `GET /api/documents/{documentId}/versions`. Plus two
**out-of-proto** binary endpoints (see [Web UI](#web-ui)):
`POST /api/documents/upload` and `GET /api/documents/download`.

ActorService: `GET /api/organization-categories` · `POST /api/actors` ·
`GET /api/actors/{id}` · `PATCH /api/actors/{id}` · `GET /api/actors/search` ·
`DELETE /api/actors/{id}`.

CoreService: `POST /api/subjects` · `GET /api/subjects/{id}` · `POST /api/relationships` ·
`DELETE /api/relationships/{relationshipId}` · `GET /api/subjects/{subjectId}/relationships` ·
`GET /api/relationship-types` · `GET /api/subjects/{subjectId}/audit` ·
`POST /api/subjects/{subjectId}/business-ref` · `GET /api/subjects:lookup?businessRef=…&namespace=…`.

```bash
BASE=http://127.0.0.1:8088
AUTH='Authorization: Bearer devtoken'

# List seeded document types
curl -s -H "$AUTH" "$BASE/api/document-types?onlyActive=true"

# Upload the bytes (server computes SHA-256, deduplicates, returns contentBlobId)
BLOB=$(curl -s -H "$AUTH" -F file=@plan-1234.pdf "$BASE/api/documents/upload" | jq -r .contentBlobId)

# Create a document with version 1 (subject_ref + record_metadata + version + audit atomically)
curl -s -H "$AUTH" -H 'Content-Type: application/json' -d '{
  "documentTypeCode":"PLAN","title":"Plan de masse v1","contentBlobId":"'"$BLOB"'",
  "initialGovernance":{"confidentialityLevel":1,"ownerOrgId":"OPC"}
}' "$BASE/api/documents"

# Full-text search + finalize
curl -s -H "$AUTH" "$BASE/api/documents/search?query=masse"
curl -s -H "$AUTH" -H 'Content-Type: application/json' -d '{"reason":"signed","alsoLockGovernance":true}' \
  "$BASE/api/documents/<DOC_ID>/finalize"
```

### Connect / gRPC (RPC path)

> **Connect over curl:** unary Connect JSON requests on the RPC path must send the
> `Connect-Protocol-Version: 1` header (this is a Connect-protocol requirement, and
> only applies to the `/goeland.v1.*` RPC path — the REST `/api/*` paths above do not
> need it).

```bash
curl -s -H 'Authorization: Bearer devtoken' -H 'Content-Type: application/json' \
  -H 'Connect-Protocol-Version: 1' \
  -d '{"onlyActive":true}' $BASE/goeland.v1.DocumentService/ListDocumentTypes
```

## Migrations

Numbered, commented dbmate files in `pkg/core/module/db/migrations/`:

```
0001_subject_core.sql        extensions + subject_kind/subject_ref/record_metadata/audit_event
0002_relationships.sql       relationship_type + subject_relationship
0003_document.sql            document_type + document (+ generated search_vector, trigger)
0004_seed_reference_data.sql seed subject kinds, relationship types, document types
0005_document_unaccent_search.sql  accent-insensitive full-text search (immutable_unaccent)
0006_actor.sql               actor + actor_contact + organization_category (+ 33 seeded categories)
0007_business_ref.sql        subject_ref.business_ref + namespace (unique per namespace) + business_ref_counter
0008_document_versions.sql   content_blob (unique SHA-256) + document_version (immutable when final/record) + lossless backfill
0009_drop_document_file_columns.sql  drop the file/version columns superseded by 0008
0010_case.sql                case_type (reference namespace) + case_file (status lifecycle) + expanded case roles
0011_relationship_end.sql    uniqueness on open edges only (ended edges kept as history) + validity order check
0012_app_user.sql            app_user: internal users recorded from verified tokens (each a USER subject)
```

The **core module owns the full schema bootstrap** for this POC because the document
and actor tables have foreign keys into the core tables. As the POC grows (Case, Thing),
migrations can be split per module.

```bash
make db-status   # dbmate status
make db-up       # apply pending
make db-down     # roll back latest
make db-new name=add_case   # scaffold a new migration
```

## Common make targets

```
make run          generate + build the frontend + run the server
make generate     buf lint + generate (Go, ConnectRPC, OpenAPI)
make front-build  bun install + build the embedded Vue/Vuetify frontend (dist/)
make build        build the frontend + test + compile bin/goeland-server
make test         go test -race with coverage
make lint         go vet + buf lint
make fmt          gofmt -w .
make docs-check   GoDoc coverage + atlas inventory + executable doc claims
make check        front-check + fmt-check + lint + test + docs-check (run before handoff)
make vuln-check   govulncheck: fail on vulnerabilities the code can reach
make release-check  check + vuln-check + version/changelog/roadmap traceability + binary --version (what CI runs)
make release      CONFIRM_RELEASE=vX.Y.Z: guarded annotated tag + atomic push of main and tag
make db-up        apply migrations (dbmate)
```

## Testing & CI

`make test` runs the unit tests. The database **integration tests** in `pkg/integration`
(migrations idempotency + full document and actor lifecycles) are gated on `GOELAND_TEST_DATABASE_URL`
and skip when it is unset, so the default run needs no database. To run them against a
disposable PostGIS database:

```bash
GOELAND_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5432/goeland_test?sslmode=disable' \
    go test ./pkg/integration/...
```

CI lives in [`.github/workflows`](.github/workflows): `ci` (runs `make release-check` on
every push/PR to `main`, the same gate as locally), `cve-trivy-scan` (image CVE scan on
push/PR to `main` **and a weekly schedule**), `docker-publish` (unit tests + build/scan/publish
the image on version tags), and `release` (cross-compiled binaries on version tags). Both Trivy
jobs upload SARIF to the Security tab **and fail on fixable HIGH/CRITICAL findings** — in
`docker-publish` this gates the publish step, so a vulnerable image is never pushed. Documented
non-applicable advisories are suppressed in [`.trivyignore`](.trivyignore). The Go version is
sourced from `go.mod`, and third-party actions are pinned to commit SHAs. The container image is
self-contained: it builds the embedded frontend in a `bun` stage before the Go build.

## Scripts (`scripts/`)

Helper scripts for the dev loop and ops (all run from the repo root):

| Script | Purpose                                                                                               |
|--------|-------------------------------------------------------------------------------------------------------|
| `createLocalDBAndUser.sh <name>` | Create a local role + database, enable pgcrypto/pg_trgm/unaccent/postgis (as admin), and write `.env` |
| `install_go_protobuf_tools.sh`   | Install `buf`, `protoc-gen-go`, `protoc-gen-connect-go`                                               |
| `buf_generate.sh`                | `buf lint` + `buf dep update` + `buf generate` (used by `make generate`)                              |
| `getAppInfo.sh`                  | Export `APP_NAME`/`APP_VERSION`/… from `pkg/version/version.go` (sourced by other scripts)            |
| `GoRunWithEnv.sh [main] [env]`   | `go run` the server with version ldflags + a dotenv loaded                                            |
| `GoTestWithEnv.sh [env]`         | `go test -race` with coverage + a dotenv loaded                                                       |
| `execWithEnv.sh <bin> [env]`     | Run a compiled binary with a dotenv loaded                                                            |
| `get_jwt_token.sh [env]`         | Fetch a JWT from go-cloud-k8s-auth (jwt mode testing)                                                 |
| `01_build_image_locally.sh`      | Build the container image, tagged from `version.go` (optional trivy scan)                             |
| `02_tag_new_release_github.sh`   | Guarded release (`make release`): `CONFIRM_RELEASE`, clean `main`, `make release-check`, annotated tag, atomic push |
| `check_release_tag.sh <tag>`     | Fail unless the tag equals `v` + `Version` (used by the release and docker-publish workflows)          |
| `check_release_traceability.sh`  | Done `GLD-*` roadmap tasks ↔ dated changelog sections, both ways (`make release-traceability-check`)    |
| `check_release_traceability_test.sh` | Accepted/rejected cases for the traceability checker (`make scripts-check`)                       |
| `changelog_section.sh <version>` | Print one CHANGELOG section (GitHub release notes)                                                     |
| `create_k8s_configmap_from_env.sh` | Render a k8s ConfigMap from `.env` (dry-run)                                                          |
| `check_documentation_claims.sh`  | Executable doc claims: stable defaults/security facts must agree across sources (`make docs-assert`) |

## Design rules honoured

Explicit domain model (no generic EAV); JSONB only for secondary data; critical
fields as typed columns; non-destructive deletes; every mutation writes an audit
event; every relationship is typed and validated; boring, explicit, testable API.

## Status & roadmap

The active spec is v2, [`requirements/goeland_poc_domain_model_agent_v2.md`](requirements/goeland_poc_domain_model_agent_v2.md);
the original [`requirements/goeland_poc_domain_model_agent.md`](requirements/goeland_poc_domain_model_agent.md)
is kept as history (both are intent, not updated as work proceeds). The living "what's done / what's left"
tracker — with intentional deviations from the spec — is
[`requirements/IMPLEMENTATION_STATUS.md`](requirements/IMPLEMENTATION_STATUS.md), and the
implementation order with `GLD-NNN` tasks is [`docs/ROADMAP.md`](docs/ROADMAP.md). For
deployment (required extensions, migrations, storage, auth, probes, secrets, and the known
POC limitations) see [`docs/PRODUCTION_READINESS.md`](docs/PRODUCTION_READINESS.md).

### Out of scope for this slice

Case timeline + circulation, Thing (parcelle/bâtiment with PostGIS
geometry), a real permission/confidentiality engine, MinIO storage, Meilisearch, and
workflow integration — all designed to sit on top of the same subject/relationship/
audit foundation. (The **Actor** domain and its addresses/role-relationship wiring beyond
this identity+contacts slice also continue on the same foundation.)
