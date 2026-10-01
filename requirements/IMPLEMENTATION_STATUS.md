# Goéland POC — Implementation Status

Living tracker of what is built vs. what the spec asks for. Update it at the end
of each slice (a few lines), and keep it honest.

- **Active spec (immutable):** [`goeland_poc_domain_model_agent_v2.md`](goeland_poc_domain_model_agent_v2.md) — spec v2, adopted 2026-09-23; cite it as "v2 §N". Do not rewrite it to match reality; record reconciliations in §3g.
- **Historical spec (immutable):** [`goeland_poc_domain_model_agent.md`](goeland_poc_domain_model_agent.md) — v1; §1–§2 below and older "spec §N" citations still refer to it.
- **This document (living):** maps the spec to the current state + records intentional deviations. Task order lives in [`docs/ROADMAP.md`](../docs/ROADMAP.md).
- **Snapshot:** as of **2026-10-01**, app version **0.10.0** (documentation contract enforced by `make release-check`; task order in [`docs/ROADMAP.md`](../docs/ROADMAP.md)). Build/vet/lint/tests green; migrations `0001–0023` applied; verified end-to-end against PostgreSQL. **Core + Document + Actor + Case + Thing + Timeline + ORG_UNIT + Task + Circulation** components live (plus internal users and reference data administration), each exercisable from the **embedded Vue 3 + Vuetify 4 web UI**; metadata-first file upload; repository SQL uses pgx **named parameters**. The Actor component was modelled from the real production `Acteur` schema (profiled read-only) — persons/organizations, typed contacts, 33 seeded org categories, roles kept as relationships.

Legend: ✅ done · 🟡 partial · ⬜ not started

---

## 0. V2 alignment (v2 §48 Phase 0, Definition of Done v2 §57)

| v2 DoD item | State | Notes |
|-------------|-------|-------|
| V2 is the active spec | ✅ | adopted 2026-09-23; v1 kept as history |
| IMPLEMENTATION_STATUS reflects V2 | 🟡 | this table + §3g; §1–§2 still map v1 sections |
| `business_ref` exists | ✅ | GLD-022: migration `0007`, `CoreService.AssignBusinessRef` / `LookupSubjects`, allocation at creation, SPA identity card (ships with the next release) |
| `content_blob` exists, SHA-256 UNIQUE on it | ✅ | GLD-023, migration `0008` |
| `document_version` exists, current Document migrated without loss | ✅ | GLD-023: `0008` backfill (verified on representative rows, reversible), `0009` drops the superseded columns |
| existing filestore still works | ✅ | `internal://` refs behind the `blobstore.Store` interface (GLD-024), with a conformance suite |
| APIs compatible or cleanly versioned | ✅ | `goeland.v1` evolved in place (§3g): `AddDocumentVersion`, `ListDocumentVersions`, `Document.current_version`; removed fields reserved |
| Document UI works | ✅ | migrated: versions panel, reuse notice, integrity on the current version |
| Actor / Document tests green | ✅ | unit + `pkg/integration` |
| global deduplication tested | ✅ | `pkg/integration/document_versions_test.go` (incl. concurrent uploads) |
| same Document linkable to several cases | ✅ | automatic reuse links the same document to each case; `GetDocument` still lists outgoing edges only (GLD-006) |
| no regression audit / auth / security / CI | ✅ | enforced by `make release-check` |

---

## 1. Building blocks (schema + service)

