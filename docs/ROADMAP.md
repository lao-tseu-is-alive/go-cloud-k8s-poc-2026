# Goéland POC Roadmap

Tracked version: **v0.11.0**.

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
v0.7.0, the Thing slice (GLD-016) in v0.8.0, and the case spine — Timeline (GLD-012), ORG_UNIT
(GLD-041), Task (GLD-026), Circulation (GLD-013) — in v0.9.0, and the post-audit hardening
(GLD-042, GLD-043) in v0.9.1, and the second review hardening (GLD-044) in v0.9.2. Phase 6,
security, is under way: real authorization (GLD-017, in steps GLD-047 to GLD-050) is complete —
roles and grants shipped in v0.10.0, filtering (GLD-049) and the follow-ups (GLD-050) in v0.11.0;
next is the legacy data import (GLD-051 to GLD-054, a one-shot local load to show the POC on
production data), then the sensitive read audit (GLD-033).

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
- [~] **GLD-009 — Frontend tests**: add unit/component tests to the SPA. Started with
  GLD-042: Vitest on the pure `utils/` modules in `make front-check`, the contact rules checked
  against the server's own cases (shared fixture); component tests remain.
- [ ] **GLD-010 — Observability**: Prometheus metrics and OpenTelemetry traces. The pool
  statistics now shown by the public `/health` move to the metrics endpoint.
- [x] **GLD-042 — Post-audit hardening** (audit of 2026-09-29, decided 2026-09-30): security
  headers on every response; a shared `core` transaction helper and pgx error-mapping base
  (each domain keeps its own messages); batched hydration instead of per-row queries in the
  case, actor, document, org unit and thing searches; every RPC exercised over REST by an API
  surface test (instead of per-adapter fakes, decided 2026-09-30); Vitest on the SPA's
  pure `utils/` wired into `make front-check` (first step of GLD-009); the SPA entry of
  `AGENTS.md` brought up to date. Deferred: rate limiting (belongs to the ingress, see
  PRODUCTION_READINESS) and the k8s smoke test in CI (manual or nightly workflow later).
- [x] **GLD-043 — Unknown REST query parameter answers 500**: the Vanguard transcoder rejects a
  query parameter that matches no request field with `UNKNOWN` / HTTP 500 (found by the API
  surface test, 2026-09-30; also in Vanguard v0.4.0, whose only option discards such parameters
  silently). A middleware on `/api/` checks the query parameters against the request message of
  the matched REST binding and answers 400 INVALID_ARGUMENT, so a mistyped filter is never
  silently ignored.
- [x] **GLD-044 — Second review hardening** (review `reports/report_20260930_gpt-5.md`, decided
  2026-09-30): the PostgreSQL integration, §50 scenario and API surface tests run in CI against a
  PostGIS service (the same `make release-check`, enabled by `GOELAND_TEST_DATABASE_URL`);
  `audit_event` and `reference_change` refuse UPDATE and DELETE in the database; stale README,
  PRODUCTION_READINESS and IMPLEMENTATION_STATUS passages corrected; `make test` works on a clean
  checkout; the PAT cache is bounded and evicts expired entries; `AUTH_SERVER_URL` must be HTTPS
  outside loopback unless explicitly allowed; `X-Request-ID` is bounded; server configuration
  tests. The CI pinning rule is reworded instead (third-party actions by SHA, GitHub's own on
  their major tag).

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

- [x] **GLD-012 — Timeline**: `case_timeline_entry` (COMMENT, OPINION,
  DECISION, REQUEST, RESPONSE, VALIDATION, SYSTEM, AI_PROPOSAL) +
  `timeline_document_link`; a validated or locked entry is immutable and a
  correction creates a new entry. Depends on GLD-011. Scope decided 2026-09-29:
  entries belong to the case (not subjects; audited on the CASE subject),
  DRAFT → VALIDATED | LOCKED | WITHDRAWN, explicit `corrects_entry_id`, business
  `occurred_at`, the linked document version is pinned at validation, linking a
  document also links it to the case, case transitions write SYSTEM entries, a
  case cannot close while drafts remain, authors may validate their own entries
  until GLD-017, `visibility` is stored but not enforced until GLD-017, and
  AI_PROPOSAL stays reserved for GLD-030.

