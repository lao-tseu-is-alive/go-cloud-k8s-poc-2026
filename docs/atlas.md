# Repository atlas — go-cloud-k8s-poc-2026

Tracked version: **v0.9.0**.

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
- `scripts/k8s_smoke_test.sh` — Deploys the published image with a disposable PostGIS on a local cluster and checks rollout, probes, version, SPA and API.
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
- `proto/goeland/v1/circulation.proto` — Authoritative `CirculationService` contract: circulations, recipients by step, responses, cancellation.
- `proto/goeland/v1/core.proto` — Authoritative `CoreService` contract: subjects, governance, typed relationships, audit.
- `proto/goeland/v1/document.proto` — Authoritative `DocumentService` contract: GED document lifecycle, integrity, search.
- `proto/goeland/v1/orgunit.proto` — Authoritative `OrgUnitService` contract: organizational units, tree nodes, dissolution and the unit type catalogue.
- `proto/goeland/v1/task.proto` — Authoritative `TaskService` contract: case tasks, lifecycle moves, assignment history, "my tasks" and the task type catalogue.
- `proto/goeland/v1/timeline.proto` — Authoritative `TimelineService` contract: case timeline entries, lifecycle, corrections and cited documents.
- `proto/goeland/v1/thing.proto` — Authoritative `ThingService` contract: things, types, parcel and building details, LV95 GeoJSON geometry, search by extent.
- `api/openapi/goeland.swagger.yaml` — OpenAPI generated from the `google.api.http` annotations; never edit by hand.
- `gen/goeland/v1/actor.pb.go` — Go messages generated from `actor.proto`; never edit by hand.
- `gen/goeland/v1/case.pb.go` — Go messages generated from `case.proto`; never edit by hand.
- `gen/goeland/v1/circulation.pb.go` — Go messages generated from `circulation.proto`; never edit by hand.
- `gen/goeland/v1/core.pb.go` — Go messages generated from `core.proto`; never edit by hand.
- `gen/goeland/v1/document.pb.go` — Go messages generated from `document.proto`; never edit by hand.
- `gen/goeland/v1/orgunit.pb.go` — Go messages generated from `orgunit.proto`; never edit by hand.
- `gen/goeland/v1/task.pb.go` — Go messages generated from `task.proto`; never edit by hand.
- `gen/goeland/v1/timeline.pb.go` — Go messages generated from `timeline.proto`; never edit by hand.
- `gen/goeland/v1/thing.pb.go` — Go messages generated from `thing.proto`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/actor.connect.go` — ConnectRPC stubs generated for `ActorService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/case.connect.go` — ConnectRPC stubs generated for `CaseService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/circulation.connect.go` — ConnectRPC stubs generated for `CirculationService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/core.connect.go` — ConnectRPC stubs generated for `CoreService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/document.connect.go` — ConnectRPC stubs generated for `DocumentService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/orgunit.connect.go` — ConnectRPC stubs generated for `OrgUnitService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/task.connect.go` — ConnectRPC stubs generated for `TaskService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/timeline.connect.go` — ConnectRPC stubs generated for `TimelineService`; never edit by hand.
- `gen/goeland/v1/goelandv1connect/thing.connect.go` — ConnectRPC stubs generated for `ThingService`; never edit by hand.

## Deployment

- `deployments/k8s/README.md` — Local Kubernetes smoke deployment: what the manifests are (and are not) and how to run them.
- `deployments/k8s/00-namespace.yaml` — Namespace `goeland-poc` of the smoke deployment.
- `deployments/k8s/10-postgis.yaml` — Disposable PostGIS Deployment and Service for the smoke deployment.
- `deployments/k8s/20-goeland.yaml` — ConfigMap, hardened Deployment with probes, and Service of the Goéland server.

## Go module and commands

- `go.mod` — Go module declaration, toolchain version and direct dependencies.
- `go.sum` — Cryptographic checksums of the resolved Go dependencies.
- `cmd/doccheck/main.go` — Documentation checker: GoDoc coverage and exact atlas inventory, parameterized by flags.
- `cmd/doccheck/main_test.go` — Accepted and rejected cases for the atlas, version-source and GoDoc checks.
- `cmd/goeland-server/config.go` — Server environment configuration: defaults, parsing and validation.
- `cmd/goeland-import-orgunits/main.go` — Optional import of the legacy org unit tree (structure only) from a read-only replica through the org unit service; idempotent, dry run by default, counts only.
- `cmd/goeland-server/main.go` — Server entry point: `--version`, config, logger, startup, listener and graceful shutdown.
- `cmd/goeland-server/scenario_test.go` — End-to-end spec v2 §50 scenario over HTTP against the real handler (REST, dev tokens, scopes, validation, error codes); env-gated on `GOELAND_TEST_DATABASE_URL`.
- `cmd/goeland-server/server.go` — Pool, migrations and module wiring onto one Vanguard transcoder; probes, app info, embedded SPA.
- `cmd/goeland-server/server_test.go` — Tests the bounded database wait at startup (retries, zero timeout, cancellation).
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

## Circulation domain (`pkg/circulation`)

- `pkg/circulation/connect_server.go` — `CirculationService` ConnectRPC adapter.
- `pkg/circulation/doc.go` — Package documentation for the case circulations.
- `pkg/circulation/mappers.go` — Circulation and recipient domain ↔ proto mappers (overdue and awaiting computed).
- `pkg/circulation/model.go` — Status, response, circulation and recipient models with `db` tags and inputs.
- `pkg/circulation/repository.go` — Circulation persistence interface.
- `pkg/circulation/service.go` — Circulation rules: texts, recipient checks and step renumbering, required answer texts and reasons.
- `pkg/circulation/service_test.go` — Tests step renumbering, recipient rejection, answer validation and names.
- `pkg/circulation/sql.go` — Raw SQL: circulations, recipients with labels and task status, answers, step progress, summaries.
- `pkg/circulation/storage_postgres.go` — Orchestration in one transaction: recipient tasks per step, answers as RESPONSE entries, next step or completion, cancellation; `EnsureNoOpenCirculationsTx`.
- `pkg/circulation/module/module.go` — Bundleable circulation module: dependency validation and lifecycle.
- `pkg/circulation/module/routes.go` — Circulation interceptor chain, Vanguard services and standalone routes.

## Core domain (`pkg/core`)

- `pkg/core/businessref.go` — Business reference request, validation, allocated-reference format and lookup filter.
- `pkg/core/businessref_test.go` — Tests business reference validation and allocated-reference formatting.
- `pkg/core/authctx.go` — Scope constants, caller requirement, server-side operator identity, timeout interceptor, error mapping.
- `pkg/core/authctx_test.go` — Tests operator identity, error mapping and request-ID context.
- `pkg/core/coretest/coretest.go` — Test helpers: a no-op `core.Repository` stub and a core service built on it for sibling-domain unit tests.
- `pkg/core/connect_server.go` — `CoreService` ConnectRPC adapter over the core service.
- `pkg/core/doc.go` — Package documentation for the transversal core domain.
- `pkg/core/email.go` — Shared e-mail address normalization (bare address, dotted lower-cased domain).
- `pkg/core/errors.go` — Domain sentinel errors shared by every domain package.
- `pkg/core/mappers.go` — Core domain ↔ proto mappers and timestamp helpers.
- `pkg/core/model.go` — Core domain model with `db` tags: subjects, record metadata, audit events, relationships.
- `pkg/core/pagination.go` — Page token encoding and page size normalization.
- `pkg/core/pagination_test.go` — Tests pagination helpers and subject kind validation.
- `pkg/core/reference.go` — Reference catalogues, the change log model, code and text validation, and `MutateReference` (change + log in one transaction).
- `pkg/core/reference_admin_test.go` — Tests that reference administration needs `goeland:admin` and the code rule.
- `pkg/core/reference_relationship.go` — Relationship type administration (create, update, kinds immutable) and the change log listing.
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
- `pkg/core/module/db/migrations/0016_thing.sql` — Schema migration: `thing_type`, `thing` (EPSG:2056 geometry, GIST, validity check), `thing_parcel` (EGRID), `thing_building` (EGID), land-rights roles, thing types administrable.
- `pkg/core/module/db/migrations/0018_org_unit.sql` — Schema migration: `org_unit_type` (7 seeded types), `org_unit` (tree without cycles, sibling-unique labels, external reference, dissolution), typed `owner_org_id`, case ↔ unit roles.
- `pkg/core/module/db/migrations/0019_task.sql` — Schema migration: `task_type` (5 seeded types), `case_task` (lifecycle stamps, one assignee, origin), `case_task_assignment` history, `USER_MEMBER_OF_ORG_UNIT`.
- `pkg/core/module/db/migrations/0020_circulation.sql` — Schema migration: `case_circulation` (steps, status stamps) and `case_circulation_recipient` (one user or unit, task, response, timeline entry), CIRCULATION_RESPONSE task type.
- `pkg/core/module/db/migrations/0017_timeline.sql` — Schema migration: `case_timeline_entry` (lifecycle stamps, same-case corrections) and `timeline_document_link` (pinned version), with immutability triggers.
- `pkg/core/module/db/migrations/0015_reference_change.sql` — Schema migration: the append-only `reference_change` log of reference data changes.
- `pkg/core/module/db/migrations/0014_actor_address.sql` — Schema migration: `address` and the typed M:N `actor_address` (one principal, ended links kept), `ACTOR_BRANCH_OF_ACTOR` and `ACTOR_CONTACT_PERSON_OF_ACTOR` types.
- `pkg/core/module/db/migrations/0013_person_identity.sql` — Schema migration: person minimal identity (salutation, last and first name, person-only) and the actor search vector over the names.
- `pkg/core/module/db/migrations/0012_app_user.sql` — Schema migration: `app_user`, the internal users recorded from verified tokens, each a USER subject.
- `pkg/core/module/db/migrations/0011_relationship_end.sql` — Schema migration: uniqueness on open relationships only (ended ones kept as history) and the validity-order check.
- `pkg/core/module/db/migrations/0010_case.sql` — Schema migration: `case_type` (with reference namespace) and `case_file` (status lifecycle, closure stamps, search vector), expanded case roles.
- `pkg/core/module/db/migrations/0007_business_ref.sql` — Schema migration: `subject_ref.business_ref` + namespace (unique per namespace) and the `business_ref_counter` allocator.

## Document domain (`pkg/document`)

- `pkg/document/connect_server.go` — `DocumentService` ConnectRPC adapter over the document service.
- `pkg/document/doc.go` — Package documentation for the GED document domain.
- `pkg/document/mappers.go` — Document domain ↔ proto mappers.
- `pkg/document/model.go` — Document, Version and ContentBlob models with `db` tags, create/version/ingest inputs and results, search filter.
- `pkg/document/reference_admin.go` — Document type administration (label, description, category, activation) over `core.MutateReference`.
- `pkg/document/repository.go` — Document persistence interface and the `ContentStore` contract for content bytes.
- `pkg/document/service.go` — Document business rules: content ingestion with deduplication, creation or reuse, versions, finalize, verify, link, soft delete.
- `pkg/document/service_test.go` — Tests creation validation, operator governance, lock propagation, hash matching, ingestion cleanup and version validation.
- `pkg/document/sql.go` — Raw SQL and alias-prefixed column projections for document tables.
- `pkg/document/storage_postgres.go` — pgx implementation: atomic create-or-reuse, versions, blob registration and hydration over core transaction helpers.
- `pkg/document/module/module.go` — Bundleable document module: dependency validation and lifecycle.
- `pkg/document/module/routes.go` — Document interceptor chain, Vanguard services and standalone routes.

## Actor domain (`pkg/actor`)

- `pkg/actor/addresses.go` — Address types, the address model and input, and their normalization (required fields, CH postal code, one principal).
- `pkg/actor/addresses_test.go` — Tests address defaults (country, principal) and rejected addresses.
- `pkg/actor/connect_server.go` — `ActorService` ConnectRPC adapter over the actor service.
- `pkg/actor/contacts.go` — Per-type validation and normalization of typed complements (E.164 phones, e-mail, website, postal box, IDE check digit, VAT, ABACUS, register).
- `pkg/actor/contacts_test.go` — Accepted/normalized and rejected values for every complement type, and the OTHER label rule.
- `pkg/actor/doc.go` — Package documentation for the external persons and organizations domain.
- `pkg/actor/mappers.go` — Actor domain ↔ proto mappers.
- `pkg/actor/model.go` — Actor domain model with `db` tags: kinds, contact types and their names, categories, inputs, filter.
- `pkg/actor/reference_admin.go` — Organization category administration (create, update, deactivate) over `core.MutateReference`.
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
- `pkg/casefile/reference_admin.go` — Case type administration (label, description, reference namespace, activation) over `core.MutateReference`.
- `pkg/casefile/repository.go` — Case persistence interface.
- `pkg/casefile/service.go` — Case business rules: validation, lifecycle transitions with reasons, search, relationships, soft delete.
- `pkg/casefile/service_test.go` — Tests the transition table, reason requirements and input validation.
- `pkg/casefile/sql.go` — Raw SQL and alias-prefixed column projections for case tables.
- `pkg/casefile/storage_postgres.go` — pgx implementation: atomic create with business reference allocation, locked transitions, audit.
- `pkg/casefile/module/module.go` — Bundleable case module: dependency validation and lifecycle.
- `pkg/casefile/module/routes.go` — Case interceptor chain, Vanguard services and standalone routes.

## Thing domain (`pkg/thing`)

- `pkg/thing/connect_server.go` — `ThingService` ConnectRPC adapter over the thing service.
- `pkg/thing/doc.go` — Package documentation for the thing (objet) domain.
- `pkg/thing/geometry.go` — Geometry rules: GeoJSON type per specialization, PostGIS validity, Swiss LV95 extent; bbox parsing.
- `pkg/thing/mappers.go` — Thing domain ↔ proto mappers, parcel and building blocks.
- `pkg/thing/model.go` — Thing, ThingType, Parcel and Building models with `db` tags, inputs, search filter and extent.
- `pkg/thing/reference_admin.go` — Thing type administration (generic types; code and specialization immutable) over `core.MutateReference`.
- `pkg/thing/repository.go` — Thing persistence interface.
- `pkg/thing/service.go` — Thing business rules: texts, detail blocks (EGRID, EGID, years), derived names, search, soft delete.
- `pkg/thing/service_test.go` — Tests geometry types, extents, bbox parsing, detail validation and derived names.
- `pkg/thing/sql.go` — Raw SQL: GeoJSON in/out, computed area and anchor, detail upserts, extent-filtered search.
- `pkg/thing/storage_postgres.go` — pgx/PostGIS implementation: atomic create and update with details and audit, hydration, search.
- `pkg/thing/module/module.go` — Bundleable thing module: dependency validation and lifecycle.
- `pkg/thing/module/routes.go` — Thing interceptor chain, Vanguard services and standalone routes.

## Org unit domain (`pkg/orgunit`)

- `pkg/orgunit/connect_server.go` — `OrgUnitService` ConnectRPC adapter; mutations require `goeland:admin`.
- `pkg/orgunit/doc.go` — Package documentation for the organizational units.
- `pkg/orgunit/mappers.go` — Unit, tree node and unit type domain ↔ proto mappers.
- `pkg/orgunit/model.go` — Unit, node, detail, unit type models with `db` tags, inputs, search filter and the display label rule.
- `pkg/orgunit/reference_admin.go` — Unit type administration over `core.MutateReference`.
- `pkg/orgunit/repository.go` — Org unit persistence interface.
- `pkg/orgunit/service.go` — Org unit business rules: abbreviation, label, e-mail and external reference normalization, search, dissolution reason.
- `pkg/orgunit/service_test.go` — Tests token and input normalization and the display label.
- `pkg/orgunit/sql.go` — Raw SQL: unit and node projections, ancestors, children, descendants check, search, type catalogue.
- `pkg/orgunit/storage_postgres.go` — pgx implementation: tree-locked create, update and dissolve with audit, parent checks, hydration.
- `pkg/orgunit/module/module.go` — Bundleable org unit module: dependency validation and lifecycle.
- `pkg/orgunit/module/routes.go` — Org unit interceptor chain, Vanguard services and standalone routes.

## Task domain (`pkg/task`)

- `pkg/task/connect_server.go` — `TaskService` ConnectRPC adapter, including "my tasks" and the lifecycle moves.
- `pkg/task/doc.go` — Package documentation for the case tasks.
- `pkg/task/mappers.go` — Task, assignment and task type domain ↔ proto mappers (overdue computed at mapping time).
- `pkg/task/model.go` — Status, origin, task, assignment, task type models with `db` tags, inputs and filters.
- `pkg/task/reference_admin.go` — Task type administration over `core.MutateReference`.
- `pkg/task/repository.go` — Task persistence interface.
- `pkg/task/service.go` — Task business rules: content limits, assignee normalization, required reasons, default "my tasks" statuses.
- `pkg/task/service_test.go` — Tests normalization, required reasons, the state machine, "my tasks" defaults and overdue.
- `pkg/task/sql.go` — Raw SQL: task projections with case and assignee labels, lifecycle updates, "my tasks" with unit membership, assignments, types.
- `pkg/task/storage_postgres.go` — pgx implementation: create, update, assign with history, lifecycle moves with SYSTEM timeline entries, lists.
- `pkg/task/tx.go` — Task state machine and transaction helpers: case and task locks, assignee checks, audit on the case; exported `EnsureNoOpenTasksTx` for the case lifecycle.
- `pkg/task/module/module.go` — Bundleable task module: dependency validation and lifecycle.
- `pkg/task/module/routes.go` — Task interceptor chain, Vanguard services and standalone routes.

## Timeline domain (`pkg/timeline`)

- `pkg/timeline/connect_server.go` — `TimelineService` ConnectRPC adapter over the timeline service.
- `pkg/timeline/doc.go` — Package documentation for the case timeline (suivis).
- `pkg/timeline/mappers.go` — Timeline entry and document link domain ↔ proto mappers.
- `pkg/timeline/model.go` — Entry types, statuses, visibilities, entry and document link models with `db` tags, inputs and list filter.
- `pkg/timeline/repository.go` — Timeline persistence interface.
- `pkg/timeline/service.go` — Timeline business rules: operator-creatable types, content limits, business date, document list, reasons.
- `pkg/timeline/service_test.go` — Tests content normalization and rejection, type and status rules, list filter and withdrawal reason.
- `pkg/timeline/sql.go` — Raw SQL: entry projections with the live correction, lifecycle updates, document links and version pinning.
- `pkg/timeline/storage_postgres.go` — pgx implementation: create with corrections and cited documents, draft edits, freeze with pinned versions, links, hydration.
- `pkg/timeline/tx.go` — Transaction helpers: open-case and draft locks, document citation with case link, audit on the case; exported `EnsureNoDraftsTx` and `RecordSystemEntryTx` for the case lifecycle.
- `pkg/timeline/module/module.go` — Bundleable timeline module: dependency validation and lifecycle.
- `pkg/timeline/module/routes.go` — Timeline interceptor chain, Vanguard services and standalone routes.

## Integration tests (`pkg/integration`)

- `pkg/integration/business_ref_test.go` — DB test: allocation, namespace uniqueness, free references, assignment, deleted guard, rollback and concurrent allocation.
- `pkg/integration/actor_address_test.go` — DB test: typed addresses with a principal, non-destructive replacement, branch linked to its head and listed from both.
- `pkg/integration/actor_lifecycle_test.go` — DB test: seeded categories, organization lifecycle, person minimal identity (derived display name, search by names, required last name, audited update).
- `pkg/integration/users_test.go` — DB test: user registration, unchanged refresh, audited profile change, batch lookup, concurrent first sight.
- `pkg/integration/reference_admin_test.go` — DB test: create, update and deactivate an entry of each catalogue, conflicts, unknown codes and the change log.
- `pkg/integration/relationship_end_test.go` — DB test: ending a relationship (history kept, relink allowed), double end, validity order, scheduled end, unlinked edge.
- `pkg/integration/orgunit_test.go` — Org units: seeded types, tree path, sibling labels, external references, no cycle (service and trigger), dissolution rules, owning unit and case roles.
- `pkg/integration/task_test.go` — Tasks: creation, reassignment history, assignee checks, state machine, SYSTEM timeline entries, "my tasks" with unit membership, closure rules.
- `pkg/integration/timeline_test.go` — Timeline lifecycle: cited documents and case link, validation with pinned version, DB-enforced immutability, corrections, drafts blocking closure, SYSTEM entries and case audit.
- `pkg/integration/thing_lifecycle_test.go` — DB test: parcel and building with geometry, containment, case and owner links, search by number and extent, refused geometries, unique identifiers, update.
- `pkg/integration/case_lifecycle_test.go` — DB test: seeded case types, lifecycle with reference allocation and typed roles, closed-case freeze, explicit reference and deletion.
- `pkg/integration/doc.go` — Package documentation for the env-gated PostgreSQL integration tests.
- `pkg/integration/document_versions_test.go` — DB test: deduplication (incl. concurrent), automatic reuse across cases, versions sharing a blob, immutability trigger, lock guard.
- `pkg/integration/circulation_test.go` — Circulations: two steps with user and unit recipients, managed tasks, answers and next step, completion summary, closure rules, cancellation.
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
- `cmd/goeland-server/goeland-front/index.html` — SPA HTML entry point; declares the Vuetify cascade layer order before any stylesheet.
- `cmd/goeland-server/goeland-front/package.json` — Frontend dependencies and bun scripts (build, type-check, lint).
- `cmd/goeland-server/goeland-front/public/favicon.ico` — Browser favicon asset.
- `cmd/goeland-server/goeland-front/src/App.vue` — Root layout: navigation, locale switch, auth controls, snackbar.
- `cmd/goeland-server/goeland-front/src/api/actorClient.ts` — REST client for `ActorService` bindings.
- `cmd/goeland-server/goeland-front/src/api/caseClient.ts` — REST client for `CaseService` bindings.
- `cmd/goeland-server/goeland-front/src/api/referenceClient.ts` — REST calls of reference data administration (create / update per catalogue) and the change log.
- `cmd/goeland-server/goeland-front/src/api/timelineClient.ts` — REST client for `TimelineService` bindings.
- `cmd/goeland-server/goeland-front/src/api/orgUnitClient.ts` — REST client for `OrgUnitService` bindings.
- `cmd/goeland-server/goeland-front/src/api/taskClient.ts` — REST client for `TaskService` bindings.
- `cmd/goeland-server/goeland-front/src/api/thingClient.ts` — REST client for `ThingService` bindings.
- `cmd/goeland-server/goeland-front/src/api/circulationClient.ts` — REST client for `CirculationService` bindings.
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
- `cmd/goeland-server/goeland-front/src/components/admin/ReferenceCatalogPanel.vue` — Generic editor of one reference catalogue: list, create, edit, (de)activate, with a logged reason.
- `cmd/goeland-server/goeland-front/src/components/admin/ReferenceChangesPanel.vue` — Read-only, paged view of the reference change log.
- `cmd/goeland-server/goeland-front/src/components/admin/referenceCatalogues.ts` — Declarative description of the four catalogues (fields, immutability, listing).
- `cmd/goeland-server/goeland-front/src/components/actor/ActorAddressesEditor.vue` — Editable list of typed addresses (role, street, number, complement, postal code, locality, country, principal star).
- `cmd/goeland-server/goeland-front/src/components/actor/ActorAddressesPanel.vue` — Read-only address cards (role, principal, formatted lines, map.geo.admin.ch link).
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
- `cmd/goeland-server/goeland-front/src/components/circulation/CaseCirculationsPanel.vue` — Case circulations panel: recipients by step with their answer, answer and cancel actions, open count for the closure rule.
- `cmd/goeland-server/goeland-front/src/components/circulation/CirculationCreateDialog.vue` — Send the case for circulation: subject, message, deadline and recipients (user or unit) by step.
- `cmd/goeland-server/goeland-front/src/components/circulation/CirculationRespondDialog.vue` — Record the answer of one recipient (response and text).
- `cmd/goeland-server/goeland-front/src/components/circulation/circulationForm.ts` — Circulation responses, colors, answer-text rule and grouping by step.
- `cmd/goeland-server/goeland-front/src/components/core/AuditTimeline.vue` — Read-only audit event timeline.
- `cmd/goeland-server/goeland-front/src/components/core/EndRelationshipDialog.vue` — Dialog ending a relationship (optional end date, reason) through `CoreService.EndRelationship`.
- `cmd/goeland-server/goeland-front/src/components/core/LinkSubjectDialog.vue` — Input dialog for a typed subject link (type, then a searched target of its kind); the parent performs the call.
- `cmd/goeland-server/goeland-front/src/components/core/RecordMetadataPanel.vue` — Read-only governance metadata panel.
- `cmd/goeland-server/goeland-front/src/components/core/RelationshipTable.vue` — Relationship table with links to both subjects, validity (ended / scheduled end) and optional end and unlink actions.
- `cmd/goeland-server/goeland-front/src/components/core/RelationshipTypeSelect.vue` — Relationship type selector filtered by subject kinds.
- `cmd/goeland-server/goeland-front/src/components/core/UserLabel.vue` — Internal user shown by name (admin icon, e-mail and id in the tooltip) from an operator id.
- `cmd/goeland-server/goeland-front/src/components/timeline/CaseTimelinePanel.vue` — Case timeline panel: type filters, withdrawn toggle, paging, entry dialog and status confirmations; reports the draft count.
- `cmd/goeland-server/goeland-front/src/components/timeline/TimelineEntryCard.vue` — One timeline entry: type, status, business date, author, cited documents with pinned version, correction links and draft actions.
- `cmd/goeland-server/goeland-front/src/components/timeline/TimelineEntryDialog.vue` — Create, edit or correct an entry: type, business date, title, body, visibility and cited documents.
- `cmd/goeland-server/goeland-front/src/components/timeline/timelineForm.ts` — Timeline type lists and styles, lifecycle helpers, datetime-local conversions and dialog payload types.
- `cmd/goeland-server/goeland-front/src/components/orgunit/OrgUnitFormDialog.vue` — Create or edit a unit: name, abbreviation, type, parent (subject picker), mailbox, mission.
- `cmd/goeland-server/goeland-front/src/components/orgunit/OrgUnitLabel.vue` — An org unit shown by name, linking to its page (used for the owning unit).
- `cmd/goeland-server/goeland-front/src/components/orgunit/orgUnitForm.ts` — Unit form model and request mapping, display label, tree building and a label cache.
- `cmd/goeland-server/goeland-front/src/components/task/AssigneePicker.vue` — Chooses a task assignee: an internal user (searched), a live org unit or nobody.
- `cmd/goeland-server/goeland-front/src/components/task/CaseTasksPanel.vue` — Case tasks panel: pending or all tasks, creation, open count for the closure rule.
- `cmd/goeland-server/goeland-front/src/components/task/TaskDialogs.vue` — Every task dialog (create, edit, assign, status moves, assignment history) behind exposed openers.
- `cmd/goeland-server/goeland-front/src/components/task/TaskTable.vue` — Task table: assignee, deadline with overdue highlight, status and the actions it allows.
- `cmd/goeland-server/goeland-front/src/components/task/taskForm.ts` — Task statuses, allowed moves mirroring the server, colors and assignee mapping.
- `cmd/goeland-server/goeland-front/src/components/thing/GeometryPreview.vue` — SVG preview of an LV95 GeoJSON geometry (north up) with its extent.
- `cmd/goeland-server/goeland-front/src/components/thing/ThingMainForm.vue` — Thing fields: detail block per specialization, texts and geometry with live preview.
- `cmd/goeland-server/goeland-front/src/components/thing/ThingTypeSelect.vue` — Thing type selector bound to the code, emitting the selected type.
- `cmd/goeland-server/goeland-front/src/components/thing/thingForm.ts` — Thing form model and its mapping to create / update requests.
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
- `cmd/goeland-server/goeland-front/src/composables/useSubjectLinks.ts` — Link and unlink handlers shared by the case, document and actor detail pages.
- `cmd/goeland-server/goeland-front/src/composables/useApiErrors.ts` — Maps API errors and validation violations to translated snackbar messages.
- `cmd/goeland-server/goeland-front/src/composables/useI18nEnum.ts` — Display-only translation of enum codes.
- `cmd/goeland-server/goeland-front/src/locales/en.json` — English UI messages.
- `cmd/goeland-server/goeland-front/src/locales/fr-CH.json` — Swiss French UI messages (default locale).
- `cmd/goeland-server/goeland-front/src/main.ts` — SPA bootstrap: registers plugins and mounts the app.
- `cmd/goeland-server/goeland-front/src/pages/admin/AdminPage.vue` — Reference data administration page (one tab per catalogue plus the change log), for administrators.
- `cmd/goeland-server/goeland-front/src/pages/actors/ActorCreatePage.vue` — Actor creation page.
- `cmd/goeland-server/goeland-front/src/pages/actors/ActorDetailPage.vue` — Actor detail: identity, addresses, complements, relationships in both directions (link, end, unlink), edit, activation, soft delete, governance and audit.
- `cmd/goeland-server/goeland-front/src/pages/actors/ActorListPage.vue` — Actor search and list page.
- `cmd/goeland-server/goeland-front/src/pages/cases/CaseCreatePage.vue` — Case creation page (type, title, optional explicit reference).
- `cmd/goeland-server/goeland-front/src/pages/cases/CaseDetailPage.vue` — Case detail, edit, status transitions with reason, relationships, soft delete, governance and audit page.
- `cmd/goeland-server/goeland-front/src/pages/cases/CaseListPage.vue` — Case search and list page with status and type filters.
- `cmd/goeland-server/goeland-front/src/pages/documents/DocumentCreatePage.vue` — Metadata-first document creation page with upload.
- `cmd/goeland-server/goeland-front/src/pages/documents/DocumentDetailPage.vue` — Document detail, edit, finalize, verify, link and delete page.
- `cmd/goeland-server/goeland-front/src/pages/documents/DocumentListPage.vue` — Document search and list page.
- `cmd/goeland-server/goeland-front/src/pages/things/ThingCreatePage.vue` — Thing creation page (type, details, geometry).
- `cmd/goeland-server/goeland-front/src/pages/things/ThingDetailPage.vue` — Thing detail: identifiers, geometry preview and map link, relationships (link, end, unlink), edit, soft delete, governance and audit.
- `cmd/goeland-server/goeland-front/src/pages/orgunits/OrgUnitDetailPage.vue` — Unit detail: breadcrumbs, summary, sub-units, relationships, governance, audit; admin edit, add sub-unit, dissolve.
- `cmd/goeland-server/goeland-front/src/pages/orgunits/OrgUnitTreePage.vue` — Organization tree with filter and dissolved toggle; admin creation.
- `cmd/goeland-server/goeland-front/src/pages/tasks/MyTasksPage.vue` — "My tasks" page: tasks assigned to the caller and their units, status filter, overdue count.
- `cmd/goeland-server/goeland-front/src/pages/things/ThingListPage.vue` — Thing search and list page (text, type).
- `cmd/goeland-server/goeland-front/src/plugins/README.md` — Scaffold note on the plugins folder.
- `cmd/goeland-server/goeland-front/src/plugins/i18n.ts` — vue-i18n setup with fr-CH default and English.
- `cmd/goeland-server/goeland-front/src/plugins/index.ts` — Registers Vuetify, Pinia, router and i18n on the app.
- `cmd/goeland-server/goeland-front/src/plugins/vuetify.ts` — Vuetify instance and theme configuration.
- `cmd/goeland-server/goeland-front/src/router/index.ts` — Client-side routes: cases, documents, things, actors and administration.
- `cmd/goeland-server/goeland-front/src/schemas/core.ui.schema.json` — UI schema for core components (input of the frontend brief; not imported at runtime).
- `cmd/goeland-server/goeland-front/src/schemas/document.ui.schema.json` — UI schema for the document resource (input of the frontend brief; not imported at runtime).
- `cmd/goeland-server/goeland-front/src/stores/auth.ts` — Auth store: `/config` bootstrap, dev token or silent JWT minting and re-mint, in-memory token.
- `cmd/goeland-server/goeland-front/src/stores/users.ts` — Operator id → user directory: batched `BatchGetUsers` calls and a page-lifetime cache.
- `cmd/goeland-server/goeland-front/src/stores/ui.ts` — Shared snackbar state.
- `cmd/goeland-server/goeland-front/src/styles/README.md` — Scaffold note on the styles folder.
- `cmd/goeland-server/goeland-front/src/styles/settings.scss` — Vuetify SASS variable overrides.
- `cmd/goeland-server/goeland-front/src/utils/address.ts` — Postal code and country rules mirroring the server, address display lines and map link.
- `cmd/goeland-server/goeland-front/src/utils/authOrigin.ts` — Detects a loopback host mismatch between the SPA and the auth service (127.0.0.1 vs localhost).
- `cmd/goeland-server/goeland-front/src/utils/contactRules.ts` — SPA mirror of the complement rules: per-type check, placeholder, display format and link.
- `cmd/goeland-server/goeland-front/src/utils/dateInput.ts` — Conversions between RFC3339 values and datetime-local input values (timeline, tasks).
- `cmd/goeland-server/goeland-front/src/utils/formatters.ts` — Display formatters for proto-JSON dates, sizes and hashes.
- `cmd/goeland-server/goeland-front/src/utils/geometry.ts` — GeoJSON parsing, SPA mirror of the geometry rules, SVG projection and map link.
- `cmd/goeland-server/goeland-front/src/utils/subjects.ts` — Per-kind subject icon and SPA detail route.
- `cmd/goeland-server/goeland-front/src/utils/validation.ts` — Vuetify rule factories mirroring the protos' buf.validate constraints.
- `cmd/goeland-server/goeland-front/tsconfig.app.json` — TypeScript configuration for the application sources.
- `cmd/goeland-server/goeland-front/tsconfig.json` — TypeScript project references root.
- `cmd/goeland-server/goeland-front/tsconfig.node.json` — TypeScript configuration for Node-side tooling config files.
- `cmd/goeland-server/goeland-front/vite.config.mts` — Vite build configuration (Vue, Vuetify, fonts, aliases).