| Spec area | Schema | Service / API | State | Notes |
|-----------|--------|---------------|-------|-------|
| §5.2–5.3 `subject_kind`, `subject_ref` | ✅ `0001` | ✅ `CoreService.CreateSubjectRef/GetSubjectRef` | ✅ | canonical identity, composite `(id,kind)` FK used to pin document kind |
| §5.4 `record_metadata` (governance) | ✅ `0001` | ✅ via Core (create/lock/soft-delete helpers) | ✅ | ownership/confidentiality/locking/versioning; non-destructive |
| §5.5 `audit_event` (append-only) | ✅ `0001` (+ `0021`) | ✅ `CoreService.ListAuditEvents` + written on every mutation | ✅ | every mutation writes an event in the same tx; the database refuses UPDATE, DELETE and TRUNCATE (`0021`, also on `reference_change`) |
| §7 `relationship_type` + `subject_relationship` | ✅ `0002` (+ `0011`) | ✅ `CoreService.LinkSubjects/EndRelationship/UnlinkSubjects/ListRelationships/ListRelationshipTypes` | ✅ | kind-compat validated; one open edge per (source, target, type); ended edges kept as history (GLD-034); unlink = soft delete of a mistake |
| §6.2 / v2 §15-22 `document_type` + `document` + `document_version` + `content_blob` | ✅ `0003` (+ `0005`, `0008`, `0009`) | ✅ `DocumentService.*` (11 RPCs) | ✅ | modern-GED slice; accent-insensitive FTS; finalize+lock; integrity |
| §14 seed: subject kinds, relationship types (10 + 4 case roles/links in `0010`), document types (7) | ✅ `0004`, `0010` | — | ✅ | |
| §13 delivery surface: REST/JSON + **embedded web UI** | — | ✅ Vanguard REST `/api/*` + Vue 3 / Vuetify 4 SPA at `/` | ✅ | every domain as a full slice in the browser (documents, actors, cases with timeline, tasks and circulations, things with geometry preview, org units, my tasks, reference data administration); core panels read-only |
| §6.2 document binary upload (metadata-first) | — | ✅ out-of-proto `POST /api/documents/upload` + `GET /download` (`pkg/blobstore/filestore`) | ✅ | proto stays `storage_ref`-only; local blob store today, MinIO later (§19) |
| §6.1 / v2 §24 `case_type` + `case_file` | ✅ `0010` | ✅ `CaseService.*` (7 RPCs) | ✅ | GLD-011: status lifecycle OPEN/IN_PROGRESS/SUSPENDED/CLOSED with reasons, closed case frozen, reference allocated in the type namespace, accent-insensitive search (also by exact reference) |
| §8 / v2 §26-27 `case_timeline_entry` + `timeline_document_link` | ✅ `0017` | ✅ `TimelineService.*` (9 RPCs) | ✅ | GLD-012: DRAFT → VALIDATED / LOCKED / WITHDRAWN, immutable once out of draft (DB trigger too), corrections as new entries, documents cited by logical id with the version pinned on validation, case status changes as SYSTEM entries, audited on the CASE subject; SPA "Suivis" panel in the case detail |
| §9 / v2 §29 `case_circulation` + `case_circulation_recipient` | ✅ `0020` | ✅ `CirculationService.*` (5 RPCs) | ✅ | GLD-013: a composition of tasks (one per recipient, user or unit, origin CIRCULATION, managed by the circulation), ordered steps, answers as locked RESPONSE timeline entries, completion with a SYSTEM summary, cancellation of open tasks, no closure with an open circulation; overdue computed; SPA case panel + answer from "Mes tâches" |
| §6.3 / v2 §25 `thing` + `thing_type` (+ `thing_parcel`, `thing_building`) | ✅ `0016` | ✅ `ThingService.*` (8 RPCs) | ✅ | GLD-016: EPSG:2056 geometry (GIST, validity, Swiss extent, type per specialization) as GeoJSON with computed area; EGRID / EGID unique; bbox search; land-rights roles `THING_HAS_ACTOR_*`; SPA list / create / detail with SVG preview |
| v2 §8 `subject_ref.business_ref` + namespace + allocator | ✅ `0007` | ✅ `CoreService.CreateSubjectRef{businessRef}` / `AssignBusinessRef` / `LookupSubjects` | ✅ | unique per namespace; free references without namespace; `YYYY-NNNNNN` per namespace and Europe/Zurich year |
| Reference data administration (case types, relationship types, organization categories, document types) | ✅ `0015` | ✅ `Create*/Update*` per catalogue + `CoreService.ListReferenceChanges` | ✅ | GLD-040: `goeland:admin` only; immutable codes, deactivation instead of deletion; every change in the append-only `reference_change` log; SPA administration page |
| v2 §5.7 / §28 internal USER (`app_user`) | ✅ `0012` | ✅ `CoreService.GetCurrentUser/BatchGetUsers` | ✅ | GLD-025: recorded from verified tokens by a verifier decorator (USER subject, audited profile changes); names shown in governance and audit; admin flag and scopes visible in the SPA; ORG_UNIT split to GLD-041 |
| v2 §32 application roles (`app_role`, `app_user_role`) | ✅ `0022` | ✅ `CoreService.ListAppRoles/ListRoleHolders/ListUserRoles/GrantUserRole/RevokeUserRole` | ✅ | GLD-047: `goeland:admin` from the ADMIN role, never the token; bootstrap by `GOELAND_BOOTSTRAP_ADMINS`; reasons, audit on the USER subject, history kept, last administrator protected; SPA Administration → Rôles |
| §6.4 `actor` + `actor_contact` + `organization_category` + v2 addresses | ✅ `0006` (+ `0013`, `0014`) | ✅ `ActorService.*` (6 RPCs) | ✅ | PERSON / ORGANIZATION; typed complements (IDE/TVA/ABACUS/RC, phones, e-mail...) validated and normalized per type (GLD-038); 33 seeded categories; roles kept as relationships; persons carry a minimal identity (salutation, last and first name; `0013`, GLD-039) plus the register link; typed M:N addresses with one principal and non-destructive replacement, branches and contact persons as linked actors (GLD-014) |
| v2 §5.7 / §31 ORG_UNIT (`org_unit_type` + `org_unit`) | ✅ `0018` | ✅ `OrgUnitService.*` (9 RPCs) | ✅ | GLD-041: one tree without cycles (service + trigger, serialized mutations), labels unique among live siblings, non-unique abbreviation, immutable `external_ref`, dissolution instead of deletion; typed `record_metadata.owner_org_id`; `CASE_HAS_ORG_UNIT_LEADER` / `_MANAGER` / `_PARTICIPANT`; optional import of the real tree (`cmd/goeland-import-orgunits`); SPA tree + detail |
| §4.1 / v2 §28 `case_task` (+ `task_type`, `case_task_assignment`) | ✅ `0019` | ✅ `TaskService.*` (13 RPCs) + `CoreService.SearchUsers` | ✅ | GLD-026: OPEN → IN_PROGRESS → DONE / CANCELLED, reopen with a reason, one assignee (user or unit) with history, `origin` for circulation / workflow / AI, "my tasks" (mine and my units' via `USER_MEMBER_OF_ORG_UNIT`), SYSTEM timeline entries on completion and cancellation, a case cannot close with open tasks; SPA case panel + "Mes tâches" + unit members |
| §10 `access_grant` + confidentiality enforcement | ✅ `0023` | ✅ `AccessService.*` (11 RPCs) | ✅ | GLD-048: grants to users, groups and units, most specific wins, confidentiality without bypass, enforced on mutations and single reads of every kind and on case-owned entities; GLD-049: searches and lists filtered in SQL (`core.ReadableSQL`, exact pagination), downloads through the document (`GET /api/documents/{id}/content`), timeline visibility by level, document reuse only among readable documents; default grants per case type in GLD-050 |
| §14.5/§14.6 seed: test users, org units, case types, thing types | 🟡 `0010`, `0016`, `0018` | — | 🟡 | case types `OPC_DEMANDE_PC` (OPC), `GENERIC_REQUEST` (GEN); thing types PARCEL, BUILDING, STREET, TREE, INFRASTRUCTURE, ADVERTISEMENT, SPORT_ZONE; org unit types (7); users are recorded from tokens; org units come from the optional import, not from seed data |

---

## 2. Minimal end-to-end scenario (spec §3.1)

The v2 §50 scenario is now played end to end over HTTP by `cmd/goeland-server/scenario_test.go`
(steps 1–25 and 28–30; 26–27 AI proposal and 31 ExportCase are logged as skipped until GLD-030 /
GLD-029). The v1 16-step demo is complete too; in addition **Actor, Case and Thing are now done** (persons/organizations creatable and
linkable as relationship targets). Document- and actor-side steps are done and verified
via ConnectRPC **and exercisable from the embedded web UI** (create → detail → verify/
lifecycle → edit blocked when locked → audit):

- ✅ (7) add a document of type PLAN — `CreateDocument`
- ✅ (8) link the document to a case — `link_to_case_id` on create / `LinkDocument` (works once a CASE subject exists)
- ✅ (9) link document to a thing (`DOCUMENT_REPRESENTS_THING`) — via `LinkDocument` (relationship type seeded; needs a THING subject)
- ✅ (16) consult the audit — `GetDocument{includeAudit}` / `GetActor{includeAudit}` / `CoreService.ListAuditEvents`
- ✅ (1) create an `OPC_DEMANDE_PC` case — `CreateCase` (reference `YYYY-NNNNNN` allocated in namespace `OPC`)
- ✅ (4–5) create an actor and link it as requester/mandatee — `CreateActor` + `LinkSubjects(CASE_HAS_ACTOR_*)`
- ✅ (2–3) create a parcel and a building as THING — `CreateThing` (with LV95 geometry, EGRID / EGID)
- ✅ (6) link the case to the parcel — `LinkSubjects(CASE_CONCERNS_THING)`; (9) `DOCUMENT_REPRESENTS_THING` now has THING targets
- ✅ (10–12) add a follow-up, cite a document, validate it → immutable — `CreateTimelineEntry{documentIds}` + `ValidateTimelineEntry` (v2 §50 steps 19–21)
- ✅ (13–14) create tasks and reassign one with history — `CreateTask` + `AssignTask` (v2 §50 steps 22–23)
- ✅ (15–16) send a circulation to two units / users and record the answers in the timeline — `CreateCirculation` + `RespondToCirculation` (v2 §50 steps 24–25)

---

## 3. Decided enhancements 🚀 (do not regress)

> **Guiding principle.** The spec is a *starting point, not a ceiling.* Where we
> deliberately went further than the original requirements, the spec's silence is
> **not** a reason to regress: do not remove capability or degrade UX just because
> an item is "not in the spec". The bar is **the best user experience, never at the
> expense of security, readability, or maintainability.** If a spec item and that
> bar ever conflict, reconcile it explicitly (note it here) rather than silently
> dropping to the lesser option.

These are deliberate betterments beyond the spec — keep them:

- 🚀 **Proto-first ConnectRPC + Vanguard + protovalidate**, instead of REST-first
  (spec §13 offered REST now, "connect-rpc later"). Gains: a typed contract,
  generated clients, edge validation, and Connect/gRPC/gRPC-Web from one handler.
  RPC paths are `/goeland.v1.<Service>/<Method>`; REST can still be added later via
  `google.api.http` annotations without breaking anything.
- 🚀 **Bundleable module architecture** (`pkg/<domain>/module`, one shared
  transcoder / pool / auth verifier). Each domain runs standalone or composes into
  a single server — a real capability the spec's flat layout didn't offer.
- 🚀 **Richer, modern-GED Document** than spec §6.2: `external_system/id/url` (no
  duplication / interop), `sha256` + `sha256_verified_at` (probative integrity),
  `is_record`, `status`, `language`, `page_count`, versioning (since v2: real
  `document_version` rows replace `previous_version_id`),
  governance locking. These serve real GED UX and must not be trimmed back.
- 🚀 **Accent-insensitive full-text search** (migration `0005`, `immutable_unaccent`):
  "chateau" finds "château". Pulled forward from the "future search" idea (spec §19.3)
  because it materially improves search UX at negligible cost.
- 🚀 **Non-destructive + fully audited by construction**: every mutation writes an
  audit event in the same transaction, soft-delete everywhere. Spec asks for this
  (§17); we treat it as a hard invariant, not an aspiration.
- 🚀 **Embedded Vue 3 + Vuetify 4 web UI** (`cmd/goeland-server/goeland-front`,
  `//go:embed`, served at `/`): the Document module is usable end-to-end in the
  browser (list/create+upload/detail/edit/finalize/verify/link/delete + read-only
  governance & audit), bilingual fr-CH/en, dynamic `dev`/`jwt` auth via `GET /config`.
  The spec (§13) offered "REST now, UI later"; a real embedded SPA over the typed
  REST surface is a betterment — keep it.
- 🚀 **Metadata-first file upload** kept **out of the proto contract**: binary bytes flow
  through `POST /api/documents/upload` → deduplicated `content_blob` → `contentBlobId` →
  `CreateDocument` / `AddDocumentVersion` (server computes sha256/size/mime;
  `pkg/blobstore/filestore` behind `blobstore.Store`, path-traversal guarded). Preserves proto validation /
  governance / audit while still supporting real file upload; swap the local blob store
  for MinIO later without touching the contract.

- 🚀 **Timeline beyond v2 §26-27 (GLD-012)**: a business date `occurred_at` distinct from
  the recording time (the timeline is ordered on it — a call from yesterday is recorded
  today); explicit corrections (`corrects_entry_id`, shown both ways); withdrawal of a draft
  with a reason instead of deletion; the cited document version pinned on validation (the
  "future need" of §27, cheap and probative); case status changes recorded as SYSTEM entries
  with structured metadata so the SPA renders them in the user's language; `draft_count` so
  the SPA explains why a case cannot be closed yet.

- 🚀 **Enforced documentation contract** (2026-09-23, beyond the spec): the
  [`docs/DOCUMENTATION.md`](../docs/DOCUMENTATION.md) contract ported from `go-pdf-forge`
  — GoDoc + Protobuf `COMMENTS` coverage, an exact file-by-file atlas, executable claims
  and one gate (`make release-check`) shared by local runs and CI. Rationale: agents work
  without memory, and the hand-synced layers (proto ↔ SQL ↔ model ↔ `types.ts`) are the
  known drift risk. Adoption runs in five slices (status table in the contract).

### 3b. Neutral architectural choices (vs the spec's suggestions)

Not betterments, just a different-but-equivalent option chosen for consistency:

- **Go layout** `pkg/<domain>` rather than `internal/domain|app|infra` CQRS (spec §11).
- **One migrations dir** owned by the core module (`0001–…`) rather than a per-domain
  `/migrations` split — document tables FK into core, so core owns the bootstrap; splittable later.
- **`record_metadata` actor/owner columns are `TEXT`, not `uuid`** (spec §5.4) — auth
  identities arrive as strings (`"system"`, app user id).
- **`confidentiality_level` range 0–5** (spec text says 0–3 in one place) — matches proto validation, leaves headroom.

### 3c. Known gaps (LESS than the spec — backlog, not enhancements)

- **Authorization follow-ups** — per-subject grants, confidentiality and filtered lists are
  enforced (GLD-048, GLD-049); default grants per case type (GLD-050) and the sensitive read
  audit (GLD-033) are not built yet. The labels of an org unit's ancestors and children and of
  the documents cited by a timeline entry are shown without a read check.

### 3d. Review quick-wins applied (2026-07-07, from `reports/report_20260707_codex.md`)

Hardened after the technical review (all verified end-to-end):

- 🔒 **Trustworthy attribution** — the operator is always the authenticated principal
  (`core.OperatorID`); the forgeable request `actor_user_id` field was removed. Operator ≠
  domain ACTOR (see AGENTS.md).
- 🔒 **Honest integrity** — `VerifyDocumentIntegrity` is now non-mutating and non-probative
  (stored-hash comparison; blank expected ≠ verified; reads no bytes).
- 🔒 **Lifecycle invariants** — update/finalize/delete/link reject locked/soft-deleted
  records atomically (`EnsureMutableTx`, `SELECT … FOR UPDATE`).
- 🔎 **Audit correlation** — `X-Request-ID` is propagated into every `audit_event.request_id`;
  access logs capture status + bytes.
- 🔗 **Consistency** — title updates sync `subject_ref.display_label`; `previous_version_id`
  now also creates the `DOCUMENT_PREVIOUS_VERSION` edge; inactive document/relationship types
  are rejected. (Also fixed a latent ambiguous-`id` bug in the relationships JOIN query.)
- 🌐 **REST surface** — `CoreService` and `DocumentService` RPCs carry `google.api.http`
  annotations (spec §13.3/§13.6), so Vanguard serves them as REST/JSON (`/api/documents/...`,
  `/api/subjects/...`, `/api/relationships/...`) alongside Connect/gRPC, and the generated
  OpenAPI documents real paths.
- 🧱 **Ops** — Dockerfile ships CA roots + version ldflags + pinned builder; the ConfigMap
  helper no longer emits secrets.
- ✅ **Tests** — added unit tests for operator identity, error mapping, request-id context,
  document validation, lock propagation, and hash comparison.

Still open from the review (bigger than quick-wins): a real authorization/confidentiality
policy, streamed probative hashing, metrics/tracing.

### 3e. Review quick-wins applied (2026-07-09, from `reports/report_20260709_gpt-5.md`)

- 🧪 **DB integration tests** — new `pkg/integration` package (see §4), env-gated on
  `GOELAND_TEST_DATABASE_URL`; migrations idempotency/seed + full document lifecycle.
- 🏗️ **CI** — three GitHub Actions workflows added (`cve-trivy-scan`, `docker-publish`,
  `release`), with Go version sourced from `go.mod` and third-party actions SHA-pinned.
- 📦 **Self-contained Docker build** — the image now builds the embedded frontend in a
  dedicated `bun` stage before the Go build, so it is reproducible from a clean checkout
  (previously `//go:embed dist/*` required a pre-built, git-ignored `dist/`).
- 🧹 **Deterministic package discovery** — `make test`/`make lint` exclude Go packages under
  the frontend `node_modules` tree, so `bun install` no longer pollutes `go list ./...`.
- 🔐 **Admin scope naming** — the wildcard admin scope is now `goeland:admin` (was the stale
  cross-project `notes:admin`).
- ⏱️ **Token remint** — the SPA now re-mints JWTs at ~80% of lifetime and never schedules a
  remint past expiry (previously a 30s floor could fire after short-lived tokens expired).
- 📄 **Production-readiness doc** — [docs/PRODUCTION_READINESS.md](../docs/PRODUCTION_READINESS.md).

Still open (kept for later): Prometheus/OpenTelemetry instrumentation (#6).

### 3f. Actor slice (2026-07-10) — modelled from real production data

The Actor component (spec §6.4) was designed against the **real production Goéland
`Acteur` schema**, profiled read-only on a local replica (aggregates only — no PII pulled):

- 🧭 **Reality-checked model** — `Acteur.IsPhysique` → `actor_kind` PERSON/ORGANIZATION;
  `ActMoral` → organization fields + a 33-term `organization_category` seeded from the real
  `DicoActMoralCategory`; `ActeurComplement` → typed `actor_contact` (contact channels +
  business identifiers IDE/TVA/ABACUS/registre du commerce, kept first-class/queryable).
- 🔗 **Roles are relationships, not attributes** — production's 1.5M-row polymorphic
  `ActeurRole` table is the legacy form of our typed `subject_relationship`. The actor entity
  carries **zero** role columns; actors attach to cases/documents/things via `relationship_type`
  edges, so the real 46-role vocabulary maps in with the Case/Thing slices (nothing blocks it).
- 🔒 **Minimal personal data** — the PERSON specialization stores only the minimal identity
  (salutation, last and first name, GLD-039, which superseded the initial register-link-only
  rule; see §3g) plus `is_ch_register` + an opaque `ch_register_ref`; no civil-registry data.
- 🖥️ **Full vertical slice** — proto-first `ActorService` (6 RPCs) + `0006_actor.sql` +
  atomic create (subject_ref + record_metadata + contacts + audit) + embedded Vue/Vuetify panel
  (bilingual) + `pkg/integration` lifecycle/specialization tests, all green against real PostGIS.
- ⬜ Deferred to later actor slices: addresses (`lien_acteur_adresse`), the CH-register person
  detail beyond the link flag, and seeding the full production role vocabulary.

### 3g. V2 reconciliation decisions (2026-09-23)

Decisions taken when adopting v2; they complete or adjust the spec without rewriting it.

- **Automatic document reuse on identical content (v2 §5.4, §20)** — kept as specified: an
  upload whose SHA-256 matches an existing `content_blob` reuses the blob *and* the existing
  document, and the new context is expressed by relationships. ⚠️ Accepted risk: until real
  authorization exists (GLD-017), this can link or reveal a document of a confidential case
  from another case, and an upload response can act as an existence oracle. GLD-017 must
  revisit reuse against confidentiality and read rights. **Revisited in GLD-049:** reuse only
  picks a document the caller may read (otherwise a new document is created on the same blob),
  so it neither reveals nor attaches an unreadable document; the upload's `reused` flag on the
  blob remains the accepted existence oracle.
- **API stays in `goeland.v1` (v2 §21)** — no `goeland.v2` package: nothing runs in
  production, so the Document/Version/Blob split evolves `goeland.v1` directly and obsolete
  `Document` fields may be removed once the SPA is migrated, without a deprecation period.
- **Order: Document alignment before Case (v2 §48)** — business_ref + content_blob +
  document_version first, while no Case/Timeline code depends on the old document model.
- **Current version is explicit** — `document.current_version_id` (set in the same
  transaction as a new version) rather than `max(version_no)`; `is_final` / `is_record` move to
  `document_version`, `record_metadata.is_locked` stays subject-level.
- **Lifecycle mapping (v2 §35)** — CLOSED in `case_file.status`, LOGICALLY_DELETED in
  `record_metadata.deleted_at`, ARCHIVED / DISPOSED in the future retention tables. DISPOSED
  needs a governed destruction path with proof, distinct from domain services.
- **Outbox only from v2 Phase 7** — the v2 §54 criterion "mutation + audit + outbox" applies
  once the outbox exists; until then "mutation + audit".
- **Minimal USER / ORG_UNIT reference before Task (v2 §28, §50)** — task assignees need real
  targets; full security stays in Phase 6. USER shipped first (GLD-025, 2026-09-24): the auth
  service owns accounts, `app_user` only mirrors what verified tokens say, and the audit log
  records name and admin changes but not e-mail addresses. ORG_UNIT follows as GLD-041.
- **business_ref allocation** — a transactional per-namespace (and per-year when relevant)
  counter plus a partial unique index on `(namespace, business_ref)`.
- **Thing geometry** — explicit SRID (EPSG:2056, Swiss LV95), geometry type and GIST index.
- **End vs undo a relationship** — "ended" sets `valid_to`; "unlinked" (soft delete) means the
  edge was a mistake. Two operations, two audit events. Implemented by GLD-034:
  `EndRelationship` (`RELATIONSHIP_ENDED`, default end = server time, a future end is a
  scheduled end, never before `valid_from`); uniqueness applies to open edges only (`0011`),
  so an ended role can be given again, and ended edges stay listed as history.
- **Document split details (GLD-023)** — a version may have no content (metadata-only
  document or external reference) and a blob may be digest-only (empty storage ref: bytes
  held elsewhere) so the backfill is lossless; declaring a record makes the version final;
  `external_*` stay document-level (decided enhancement §3); content identity is the
  server-registered `content_blob_id`, never a client-supplied digest (otherwise reuse would
  let a client attach any document by quoting its hash); the upload endpoint now requires
  `goeland:write` (download `goeland:read`); unregistered duplicate bytes are removed at once,
  orphan blobs of abandoned uploads are left for a later GC.
- **Case lifecycle (GLD-011, v2 §24, §35)** — `case_file.status` is OPEN / IN_PROGRESS /
  SUSPENDED / CLOSED with an explicit transition table (no self transitions; a closed case can
  only be reopened); closing and reopening require a reason, recorded in the
  `CASE_STATUS_CHANGED` audit event and, on close, in the closure stamps; a closed case
  rejects edits (FAILED_PRECONDITION) until reopened. Soft deletion stays in
  `record_metadata`. The type's `business_ref_namespace` drives default reference allocation;
  an explicit reference request overrides it.
- **Person minimal identity (2026-09-24, GLD-039)** — supersedes "persons carry no PII":
  a PERSON actor stores salutation, last name and first name, the minimum to identify and
  address a person; no birth date, AVS number or civil-registry data. The real-data import
  stays aggregates-only for profiling (GLD-018/019 decide what enters the POC).
- **Branches (2026-09-24, GLD-014)** — addresses are M:N and typed (head office, branch,
  correspondence, billing) with one principal; a branch acting as a distinct party (its own
  complements and cases) is a separate ORGANIZATION linked by `ACTOR_BRANCH_OF_ACTOR`.
- **Timeline (GLD-012, v2 §26-27, 2026-09-29)** — an entry belongs to its case and is not a
  subject: no `subject_ref`, its audit events go to the CASE subject with the entry id in
  metadata. `timeline_document_link` has its own id and a removal stamp instead of the v2
  composite primary key, so a document removed from a draft stays as history and can be cited
  again. VALIDATED (endorsed) and LOCKED (frozen as is) are both immutable; a correction is a
  new entry (`corrects_entry_id`, same case, at most one live correction). A case cannot close
  while drafts remain (validate, lock or withdraw them) and a closed case accepts no timeline
  change. Authors may validate their own entries and `visibility` is stored but not enforced
  until GLD-017; SYSTEM entries are server-written only and AI_PROPOSAL is reserved for GLD-030.
- **ORG_UNIT (GLD-041, v2 §5.7, §31, 2026-09-29)** — modelled from the production structure
  (aggregates only). There is no natural unique code: the abbreviation names the service a unit
  belongs to and is inherited by most sub-units (111 values for 738 units) and labels repeat
  across services, so identity is the subject id, live siblings never share a label, and
  `external_ref` keeps the source id. A unit is dissolved, never deleted, and only without live
  sub-units; a dissolved unit takes no child, owned subject or relationship. `owner_org_id`
  became a typed foreign key (no unit existed before, so earlier free-text values were dropped).
  User ↔ unit membership is deferred to GLD-026, where tasks need it.
- **Task (GLD-026, v2 §28, 2026-09-29)** — the legacy system has no task entity, so the model is
  new. A task belongs to its case (not a subject; audited on the CASE subject), like a timeline
  entry. One assignee at a time (an `app_user` or a live org unit) or none; every (re)assignment
  is a `case_task_assignment` row. A done or cancelled task may be reopened with a reason (its
  completion stamps are cleared, the audit keeps them). A case cannot close with open tasks.
  Only completion and cancellation write SYSTEM timeline entries (creation and reassignment stay
  in the history and the audit). Unit membership is the `USER_MEMBER_OF_ORG_UNIT` relationship;
  the SPA offers it to administrators, but until GLD-017 any writer may create it through the API.
- **Circulation (GLD-013, v2 §29, spec v1 §9, 2026-09-29)** — modelled from the production
  structure (aggregates only: 134k circulations, 3.1 recipients on average, 60% with several
  ordered steps). A circulation is a composition of tasks: each recipient (one user or one live
  unit) gets a task (origin CIRCULATION) when its step opens; such a task can only be started
  directly, its completion and cancellation belong to the circulation. Every answer (even
  NOT_CONCERNED) is a locked RESPONSE timeline entry authored by the operator who records it
  (until GLD-017 any writer may record an answer, e.g. one received by mail). Deviation from v1 §9:
  no EXPIRED status — overdue is computed and late answers are accepted; automatic expiry with its
  audit event comes with the scheduler of GLD-028. "For information" recipients (a copy on almost
  every production circulation) are deferred until notifications exist.
- **v2 SQL snippets are illustrative** — implementations follow repo conventions
  (`NOT NULL DEFAULT ''` strings, enum-backed `SMALLINT` statuses, alias-prefixed projections).

### 3h. Review quick-wins applied (2026-09-30, from `reports/report_20260929_opus-5.5.md`, GLD-042)

- 🔐 **Browser security headers** on every response (`cmd/goeland-server/headers.go`): CSP
  limited to the own origin (inline styles for Vuetify, the auth server in `connect-src` in jwt
  mode), nosniff, no framing, referrer, opener and permissions policies; checked in Chrome on
  every SPA page without a violation. HSTS stays with the TLS terminator.
- 🧹 **Shared database plumbing** — `core.InTx` and a `core.MapDBError` base (SQLSTATE
  constants, `PgErrorWithCode`) replace the per-domain copies; each domain keeps its messages.
- ⚡ **Batched search hydration** — case, actor, document, thing and org unit searches load a
  page's related rows in a fixed number of queries (`core` batch loaders), guarded by a
  query-counting integration test (was three to five queries per row).
- 🧪 **API surface test** — every RPC the §50 scenario does not reach is called once over REST;
  Connect adapters 72-79 % covered (were 13-56 %), 74.8 % overall. It found GLD-043 (an unknown
  query parameter answers 500).
- 🧪 **SPA unit tests** — Vitest on the pure `utils/` modules in `make front-check`; the contact
  rules run against the server's own cases (`pkg/actor/testdata/contact_values.json`).
- 🛡️ **Unknown REST query parameters answer 400** (GLD-043, `cmd/goeland-server/queryparams.go`):
  checked against the request message of the matched binding instead of the transcoder's 500.

Deferred: rate limiting (ingress, see PRODUCTION_READINESS) and the k8s smoke test in CI.

### 3i. Review quick-wins applied (2026-09-30, from `reports/report_20260930_gpt-5.md`, GLD-044)

- 🧪 **Database tests in CI** — a PostGIS service runs the integration tests, the §50 scenario and
  the API surface test in the same `make release-check`; `GOELAND_REQUIRE_DB_TESTS=true` makes a
  missing database a failure, so a green CI can no longer mean "skipped".
- 🔐 **Append-only logs in the database** (`0021`): `audit_event` and `reference_change` refuse
  UPDATE, DELETE and TRUNCATE; separating the DDL role from the runtime is GLD-046.
- 🔐 **Auth hardening** — bounded PAT cache; HTTPS required for `AUTH_SERVER_URL` outside
  loopback (opt-in for mesh-encrypted clusters); bounded, validated `X-Request-ID`.
- 🧹 **Developer experience** — `make test` / `make lint` work on a clean checkout; configuration
  tests; stale documentation corrected. CI pinning rule reworded (third-party actions by SHA,
  GitHub's own on their major tag).

Planned from the same review: governed downloads and a server-side confidentiality ceiling
(GLD-017), orphan upload collection (GLD-045), migration/runtime separation (GLD-046).

### 3j. Authorization model (decided 2026-09-30, GLD-017, from the legacy rights profile)

The legacy grants rights per case to an employee, an org unit or a security group (2.4M rows on
512k cases: 78% to units, 22% to employees, median 3 per case), on the scale Contrôle total /
Edition / Ajout suivis-documents / Consultation / Aucun accès; non-confidential cases are readable
by every employee; about 120 security groups act as application roles; documents use a separate
0-6 confidentiality level relative to the poster's unit and 436k per-document group grants.
Decisions for the POC (v2 §32):

- 🚀 **One model for every subject** (CASE, DOCUMENT, ACTOR, THING, ORG_UNIT) — the legacy could not
  protect actors or things, a known pain point.
- **Levels** READ < CONTRIBUTE < MANAGE < FULL_CONTROL; **no deny level** (63 legacy rows): a
  restriction is a more specific, lower grant.
- **Most specific grant wins**: personal, then the caller's groups (highest among them), then the
  nearest org unit (a unit grant covers its sub-units), then a kind-wide application role, then
  the baseline — READ when `confidentiality_level` < 2, nothing otherwise, and no administrator
  bypass on confidential subjects. The legacy's first-match order is dropped.
- **Groups** are GROUP subjects (named sets of users, not nested at first) — needed for the
  cross-unit audiences of sensitive documents.
- **No live inheritance** between a case and its documents: linking never changes a document's
  confidentiality or widens its readers (READ on the document and CONTRIBUTE on the case are
  required); depositing a document from a case copies the case's grants and level once.
- 🚀 **Grants carry grantor, date and reason and keep their history** (the legacy overwrites in
  place); application roles live in Goéland (`goeland:admin` from the ADMIN role, not the token);
  first administrators from `GOELAND_BOOTSTRAP_ADMINS`.

---

## 4. Tests

- ✅ Pure unit tests: pagination, subject-kind validation, dbmate migration parser, authadapter.
- ✅ Automated DB integration tests (`pkg/integration`): migrations idempotency + seed data,
  the full document lifecycle (create → search → metadata update → link → finalize+lock →
  locked-update rejected → soft delete → deleted-mutation rejected → audit trail), and the
  **actor lifecycle** (org create with contacts+category → search → update/label-sync →
  case→actor link → soft-delete+rejection → audit; person minimal identity (derived display name, search by names, required last name);
  organization `legal_name` required; 33 categories seeded), and the **case lifecycle**
  (seeded types → create with allocated reference → actor roles + document links →
  transitions with reasons → closed-case freeze → reopen → explicit reference → soft delete),
  and the **timeline** (draft citing a document → auto case link → edit → validate with
  pinned version → immutability through the service and the DB triggers → corrections rules
  → drafts block closure → SYSTEM entry on close → closed case rejects entries → case audit),
  the **circulations** (two steps with user and unit recipients, managed tasks, answers opening
  the next step, completion summary, closure refused while open, cancellation),
  the **tasks** (creation, reassignment history, assignee checks, state machine, SYSTEM entries,
  "my tasks" through unit membership, closure refused with open tasks, closed case frozen),
  and the **org units** (seeded types → tree path → sibling labels and external references →
  no cycle through the service and the trigger → dissolution order → owning unit and case roles
  refused for a dissolved unit).
  Env-gated on
  `GOELAND_TEST_DATABASE_URL` (needs PostGIS/pgcrypto/pg_trgm/unaccent); skipped when unset so
  `go test ./...` stays green without a database.
- ✅ **End-to-end scenario** (`cmd/goeland-server/scenario_test.go`, 2026-09-29): v2 §50 over HTTP
  against the real handler (REST JSON through Vanguard, dev tokens), covering authentication (401),
  scopes (403 without `goeland:admin`), protovalidate (400), error mapping (FAILED_PRECONDITION on
  a closed case) and the module wiring. The **API surface test** (`api_surface_test.go`,
  2026-09-30) calls every other RPC once. Combined coverage with the integration tests: 74.8%.
- ⬜ Broader DB integration coverage (spec §16: relationship / timeline / circulation /
  security) — add alongside each new domain, following the `pkg/integration` pattern.
- 🟡 Frontend: Vitest unit tests of the pure `utils/` modules (44 tests, contact rules shared
  with the server) in `make front-check`, beside type-check, lint and build; component tests
  are still to come (GLD-009).
- ✅ CI (`.github/workflows`): Trivy image CVE scan on push/PR to `main`; unit tests +
  image build/scan/publish on version tags; cross-compiled binary release on version tags.

---

## 5. Next slices

Implementation order and task state now live in [`docs/ROADMAP.md`](../docs/ROADMAP.md)
(`GLD-NNN` task IDs, 2026-09-23): Case spine (Case, Timeline, Circulation), Actor
follow-ups, Thing, real authorization, real-data import, plus the contract-honesty
fixes surfaced while documenting the API. This section no longer duplicates that order.

Keep honouring the design rules (spec §17): explicit model, no EAV, JSONB only for
secondary data, non-destructive deletes, every mutation audited, every relationship
typed & validated.
