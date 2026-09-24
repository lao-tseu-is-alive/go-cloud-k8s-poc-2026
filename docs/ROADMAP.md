# Goéland POC Roadmap

Tracked version: **v0.8.0**.

This document is the source of truth for implementation order, scope and task
state. How the built system relates to the spec (active: v2) lives in
[`requirements/IMPLEMENTATION_STATUS.md`](../requirements/IMPLEMENTATION_STATUS.md);
what each release delivered lives in [`CHANGELOG.md`](../CHANGELOG.md). The
rules binding the three are in [DOCUMENTATION.md](DOCUMENTATION.md#roadmap-changelog-and-version-traceability).
Phases follow v2 §48; "v2 §N" cites
[`requirements/goeland_poc_domain_model_agent_v2.md`](../requirements/goeland_poc_domain_model_agent_v2.md).

## Conventions

- `[ ]` to do, `[~]` in progress, `[x]` done and verified, `[-]` dropped or
  superseded (the entry says by what).
- Every task has a unique, stable `GLD-NNN` ID; IDs are never reused or renumbered.
- A task is done only when its tests and acceptance criteria pass. A `[x]` task
  MUST be named in a dated changelog section, and a task named in a dated
  changelog section MUST be `[x]` here (`make release-traceability-check`), so a
  finished task stays `[~]` until the release that ships it.
- A change of scope or order is recorded here before it is implemented.
- Work delivered before this roadmap existed (v0.1.0 to v0.4.0: core, document,
  actor, embedded SPA) is recorded in the changelog without task IDs.

## Next action

Phase 1b (usable Actor and Case: GLD-035, 036, 037, 025, 038, 039, 014, 040) shipped in
v0.7.0, the Thing slice (GLD-016) in v0.8.0; next is the Timeline (GLD-012).

## Cross-cutting quality

- [x] **GLD-001 — Documentation quality contract**: adopt the
  [normative contract](DOCUMENTATION.md) in five slices (checker and gates,
  atlas, GoDoc, Protobuf comments, roadmap and guarded release) so drift fails
  `make release-check`, CI and the release workflows.
- [x] **GLD-002 — Requirements v2 review**: v2 adopted as the active spec
  (2026-09-23), v1 kept as history, reconciliation decisions recorded in
  `IMPLEMENTATION_STATUS.md` §3g and this roadmap re-ordered on v2 §48.
- [-] **GLD-003 — Duplicate digest error**: superseded by GLD-023 — uniqueness
  moves to `content_blob` and an identical upload reuses the existing content
  and document instead of failing.
- [ ] **GLD-004 — Cross-kind actor update error**: non-empty fields of the
  other kind in `UpdateActor` violate a CHECK constraint and surface as
  `INTERNAL`; reject them as `INVALID_ARGUMENT` in the service.
- [ ] **GLD-005 — Partial actor rename**: `UpdateActorRequest.display_name`
  is required by validation, which makes the adapter's "empty means unchanged"
  branch unreachable; make it truly optional or document it as required.
- [ ] **GLD-006 — Document incoming relationships**: `GetDocument` returns only
  outgoing edges, so the `CASE_HAS_DOCUMENT` links to cases are not shown with
  the document; return both directions (proto, service, SPA panel). Needed for
  "same document in several cases" (v2 §20).
- [ ] **GLD-007 — Non-destructive contact replacement**: `replace_contacts`
  physically deletes `actor_contact` rows and the audit event keeps only the
  display name; preserve the replaced contacts (soft delete or before/after
  state), as v2 §35 requires.
- [ ] **GLD-008 — Generated-code reproducibility gate**: add a
  `generated-check` (regenerate, diff `gen/` and `api/openapi/`) to
  `make release-check`; needs network access for the remote OpenAPI plugin.
- [ ] **GLD-009 — Frontend tests**: add unit/component tests to the SPA; the
  gate is currently type-check, lint and build only.
- [ ] **GLD-010 — Observability**: Prometheus metrics and OpenTelemetry traces.

## Phase 0 — V2 alignment without regression (v2 §8, §15-23, §57)

- [x] **GLD-022 — Business reference**: `subject_ref.business_ref` +
  `business_ref_namespace`, a partial unique index on `(namespace,
  business_ref)`, a transactional per-namespace allocator (e.g. `2026-001245`),
  exposed on `SubjectRef` and filterable. The display label stays non-unique.
- [x] **GLD-023 — Document / DocumentVersion / ContentBlob**: additive
  migration creating `content_blob` (SHA-256 UNIQUE, storage ref, size, mime,
  `verified_at`) and `document_version` (`version_no`, blob, `is_final`,
  `is_record`, validation stamps, immutable once validated or record), plus
  `document.current_version_id`; backfill the current documents without loss;
  ingestion per v2 §20 with automatic reuse of an existing blob and document
  (decision in §3g); `goeland.v1` evolved in place (`AddDocumentVersion`,
  version listing), SPA migrated, obsolete `document` columns dropped after
  tests; v2 §49 tests (dedup, shared blob across versions, immutability,
  current version, one document linked to several cases).
- [x] **GLD-024 — BlobStore interface**: domain-neutral `pkg/blobstore.Store`
  (`Put` / `Get` / `Delete`, context-aware, v2 §23) with the local
  `pkg/blobstore/filestore` implementation and a `blobstoretest` conformance
  suite, so S3 or an institutional GED can replace it without touching the
  document model.

Exit criteria: every item of v2 §57 is checked in `IMPLEMENTATION_STATUS.md` §0.

## Phase 1 — Case (v2 §24)

- [x] **GLD-011 — Case slice**: `case_type` + `case_file` + `CaseService`
  (proto-first, mirroring Document/Actor) with `business_ref`, open/close
  lifecycle independent of any workflow, the first `CASE`-source relationship
  types including expanded `CASE_HAS_ACTOR_*` roles, a Vue panel and a
  `pkg/integration` lifecycle test.
- [x] **GLD-034 — End a relationship**: an operation that sets `valid_to`
  ("the relationship ended"), distinct from `UnlinkSubjects` ("the edge was a
  mistake"), each with its own audit event (§3g).

## Phase 1b — Usable Actor and Case (review of v0.6.0, 2026-09-24)

Ordered before Thing at the user's request: v0.6.0 could not be used for real work.

- [x] **GLD-035 — Local SSO documentation**: how to run the SPA in `jwt` mode
  against go-cloud-k8s-auth locally (`AUTH_SERVER_URL`, shared JWT settings,
  redirect allowlist and CORS origins; `localhost` and `127.0.0.1` are distinct
  origins), plus a sign-in panel hint when the redirect is refused.
- [x] **GLD-036 — Navigable relationships**: every relationship row links to
  the page of the related subject (case, document, actor), by mouse and
  keyboard.
- [x] **GLD-037 — Subject picker**: the link dialog searches subjects of the
  relationship type's target kind (actors, documents, cases) instead of asking
  for a UUID.
- [x] **GLD-025 — Minimal USER reference** (moved from Phase 4; ORG_UNIT split
  to GLD-041 on 2026-09-24): internal identities recorded from the token (id,
  name, e-mail) as USER subjects so governance and audit show who acted instead
  of a numeric id, the signed-in user's admin scope is visible, and tasks can
  later target users, without the full security model (§3g).
- [x] **GLD-038 — Actor form clarity and typed complements**: explain display
  name versus legal name (RC), rename "contacts" to typed complements (phone,
  e-mail, IDE, VAT, ...), and validate each complement type in the SPA and the
  API (protovalidate + service).
- [x] **GLD-039 — Person minimal identity**: salutation, last name and first
  name for PERSON actors, the minimum to identify and address a person (§3g
  decision of 2026-09-24).
- [x] **GLD-014 — Actor addresses** (moved from Actor follow-ups): `address` +
  M:N `actor_address` typed (head office, branch, correspondence, billing) with
  one principal address (production `acteur_adresse` + `lien_acteur_adresse`),
  and an `ACTOR_BRANCH_OF_ACTOR` relationship for a branch acting as a distinct
  party, in the API and the Actor UI.
- [x] **GLD-040 — Reference data administration**: admin-scoped screens for
  case types, relationship types, organization categories and document types.

## Phase 2 — Thing (v2 §25)

- [x] **GLD-016 — Thing slice**: `thing` + `thing_type` with `thing_parcel`
  and `thing_building` specializations and PostGIS geometry (EPSG:2056, typed,
  GIST-indexed); `CASE_CONCERNS_THING`, `DOCUMENT_REPRESENTS_THING` and the
  land-rights actor roles (propriétaire, locataire, superficiaire, fermier,
  servitude).

## Phase 3 — Timeline (v2 §26-27)

- [ ] **GLD-012 — Timeline**: `case_timeline_entry` (COMMENT, OPINION,
  DECISION, REQUEST, RESPONSE, VALIDATION, SYSTEM, AI_PROPOSAL) +
  `timeline_document_link`; a validated or locked entry is immutable and a
  correction creates a new entry. Depends on GLD-011.

## Phase 4 — Task (v2 §28)

- [ ] **GLD-041 — Minimal ORG_UNIT reference**: internal organizational units
  (code, label, parent) as ORG_UNIT subjects that tasks and circulations can
  target and that `record_metadata.owner_org_id` can name; split from GLD-025.
- [ ] **GLD-026 — Task**: `case_task` independent of any workflow, assigned to
  a USER or ORG_UNIT, with deadlines, completion and a reassignment history.
  Depends on GLD-011, GLD-025 and GLD-041.

## Phase 5 — Circulation (v2 §29)

- [ ] **GLD-013 — Circulation**: parallel recipients composed of tasks,
  responses (FAVORABLE, UNFAVORABLE, COMMENT, NOT_CONCERNED, NEED_MORE_INFO),
  deadline and completion; a significant response creates a timeline entry.
  Depends on GLD-012 and GLD-026.

Exit criteria for Phases 1-5: the v2 §50 scenario steps 1-25 run end to end,
covered by an integration test.

## Phase 6 — Security (v2 §32, §34)

- [ ] **GLD-017 — Real authorization**: `Can(ctx, user, action, subject)` over
  scope, role, group, org unit, ownership, case participation, typed
  relationships and confidentiality, deny by default (Casbin/OpenFGA evaluated
  behind the interface). Must revisit the automatic document reuse of GLD-023
  against read rights (§3g). Today every authenticated caller gets both
  `goeland:read` and `goeland:write`.
- [ ] **GLD-033 — Sensitive read audit**: `access_audit_event` for
  READ_SENSITIVE, DOWNLOAD and EXPORT on sensitive scopes, distinct from the
  mutation audit.

## Phase 7 — Provenance, outbox, export (v2 §36-38, §51)

- [ ] **GLD-027 — Provenance**: `subject_provenance` (source system, source
  id, import batch); legacy IDs are provenance, never the new UUIDs. May be
  pulled forward with GLD-019.
- [ ] **GLD-028 — Transactional outbox**: `outbox_event` written in the same
  transaction as the mutation and its audit event (v2 §38 event list); from
  then on the v2 §54 "mutation + audit + outbox" criterion applies.
- [ ] **GLD-029 — ExportCase**: JSON + blobs in a ZIP (case, timeline, tasks,
  circulations, relationships, documents, versions, blob metadata and hashes,
  audit summary, provenance, retention), each blob stored once.
- [ ] **GLD-032 — Retention policy**: `retention_policy` + `subject_retention`
  evolving `record_metadata.retention_until` / `sort_final` without breaking
  them, and a governed, proven disposition path (§3g).

## Phase 8 — AI proposal and MCP readiness (v2 §39-41)

- [ ] **GLD-030 — AI proposals with human validation**: `ai_action` trace
  (model, prompt template, inputs, output, PROPOSED / validated / rejected) and
  proposal flows for timeline entries, tasks and relationships; no free SQL,
  no silent mutation; read-and-propose tools shaped for a future MCP server.

## Phase 9 — Workflow (v2 §30)

- [ ] **GLD-031 — Workflow abstraction and engine evaluation**:
  ProcessDefinition / Version / Instance referencing a case (a case never
  depends on one), and an evaluation of Flowable, Temporal and others answering
  "what happens to running instances when a definition changes?".

## Actor follow-ups

- [ ] **GLD-015 — Actor role vocabulary**: seed the remaining production roles
  (`dico_acteur_role`) into `relationship_type` as their target domains land;
  incremental with GLD-011 and GLD-016, not a standalone slice.

Person identity is limited to the minimum of GLD-039; any richer CH-register detail
stays out of scope unless a real need appears, and then only through opaque references.

## Real-data import (MSSQL replica → POC)

Loading the POC is a transform from the legacy-shape replica, not a copy.
Subject IDs are deterministic (`UUIDv5(namespace, "<kind>:<legacyId>")`) so
reruns are idempotent and relationships can be rebuilt later; GLD-027 records
the legacy IDs as provenance. Profiling stays aggregates-only.

- [ ] **GLD-018 — Synthetic actor fixture**: a deterministic, fixed-seed
  generator of fake actors matching the profiled real distributions (kind
  ratio, 33-category histogram, contact-type mix); zero PII, committable, used
  by integration tests and demos. Build before GLD-019.
- [ ] **GLD-019 — Nightly internal refresh**: full rebuild of ACTOR rows only
  (never seeds, migrations or other domains), set-based SQL on the same
  instance, the full `subject_ref` → `record_metadata` → `actor` →
  `actor_contact` chain, a data-cleaning step for legacy rows that break CHECK
  constraints, one import marker instead of per-row audit, advisory-locked. Real
  person data requires the product owner's data-governance sign-off.

## Infrastructure

- [ ] **GLD-020 — Object storage**: an S3-compatible (MinIO) implementation of
  the GLD-024 `blobstore.Store` that passes `blobstoretest.Run` (no proto change).
- [ ] **GLD-021 — Probative integrity verification**: stream the stored bytes,
  recompute SHA-256, set `content_blob.verified_at` and write an audited
  verification event under the write scope (v2 §49 "streaming verification").
