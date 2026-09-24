# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This POC uses [Semantic Versioning](https://semver.org/); while pre-1.0, a breaking
change bumps the **minor** version and features/fixes bump the **patch** version.

## [Unreleased]

## [0.6.0] - 2026-09-24

This release opens spec v2 Phase 1: the Case slice (**GLD-011**) and ending a relationship as
distinct from unlinking it (**GLD-034**), both usable from the embedded SPA. Migrations `0010`
and `0011` apply automatically at startup; no breaking change for existing `goeland.v1` clients.

### Added

- **GLD-011** — Case slice: `CaseService` (`goeland.v1`, 7 RPCs with REST bindings under
  `/api/cases` and `/api/case-types`) over migration `0010` (`case_type` with a reference
  namespace, `case_file` with its status lifecycle and search vector). A case gets a business
  reference allocated in its type namespace; statuses OPEN / IN_PROGRESS / SUSPENDED / CLOSED
  follow an explicit transition table, closing and reopening require a reason, and a closed
  case rejects edits until reopened. New relationship types `CASE_HAS_ACTOR_OWNER`,
  `_ARCHITECT`, `_CONTRACTOR` and `CASE_RELATED_TO_CASE`. SPA case list, creation and detail
  pages (edit, transitions, links, audit), and a `pkg/integration` lifecycle test.
- **GLD-034** — `CoreService.EndRelationship` (`POST /api/relationships/{id}/end`): records
  that a relationship ended by setting `valid_to` (server time by default, a future date
  schedules the end, never before `valid_from`) with a `RELATIONSHIP_ENDED` audit event; the
  edge stays listed as history, distinct from `UnlinkSubjects` for a mistaken edge. Migration
  `0011` limits uniqueness to open edges, so an ended role can be given again, and adds a
  validity-order check. The SPA relationship tables show validity and an "end" action.

### Changed

- `core.ErrInvalidState` maps to `FAILED_PRECONDITION` for operations the current state
  forbids; wire helpers (`core.ParseUUID`, `StructFromMap`, `ToConnectError`, …) and the
  `coretest` stub repository replace per-domain copies in the Connect adapters and tests.

### Fixed

- SPA sign-in: in `jwt` mode the app bar "Sign in" button was invisible (primary on the primary
  app bar), leaving only "Sign in to access the application." with no way forward. The signed-out
  screen now explains how to sign in for the configured mode (SSO button and retry, or the dev
  token field) and reports an unreachable authentication service; navigation is hidden until
  signed in.
- SPA: the case search field showed a raw i18n key, the link dialog title always said
  "document", and the subject identity card used the document icon for every kind.

### Security

- `make vuln-check` (govulncheck, pinned as a `go.mod` tool) joins `make release-check`: a
  vulnerability the code can actually reach now fails CI and the release, before the image scan.
- `.trivyignore`: `GO-2026-5932` re-verified against x/crypto v0.57.0 (not linked, not reachable
  per govulncheck) and given an `exp:2026-12-31` date so the suppression lapses and must be
  re-reviewed; every entry now needs an expiry.

## [0.5.0] - 2026-09-24

This release closes the spec v2 Phase 0 alignment: spec v2 adopted (**GLD-002**), a business
reference on every subject (**GLD-022**), the Document / DocumentVersion / ContentBlob split with
global deduplication and automatic document reuse (**GLD-023**), and a domain-neutral BlobStore
interface (**GLD-024**). It also clears the SonarQube dashboard with local parity gates and moves
to Go 1.27.1. **Breaking** for `goeland.v1` document clients (no production client yet);
migrations `0007`–`0009` apply automatically at startup.

### Added

- **GLD-024** — `pkg/blobstore`: domain-neutral, context-aware content-bytes contract (`Store`
  with `Put` / `Get` / `Delete`, `ErrNotFound`, `ErrInvalidRef`; spec v2 §23). The local store
  moves to `pkg/blobstore/filestore` and implements it (cancellation-aware writes, no partial
  file on failure); `pkg/blobstore/blobstoretest` is the conformance suite any implementation
  (e.g. the future S3 store) must pass. The document service and the download
  endpoint depend only on the interface; downloads keep range support for seekable objects.
