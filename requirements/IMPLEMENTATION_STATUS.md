# Goéland POC — Implementation Status

Living tracker of what is built vs. what the spec asks for. Update it at the end
of each slice (a few lines), and keep it honest.

- **Intent / spec (immutable):** [`goeland_poc_domain_model_agent.md`](goeland_poc_domain_model_agent.md) — the original statement of intent. Do not rewrite it to match reality; cite it (e.g. "spec §7.2").
- **This document (living):** maps each spec area to its current state + records intentional deviations.
- **Snapshot:** as of **2026-07-08**, app version **0.2.1**. Build/vet/lint/tests green; migrations `0001–0005` applied; verified end-to-end against PostgreSQL. Now with an **embedded Vue 3 + Vuetify 4 web UI** (Document module exercisable from the browser) + metadata-first file upload; repository SQL uses pgx **named parameters**.

Legend: ✅ done · 🟡 partial · ⬜ not started

---

## 1. Building blocks (schema + service)

| Spec area | Schema | Service / API | State | Notes |
|-----------|--------|---------------|-------|-------|
| §5.2–5.3 `subject_kind`, `subject_ref` | ✅ `0001` | ✅ `CoreService.CreateSubjectRef/GetSubjectRef` | ✅ | canonical identity, composite `(id,kind)` FK used to pin document kind |
| §5.4 `record_metadata` (governance) | ✅ `0001` | ✅ via Core (create/lock/soft-delete helpers) | ✅ | ownership/confidentiality/locking/versioning; non-destructive |
| §5.5 `audit_event` (append-only) | ✅ `0001` | ✅ `CoreService.ListAuditEvents` + written on every mutation | ✅ | every mutation writes an event in the same tx |
| §7 `relationship_type` + `subject_relationship` | ✅ `0002` | ✅ `CoreService.LinkSubjects/UnlinkSubjects/ListRelationships/ListRelationshipTypes` | ✅ | kind-compat validated; active-edge partial unique index; soft-delete |
| §6.2 `document_type` + `document` | ✅ `0003` (+ `0005` unaccent) | ✅ `DocumentService.*` (9 RPCs) | ✅ | modern-GED slice; accent-insensitive FTS; finalize+lock; integrity |
| §14 seed: subject kinds, relationship types (10), document types (7) | ✅ `0004` | — | ✅ | |
| §13 delivery surface: REST/JSON + **embedded web UI** | — | ✅ Vanguard REST `/api/*` + Vue 3 / Vuetify 4 SPA at `/` | 🟡 | Document module full slice in the browser; core panels read-only; Case/Thing/Actor UI pending their services |
| §6.2 document binary upload (metadata-first) | — | ✅ out-of-proto `POST /api/documents/upload` + `GET /download` (`pkg/document/filestore`) | ✅ | proto stays `storage_ref`-only; local blob store today, MinIO later (§19) |
| §6.1 `case_type` + `case_file` | ⬜ | ⬜ `CaseService` | ⬜ | next natural slice |
| §8 `case_timeline_entry` + `timeline_document_link` | ⬜ | ⬜ `TimelineService` | ⬜ | timeline is the primary case history (spec §17.8) |
| §9 `case_circulation` + `case_circulation_recipient` | ⬜ | ⬜ `CirculationService` | ⬜ | depends on Case + Timeline |
| §6.3 `thing` + `thing_type` (+ `thing_parcel`, `thing_building`) | ⬜ | ⬜ `ThingService` | ⬜ | PostGIS geometry (extension already enabled in `0001`) |
| §6.4 `actor` | ⬜ | ⬜ `ActorService` | ⬜ | PERSON / ORGANIZATION |
| §4.1 `case_task` | ⬜ | ⬜ | ⬜ | listed in the overview; no schema in spec yet |
| §10 `access_grant` + confidentiality enforcement | ⬜ | 🟡 `SecurityService` | 🟡 | see Deviations — only scope-based auth today |
| §14.5/§14.6 seed: test users, org units, case types, thing types | ⬜ | — | ⬜ | |

---

## 2. Minimal end-to-end scenario (spec §3.1)

The 16-step demo needs Case + Thing + Actor + Timeline + Circulation, so it is
mostly pending. Document-side steps are done and verified via ConnectRPC **and now
exercisable from the embedded web UI** (create-with-upload → detail → verify →
finalize/lock → edit blocked when locked → audit):

- ✅ (7) add a document of type PLAN — `CreateDocument`
- ✅ (8) link the document to a case — `link_to_case_id` on create / `LinkDocument` (works once a CASE subject exists)
- ✅ (9) link document to a thing (`DOCUMENT_REPRESENTS_THING`) — via `LinkDocument` (relationship type seeded; needs a THING subject)
- ✅ (16) consult the audit — `GetDocument{includeAudit}` / `CoreService.ListAuditEvents`
- ⬜ (1–6, 10–15) case creation, thing/actor, timeline add + validate + immutability, circulation + response — pending their services

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
  `is_record`, `status`, `language`, `page_count`, `previous_version_id` (versioning),
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
- 🚀 **Metadata-first file upload** kept **out of the proto contract**: `CreateDocument`
  stays `storage_ref`-only, while binary bytes flow through `POST /api/documents/upload`
  → `internal://<uuid>` → `CreateDocument` (server computes sha256/size/mime;
  `pkg/document/filestore`, path-traversal guarded). Preserves proto validation /
  governance / audit while still supporting real file upload; swap the local blob store
  for MinIO later without touching the contract.

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
policy, streamed probative hashing, DB integration tests, CI, metrics/tracing.

---

## 4. Tests

- ✅ Pure unit tests: pagination, subject-kind validation, dbmate migration parser, authadapter.
- 🟡 Migrations + full document lifecycle verified **manually** end-to-end against PostgreSQL
  (create → verify → search → finalize+lock → locked-update rejected → audit).
- ⬜ Automated DB integration tests (spec §16: relationship / timeline / circulation / audit /
  security) — none yet; add alongside each new domain.
- 🟡 Frontend has no unit tests yet; the gate is `bun run type-check` + `bun run lint`
  + `bun run build` (green). Add component/e2e tests as the UI grows.

---

## 5. Suggested next slices (in order)

1. **Actor** (§6.4) — smallest new subject; unlocks `CASE_HAS_ACTOR_*` links and part of the demo.
2. **Case** (§6.1) — `case_type` + `case_file` + `CaseService`; the spine of the scenario.
3. **Timeline** (§8) — `case_timeline_entry` + document links + validation/immutability (spec §17.8).
4. **Thing** (§6.3) — PostGIS geometry, parcel/building specializations; enables `CASE_CONCERNS_THING`.
5. **Circulation** (§9) — depends on Case + Timeline.
6. **Security** (§10) — `access_grant` + confidentiality deny-by-default (or integrate Casbin/OpenFGA).
7. **Integration tests** for the full §3.1 scenario, added with each slice above.

Keep honouring the design rules (spec §17): explicit model, no EAV, JSONB only for
secondary data, non-destructive deletes, every mutation audited, every relationship
typed & validated.
