# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This POC uses [Semantic Versioning](https://semver.org/); while pre-1.0, a breaking
change bumps the **minor** version and features/fixes bump the **patch** version.

## [0.2.0] - 2026-07-07

First embedded web UI for the POC plus document file upload. The document
component is now exercisable end-to-end from the browser by an authenticated
user (dev-token or JWT via `go-cloud-k8s-auth`). Verified against PostgreSQL
(Go build/vet/tests green; frontend type-check, lint, and build green).

### Added

- **Embedded web UI** (`cmd/goeland-server/goeland-front`): a Vue 3 + Vuetify 4
  SPA (Vite/bun, vue-i18n, Pinia, vue-router) embedded via `//go:embed` and served
  at `/` with SPA fallback to `index.html`. First vertical slice of the **Document**
  module — search/list, create (with file upload), detail, edit metadata, finalize,
  verify integrity, link/unlink subjects, soft-delete — plus read-only **governance**
  and **audit** panels and the transversal **Core** components (subject identity,
  record metadata, audit timeline, relationship table). Bilingual (fr-CH default, en)
  with strict codes-to-API / labels-via-i18n discipline and state rules
  (locked/final/deleted disable mutations; audit is read-only; critical actions are
  dialog-confirmed). Typed REST-fetch client with humanized/translated errors.
- **Document file upload** (metadata-first): out-of-proto `POST /api/documents/upload`
  (multipart) streams bytes to a local blob store, computes sha256/size/mime
  server-side, and returns an `internal://…` `storage_ref` that the client passes to
  `CreateDocument` (so validation/governance/audit still flow through the proto path).
  `GET /api/documents/download?ref=…` streams a blob back. Both endpoints carry their
  own bearer check (they bypass the Connect interceptor) and a larger body cap. New
  `pkg/document/filestore` package with unit tests and a path-traversal guard.
- **`GET /config`** endpoint exposing `{authMode, authBaseUrl}` so the SPA drives
  either dev-token or JWT (silent-mint from the `go-cloud-k8s-auth` SSO session) auth.
- **Makefile `front-build`** target (`bun install && bun run build`), wired as a
  prerequisite of `run` and `build` so the embedded `dist/` is current before
  `go build`/`go test` (required by `go:embed`).
- Config: `GOELAND_DOCUMENT_PATH` (default `./go_documents`, gitignored) and
  `GOELAND_MAX_UPLOAD_BYTES` (default 100 MiB).

### Changed

- The Vanguard transcoder is now mounted on explicit prefixes (`/api/` and each RPC
  service path) instead of the root catch-all, letting the embedded SPA own `/`.

## [0.1.0] - 2026-07-07

Hardening pass from the technical review (`reports/report_20260707_codex.md`, all 10
quick wins) plus a first-class REST surface. Verified end-to-end against PostgreSQL
(build, vet, `buf lint`, and tests green).

### ⚠ BREAKING CHANGES

- Removed the client-supplied `actor_user_id` field from every request message.
  The acting **operator** is now always derived server-side from the authenticated
  principal (`core.OperatorID`), so audit attribution can no longer be forged.
  This is distinct from a domain **ACTOR** (an external person/organization, e.g. a
  document's author) which is never authenticated and is recorded as an `ACTOR`
  subject linked by a typed relationship (`DOCUMENT_AUTHORED_BY_ACTOR`, …).

### Added

- **REST/JSON surface** via `google.api.http` annotations on every `CoreService`
  (spec §13.6) and `DocumentService` (spec §13.3) RPC. Vanguard serves these
  alongside Connect/gRPC (`/api/subjects`, `/api/relationships`,
  `/api/relationship-types`, `/api/documents`, `/api/document-types`, …). The REST
  paths need no `Connect-Protocol-Version` header.
- Regenerated **OpenAPI** (`api/openapi/goeland.swagger.yaml`) now documents the
  real REST paths (previously empty).
- Atomic lifecycle guard `core.EnsureMutableTx` (`SELECT … FOR UPDATE`) and a
  request-id context (`X-Request-ID` → `audit_event.request_id`).
- Unit tests: operator identity, error mapping, request-id context, document
  validation, lock propagation, and hash comparison.

### Changed

- `VerifyDocumentIntegrity` is now **non-mutating and non-probative**: it reads no
  storage bytes, writes nothing (read-scoped), and a blank expected hash is never
  reported as verified. Proto/docs no longer overclaim "strong integrity".
- Title updates keep `subject_ref.display_label` in sync with `document.title`.
- Access logs record HTTP status + response bytes; the server mounts the shared
  Vanguard transcoder as the catch-all route.
- Dockerfile: ships CA roots, injects version ldflags via build args, and pins the
  builder to `golang:1.26-alpine`.
- `scripts/create_k8s_configmap_from_env.sh` renders only non-secret keys and never
  prints secret values (Secret creation is handled separately).

### Fixed

- Lifecycle invariants are now enforced: `UpdateDocumentMetadata`, `FinalizeDocument`,
  `DeleteDocument`, and relationship links reject **locked** or **soft-deleted**
  records (`failed_precondition`).
- `previous_version_id` now also creates the `DOCUMENT_PREVIOUS_VERSION` graph edge
  atomically; inactive document/relationship types are rejected on use.
- Ambiguous `id` column reference in the relationships JOIN query (now `sr`-qualified).
- Dockerfile `make mod-download` ran before the Makefile was copied; now uses
  `go mod download` directly.

### Security

- Trustworthy audit attribution (operator derived from the authenticated principal;
  see BREAKING CHANGES).
- Honest, non-mutating integrity verification (no false assurance).
- Container image includes CA roots so outbound TLS (PAT introspection,
  certificate-verifying PostgreSQL) works.

## [0.0.2] - 2026-07-06

### Added

- Accent-insensitive full-text search for documents (migration `0005`,
  `immutable_unaccent`): "chateau" matches "château".
- `requirements/IMPLEMENTATION_STATUS.md` living tracker (spec vs. implementation).

## [0.0.1] - 2026-07-06

### Added

- Initial Goéland POC scaffold: proto-first (buf), ConnectRPC + Vanguard, pgx raw
  SQL, PostgreSQL with embedded dbmate migrations, PostGIS-ready.
- **Transversal core** (`pkg/core`): `subject_ref`, `record_metadata`, `audit_event`,
  `relationship_type`, `subject_relationship` → `CoreService`.
- **Document** component (`pkg/document`, modern GED slice) → `DocumentService`.
- Bundleable `pkg/<domain>/module` pattern; `cmd/goeland-server` composes both
  modules on one shared pool/transcoder/auth verifier.

[0.2.0]: #020---2026-07-07
[0.1.0]: #010---2026-07-07
[0.0.2]: #002---2026-07-06
[0.0.1]: #001---2026-07-06
