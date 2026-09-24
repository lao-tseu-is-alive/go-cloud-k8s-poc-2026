# Goéland POC — Implementation Status

Living tracker of what is built vs. what the spec asks for. Update it at the end
of each slice (a few lines), and keep it honest.

- **Active spec (immutable):** [`goeland_poc_domain_model_agent_v2.md`](goeland_poc_domain_model_agent_v2.md) — spec v2, adopted 2026-09-23; cite it as "v2 §N". Do not rewrite it to match reality; record reconciliations in §3g.
- **Historical spec (immutable):** [`goeland_poc_domain_model_agent.md`](goeland_poc_domain_model_agent.md) — v1; §1–§2 below and older "spec §N" citations still refer to it.
- **This document (living):** maps the spec to the current state + records intentional deviations. Task order lives in [`docs/ROADMAP.md`](../docs/ROADMAP.md).
- **Snapshot:** as of **2026-09-23**, app version **0.5.0** (documentation contract enforced by `make release-check`; task order in [`docs/ROADMAP.md`](../docs/ROADMAP.md)). Build/vet/lint/tests green; migrations `0001–0006` applied; verified end-to-end against PostgreSQL. **Core + Document + Actor** components live, each exercisable from the **embedded Vue 3 + Vuetify 4 web UI**; metadata-first file upload; repository SQL uses pgx **named parameters**. The Actor component was modelled from the real production `Acteur` schema (profiled read-only) — persons/organizations, typed contacts, 33 seeded org categories, roles kept as relationships.

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
| §5.5 `audit_event` (append-only) | ✅ `0001` | ✅ `CoreService.ListAuditEvents` + written on every mutation | ✅ | every mutation writes an event in the same tx |
| §7 `relationship_type` + `subject_relationship` | ✅ `0002` (+ `0011`) | ✅ `CoreService.LinkSubjects/EndRelationship/UnlinkSubjects/ListRelationships/ListRelationshipTypes` | ✅ | kind-compat validated; one open edge per (source, target, type); ended edges kept as history (GLD-034); unlink = soft delete of a mistake |
| §6.2 / v2 §15-22 `document_type` + `document` + `document_version` + `content_blob` | ✅ `0003` (+ `0005`, `0008`, `0009`) | ✅ `DocumentService.*` (11 RPCs) | ✅ | modern-GED slice; accent-insensitive FTS; finalize+lock; integrity |
| §14 seed: subject kinds, relationship types (10 + 4 case roles/links in `0010`), document types (7) | ✅ `0004`, `0010` | — | ✅ | |
| §13 delivery surface: REST/JSON + **embedded web UI** | — | ✅ Vanguard REST `/api/*` + Vue 3 / Vuetify 4 SPA at `/` | 🟡 | Document, Actor and Case modules as full slices in the browser; core panels read-only; Thing UI pending its service |
| §6.2 document binary upload (metadata-first) | — | ✅ out-of-proto `POST /api/documents/upload` + `GET /download` (`pkg/blobstore/filestore`) | ✅ | proto stays `storage_ref`-only; local blob store today, MinIO later (§19) |
| §6.1 / v2 §24 `case_type` + `case_file` | ✅ `0010` | ✅ `CaseService.*` (7 RPCs) | ✅ | GLD-011: status lifecycle OPEN/IN_PROGRESS/SUSPENDED/CLOSED with reasons, closed case frozen, reference allocated in the type namespace, accent-insensitive search (also by exact reference) |
| §8 `case_timeline_entry` + `timeline_document_link` | ⬜ | ⬜ `TimelineService` | ⬜ | timeline is the primary case history (spec §17.8) |
| §9 `case_circulation` + `case_circulation_recipient` | ⬜ | ⬜ `CirculationService` | ⬜ | depends on Case + Timeline |
| §6.3 `thing` + `thing_type` (+ `thing_parcel`, `thing_building`) | ⬜ | ⬜ `ThingService` | ⬜ | PostGIS geometry (extension already enabled in `0001`) |
| v2 §8 `subject_ref.business_ref` + namespace + allocator | ✅ `0007` | ✅ `CoreService.CreateSubjectRef{businessRef}` / `AssignBusinessRef` / `LookupSubjects` | ✅ | unique per namespace; free references without namespace; `YYYY-NNNNNN` per namespace and Europe/Zurich year |
| §6.4 `actor` + `actor_contact` + `organization_category` | ✅ `0006` | ✅ `ActorService.*` (6 RPCs) | ✅ | PERSON / ORGANIZATION; typed contacts (IDE/TVA/ABACUS/RC); 33 seeded categories; roles kept as relationships; persons carry no PII (register link only) |
| §4.1 `case_task` | ⬜ | ⬜ | ⬜ | listed in the overview; no schema in spec yet |
| §10 `access_grant` + confidentiality enforcement | ⬜ | 🟡 `SecurityService` | 🟡 | see Deviations — only scope-based auth today |
| §14.5/§14.6 seed: test users, org units, case types, thing types | 🟡 `0010` (case types) | — | 🟡 | `OPC_DEMANDE_PC` (namespace OPC), `GENERIC_REQUEST` (GEN); users, org units, thing types pending |