- **GLD-023** — Document / DocumentVersion / ContentBlob split (spec v2 §15-22). Migration `0008`
  adds `content_blob` (SHA-256 unique: identical content stored once) and `document_version`
  (append-only, explicit `document.current_version_id`; final and record versions immutable and
  versions never deleted, enforced by a trigger) with a lossless backfill; `0009` drops the
  superseded document columns. The upload endpoint ingests content through
  `document.Service.IngestContent` (server-side digest, global deduplication, duplicate bytes
  removed) and returns a `contentBlobId`; `CreateDocument` takes it and **automatically reuses**
  the live document already holding that content (`reused`, `DOCUMENT_REUSED`, case link). New
  `AddDocumentVersion` / `ListDocumentVersions` RPCs; finalize validates the current version. SPA:
  versions panel, reuse notice, integrity on the current version. Integration tests cover
  deduplication (incl. concurrent uploads), reuse across cases, shared blobs, immutability.
- **GLD-022** — Business reference on every subject (v2 §8): migration `0007` adds
  `subject_ref.business_ref` + `business_ref_namespace` (unique per namespace, free and
  non-unique without one) and a `business_ref_counter` allocator producing `YYYY-NNNNNN` per
  namespace and Europe/Zurich year, serialized by a row lock and gap-free on rollback.
  `CoreService.CreateSubjectRef` accepts an optional `business_ref` (explicit value or
  allocation); new `AssignBusinessRef` (`POST /api/subjects/{subject_id}/business-ref`, audited
  as `BUSINESS_REF_ASSIGNED`, one reference per subject) and `LookupSubjects`
  (`GET /api/subjects:lookup`). The SPA identity card shows the reference. Covered by unit tests
  and PostgreSQL integration tests, including concurrent allocation.

### Changed

- **Breaking (goeland.v1, no production client yet):** `Document` loses `storage_ref`,
  `mime_type`, `file_size_bytes`, `sha256`, `sha256_verified_at`, `version`,
  `previous_version_id`, `is_final`, `is_record`, `page_count` (now on `current_version` /
  its `content`); `CreateDocumentRequest` loses the file fields and `previous_version_id` in
  favour of `content_blob_id`. Removed field numbers and names are reserved.
- **Go 1.27.1** (`go.mod`, `golang:1.27-alpine` builder, README); `go fix` modernizers applied
  (`errors.AsType`, `sync.WaitGroup.Go`).