## Phase 4 — Task (v2 §28)

- [x] **GLD-041 — Minimal ORG_UNIT reference**: internal organizational units
  (code, label, parent) as ORG_UNIT subjects that tasks and circulations can
  target and that `record_metadata.owner_org_id` can name; split from GLD-025.
  Scope decided 2026-09-29 from the production structure (aggregates only: one
  tree of 738 units, depth 7, 7 official types, dissolved units kept, 1.7M
  case ↔ unit role links): `pkg/orgunit` + `OrgUnitService`, administrable
  `org_unit_type` (7 seeded types), `org_unit` (label unique among live
  siblings, non-unique abbreviation — revised the same day: the production
  abbreviation is inherited by sub-units, 111 values for 738 units — immutable
  `external_ref` of the source, parent without cycles, functional e-mail,
  dissolution instead of deletion), mutations `goeland:admin` only; `CASE_HAS_ORG_UNIT_LEADER`,
  `_MANAGER` and `_PARTICIPANT` relationship types; `owner_org_id` becomes a
  typed foreign key; SPA tree + detail, picker and governance label; an
  optional local import script of the real tree (code, label, type, parent,
  state only; nothing committed). User ↔ unit membership is deferred to GLD-026.
- [x] **GLD-026 — Task**: `case_task` independent of any workflow, assigned to
  a USER or ORG_UNIT, with deadlines, completion and a reassignment history.
  Depends on GLD-011, GLD-025 and GLD-041. Scope decided 2026-09-29 (the legacy
  system has no task entity): tasks belong to the case (not subjects; audited on
  the CASE subject), administrable `task_type`, OPEN → IN_PROGRESS → DONE |
  CANCELLED (reason) and reopen with a reason, one assignee (user or unit) or
  none, `case_task_assignment` history, `origin` (MANUAL now; CIRCULATION,
  WORKFLOW, AI reserved), a case cannot close with open tasks, completion and
  cancellation write SYSTEM timeline entries; user ↔ unit membership
  (`USER_MEMBER_OF_ORG_UNIT`), `SearchUsers` and a "my tasks" view (mine and
  my units').

## Phase 5 — Circulation (v2 §29)

- [x] **GLD-013 — Circulation**: parallel recipients composed of tasks,
  responses (FAVORABLE, UNFAVORABLE, COMMENT, NOT_CONCERNED, NEED_MORE_INFO),
  deadline and completion; a significant response creates a timeline entry.
  Depends on GLD-012 and GLD-026. Scope decided 2026-09-29 from the production
  structure (aggregates only: 134k circulations, 3.1 recipients on average, 60%
  with several ordered steps, a "for information" copy on almost every one):
  each recipient (user or unit) gets a task (origin CIRCULATION) managed by the
  circulation; recipients are grouped in steps, a step opens when the previous
  one has fully answered; every response completes its task and writes a
  locked RESPONSE timeline entry; the last response completes the circulation
  with a SYSTEM summary entry; cancelling cancels the open tasks; overdue is
  computed (no EXPIRED status, late responses accepted, automatic expiry with
  GLD-028); "for information" recipients deferred until notifications exist.

Exit criteria for Phases 1-5: the v2 §50 scenario steps 1-25 run end to end,
covered by an integration test.

## Phase 6 — Security (v2 §32, §34)

- [x] **GLD-017 — Real authorization** (umbrella of GLD-047 to GLD-050; model decided
  2026-09-30 from the profile of the legacy rights, see IMPLEMENTATION_STATUS §3j): levels
  READ < CONTRIBUTE < MANAGE < FULL_CONTROL, no deny level; grants on any subject (CASE,
  DOCUMENT, ACTOR, THING, ORG_UNIT) to a USER, a GROUP or an ORG_UNIT (covering its sub-units);
  the most specific grant wins — personal, then the caller's groups (highest), then the nearest
  unit, then an application role covering a kind, then the baseline (READ unless confidential,
  `confidentiality_level` >= 2, where no administrator bypass applies); a subject always keeps
  a FULL_CONTROL grant. No live inheritance between a case and its documents: attaching a
  document needs READ on it and CONTRIBUTE on the case and never changes its confidentiality; a
  document deposited from a case copies the case's grants and level once. `Can(ctx, user,
  action, subject)` behind a `core` interface (Casbin/OpenFGA may sit behind it later).
- [x] **GLD-047 — Application roles in Goéland**: `app_role` (ADMIN seeded) and an audited,
  non-destructive `app_user_role` history; `goeland:admin` comes from the ADMIN role, never
  from the token (the auth server's `IsAdmin` is ignored); first administrators from
  `GOELAND_BOOTSTRAP_ADMINS` (user ids, applied on their next request, audited); the last
  administrator cannot be revoked; role administration in the SPA.
- [x] **GLD-048 — Grants and groups**: `access_grant` (typed grantee, level, grantor, reason,
  revocation kept as history, audited on the subject), GROUP subjects with members, the
  `Authorizer` evaluating the precedence above, checks on every mutation and single read of
  the five subject kinds, creator FULL_CONTROL and owner unit MANAGE at creation (backfilled
  for existing subjects), kind-wide roles (ACTOR_MANAGER, THING_MANAGER), "Accès" panel.
  Done 2026-10-01; org unit edits stay with administrators (their grants and memberships use
  the model), searches and lists are filtered in GLD-049.
- [x] **GLD-049 — Filtering and confidentiality**: searches and lists filtered by a shared
  SQL access predicate (pagination stays exact), confidentiality applied, the search ceiling
  derived server-side, downloads through a document or version instead of a raw `?ref=`,
  timeline visibility applied (INTERNAL needs CONTRIBUTE, RESTRICTED needs MANAGE), the
  automatic document reuse of GLD-023 revisited against read rights (§3g). Done 2026-10-01:
  `core.ReadableSQL` + `core.Viewer` on the case, document, actor, thing, org unit (search and
  tree), group, business-reference and relationship lists; `GET /api/documents/{id}/content`
  (`?versionId=`) replaces `/api/documents/download?ref=`. Left as they are: the labels of the
  ancestors and children on an org unit page and of the documents cited by a timeline entry.
- [x] **GLD-050 — Authorization follow-ups**: default grants per case type (placeholders
  creator and creator's unit); the two go-cloud-k8s-auth findings fixed or explicitly
  accepted in that repository (accounts linked by e-mail without the identity provider's
  `email_verified`, user list visible to every authenticated user); unknown JSON body fields
  rejected or kept lenient by an explicit decision (unknown query parameters answer 400,
  GLD-043). Done 2026-10-01 (decided with the user): a case type carries a minimum
  confidentiality and a template of grants (users, groups, units, and CREATOR_UNITS — the
  creator's direct units), copied once at creation (`core.ApplyDefaultGrantsTx`, migration
  `0024`, `SetCaseTypeDefaultGrants`, SPA Administration → Types d'affaire); go-cloud-k8s-auth (0a829b5)
  links by e-mail only when the provider asserts it verified (never for Microsoft) and refuses
  the login otherwise, and keeps the user directory for administrators; request bodies with an
  unknown field answer 400 on REST and Connect (`core.StrictJSONOption`,
  `core.NewStrictJSONCodec`).
- [ ] **GLD-033 — Sensitive read audit**: `access_audit_event` for
  READ_SENSITIVE, DOWNLOAD and EXPORT on sensitive scopes, distinct from the
  mutation audit.

## Phase 7 — Provenance, outbox, export (v2 §36-38, §51)

- [~] **GLD-027 — Provenance**: `subject_provenance` (source system, source
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
the legacy IDs as provenance. Profiling stays aggregates-only. The rules and the
runbook live in [IMPORT_MAPPING.md](IMPORT_MAPPING.md); real data stays local.

- [~] **GLD-051 — Import framework** (started 2026-10-01): `cmd/goeland-import` from the
  replica into a brand-new local database rebuilt on every run (`scripts/import_rebuild.sh`),
  deterministic UUIDv5 ids, provenance and one `import_batch` marker per run (GLD-027 pulled
  forward), set-based loading, a counts-only rejects report, dry run by default.
- [~] **GLD-052 — Import wave 1**: employees (`app_user`, unit membership), org units, security
  groups, case types, cases with status and confidentiality, grants, actors with contacts and a
  correspondence address, and the actor, employee and unit roles on cases.
- [~] **GLD-053 — Behaviour at production volume**: measure searches, the read filter, pagination
  totals and detail pages on the imported data (~610k subjects, ~2.4M grants, ~2.8M
  relationships) and fix what does not hold (indexes, estimated totals, ...). Done 2026-10-01:
  totals counted up to 10 000 (`total_size_capped`, `core.CappedPageSQL`), governance joined
  through a LATERAL subquery so scans follow the sort index (migration `0026`), `jit=off` and
  `plan_cache_mode=force_custom_plan` on the server's connections, relationship panels paged
  ("Charger plus", members of a unit loaded whole): unfiltered case search 2.7 s → 0.04 s, text
  search 0.23 s, a unit with ~125k incoming relationships 0.07 s. After wave 2 (~2.3M
  documents, 81% confidential): document searches scoped to a case or thing are top-level `IN`
  (2 s → 20–90 ms) and the unscoped one counts within a window of the newest 20 000 documents
  (`core.WindowedPageSQL`); with the confirmed document levels every document search takes
  30–120 ms (a table where most rows are unreadable stays the read filter's worst case).
- [~] **GLD-054 — Import wave 2**: timeline entries, document metadata (external reference, no
  bytes), things, links between cases. Done 2026-10-01 (see IMPORT_MAPPING.md): things with an
  approximate location, parcel and building details, documents with their current content known
  by its SHA-256, readers of the confidential documents (levels of the legacy UI, confirmed),
  follow-ups with cited documents and final status, case–thing, case–document, thing–document
  and case–case links, actor roles on things and documents.

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

## UI feedback on real data (2026-10-02)

Remarks of the product owner after seeing the POC on the imported production data.

- [~] **GLD-055 — Sortable lists**: every list sorts by a click on a column header. The paged
  lists (cases, documents, actors, things, my tasks, the tasks of a case) sort on the server
  (`order_by` on their RPC, a whitelist of fields per list, `core.ParseOrderBy` /
  `core.SortedQueries`, indexes of migration `0027`); the short lists (groups) in the browser.
  Done 2026-10-02: `SortableHeader` (aria-sort, keyboard), 0.04–0.3 s on ~512k cases and ~2.3M
  documents except the document "final" and "type" columns (~1.2 s, few distinct values over the
  whole table). The detail panels (relationships, grants, timeline) keep their order for now.

## Infrastructure

- [ ] **GLD-020 — Object storage**: an S3-compatible (MinIO) implementation of
  the GLD-024 `blobstore.Store` that passes `blobstoretest.Run` (no proto change).
- [ ] **GLD-021 — Probative integrity verification**: stream the stored bytes,
  recompute SHA-256, set `content_blob.verified_at` and write an audited
  verification event under the write scope (v2 §49 "streaming verification").
- [ ] **GLD-045 — Orphan upload collection**: bytes uploaded but never attached by
  `CreateDocument` / `AddDocumentVersion` stay forever (blob row and file); give
  unattached blobs an age limit and a cautious collector that never removes a
  referenced blob (review 2026-09-30).
- [ ] **GLD-046 — Separate migrations from the runtime**: a migration job (or
  init container) with the DDL role, and an application role limited to DML, so
  a faulty migration does not block every new pod and the runtime cannot alter
  the schema or drop the audit triggers (review 2026-09-30).