---

## 2. Minimal end-to-end scenario (spec §3.1)

The 16-step demo still needs Thing + Timeline + Circulation, so it is
partly pending — but **Actor and Case are now done** (persons/organizations creatable and
linkable as relationship targets). Document- and actor-side steps are done and verified
via ConnectRPC **and exercisable from the embedded web UI** (create → detail → verify/
lifecycle → edit blocked when locked → audit):

- ✅ (7) add a document of type PLAN — `CreateDocument`
- ✅ (8) link the document to a case — `link_to_case_id` on create / `LinkDocument` (works once a CASE subject exists)
- ✅ (9) link document to a thing (`DOCUMENT_REPRESENTS_THING`) — via `LinkDocument` (relationship type seeded; needs a THING subject)
- ✅ (16) consult the audit — `GetDocument{includeAudit}` / `GetActor{includeAudit}` / `CoreService.ListAuditEvents`
- ✅ (1) create an `OPC_DEMANDE_PC` case — `CreateCase` (reference `YYYY-NNNNNN` allocated in namespace `OPC`)
- ✅ (4–5) create an actor and link it as requester/mandatee — `CreateActor` + `LinkSubjects(CASE_HAS_ACTOR_*)`
- ⬜ (2–3, 6, 9 target, 10–15) thing, timeline add + validate + immutability, circulation + response — pending their services

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

- **Authorization is scope-based only** (`goeland:read` / `goeland:write`). The `Permission`
  enum exists in proto, but `access_grant`, per-subject grants and deny-by-default
  confidentiality (spec §10) are **not** enforced yet. Tracked in §1 (🟡) and §5.

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
- 🔒 **PII-free by construction** — the PERSON specialization stores only `is_ch_register` +
  an opaque `ch_register_ref`; no civil-registry personal data enters the POC.
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
  revisit reuse against confidentiality and read rights.
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
  targets; full security stays in Phase 6.
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
- **v2 SQL snippets are illustrative** — implementations follow repo conventions
  (`NOT NULL DEFAULT ''` strings, enum-backed `SMALLINT` statuses, alias-prefixed projections).

---

## 4. Tests

- ✅ Pure unit tests: pagination, subject-kind validation, dbmate migration parser, authadapter.
- ✅ Automated DB integration tests (`pkg/integration`): migrations idempotency + seed data,
  the full document lifecycle (create → search → metadata update → link → finalize+lock →
  locked-update rejected → soft delete → deleted-mutation rejected → audit trail), and the
  **actor lifecycle** (org create with contacts+category → search → update/label-sync →
  case→actor link → soft-delete+rejection → audit; person PII-free specialization;
  organization `legal_name` required; 33 categories seeded), and the **case lifecycle**
  (seeded types → create with allocated reference → actor roles + document links →
  transitions with reasons → closed-case freeze → reopen → explicit reference → soft delete).
  Env-gated on
  `GOELAND_TEST_DATABASE_URL` (needs PostGIS/pgcrypto/pg_trgm/unaccent); skipped when unset so
  `go test ./...` stays green without a database.
- ⬜ Broader DB integration coverage (spec §16: relationship / timeline / circulation /
  security) — add alongside each new domain, following the `pkg/integration` pattern.
- 🟡 Frontend has no unit tests yet; the gate is `bun run type-check` + `bun run lint`
  + `bun run build` (green). Add component/e2e tests as the UI grows.
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