- SonarQube maintainability/reliability/security findings of the full dashboard: production Go
  functions refactored to cognitive complexity <= 15 (doccheck, document create, config, migrator,
  actor update), SPA error mapping and API client split, clickable list rows keyboard-accessible,
  `String(unknown)` removed from validation rules, deprecated `word-break: break-word`, error
  output to stderr and positional-parameter locals in scripts, `get_jwt_token.sh` refuses to send
  credentials in clear text to a non-loopback host. False positives are handled at the source:
  `.sonarcloud.properties` declares the tests as tests and excludes generated code and the
  PostgreSQL migrations (analysed by Sonar's Oracle PL/SQL rules).
- Local Sonar-parity gates so these findings do not come back: `make cognitive-check`
  (`gocognit`, pinned as a `go.mod` tool) inside `make check`; the SPA ESLint config enables
  `sonarjs/cognitive-complexity`, `vuejs-accessibility` keyboard rules and a clickable-`<tr>` rule.
- SonarQube findings: table headers carry `scope="col"` (21 × Web:TableHeaderHasIdOrScopeCheck);
  CI/release/docker-publish install buf through `bufbuild/buf-action` pinned by SHA with the
  binary's sha256 verified instead of `go install` (githubactions:S8545); the graceful shutdown
  derives its timeout from `context.WithoutCancel(ctx)` (godre:S8239).
- `POST /api/documents/upload` now requires `goeland:write` and `GET /api/documents/download`
  `goeland:read` (previously any valid token).
- Helper scripts (`buf_generate.sh`, `create_k8s_configmap_from_env.sh`, `execWithEnv.sh`,
  `get_jwt_token.sh`) use bash `[[ … ]]` tests instead of `[ … ]` (SonarQube shell rule).
- **GLD-002** — **Spec v2 adopted** as the active statement of intent:
  `requirements/goeland_poc_domain_model_agent_v2.md` (renamed from its review draft); the v1
  spec stays as immutable history. `IMPLEMENTATION_STATUS.md` gains the v2 alignment table
  (§0, v2 §57) and the reconciliation decisions (§3g): automatic document reuse on identical
  content (confidentiality risk accepted until real authorization), API evolved in place in
  `goeland.v1`, Document alignment before Case, explicit current version, lifecycle mapping,
  outbox from Phase 7, minimal USER/ORG_UNIT before Task, business_ref allocation, EPSG:2056.
- `docs/ROADMAP.md` re-ordered on the v2 phases (Phase 0 alignment, Case, Thing, Timeline,
  Task, Circulation, Security, Provenance/Outbox/Export, AI, Workflow) with new tasks for
  business_ref, the Document/Version/Blob split, the BlobStore interface, USER/ORG_UNIT, Task,
  provenance, outbox, export, retention, sensitive read audit, AI proposals, workflow and
  relationship ending; `[-]` marks a superseded task.

### Fixed

- `core.LinkSubjectsTx` mapped the active-edge unique violation only on `Query`, but pgx reports
  it when rows are read: a duplicate link surfaced as `INTERNAL` instead of `ALREADY_EXISTS`.

## [0.4.3] - 2026-09-23

Maintenance patch clearing the remaining fixable MEDIUM findings of the v0.4.2 image scan.

### Security

- `golang.org/x/crypto` v0.55.0 → v0.57.0 (CVE-2026-56855, CVE-2026-78662: `x/crypto/ssh`
  denial of service).
- CEL engine `github.com/google/cel-go` v0.26.1 (GHSA-gcjh-h69q-9w9g) replaced by
  `cel.dev/cel-go` v0.32.0, through `connectrpc.com/validate` v0.6.0 → v0.7.0 and
  `buf.build/go/protovalidate` v1.0.0 → v1.4.0 (request validation behavior re-checked).
  Transitive `x/net`, `x/sync`, `x/sys`, `x/text`, `x/exp`, `genproto` and `protobuf`
  v1.36.12 follow. The rebuilt binary scans clean for every fixable finding.

## [0.4.2] - 2026-09-23

Security patch: the v0.4.1 image was blocked by the Trivy publication gate, so no v0.4.1
image exists; use v0.4.2.

### Security

- `github.com/labstack/echo/v4` v4.14.0 → v4.15.3 (CVE-2026-55677, HIGH: information disclosure
  via URL path decoding discrepancy).
- `golang.org/x/crypto` v0.54.0 → v0.55.0 (CVE-2026-56854, HIGH: `x/crypto/ssh` authentication
  bypass). Both are indirect dependencies (via go-cloud-k8s-common-libs); `go mod tidy` also
  raised `labstack/gommon`, `mattn/go-isatty`, `x/text` and `x/time`. The rebuilt binary scans
  clean for fixable HIGH/CRITICAL findings.

## [0.4.1] - 2026-09-23

This release makes documentation part of the build: **GLD-001** adopts the go-pdf-forge
documentation quality contract, and one gate (`make release-check`) now runs identically
locally, in CI and before any publication. No API, wire or schema change.

### Added

- **GLD-001** — **Documentation quality contract** (`docs/DOCUMENTATION.md`, slice 1 of 5): the normative
  contract for human and agent contributors, ported from `go-pdf-forge` and referenced from
  `AGENTS.md` and the README.
- `cmd/doccheck`: parameterized port of the go-pdf-forge checker (`--version-file`,
  `--version-name`, `--banner-prefix`) with accepted/rejected-case tests; it refuses a
  `Version` declared as a variable.
- `scripts/check_documentation_claims.sh`: executable claims tying stable defaults and security
  facts (upload limit, blob dir, jwt default, scopes, server-side operator identity, migration
  lock key) to their sources; reports every failing claim.
- Make gates `godoc-check`, `atlas-check`, `docs-assert`, `docs-check`, `fmt-check`,
  `front-check`, `check`, `version-check`, `changelog-check`, `scripts-check`, `binary` and
  `release-check`; new `.github/workflows/ci.yml` runs `make release-check` on every push/PR.
- `docs/atlas.md` (slice 2 of 5): one responsibility line for each of the ~190 non-ignored files,
  grouped by area, checked in both directions by `make atlas-check`.
- `requirements/goeland_poc_domain_model_agent_v2_from_ChatGPT_20260923.md`: proposed v2 revision
  of the spec, versioned for review (not yet normative).

- GoDoc contracts (slice 3 of 5) for every package and exported API, including all `db`-tagged
  model fields: nullability, units, bounds, enum values, generated columns, operator-vs-actor
  identity, transaction and audit responsibilities, and the errors each tx helper returns.

- Protobuf contract comments (slice 4 of 5): `buf.yaml` enables the `COMMENTS` lint category;
  every service, RPC (with its required scope, audit event and error codes), message, field,
  enum and enum value in `core.proto`, `document.proto` and `actor.proto` is documented. The
  regenerated Go bindings differ only in comments and the OpenAPI gains field and operation
  descriptions on unchanged routes; `buf breaking` against the previous commit is clean.

- Roadmap and guarded release (slice 5 of 5): `docs/ROADMAP.md` owns implementation order with
  stable `GLD-NNN` task IDs (backlog migrated from the local scratch notes, plus five API-honesty
  fixes found while documenting the contracts, now open roadmap tasks).
  `scripts/check_release_traceability.sh` (with self-tests) enforces done tasks ↔ dated changelog
  sections in both directions; `make roadmap-check` and `make release-traceability-check` join
  `make release-check`, which also requires `goeland-server --version` to report `Version`.
- `goeland-server --version` prints `<app> v<Version> (revision …, built …)` and exits.
- `scripts/check_release_tag.sh` and `scripts/changelog_section.sh`: tag = `v` + `Version`
  guard and changelog-backed release notes for the publication workflows.

### Changed

- `make release` / `scripts/02_tag_new_release_github.sh` is now a guarded release:
  `CONFIRM_RELEASE=vX.Y.Z`, clean `main`, tag absent locally and on origin, `make release-check`,
  annotated tag, atomic push of `main` and the tag (previously a lightweight tag and
  `git push --tags`).
- `release.yml` and `docker-publish.yml` verify the tag against `Version` and run
  `make release-check` before building or publishing anything (the release workflow previously
  ran no check; docker-publish ran only the unit tests). Release notes come from the changelog.
- `requirements/IMPLEMENTATION_STATUS.md` §5 now points to the roadmap instead of duplicating
  the slice order.
- `pkg/core` list scans embed `SubjectRelationship` / `AuditEvent` in their row structs (the pattern
  the document and actor domains already use) instead of duplicating every column field.
- `pkg/version`: identity values and `Version` are now constants; only `Revision` and
  `BuildStamp` stay variables injected with `-ldflags -X`.
- README announces `Current version: **v0.4.0**` (checked by `make version-check`).

### Fixed

- `Makefile`: the no-`.env` branch was tab-indented before the first target, so every `make`
  invocation on a checkout without `.env` (CI, release runners) stopped with "recipe commences
  before first target". Found by the first CI run of the new gate; nothing had been published.
- Frontend `bun.lock` regenerated: it predated the `overrides` block of `package.json`, so
  `bun install --frozen-lockfile` failed with current bun (no package version changed).
- Pre-existing ESLint (`unicorn/numeric-separators-style`) and `gofmt` (`pkg/actor/model.go`)
  violations, surfaced by the new gates.

## [0.4.0] - 2026-07-10

### Added

- **Actor component** (`pkg/actor`, spec §6.4) — the external-party domain: physical
  **persons** and moral **organizations**, modelled from the real production Goéland
  `Acteur` schema. New `ActorService` (proto-first, `proto/goeland/v1/actor.proto`) with
  6 RPCs (`CreateActor`, `GetActor`, `UpdateActor`, `SearchActors`, `DeleteActor`,
  `ListOrganizationCategories`), REST-annotated under `/api/actors`.
  - Migration `0006_actor.sql`: `actor` (1:1 with an ACTOR `subject_ref`, `actor_kind`
    PERSON/ORGANIZATION discriminator, accent-insensitive `search_vector`), `actor_contact`
    (typed contact channels + business identifiers IDE/VAT/ABACUS/registre du commerce), and
    `organization_category` seeded with the **33 real production categories**. DB CHECK
    constraints keep person-only and organization-only columns from crossing over.
  - **Roles stay out of the entity**: actors attach to cases/documents/things only through
    typed `CoreService` relationships (`CASE_HAS_ACTOR_*`, `DOCUMENT_*_ACTOR`), so the
    production 46-role vocabulary maps onto `relationship_type` in the later Case/Thing slices.
  - **No personal data**: the PERSON specialization carries only an `is_ch_register` flag +
    an opaque `ch_register_ref` — civil-registry identity stays in the source system.
  - Reuses the core primitives (subject_ref + record_metadata + audit_event) so an actor and
    its governance are created atomically; every mutation is non-destructive and audited.
  - **Embedded web UI**: full Actor panel in the Vue 3 + Vuetify 4 SPA (list/search, create
    with typed contacts, detail/edit, activate/deactivate, soft-delete, read-only governance/
    audit and incoming relationships), bilingual fr-CH/en.
  - **Tests**: new `pkg/integration` actor lifecycle + specialization coverage (org create with
    contacts → search → update/label-sync → case→actor link → soft-delete → audit; person
    PII-free; organization `legal_name` required), verified against a real PostGIS database.

## [0.3.3] - 2026-07-10

### Security

- **CI vulnerability gate**: `docker-publish` and `cve-trivy-scan` now fail on fixable
  HIGH/CRITICAL Trivy findings. In `docker-publish` the gate runs before the push step,
  so a vulnerable image is never published; both jobs still upload SARIF to the Security
  tab (the gate runs after the upload and reuses the cached DB).
- **Weekly re-scan**: `cve-trivy-scan` now also runs on a schedule (Mondays 06:00 UTC)
  to catch CVE drift against an already-shipped image, since new advisories can turn a
  previously clean image red with no code change.
- **`.trivyignore`**: documents and suppresses `GO-2026-5932` (`x/crypto/openpgp` is
  unmaintained but not linked into the binary — it is pulled transitively via Echo's
  `acme/autocert`, and the advisory is matched only from the module version in build
  metadata). Scoped to that one advisory so real CVEs still surface and still trip the gate.

## [0.3.2] - 2026-07-09

### Security

- **Dependency & toolchain bump to clear CVEs in the published image** (Trivy
  `gobinary` scan of `ghcr.io/.../go-cloud-k8s-poc-2026` went from 22 findings —
  13 HIGH / 7 MEDIUM — to 1 informational advisory). All findings were in
  transitive `golang.org/x/*` modules and the Go stdlib; the app imports none of
  the vulnerable SSH/OpenPGP code paths, but the modules were flagged by version.
  - `golang.org/x/crypto` v0.46.0 → v0.54.0 (clears the `x/crypto/ssh` disclosure
    batch, CVE-2026-39827…46598).
  - `golang.org/x/net` v0.48.0 → v0.57.0 (clears `x/net/html`, `http2`, `idna`).
  - `golang.org/x/sys` v0.39.0 → v0.47.0.
  - Go directive `1.26.4` → `1.26.5` (clears stdlib CVE-2026-39822, CVE-2026-42505).
  - Remaining `GO-2026-5932` (`x/crypto/openpgp` unmaintained) has no fix and is
    not imported; it is `UNKNOWN` severity and outside the CI Trivy scope.

## [0.3.1] - 2026-07-09

### Fixed

- **CI (`docker-publish`)**: the `Run Unit Tests` step compiled `cmd/goeland-server`,
  whose `//go:embed goeland-front/dist/*` requires the built SPA, but `dist/` is
  git-ignored and had not been produced yet on a clean checkout — the job failed with
  `pattern goeland-front/dist/*: no matching files found`. A Bun setup + frontend build
  step (mirroring the Dockerfile, with `oven-sh/setup-bun` pinned to a commit SHA) now
  runs before the tests so the embed resolves.

## [0.3.0] - 2026-07-09

CI, reproducible container builds, and the first automated database integration
tests, from the technical review (`reports/report_20260709_gpt-5.md`, quick wins
1, 2, 3, 4, 5, 7, 8; #6 metrics/tracing deferred). Verified against PostgreSQL
(Go build/vet/tests green; integration tests green against a PostGIS database).

### Added

- **Database integration tests** (`pkg/integration`): the embedded migrations are
  applied to a real PostgreSQL, then a document is walked through its whole lifecycle
  — create, full-text search, metadata update, link to an ACTOR subject, finalize+lock,
  locked-update rejection (`ErrLocked`), soft delete, deleted-mutation rejection
  (`ErrDeleted`), and audit-trail assertions — plus a migrations idempotency + seed-data
  check. Gated on `GOELAND_TEST_DATABASE_URL` and skipped when unset, so `go test ./...`
  stays green without a database. The target DB needs the PostGIS/pgcrypto/pg_trgm/unaccent
  extensions.
- **CI** (`.github/workflows`): `cve-trivy-scan` (Trivy image CVE scan on push/PR to
  `main`), `docker-publish` (unit tests + build/scan/publish the image on version tags),
  and `release` (cross-compiled `linux/amd64` + `linux/arm64` binaries on version tags).
  The Go version is sourced from `go.mod` (`go-version-file`) and third-party actions are
  pinned to commit SHAs; Trivy uses a post-incident-safe pinned version.
- **`docs/PRODUCTION_READINESS.md`**: deployment contract covering required PostgreSQL
  extensions, startup-migration behavior, blob-storage persistence, auth settings, health/
  readiness probes, secrets, and the known POC limitations (coarse authz, node-local blobs).

### Changed

- **Self-contained Docker build**: the image now builds the embedded Vue/Vuetify frontend
  in a dedicated `oven/bun` stage before the Go build, so `docker build` is reproducible
  from a clean checkout. Previously `//go:embed goeland-front/dist/*` required a pre-built,
  git-ignored `dist/` to already be present in the build context. `.dockerignore` now
  excludes `node_modules`/`dist` to keep the context lean.
- **Deterministic Go package discovery**: `make test` and `make lint` exclude packages under
  the frontend `node_modules` tree, so `bun install` no longer pollutes `go list ./...`
  (previously a `flatted/golang` package leaked into `./...`).
- **Admin scope naming**: the wildcard admin scope is now `goeland:admin` (was the stale
  cross-project `notes:admin`) across the auth adapter and its tests. This is internal
  naming only — the scope is assigned from the verified `IsAdmin` claim, not read from
  client-supplied token scopes — so no external token depends on the old string.

### Fixed

- **Frontend token remint** (`src/stores/auth.ts`): JWTs are re-minted at ~80% of their
  lifetime and never scheduled past expiry. The previous 30-second floor could schedule a
  remint *after* a short-lived token had already expired, causing an avoidable auth lapse.

## [0.2.1] - 2026-07-08

Internal readability refactor with no behavior change. Verified against PostgreSQL
(Go build, vet, and tests green).

### Changed

- Repository queries now use pgx v5 **named parameters** (`@name` bound through
  `pgx.NamedArgs`) instead of positional `$1, $2, …` placeholders. The SQL and the
  Go call sites read against each other by name, so the 21-column document insert and
  the multi-filter search/list queries are no longer position-fragile. Column-to-`db`-tag
  scanning is unchanged. The single-parameter migration bookkeeping queries in
  `pkg/core/module/migrate.go` stay positional.

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

[0.3.2]: #032---2026-07-09
[0.3.1]: #031---2026-07-09
[0.3.0]: #030---2026-07-09
[0.2.1]: #021---2026-07-08
[0.2.0]: #020---2026-07-07
[0.1.0]: #010---2026-07-07
[0.0.2]: #002---2026-07-06
[0.0.1]: #001---2026-07-06
