# Repository atlas — go-cloud-k8s-poc-2026

Tracked version: **v0.6.0**.

This index gives every non-ignored repository file one explicit responsibility
and authority note. Paths are checked in both directions by `make atlas-check`
(see [DOCUMENTATION.md](DOCUMENTATION.md#repository-atlas-contract)): add,
remove or rename an entry in the same change as the file. Git-ignored outputs
(`dist/`, `node_modules/`, `bin/`, `go_documents/`, `.env`) are not listed.

## Governance, documentation and automation

- `.dockerignore` — Excludes secrets, local outputs and dependency trees from the Docker build context.
- `.env_sample` — Secret-free example of the supported `GOELAND_*`, `DB_*` and JWT environment variables.
- `.gitignore` — Keeps secrets, binaries, coverage, blobs, `dist/` and `node_modules/` out of Git.
- `.sonarcloud.properties` — SonarCloud automatic-analysis settings: tests declared as tests; generated code and PostgreSQL migrations excluded.
- `.trivyignore` — Documented, expiring (`exp:`) Trivy suppressions for advisories proven not to apply to the binary.
- `AGENTS.md` — Durable instructions for coding agents: conventions, layers to keep in sync, gotchas.
- `CHANGELOG.md` — Versioned history of delivered changes (Keep a Changelog); authoritative for what a release shipped.
- `Dockerfile` — Self-contained multi-stage image: bun frontend stage, Go build with provenance ldflags, `scratch` runtime.
- `LICENSE` — MIT license of the project.
- `Makefile` — Reproducible entry point for generation, run, build, quality gates (`check`, `release-check`) and dbmate.
- `README.md` — Project overview, operator walkthrough and current-version banner.
- `docs/DOCUMENTATION.md` — Normative documentation contract for human and agent contributors.
- `docs/ROADMAP.md` — Authoritative implementation order and `GLD-NNN` task state; version-bannered, traced against the changelog.
- `docs/PRODUCTION_READINESS.md` — Deployment contract: extensions, migrations, storage, auth, probes, secrets, limits.
- `docs/atlas.md` — This file: exact file-by-file responsibility index, version-bannered.
- `requirements/IMPLEMENTATION_STATUS.md` — Living state against the spec: built areas, decided enhancements, deviations, gaps.
- `requirements/goeland_frontend_proto_to_vuetify_i18n_agent_brief.md` — Input brief (French) for generating the Vue/Vuetify UI from the protos via UI schemas and i18n.
- `requirements/goeland_poc_domain_model_agent.md` — Historical v1 spec (French) of the OOA domain model; immutable, superseded by v2.
- `requirements/goeland_poc_domain_model_agent_v2.md` — Active spec v2 (French): baseline-preserving target model and phase order; immutable, reconciled in `IMPLEMENTATION_STATUS.md` §3g.

## CI and release workflows

- `.github/workflows/ci.yml` — Runs `make release-check` on every push and pull request to `main`.
- `.github/workflows/cve-trivy-scan.yml` — Builds the image and fails on fixable HIGH/CRITICAL CVEs on push, PR and a weekly schedule.
- `.github/workflows/docker-publish.yml` — On version tags: tag = version check, `make release-check`, Trivy-gated image build and GHCR publish.
- `.github/workflows/release.yml` — On version tags: tag = version check, `make release-check`, linux amd64/arm64 binaries, release notes from the changelog.

## Scripts

- `scripts/01_build_image_locally.sh` — Builds the container image tagged from `pkg/version/version.go`, with optional Trivy scan.
- `scripts/02_tag_new_release_github.sh` — Guarded release behind `make release`: confirmation, clean `main`, `make release-check`, annotated tag, atomic push.
- `scripts/GoRunWithEnv.sh` — Runs the server with `go run`, version ldflags and a dotenv file loaded.
- `scripts/GoTestWithEnv.sh` — Runs `go test -race` with coverage and a dotenv file loaded.
- `scripts/buf_generate.sh` — `buf lint`, `buf dep update` and `buf generate`; the body of `make generate`.
- `scripts/changelog_section.sh` — Prints one version's CHANGELOG section, used as GitHub release notes.
- `scripts/check_release_tag.sh` — Fails unless a release tag equals `v` + the `Version` constant; used by the publication workflows.
- `scripts/check_release_traceability.sh` — Bidirectional check between done roadmap tasks and dated changelog sections.
- `scripts/check_release_traceability_test.sh` — Accepted and rejected cases for the traceability checker, run by `make scripts-check`.
- `scripts/check_documentation_claims.sh` — Executable documentation claims tying stable defaults and security facts to their sources.
- `scripts/createLocalDBAndUser.sh` — Creates a local role and database, enables the required extensions as admin, writes `.env`.
- `scripts/create_k8s_configmap_from_env.sh` — Renders a Kubernetes ConfigMap from `.env` as a dry run.
- `scripts/execWithEnv.sh` — Runs a compiled binary with a dotenv file loaded.
- `scripts/getAppInfo.sh` — Exports `APP_NAME`, `APP_VERSION` and related values parsed from `pkg/version/version.go`.
- `scripts/get_jwt_token.sh` — Fetches a JWT from the auth server for `jwt`-mode testing and prints it on stdout.
- `scripts/install_go_protobuf_tools.sh` — Installs `buf`, `protoc-gen-go` and `protoc-gen-connect-go`.

## API contracts and generated bindings

- `buf.gen.yaml` — Buf generation plan: Go, ConnectRPC and OpenAPI outputs.
- `buf.lock` — Pinned Buf dependencies (protovalidate, googleapis); updated by `make generate`.
- `buf.yaml` — Buf module rooted at `proto/`, its dependencies and lint/breaking rules.
- `proto/.gitignore` — Ignores the locally exported third-party proto tree.
- `proto/goeland/v1/actor.proto` — Authoritative `ActorService` contract: persons, organizations, typed contacts, categories.
- `proto/goeland/v1/case.proto` — Authoritative `CaseService` contract: case types, case lifecycle (open → close/reopen), search.
- `proto/goeland/v1/core.proto` — Authoritative `CoreService` contract: subjects, governance, typed relationships, audit.
- `proto/goeland/v1/document.proto` — Authoritative `DocumentService` contract: GED document lifecycle, integrity, search.
- `api/openapi/goeland.swagger.yaml` — OpenAPI generated from the `google.api.http` annotations; never edit by hand.
- `gen/goeland/v1/actor.pb.go` — Go messages generated from `actor.proto`; never edit by hand.
- `gen/goeland/v1/case.pb.go` — Go messages generated from `case.proto`; never edit by hand.
- `gen/goeland/v1/core.pb.go` — Go messages generated from `core.proto`; never edit by hand.
- `gen/goeland/v1/document.pb.go` — Go messages generated from `document.proto`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/actor.connect.go` — ConnectRPC stubs generated for `ActorService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/case.connect.go` — ConnectRPC stubs generated for `CaseService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/core.connect.go` — ConnectRPC stubs generated for `CoreService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/document.connect.go` — ConnectRPC stubs generated for `DocumentService`; never edit by hand.

## Go module and commands

- `go.mod` — Go module declaration, toolchain version and direct dependencies.
- `go.sum` — Cryptographic checksums of the resolved Go dependencies.
- `cmd/doccheck/main.go` — Documentation checker: GoDoc coverage and exact atlas inventory, parameterized by flags.
- `cmd/doccheck/main_test.go` — Accepted and rejected cases for the atlas, version-source and GoDoc checks.
- `cmd/goeland-server/config.go` — Server environment configuration: defaults, parsing and validation.
- `cmd/goeland-server/main.go` — Server entry point: `--version`, config, logger, startup, listener and graceful shutdown.
- `cmd/goeland-server/server.go` — Pool, migrations and module wiring onto one Vanguard transcoder; probes, app info, embedded SPA.
- `cmd/goeland-server/upload.go` — Out-of-proto upload (content ingestion) and download endpoints with their own bearer and scope check, and the frontend config handler.

## Shared Go packages

- `pkg/blobstore/blobstore.go` — Domain-neutral content-bytes contract (`Store`: Put/Get/Delete) with its sentinel errors (spec v2 §23).
- `pkg/blobstore/blobstoretest/blobstoretest.go` — Conformance suite of the `blobstore.Store` contract, run by every implementation.
- `pkg/blobstore/filestore/filestore.go` — Local-filesystem `blobstore.Store` with `internal://` references and path-traversal guards.
- `pkg/blobstore/filestore/filestore_test.go` — Runs the conformance suite plus extension, unsafe-reference and failed-write tests.
- `pkg/version/version.go` — Release version constant (source of truth) and build provenance variables injected by ldflags.
- `pkg/authadapter/composite_verifier.go` — Routes `pat_` tokens to introspection and other bearer tokens to the JWT verifier.
- `pkg/authadapter/context.go` — Authenticated user model, context storage and scope checks.
- `pkg/authadapter/context_test.go` — Tests the authenticated-user context round trip.
- `pkg/authadapter/doc.go` — Package documentation for the shared token verification adapter.
- `pkg/authadapter/interceptor.go` — `TokenVerifier` interface and the Connect authentication interceptor.
- `pkg/authadapter/interceptor_test.go` — Tests the interceptor, admin scope wildcard and composite nil handling.
- `pkg/authadapter/pat_verifier.go` — Personal access token verification by cached introspection against the auth server.
- `pkg/authadapter/pat_verifier_test.go` — Tests PAT introspection, server failure and prefix routing.
- `pkg/authadapter/verifiers.go` — Local JWT verifier (signature, issuer, scopes) and the single-user dev token verifier.
- `pkg/authadapter/verifiers_test.go` — Tests dev token and JWT claim mapping.

## Core domain (`pkg/core`)

- `pkg/core/businessref.go` — Business reference request, validation, allocated-reference format and lookup filter.
- `pkg/core/businessref_test.go` — Tests business reference validation and allocated-reference formatting.
- `pkg/core/authctx.go` — Scope constants, caller requirement, server-side operator identity, timeout interceptor, error mapping.
- `pkg/core/authctx_test.go` — Tests operator identity, error mapping and request-ID context.
- `pkg/core/coretest/coretest.go` — Test helpers: a no-op `core.Repository` stub and a core service built on it for sibling-domain unit tests.
- `pkg/core/connect_server.go` — `CoreService` ConnectRPC adapter over the core service.
- `pkg/core/doc.go` — Package documentation for the transversal core domain.
- `pkg/core/errors.go` — Domain sentinel errors shared by every domain package.
- `pkg/core/mappers.go` — Core domain ↔ proto mappers and timestamp helpers.
- `pkg/core/model.go` — Core domain model with `db` tags: subjects, record metadata, audit events, relationships.
- `pkg/core/pagination.go` — Page token encoding and page size normalization.
- `pkg/core/pagination_test.go` — Tests pagination helpers and subject kind validation.
- `pkg/core/repository.go` — Core persistence interface.
- `pkg/core/requestctx.go` — Request ID propagation through the context.
- `pkg/core/service.go` — Core business rules: subject creation, typed linking, audit listing.
- `pkg/core/sql.go` — Raw SQL and alias-prefixed column projections for core tables.
- `pkg/core/storage_postgres.go` — pgx implementation of the core repository.
- `pkg/core/storage_users.go` — pgx persistence of internal users: first-sight registration (USER subject, governance, audit), profile updates, batch lookup.
- `pkg/core/tx.go` — Exported transaction-scoped helpers reused by sibling domains for atomic identity, governance and audit.
- `pkg/core/user.go` — Internal user model, token profile, admin scope and the `RecordingVerifier` that records every verified caller.
- `pkg/core/user_test.go` — Tests token profiles, labels and the recording verifier (change-only writes, best effort, invalid tokens).
- `pkg/core/users_service_test.go` — Tests `BatchGetUsers` validation (blank, too many, repeated ids).
- `pkg/core/wire.go` — Wire helpers shared by every ConnectRPC adapter: UUID parsing, `structpb` conversion, domain error to Connect error.
- `pkg/core/module/migrate.go` — Embedded dbmate-format migrator serialized by a PostgreSQL advisory lock.
- `pkg/core/module/migrate_test.go` — Tests migration parsing, PL/pgSQL block handling and version keys.
- `pkg/core/module/module.go` — Bundleable core module: dependency validation, service construction, lifecycle.
- `pkg/core/module/routes.go` — Core interceptor chain, Vanguard services and standalone route registration.
- `pkg/core/module/db/migrations/0001_subject_core.sql` — Schema migration: extensions, `subject_kind`, `subject_ref`, `record_metadata`, `audit_event`.
- `pkg/core/module/db/migrations/0002_relationships.sql` — Schema migration: `relationship_type` and `subject_relationship` with active-edge uniqueness.
- `pkg/core/module/db/migrations/0003_document.sql` — Schema migration: `document_type` and `document` with generated search vector.
- `pkg/core/module/db/migrations/0004_seed_reference_data.sql` — Seed migration: reference document and relationship types.
- `pkg/core/module/db/migrations/0005_document_unaccent_search.sql` — Migration: `immutable_unaccent()` and accent-insensitive document search.
- `pkg/core/module/db/migrations/0006_actor.sql` — Schema migration: `actor`, `actor_contact` and seeded `organization_category`.
- `pkg/core/module/db/migrations/0008_document_versions.sql` — Schema migration: `content_blob` (unique SHA-256), `document_version` with its immutability trigger, `document.current_version_id`, lossless backfill.
- `pkg/core/module/db/migrations/0009_drop_document_file_columns.sql` — Schema migration: drops the document file/version columns superseded by 0008 (reversible from the current version).
- `pkg/core/module/db/migrations/0012_app_user.sql` — Schema migration: `app_user`, the internal users recorded from verified tokens, each a USER subject.
- `pkg/core/module/db/migrations/0011_relationship_end.sql` — Schema migration: uniqueness on open relationships only (ended ones kept as history) and the validity-order check.
- `pkg/core/module/db/migrations/0010_case.sql` — Schema migration: `case_type` (with reference namespace) and `case_file` (status lifecycle, closure stamps, search vector), expanded case roles.
- `pkg/core/module/db/migrations/0007_business_ref.sql` — Schema migration: `subject_ref.business_ref` + namespace (unique per namespace) and the `business_ref_counter` allocator.

## Document domain (`pkg/document`)

- `pkg/document/connect_server.go` — `DocumentService` ConnectRPC adapter over the document service.
- `pkg/document/doc.go` — Package documentation for the GED document domain.
- `pkg/document/mappers.go` — Document domain ↔ proto mappers.
- `pkg/document/model.go` — Document, Version and ContentBlob models with `db` tags, create/version/ingest inputs and results, search filter.
- `pkg/document/repository.go` — Document persistence interface and the `ContentStore` contract for content bytes.
- `pkg/document/service.go` — Document business rules: content ingestion with deduplication, creation or reuse, versions, finalize, verify, link, soft delete.
- `pkg/document/service_test.go` — Tests creation validation, operator governance, lock propagation, hash matching, ingestion cleanup and version validation.
- `pkg/document/sql.go` — Raw SQL and alias-prefixed column projections for document tables.
- `pkg/document/storage_postgres.go` — pgx implementation: atomic create-or-reuse, versions, blob registration and hydration over core transaction helpers.
- `pkg/document/module/module.go` — Bundleable document module: dependency validation and lifecycle.
- `pkg/document/module/routes.go` — Document interceptor chain, Vanguard services and standalone routes.

## Actor domain (`pkg/actor`)

- `pkg/actor/connect_server.go` — `ActorService` ConnectRPC adapter over the actor service.
- `pkg/actor/contacts.go` — Per-type validation and normalization of typed complements (E.164 phones, e-mail, website, postal box, IDE check digit, VAT, ABACUS, register).
- `pkg/actor/contacts_test.go` — Accepted/normalized and rejected values for every complement type, and the OTHER label rule.
- `pkg/actor/doc.go` — Package documentation for the external persons and organizations domain.
- `pkg/actor/mappers.go` — Actor domain ↔ proto mappers.
- `pkg/actor/model.go` — Actor domain model with `db` tags: kinds, contact types and their names, categories, inputs, filter.
- `pkg/actor/repository.go` — Actor persistence interface.
- `pkg/actor/service.go` — Actor business rules: validation, per-type contact normalization, search, soft delete.
- `pkg/actor/sql.go` — Raw SQL and alias-prefixed column projections for actor tables.
- `pkg/actor/storage_postgres.go` — pgx implementation composing core transaction helpers for atomic actor mutations.
- `pkg/actor/module/module.go` — Bundleable actor module: dependency validation and lifecycle.
- `pkg/actor/module/routes.go` — Actor interceptor chain, Vanguard services and standalone routes.

## Case domain (`pkg/casefile`)

- `pkg/casefile/connect_server.go` — `CaseService` ConnectRPC adapter over the case service.
- `pkg/casefile/doc.go` — Package documentation for the case (affaire) domain.
- `pkg/casefile/mappers.go` — Case domain ↔ proto mappers, status enum conversion.
- `pkg/casefile/model.go` — Case and CaseType models with `db` tags, status transition table, inputs and search filter.
- `pkg/casefile/repository.go` — Case persistence interface.
- `pkg/casefile/service.go` — Case business rules: validation, lifecycle transitions with reasons, search, relationships, soft delete.
- `pkg/casefile/service_test.go` — Tests the transition table, reason requirements and input validation.
- `pkg/casefile/sql.go` — Raw SQL and alias-prefixed column projections for case tables.
- `pkg/casefile/storage_postgres.go` — pgx implementation: atomic create with business reference allocation, locked transitions, audit.
- `pkg/casefile/module/module.go` — Bundleable case module: dependency validation and lifecycle.
- `pkg/casefile/module/routes.go` — Case interceptor chain, Vanguard services and standalone routes.

## Integration tests (`pkg/integration`)

- `pkg/integration/business_ref_test.go` — DB test: allocation, namespace uniqueness, free references, assignment, deleted guard, rollback and concurrent allocation.
- `pkg/integration/actor_lifecycle_test.go` — DB test: seeded categories, organization lifecycle, PII-free person specialization.
- `pkg/integration/users_test.go` — DB test: user registration, unchanged refresh, audited profile change, batch lookup, concurrent first sight.
- `pkg/integration/relationship_end_test.go` — DB test: ending a relationship (history kept, relink allowed), double end, validity order, scheduled end, unlinked edge.
- `pkg/integration/case_lifecycle_test.go` — DB test: seeded case types, lifecycle with reference allocation and typed roles, closed-case freeze, explicit reference and deletion.
- `pkg/integration/doc.go` — Package documentation for the env-gated PostgreSQL integration tests.
- `pkg/integration/document_versions_test.go` — DB test: deduplication (incl. concurrent), automatic reuse across cases, versions sharing a blob, immutability trigger, lock guard.
- `pkg/integration/document_lifecycle_test.go` — DB test: idempotent seeded migrations and the full document lifecycle.
- `pkg/integration/harness_test.go` — Test harness gated on `GOELAND_TEST_DATABASE_URL`: migrate and connect.

## Embedded frontend (`cmd/goeland-server/goeland-front`)

- `cmd/goeland-server/goeland-front/.ruler/AGENTS.md` — Ruler source rules applied to agent configuration files by `bun run mcp`.
- `cmd/goeland-server/goeland-front/.ruler/ruler.toml` — Ruler configuration distributing agent rules and the Vuetify MCP server.
- `cmd/goeland-server/goeland-front/AGENTS.md` — Frontend agent rules: bun, TypeScript, stack and enabled features.
- `cmd/goeland-server/goeland-front/README.md` — Vuetify scaffold readme for the SPA.
- `cmd/goeland-server/goeland-front/bun.lock` — Pinned frontend dependency graph used by frozen installs.
- `cmd/goeland-server/goeland-front/env.d.ts` — Vite client type references.
- `cmd/goeland-server/goeland-front/eslint.config.js` — ESLint configuration (Vuetify preset, TypeScript).
- `cmd/goeland-server/goeland-front/index.html` — SPA HTML entry point.
- `cmd/goeland-server/goeland-front/package.json` — Frontend dependencies and bun scripts (build, type-check, lint).
- `cmd/goeland-server/goeland-front/public/favicon.ico` — Browser favicon asset.
- `cmd/goeland-server/goeland-front/src/App.vue` — Root layout: navigation, locale switch, auth controls, snackbar.
- `cmd/goeland-server/goeland-front/src/api/actorClient.ts` — REST client for `ActorService` bindings.
- `cmd/goeland-server/goeland-front/src/api/caseClient.ts` — REST client for `CaseService` bindings.
- `cmd/goeland-server/goeland-front/src/api/client.ts` — Minimal fetch client: bearer token, JSON, query params, typed `ApiError`.
- `cmd/goeland-server/goeland-front/src/api/coreClient.ts` — REST client for `CoreService` bindings (relationships, types, audit).
- `cmd/goeland-server/goeland-front/src/api/documentClient.ts` — REST client for `DocumentService` plus blob upload/download.
- `cmd/goeland-server/goeland-front/src/api/types.ts` — Hand-maintained proto3-JSON projections of the goeland.v1 messages; keep in sync with the protos.
- `cmd/goeland-server/goeland-front/src/assets/logo.png` — Raster logo asset.
- `cmd/goeland-server/goeland-front/src/assets/logo.svg` — Vector logo asset.
- `cmd/goeland-server/goeland-front/src/components/AppAuthControls.vue` — App bar login/logout controls for dev-token and JWT modes.
- `cmd/goeland-server/goeland-front/src/components/DevTokenForm.vue` — Dev-mode static token entry shared by the app bar and the sign-in panel.
- `cmd/goeland-server/goeland-front/src/components/README.md` — Scaffold note on component auto-import.
- `cmd/goeland-server/goeland-front/src/components/SignInPanel.vue` — Signed-out screen: how to sign in for the configured mode, retry, unreachable auth service, loopback host mismatch.
- `cmd/goeland-server/goeland-front/src/components/actor/ActorContactsEditor.vue` — Editable list of typed complements (type, value checked against its type, note, primary).
- `cmd/goeland-server/goeland-front/src/components/actor/ActorContactsPanel.vue` — Read-only display of an actor's complements, formatted, with tel:/mailto:/web links.
- `cmd/goeland-server/goeland-front/src/components/actor/ActorKindSelect.vue` — Person/organization kind selector.
- `cmd/goeland-server/goeland-front/src/components/actor/ActorMainForm.vue` — Actor create/edit form with kind-specific sections and hints (usual name, legal name, name complement).
- `cmd/goeland-server/goeland-front/src/components/actor/ActorSearchFilters.vue` — Actor search filter bar.
- `cmd/goeland-server/goeland-front/src/components/actor/OrganizationCategorySelect.vue` — Organization category selector bound to the category code.
- `cmd/goeland-server/goeland-front/src/components/actor/actorForm.ts` — Actor form model and mappers to create/update requests.
- `cmd/goeland-server/goeland-front/src/components/case/CaseStatusChip.vue` — Colored case status chip.
- `cmd/goeland-server/goeland-front/src/components/case/CaseTypeSelect.vue` — Case type selector bound to the type code.
- `cmd/goeland-server/goeland-front/src/components/case/caseForm.ts` — Case form model, status list and the client mirror of the transition table.
- `cmd/goeland-server/goeland-front/src/components/core/AuditTimeline.vue` — Read-only audit event timeline.
- `cmd/goeland-server/goeland-front/src/components/core/EndRelationshipDialog.vue` — Dialog ending a relationship (optional end date, reason) through `CoreService.EndRelationship`.
- `cmd/goeland-server/goeland-front/src/components/core/LinkSubjectDialog.vue` — Input dialog for a typed subject link (type, then a searched target of its kind); the parent performs the call.
- `cmd/goeland-server/goeland-front/src/components/core/RecordMetadataPanel.vue` — Read-only governance metadata panel.
- `cmd/goeland-server/goeland-front/src/components/core/RelationshipTable.vue` — Relationship table with links to both subjects, validity (ended / scheduled end) and optional end and unlink actions.
- `cmd/goeland-server/goeland-front/src/components/core/RelationshipTypeSelect.vue` — Relationship type selector filtered by subject kinds.
- `cmd/goeland-server/goeland-front/src/components/core/UserLabel.vue` — Internal user shown by name (admin icon, e-mail and id in the tooltip) from an operator id.
- `cmd/goeland-server/goeland-front/src/components/core/SubjectIdentityCard.vue` — Subject identity summary card, including the business reference.
- `cmd/goeland-server/goeland-front/src/components/core/SubjectLink.vue` — Subject label linking to its detail page (plain text for the current page).
- `cmd/goeland-server/goeland-front/src/components/core/SubjectPicker.vue` — Server-side search of subjects of one kind (actors, cases, documents) binding the chosen id.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentAuditPanel.vue` — Document detail wrapper around the audit timeline.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentFinalizeDialog.vue` — Confirmation dialog for finalizing (and optionally locking) a document.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentIntegrityPanel.vue` — Integrity verification and download panel for the current version's content.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentMetadataForm.vue` — Mutable document metadata form shared by create and edit.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentRelationshipsPanel.vue` — Presentational document relationships panel.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentSearchFilters.vue` — Document search filter bar.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentStatusChip.vue` — Colored document status chip.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentTypeSelect.vue` — Document type selector bound to the type code.
- `cmd/goeland-server/goeland-front/src/components/document/DocumentUploadField.vue` — File upload field returning the registered content (blob id, digest, reuse flag).
- `cmd/goeland-server/goeland-front/src/components/document/DocumentVersionsPanel.vue` — Version list of a document and adding a new version from an upload.
- `cmd/goeland-server/goeland-front/src/components/document/documentForm.ts` — Document metadata form model.
- `cmd/goeland-server/goeland-front/src/composables/useApiErrors.ts` — Maps API errors and validation violations to translated snackbar messages.
- `cmd/goeland-server/goeland-front/src/composables/useI18nEnum.ts` — Display-only translation of enum codes.
- `cmd/goeland-server/goeland-front/src/locales/en.json` — English UI messages.
- `cmd/goeland-server/goeland-front/src/locales/fr-CH.json` — Swiss French UI messages (default locale).
- `cmd/goeland-server/goeland-front/src/main.ts` — SPA bootstrap: registers plugins and mounts the app.
- `cmd/goeland-server/goeland-front/src/pages/actors/ActorCreatePage.vue` — Actor creation page.
- `cmd/goeland-server/goeland-front/src/pages/actors/ActorDetailPage.vue` — Actor detail, edit, activation, soft delete, governance and audit page.
- `cmd/goeland-server/goeland-front/src/pages/actors/ActorListPage.vue` — Actor search and list page.
- `cmd/goeland-server/goeland-front/src/pages/cases/CaseCreatePage.vue` — Case creation page (type, title, optional explicit reference).
- `cmd/goeland-server/goeland-front/src/pages/cases/CaseDetailPage.vue` — Case detail, edit, status transitions with reason, relationships, soft delete, governance and audit page.
- `cmd/goeland-server/goeland-front/src/pages/cases/CaseListPage.vue` — Case search and list page with status and type filters.
- `cmd/goeland-server/goeland-front/src/pages/documents/DocumentCreatePage.vue` — Metadata-first document creation page with upload.
- `cmd/goeland-server/goeland-front/src/pages/documents/DocumentDetailPage.vue` — Document detail, edit, finalize, verify, link and delete page.
- `cmd/goeland-server/goeland-front/src/pages/documents/DocumentListPage.vue` — Document search and list page.
- `cmd/goeland-server/goeland-front/src/plugins/README.md` — Scaffold note on the plugins folder.
- `cmd/goeland-server/goeland-front/src/plugins/i18n.ts` — vue-i18n setup with fr-CH default and English.
- `cmd/goeland-server/goeland-front/src/plugins/index.ts` — Registers Vuetify, Pinia, router and i18n on the app.
- `cmd/goeland-server/goeland-front/src/plugins/vuetify.ts` — Vuetify instance and theme configuration.
- `cmd/goeland-server/goeland-front/src/router/index.ts` — Client-side routes for the document and actor pages.
- `cmd/goeland-server/goeland-front/src/schemas/core.ui.schema.json` — UI schema for core components (input of the frontend brief; not imported at runtime).
- `cmd/goeland-server/goeland-front/src/schemas/document.ui.schema.json` — UI schema for the document resource (input of the frontend brief; not imported at runtime).
- `cmd/goeland-server/goeland-front/src/stores/auth.ts` — Auth store: `/config` bootstrap, dev token or silent JWT minting and re-mint, in-memory token.
- `cmd/goeland-server/goeland-front/src/stores/users.ts` — Operator id → user directory: batched `BatchGetUsers` calls and a page-lifetime cache.
- `cmd/goeland-server/goeland-front/src/stores/ui.ts` — Shared snackbar state.
- `cmd/goeland-server/goeland-front/src/styles/README.md` — Scaffold note on the styles folder.
- `cmd/goeland-server/goeland-front/src/styles/settings.scss` — Vuetify SASS variable overrides.
- `cmd/goeland-server/goeland-front/src/utils/authOrigin.ts` — Detects a loopback host mismatch between the SPA and the auth service (127.0.0.1 vs localhost).
- `cmd/goeland-server/goeland-front/src/utils/contactRules.ts` — SPA mirror of the complement rules: per-type check, placeholder, display format and link.
- `cmd/goeland-server/goeland-front/src/utils/formatters.ts` — Display formatters for proto-JSON dates, sizes and hashes.
- `cmd/goeland-server/goeland-front/src/utils/subjects.ts` — Per-kind subject icon and SPA detail route.
- `cmd/goeland-server/goeland-front/src/utils/validation.ts` — Vuetify rule factories mirroring the protos' buf.validate constraints.
- `cmd/goeland-server/goeland-front/tsconfig.app.json` — TypeScript configuration for the application sources.
- `cmd/goeland-server/goeland-front/tsconfig.json` — TypeScript project references root.
- `cmd/goeland-server/goeland-front/tsconfig.node.json` — TypeScript configuration for Node-side tooling config files.
- `cmd/goeland-server/goeland-front/vite.config.mts` — Vite build configuration (Vue, Vuetify, fonts, aliases).
