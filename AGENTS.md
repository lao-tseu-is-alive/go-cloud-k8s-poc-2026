# go-cloud-k8s-poc-2026 (Goéland POC) — Agent Instructions

Read this before making changes. It captures the conventions, the layers that
must stay in sync, and the non-obvious gotchas discovered while building the POC.

## Security

- Never copy, print, log, or commit real credentials, passwords, tokens, keys,
  cookies, or connection strings from `.env`, local config, or command output.
- Use obvious placeholders in examples: `<db-password>`, `<jwt-secret>`,
  `<dev-token>`, `pat_<redacted>`.
- Do not read or display secret-bearing environment files unless the task
  requires inspecting their structure, and redact all values.
- Values in `.env` must not be quoted: the Makefile includes and exports the
  file, so quotes become part of the value.
- `.env` and coverage files are Git-ignored, but that does not make their
  contents safe to expose.

## Documentation quality contract

`docs/DOCUMENTATION.md` is the normative documentation contract for human and
agent contributors. Read and follow it whenever a change affects Go or
Protobuf contracts, repository files, stable operational claims, roadmap state
or a release. Keep the detailed rules centralized there; references in this
file and the README are entry points, not competing copies. Run
`make docs-check` for documentation-sensitive changes and `make check` before
handoff; never bypass a failing documentation gate.

## What this project is

A proto-first POC rebuilding the conceptual core of **Goéland** (territorial
administrative case management) from the active spec
`requirements/goeland_poc_domain_model_agent_v2.md` (v2; v1 kept as history).
It is **not** a re-code of legacy Goéland — it is a clean, durable core:

> a graph of durable business **subjects**, linked by **typed relationships**,
> animated by chronological history, protected by rights, made trustworthy by
> **auditability**.

Stack: proto-first (buf) · ConnectRPC + Vanguard · pgx raw SQL with `db:"..."`
tags · bundleable `pkg/<domain>/module` pattern · embedded dbmate migrations ·
PostGIS-ready from migration 0001 · an embedded **Vue 3 + Vuetify 4 SPA**
(`cmd/goeland-server/goeland-front`, `//go:embed`) served at `/`.

**Progress tracking:** the **active spec is v2**
(`requirements/goeland_poc_domain_model_agent_v2.md`, cite as "v2 §N"); the original
`requirements/goeland_poc_domain_model_agent.md` is kept as **historical v1** (older
"spec §N" citations refer to it). Both are immutable statements of intent — do not rewrite
them to match reality; record reconciliations in `IMPLEMENTATION_STATUS.md` §3g.
The living state against the spec (built areas / decided enhancements / deviations) is
`requirements/IMPLEMENTATION_STATUS.md` — **update it (a few lines) at the end of
each slice**, and add automated tests as you land each new domain. Implementation
**order and task state** live only in `docs/ROADMAP.md` (`GLD-NNN` task IDs): update it
whenever a task starts, completes, changes scope or order.

### Implemented so far

- **core** (`pkg/core`) — transversal: `subject_ref` (+ optional `business_ref` in a
  namespace, allocated `YYYY-NNNNNN` by `business_ref_counter`), `record_metadata`,
  `audit_event`, `relationship_type`, `subject_relationship` → `CoreService`.
- **document** (`pkg/document`) — modern GED entity → `DocumentService`.
- **actor** (`pkg/actor`) — external persons & organizations → `ActorService`.
  Modelled from the real production `Acteur` schema (`actor_kind` PERSON/ORGANIZATION,
  typed `actor_contact`, seeded `organization_category`). Roles are NOT columns —
  actors attach via typed `CoreService` relationships (`CASE_HAS_ACTOR_*`,
  `DOCUMENT_*_ACTOR`); a person carries only a minimal identity (salutation, last and first
  name, GLD-039; display name derived as "<first> <last>" when blank) plus the register link
  — no birth date, AVS number or civil-registry data. Typed complements
  ("contacts": phone, e-mail, IDE, VAT, ...) are validated and stored normalized per type
  by `pkg/actor/contacts.go` (E.164 phones, IDE check digit, ...); the SPA mirrors the rules
  in `utils/contactRules.ts` — change both together (both test suites read the shared cases in
  `pkg/actor/testdata/contact_values.json`, so a one-sided change fails `make check`). Addresses (GLD-014) are `address` rows
  linked M:N by `actor_address` with a role (head office, branch, correspondence, billing,
  residence, other) and one principal; replacing them ends the old links. A branch acting as a
  distinct party or a contact person is another actor linked by `ACTOR_BRANCH_OF_ACTOR` /
  `ACTOR_CONTACT_PERSON_OF_ACTOR`.
- **case** (`pkg/casefile`) — the affaire: `case_type` (with a business-reference
  namespace) + `case_file` → `CaseService`. Status lifecycle OPEN / IN_PROGRESS /
  SUSPENDED / CLOSED (`casefile.transitions`; closing and reopening need a reason, a
  closed case is frozen → `core.ErrInvalidState` = FAILED_PRECONDITION), independent of
  any workflow. The reference is allocated in the type namespace by default
  (`2026-000123` in namespace `OPC`). Participants, documents and related cases are typed relationships.
- **thing** (`pkg/thing`) — business objects (parcel, building, street, tree...) →
  `ThingService`. Geometry is `geometry(Geometry, 2056)` (LV95) with a GIST index, exchanged
  as GeoJSON; it must be valid, inside Switzerland (rejects WGS84 degrees) and of a type
  fitting the thing type (polygonal parcel; point or polygonal building). Parcels carry commune
  OFS number + parcel number (unique) and EGRID, buildings EGID (unique), ECA number and RegBL
  status in 1:1 tables; land-rights roles are `THING_HAS_ACTOR_*` relationships.
- **timeline** (`pkg/timeline`) — the case "suivis": `case_timeline_entry` +
  `timeline_document_link` → `TimelineService`. Entries belong to the case (not subjects;
  audited on the CASE subject). Only a DRAFT changes; VALIDATED / LOCKED / WITHDRAWN are
  immutable (DB triggers too) and a correction is a new entry (`corrects_entry_id`). Citing a
  document links it to the case (`CASE_HAS_DOCUMENT`) and validation pins its current version.
  `casefile` calls `timeline.EnsureNoDraftsTx` before closing and `timeline.RecordSystemEntryTx`
  on every status change (timeline never imports casefile).
- **orgunit** (`pkg/orgunit`) — internal organizational units (never ACTORs): `org_unit_type`
  + `org_unit` → `OrgUnitService`, ORG_UNIT subjects in one tree without cycles (service check +
  trigger, tree mutations serialized by an advisory lock). No unique code: the abbreviation is
  inherited by sub-units; live siblings never share a label; `external_ref` keeps a source id.
  Dissolved, never deleted (only without live sub-units); a dissolved unit takes no child,
  owned subject (`record_metadata.owner_org_id`, a typed FK checked by
  `core.InsertRecordMetadataTx`) or relationship (`core.LinkSubjectsTx`). Mutations need
  `goeland:admin`. `cmd/goeland-import-orgunits` imports the real tree (structure only).
- **task** (`pkg/task`) — case tasks: `task_type` + `case_task` + `case_task_assignment` →
  `TaskService`. Tasks belong to the case (not subjects; audited on the CASE subject; every
  mutation guarded by `core.EnsureOpenCaseTx`). OPEN → IN_PROGRESS → DONE | CANCELLED, reopen
  with a reason (`task.moves`); one assignee (app_user or live org unit) or none, history kept;
  `origin` MANUAL today. Completion / cancellation write SYSTEM timeline entries
  (`timeline.RecordSystemEntryTx`); `casefile` calls `task.EnsureNoOpenTasksTx` before closing.
  "My tasks" include the caller's units through `USER_MEMBER_OF_ORG_UNIT`.
- **circulation** (`pkg/circulation`) — `case_circulation` + `case_circulation_recipient` →
  `CirculationService`: a composition of tasks. Each recipient (user or live unit) gets a task
  (`task.CreateTx`, origin CIRCULATION) when its step opens; only the circulation completes or
  cancels it (`task.MoveTx` with `MoveOptions{ByOrigin}`). An answer writes a locked RESPONSE
  entry (`timeline.RecordSystemEntryTx` with a Type), a fully answered step opens the next one,
  the last answer completes the circulation with a SYSTEM summary. `casefile` calls
  `circulation.EnsureNoOpenCirculationsTx` before closing. Import order: casefile → circulation →
  task → timeline → core (never the reverse).
- **access** (`pkg/access`) — grants and security groups → `AccessService` (GLD-048). A grant gives
  READ < CONTRIBUTE < MANAGE < FULL_CONTROL on one subject to a user, a GROUP subject or an org
  unit (covering its sub-units); changes keep history, are audited on the subject, and the last
  FULL_CONTROL is kept. Membership is `USER_MEMBER_OF_GROUP`. The effective level lives in `core`
  (see "Authorization" below).
- **frontend** (`cmd/goeland-server/goeland-front`) — Vue 3 + Vuetify 4 SPA, vertical
  slices of the Document module (list/create+upload/detail/edit/finalize/verify/link/
  delete), the Actor module (list/create/detail/edit/activate/delete, addresses), the Case
  module (list/create/detail/edit/transition/link/delete, with its timeline, tasks and
  circulations panels), the Thing module (list/create/detail with SVG geometry preview/edit/
  link/delete), "my tasks", the org unit tree (edited by admins) and the admin page
  (reference data), plus read-only governance/audit and core panels. Embedded via
  `//go:embed` and served at `/`. See "Frontend" below.

### Not yet built (same foundation)

The sensitive read audit (GLD-033), storage (MinIO),
search (Meilisearch), provenance and outbox, export, AI proposals, workflow. The Actor
domain continues too (the full production role vocabulary mapped onto
`relationship_type` with Case/Thing).
Design new domains as first-class subjects that reuse the core primitives.

**Legacy data import (GLD-051, GLD-052):** `cmd/goeland-import` (`pkg/legacyimport`) loads a
brand-new local database from the read-only legacy replica in one transaction (dry run = full run
rolled back), set-based with COPY, deterministic UUIDv5 ids, a `subject_provenance` row per
subject and one `import_batch` marker instead of per-row audit; it bypasses the domain services on
purpose, so it must keep their invariants by construction (one open edge per type, live units and
groups, CHECK rules, `actor.NormalizeContactValue` for contacts). Real data never leaves the
machine: print counts only, never values, and keep screenshots of imported data out of the
conversation. Rules and runbook: `docs/IMPORT_MAPPING.md`; rebuild with `scripts/import_rebuild.sh`.

## Key paths

```text
proto/goeland/v1/            core.proto, document.proto, actor.proto, case.proto, thing.proto, timeline.proto, orgunit.proto, task.proto, circulation.proto  (API contract, source of truth)
gen/goeland/v1/              generated Go + ConnectRPC          (never hand-edit)
api/openapi/                 generated OpenAPI (goeland.swagger.yaml, from google.api.http; never hand-edit)
pkg/version/                 build/version metadata
pkg/authadapter/             JWT + PAT + dev token verification (shared, ecosystem-wide)
pkg/core/                    transversal domain
  ├── tx.go                  exported tx-scoped helpers reused by sibling domains
  ├── module/                bundleable module + OWNS the full schema bootstrap
  │   └── db/migrations/     0001..0025 (dbmate format)
pkg/document/                document domain (reuses core primitives)
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/blobstore/               content-bytes contract (Put/Get/Delete, spec v2 §23), domain-neutral
  ├── filestore/             local-filesystem implementation (internal:// refs)
  └── blobstoretest/         conformance suite every implementation must pass
pkg/actor/                   actor domain: persons & organizations (reuses core primitives)
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/casefile/                case (affaire) domain: types, status lifecycle (reuses core primitives)
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/thing/                   thing (objet) domain: parcels, buildings, LV95 PostGIS geometry
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/orgunit/                 organizational units (ORG_UNIT subjects in one tree, dissolution)
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/access/                  grants and security groups (AccessService; the effective level is in core)
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/circulation/             case circulations (recipients by step, answers in the timeline)
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/task/                    case tasks (assignment history, lifecycle, "my tasks")
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/timeline/                case timeline (suivis): entries, corrections, cited documents
  └── module/                bundleable module (NO migrations; core owns schema)
pkg/integration/             env-gated DB integration tests (migrations + document/actor lifecycles)
cmd/goeland-server/          server: pool → migrate → wire the modules → one shared transcoder
  ├── server.go              routes; embeds + serves the SPA (SPA fallback to index.html)
  ├── upload.go              out-of-proto POST /upload + GET /{id}/content (own bearer check)
  ├── config.go              server config incl. GOELAND_DOCUMENT_PATH / _MAX_UPLOAD_BYTES / GET /config
  └── goeland-front/         Vue 3 + Vuetify 4 SPA (bun/Vite); dist/ is //go:embed'd (gitignored)
cmd/doccheck/                documentation checker (GoDoc coverage + exact atlas inventory)
cmd/goeland-import-orgunits/ optional import of the legacy org unit tree from a read-only replica (structure only)
cmd/goeland-import/          one-shot legacy data import (pkg/legacyimport) into a brand-new local database
.github/workflows/           CI: ci (make release-check), cve-trivy-scan, docker-publish, release
docs/                        DOCUMENTATION.md (normative doc contract), ROADMAP.md (GLD-NNN tasks), atlas.md, PRODUCTION_READINESS.md
```

## Commands

- `make run` — `buf generate` + run the server from source (default `127.0.0.1:8080`).
- `make build` — test + build `bin/goeland-server`.
- `make test` — all Go tests with the race detector + `coverage.out`. Package discovery
  excludes the frontend `node_modules` tree, so it is stable after `bun install`.
- `make lint` — `go vet` (same filtered package set) + `buf lint`.
- `make docs-check` — `godoc-check` + `atlas-check` (both `cmd/doccheck`) + `docs-assert`
  (`scripts/check_documentation_claims.sh`). See `docs/DOCUMENTATION.md`.
- `make check` — the full local gate: `front-check` (frozen bun install, type-check,
  ESLint, build) + `fmt-check` + `lint` + `test` + `docs-check` + `git diff --check`.
- `make release-check` — `check` + `vuln-check` (govulncheck: reachable vulnerabilities fail) +
  version/changelog/scripts consistency + roadmap and
  bidirectional roadmap↔changelog traceability + binary build whose `--version` must report
  `Version`. This is exactly what CI (`ci.yml`) runs on every push and PR, and what the
  `release` / `docker-publish` workflows run (after checking tag = `v` + `Version`) before
  publishing.
- **Release:** bump `Version` in `pkg/version/version.go`, the README / roadmap / atlas
  banners and a dated `CHANGELOG.md` section, mark released `GLD-*` tasks `[x]`, run
  `make release-check`, commit on `main`, then `CONFIRM_RELEASE=vX.Y.Z make release`
  (guarded annotated tag + atomic push of `main` and the tag). Never push a tag by hand.
- **DB integration tests** (`pkg/integration`) are gated on `GOELAND_TEST_DATABASE_URL`
  and skip when unset. Run them against a disposable PostGIS database (needs PostGIS/
  pgcrypto/pg_trgm/unaccent):
  `GOELAND_TEST_DATABASE_URL='postgres://…?sslmode=disable' go test ./pkg/integration/...`.
  Do not point them at a working database — they migrate and write test rows.
  The same variable enables the **end-to-end scenario** `cmd/goeland-server/scenario_test.go`
  (spec v2 §50 over HTTP against the real handler) and the **API surface test**
  `cmd/goeland-server/api_surface_test.go` (every RPC the scenario does not reach, once, with
  its not-found mapping): run both with
  `go test ./cmd/goeland-server -run 'TestScenarioV2|TestAPISurface'` after every slice that
  changes an API, and add each new RPC to one of them. CI runs all of them against a PostGIS
  service with `GOELAND_REQUIRE_DB_TESTS=true`, which turns a missing database into a failure
  (`coretest.TestDatabaseURL`); use that helper in any new database test.
- `make fmt` — `gofmt -w .` (repo-wide; prefer `gofmt -w` on touched files only).
- `make generate` — lint protos, update buf deps, regenerate Go + ConnectRPC + OpenAPI.
  OpenAPI paths come from the `google.api.http` annotations; when you add an RPC,
  annotate it (see `document.proto`) so it gets a REST binding + OpenAPI entry.
- `make db-status | db-up | db-down` — dbmate against `.env`.
- `make db-new name=add_case` — scaffold a new migration.
- `make front-build` — `bun install && bun run build` in `goeland-front/` to
  produce `dist/`. It is a prerequisite of both `make run` and `make build`
  because `//go:embed all:goeland-front/dist` fails if `dist/` is absent (it is
  gitignored). On a clean checkout, build the frontend before `go build`/`go test`.
  The **Docker image** is self-contained: it builds the frontend in a dedicated `bun`
  stage before the Go build, so `docker build` works from a clean checkout.

Helper scripts live in `scripts/` (all run from the repo root; documented in the
README's Scripts table). Notable ones: `createLocalDBAndUser.sh` (creates the
role/db and enables pgcrypto/pg_trgm/unaccent/postgis as admin so the app role's
`CREATE EXTENSION IF NOT EXISTS` is a privilege-free no-op),
`install_go_protobuf_tools.sh`, `getAppInfo.sh` (sourced by build/release scripts
to read `pkg/version/version.go`), `GoRunWithEnv.sh` / `GoTestWithEnv.sh` /
`execWithEnv.sh` (dotenv-loading wrappers), `get_jwt_token.sh`,
`01_build_image_locally.sh`, `02_tag_new_release_github.sh`. Keep scripts POSIX-
bash, `set -euo pipefail` where practical, and never echo secret values.

## Authentication

Reuses `pkg/authadapter`. `GOELAND_AUTH_MODE`:

- `jwt` (default): non-`pat_` bearer tokens verified locally (needs `JWT_SECRET`,
  `JWT_ISSUER_ID`, `JWT_CONTEXT_KEY`); `pat_` tokens introspected against
  `<AUTH_SERVER_URL>/goapi/v1/auth/introspect` (cached ~60s).
- `dev`: accepts `GOELAND_DEV_TOKEN` (required in dev mode) for one user
  (`GOELAND_DEV_USER_ID` / `_EMAIL` / `_NAME`; `GOELAND_DEV_USER_ADMIN=true` bootstraps it as
  administrator).

**Authorization (GLD-048, model GLD-017):** the effective level of a user on a subject is the
most specific grant — personal, then its groups (highest), then the nearest of its units or their
ancestors, then a kind-wide application role (ADMIN covers every kind), then the baseline READ;
a confidential subject (`confidentiality_level` >= 2) gets neither roles nor the baseline
(`core.EffectiveAccessTx`, one SQL query). Where it is checked:

- RPCs addressing a subject: `core.Authorizer.Caller(ctx, scope, id, level)` in the Connect
  adapter (READ for reads, MANAGE for edits and lifecycle, FULL_CONTROL for deletion and grants);
- case-owned entities: `core.EnsureAccessTx` on the case inside their transaction (timeline:
  CONTRIBUTE, lock/withdraw MANAGE; tasks: MANAGE, start/complete by the assignee or CONTRIBUTE;
  circulations: MANAGE, an answer by the recipient or MANAGE) — `core.EnsureAssigneeOrAccessTx`;
- relationships: `core.LinkSubjectsTx` / `EndRelationshipTx` / unlink require MANAGE on the source
  (CONTRIBUTE to attach a document to a case) and READ on the target; a membership needs MANAGE on
  the group or unit, never one's own subject.

Every new subject gets its creator's FULL_CONTROL and its owning unit's MANAGE
(`core.InsertRecordMetadataTx`); a document deposited from a case copies the case's grants and
confidentiality once (`core.CopyGrantsTx`), and attaching an existing document copies nothing.
New RPCs must add their check.

**Filtering (GLD-049):** a search or list returns only what the caller may read, inside the SQL
query so pagination and totals stay exact: the adapter resolves `core.Viewer` (user id plus its
groups, units and their ancestors) with `Authorizer.Reader` / `ViewerOf`, the filter carries it,
and the query adds `core.ReadableSQL(idExpr, rmAlias)` with `viewer.AddTo(args)` (a zero Viewer
sees public subjects only). Relationship lists drop edges whose other end is unreadable. A
timeline entry's visibility needs READ (participants), CONTRIBUTE (internal) or MANAGE
(restricted) on the case, except for its author, to read or to write. Document bytes are
downloaded through `GET /api/documents/{id}/content[?versionId=]` (READ on the document), and the
automatic reuse of identical content only picks a document the caller may read. New lists must
apply the predicate.

**Default access per case type (GLD-050):** `case_type.default_confidentiality_level` is a minimum
for new cases and `case_type_default_grant` a template (users, groups, units, `CREATOR_UNITS`)
copied once by `core.ApplyDefaultGrantsTx` in the case creation transaction (highest level kept
per grantee; archived groups and dissolved units skipped); `SetCaseTypeDefaultGrants` replaces a
template and logs it in `reference_change`. Existing cases never follow a template change.

**Strict JSON (GLD-050):** request bodies with an unknown field answer 400 on both surfaces: every
module adds `core.StrictJSONOption()` to its Connect handler options and every transcoder is built
with `vanguard.WithCodec(core.NewStrictJSONCodec)` (the bundle in `server.go` too). Keep both when
adding a module; the SPA must send only fields of the request message.

**Application roles (GLD-047):** administrators are decided in Goéland, never by the token.
`core.RecordingVerifier` strips `goeland:admin` from what the token says and adds it back for a
holder of the ADMIN role (`app_role` / `app_user_role`, cached 30 s, forgotten at once on a change
made through the core service). `GOELAND_BOOTSTRAP_ADMINS` lists user ids granted ADMIN on their
next request (`system:bootstrap`); the last administrator cannot be revoked. Grants and
revocations need a reason and are audited on the user's USER subject (`USER_ROLE_GRANTED` /
`USER_ROLE_REVOKED`); a revocation keeps the row as history.

**Reference data administration (GLD-040):** each domain administers its catalogue with
`Create*` / `Update*` RPCs that require `goeland:admin` (case types, relationship types,
organization categories, document types). Codes are immutable, entries are never deleted
(deactivate them), and every change is logged in `reference_change` (catalogues are not
subjects, so not in `audit_event`) through `core.MutateReference` in the same transaction.

**Internal users (GLD-025):** `server.go` wraps the verifier in `core.RecordingVerifier`, so
every verified caller is recorded in `app_user` (a USER subject; created on first sight,
`USER_PROFILE_UPDATED` on change, otherwise at most one `last_seen_at` write per 15 min;
best effort, never fails authentication). Governance/audit keep storing the operator id;
the SPA resolves ids to names through `BatchGetUsers` (`stores/users.ts`, `UserLabel.vue`)
and reads the caller, its effective scopes, roles and admin flag from `GetCurrentUser` (`GET /api/me`).

Scopes: `goeland:read` (read RPCs), `goeland:write` (mutations). Env vars are
`GOELAND_*`; DB vars are `DB_*` / `DATABASE_URL`.

### Operator vs domain ACTOR — do not conflate

Two distinct identities; keep them separate:

- **Operator** = the authenticated system user (employee) performing a mutation.
  ALWAYS derived server-side via `core.OperatorID(user)` — never from the request
  (requests carry no `actor_user_id`; the field was removed to prevent forgery).
  Recorded in `audit_event.actor_user_id`, `record_metadata.created_by`/`owner_user_id`,
  `subject_relationship.created_by`.
- **Domain ACTOR** = an external person/organization (subject kind `ACTOR`), e.g. a
  document's author. Never authenticated; managed by `ActorService` (`pkg/actor`) and
  recorded as an `ACTOR` subject linked by a typed relationship
  (`DOCUMENT_AUTHORED_BY_ACTOR`, `CASE_HAS_ACTOR_REQUESTER`, ...). Authors go here —
  never through the operator identity.

## API routing — two surfaces (REST + RPC)

The shared Vanguard transcoder is mounted on **explicit prefixes** — `/api/` and
each fully-qualified RPC service path (`/goeland.v1.<Service>/`) — so the embedded
SPA can own the `/` catch-all. The specific `/health`, `/readiness`, `/goAppInfo`,
`/config` routes win by ServeMux specificity. The transcoder serves every RPC two ways:

1. **REST/JSON** from `google.api.http` annotations, e.g. `GET /api/documents/{id}`,
   `POST /api/documents`. Plain HTTP — **no special header**. Documented in the
   generated OpenAPI. When you add an RPC, add an annotation (see `document.proto`)
   or it will have no REST binding / OpenAPI entry.
2. **Connect / gRPC / gRPC-Web** on `/<fully-qualified-service>/<Method>`
   (`/goeland.v1.DocumentService/...`).

⚠️ **Connect-over-curl gotcha (RPC path only):** a unary Connect JSON request on the
`/goeland.v1.*` RPC path MUST send `Connect-Protocol-Version: 1`; otherwise Vanguard
treats the `application/json` POST as REST, finds no matching REST route on that path,
and returns 404. This is a Connect-protocol requirement — real Connect clients send it
automatically. The REST `/api/*` paths do **not** need it.

```bash
# REST (no header):
curl -s -H 'Authorization: Bearer <dev-token>' \
  'http://127.0.0.1:8088/api/document-types?onlyActive=true'

# Connect on the RPC path (header required):
curl -s -H 'Authorization: Bearer <dev-token>' -H 'Content-Type: application/json' \
  -H 'Connect-Protocol-Version: 1' \
  -d '{"onlyActive":true}' \
  http://127.0.0.1:8088/goeland.v1.DocumentService/ListDocumentTypes
```

`CoreService`, `DocumentService`, `ActorService` and `CaseService` are all annotated, so each has REST
bindings (CoreService: `/api/subjects`, `/api/relationships`, `/api/relationship-types`,
`/api/subjects/{id}/relationships`, `/api/subjects/{id}/audit`, `/api/subjects/{id}/business-ref`,
`/api/subjects:lookup`, `/api/relationships/{id}/end`, `/api/me`, `/api/users:batchGet`, `/api/reference-changes`; ActorService:
`/api/actors`, `/api/actors/{id}`, `/api/actors/search`, `/api/organization-categories`;
CaseService: `/api/cases`, `/api/cases/{id}`, `/api/cases/{id}/transition`,
`/api/cases/search`, `/api/case-types`; ThingService: `/api/things`, `/api/things/{id}`,
`/api/things/search` (text, type, `bbox=e_min,n_min,e_max,n_max` in LV95), `/api/thing-types`;
CirculationService: `/api/cases/{case_id}/circulations`, `/api/circulations/{id}` (+ `/cancel`),
`/api/circulation-recipients/{recipient_id}/respond`;
TaskService: `/api/cases/{case_id}/tasks`, `/api/tasks/mine`, `/api/tasks/{id}` with `/assign`,
`/start`, `/complete`, `/cancel`, `/reopen`, `/api/task-types`; CoreService also `/api/users/search`;
AccessService: `/api/subjects/{subject_id}/access`, `/api/subjects/{subject_id}/grants`,
`/api/grants/{grant_id}/revoke`, `/api/groups` with `/{id}`, `/{id}/archive` and
`/{group_id}/members[/{user_id}/remove]`;
OrgUnitService: `/api/org-units` (flat tree), `/api/org-units/search`, `/api/org-units/{id}`,
`/api/org-units/{id}/dissolve`, `/api/org-unit-types`;
TimelineService: `/api/cases/{case_id}/timeline`, `/api/timeline-entries/{id}` with
`/validate`, `/lock`, `/withdraw` and `/documents[/{document_id}]`).

## Frontend (embedded SPA)

`cmd/goeland-server/goeland-front` is a **Vue 3 + Vuetify 4** SPA (Vite, bun,
Pinia, vue-router, vue-i18n; fr-CH default, en). `make front-build` produces
`dist/`, which the server embeds with `//go:embed all:goeland-front/dist` (`all:` keeps the
chunks whose names start with `_`, such as Vite's `_plugin-vue_export-helper`) and serves
at `/` with an SPA fallback to `index.html` (client-side routing). `dist/` is a
**gitignored build artifact** — never hand-edit it; edit `src/` and rebuild.

- **API layer:** plain REST `fetch` against the Vanguard `/api/...` bindings — **no**
  connect-es codegen. Types are hand-maintained in `src/api/types.ts` and must be
  kept in sync with the proto by hand. Mind the proto3-JSON quirks: `int64` fields
  serialize as **strings** (e.g. `fileSizeBytes:"38"`), enums as string names,
  timestamps as RFC3339.
- **Auth (dynamic):** the SPA calls `GET /config` → `{authMode, authBaseUrl}` and
  branches — `dev` uses a manual static token; `jwt` silently mints from
  `${authBaseUrl}/auth/token` (SSO cookie) and re-mints at ~80% of token lifetime.
  The token is held **in memory only** and mirrored into the fetch client.
- **Upload is metadata-first (out-of-proto).** The `DocumentService` proto contract
  deliberately has **no upload RPC**. Binary bytes go through two plain-HTTP endpoints
  that **bypass the Connect interceptor** and carry their own bearer + scope check
  (`httpAuthMiddleware`): `POST /api/documents/upload` (multipart, field `file`,
  `goeland:write`) calls `document.Service.IngestContent`, which stores the bytes via
  the configured `blobstore.Store` (`pkg/blobstore/filestore` today), computes SHA-256/size
  server-side and registers a globally
  deduplicated `content_blob` (identical content → existing blob, new bytes removed); it
  returns a `contentBlobId` that the SPA passes to `CreateDocument` / `AddDocumentVersion`
  (so validation/governance/audit still flow through the proto path). Never accept a
  client-supplied digest as content identity: automatic document reuse keys on the
  server-registered blob. `GET /api/documents/{id}/content[?versionId=]` (`goeland:read` + READ on the document) streams a version's bytes back (GLD-049).
- **Document model (spec v2):** `document` (logical object) → `document_version`
  (append-only; `document.current_version_id` is explicit; final/record versions are
  immutable and versions are never deleted — DB trigger) → `content_blob` (unique SHA-256).
- Config: `GOELAND_DOCUMENT_PATH` (blob dir, default `./go_documents`, gitignored)
  and `GOELAND_MAX_UPLOAD_BYTES` (default 100 MiB).

When you add or change an RPC the SPA uses, update `src/api/types.ts` and the
relevant `src/api/*` client + component, then `bun run type-check && bun run lint && bun run test`
(Vitest unit tests of the pure modules live in `src/utils/__tests__/`).

## Generated code and protobuf

- `proto/` is the source of truth. Never hand-edit `gen/` or `api/openapi/`.
- After changing protos, run `make generate` then `make lint`.
- `make generate` may change `buf.lock`, `gen/` and `api/openapi/` — review and
  commit intended generated changes together.
- Requires `buf`, local `protoc-gen-go` + `protoc-gen-connect-go`, and the remote
  OpenAPI plugin.
- protovalidate's `ignore` enum in the pinned buf schema is
  **`IGNORE_IF_ZERO_VALUE`** (not `IGNORE_IF_UNPOPULATED`). It is used on
  `RecordMetadata.subject_id` so that message doubles as an input (initial
  governance, server-assigned id) without tripping the `uuid` rule. Request
  validation runs via `connectvalidate.NewInterceptor()`; responses are not
  validated.

## Keeping layers in sync

The proto generates client/server types only. These layers are **not**
generated and must be updated by hand when the contract changes:

- DB schema: `pkg/core/module/db/migrations/*.sql` (add a new migration; never
  rewrite an applied one).
- Raw SQL + column projections: `pkg/<domain>/sql.go` (the `db.`-prefixed
  projections; see the INSERT-alias note below).
- Domain model: `pkg/<domain>/model.go` — the `db:"..."` tags drive pgx named
  scanning (`RowTo*ByNameLax`). Nullable columns use pointer fields.
- Mappers: `pkg/<domain>/mappers.go` (domain ↔ proto).
- Business rules/normalization: `pkg/<domain>/service.go`.
- Wire adapters: `pkg/<domain>/connect_server.go`.

### Checklist: adding a field to Document (or similar)

1. Edit `proto/goeland/v1/document.proto` (message + validation).
2. `make generate` + `make lint`.
3. Add a new migration under `pkg/core/module/db/migrations/`.
4. Update the column projection + affected DML in `pkg/document/sql.go`.
5. Add the field + `db` tag to the struct in `pkg/document/model.go`
   (and to `CreateInput`/`UpdateInput` if user-supplied).
6. Update `DomainToProto` (and any input parsing) in `mappers.go`.
7. Update normalization/defaults in `service.go`.
8. Wire it in `connect_server.go`.
9. Add/adjust tests; run `go test ./pkg/... -count=1` then `make lint`.

Named scanning means you usually do **not** touch individual `Scan(...)` calls —
only the struct + SQL projection.

## SQL conventions (learned the hard way)

- Column projections are `d.`/table-alias-prefixed so they can be shared between
  single-table queries and JOINs. Because of this, **`INSERT` statements must
  alias the target** (`INSERT INTO document AS d (...) ... RETURNING <d.cols>`),
  otherwise `RETURNING d.col` fails with `missing FROM-clause entry for table "d"`.
- Non-destructive by rule: never physically delete domain records — use
  `record_metadata.deleted_at` / soft-delete relationships. Every mutation must
  write an `audit_event` (in the same transaction as the mutation).
- Relationships are typed and validated: source/target kinds must match the
  `relationship_type`; a partial unique index allows one *open* edge (not unlinked,
  no `valid_to`) per (source, target, type). Two distinct operations: `EndRelationship`
  sets `valid_to` (the relationship ended; kept as history, `RELATIONSHIP_ENDED`) and
  `UnlinkSubjects` soft-deletes a mistaken edge (`RELATIONSHIP_UNLINKED`).
- `document.search_vector` is a Postgres **`GENERATED ALWAYS AS (...) STORED`**
  column — the app must NOT insert/scan it; Postgres computes it on every
  write. It is accent-folded via `immutable_unaccent()` (migration 0005), an
  IMMUTABLE wrapper around `unaccent()` (the raw `unaccent()` is only STABLE and
  cannot be used in a generated column/index). `searchDocumentsSQL` folds the
  query term through the same wrapper so search is accent-insensitive.

## Database migrations

- All migration SQL lives in **one** place: `pkg/core/module/db/migrations/`,
  embedded in the core module and applied at startup via `coremodule.Migrate`
  under a PG advisory lock keyed to `'go-cloud-k8s-poc-2026:migrations'`.
- **The core module owns the full schema bootstrap** (core *and* document
  tables + seed) because document tables have foreign keys into core tables.
  The document module has no migrations. As the POC grows this can be split.
- dbmate and the embedded migrator share the same `schema_migrations` version
  keys (`0001`…, the prefix before the first `_`), so the two paths are
  interchangeable. `make db-up` is optional; the server self-migrates.
- Zero-padded sequential names with `-- migrate:up` / `-- migrate:down`.
  Wrap PL/pgSQL (functions/triggers) in `-- migrate:statementbegin/end`.
- Never rewrite an applied migration — add a new one.
- `make db-down` rolls back the latest migration; the `0004` seed-down `DELETE`
  will fail if documents still reference a `document_type`, so a clean rollback
  assumes no domain data — expected for a reset, not a bug.
- Requires PostGIS, pgcrypto, pg_trgm, unaccent available on the server.
- **Never** run migrations or a migrating server startup against an unknown,
  shared, staging, or production database without explicit approval and verified
  config.

## Module & bundle architecture

Each `pkg/<domain>/module` is an importable Go package with two modes:

- **Standalone**: `RegisterRoutes(mux)` builds a transcoder for that module.
- **Bundle** (what `cmd/goeland-server` does): collect `VanguardServices()` from
  every module, build **one** shared `vanguard.NewTranscoder`, mount per service
  name. Modules share one `*pgxpool.Pool`, `TokenVerifier`, and `*slog.Logger`.

Rules:

- `New` validates deps at construction (`Pool`, `Verifier` required; the document
  module also requires `CoreService`; a nil `Logger` falls back to `slog.Default`).
- The interceptor chain (timeout → auth → protovalidate) is assembled once in
  `connectOption()`. Do not duplicate or reorder it.
- The document domain composes atomic transactions using the exported core
  helpers in `pkg/core/tx.go` (`InsertSubjectRefTx`, `InsertRecordMetadataTx`,
  `InsertAuditEventTx`, `LinkSubjectsTx`, …) so identity + governance + entity +
  audit are created in one tx.
- Dependency flow is one-way: `cmd` → `pkg`. Never import `cmd` from `pkg`.

## Code quality rules (SonarQube parity)

SonarCloud analyses `main` (settings in `.sonarcloud.properties`). Its recurring findings are
enforced locally so they fail `make check` instead of reappearing on the dashboard:

- **Cognitive complexity <= 15** for production Go functions (`make cognitive-check`,
  `gocognit` pinned as a `go.mod` tool) and for SPA functions (`sonarjs/cognitive-complexity`).
  Split long functions into named helpers rather than raising the threshold.
- **Accessible SPA markup:** every `<th>` has `scope="col"` (or `scope="row"`); a clickable
  element or table row also has `tabindex="0"` and `@keydown.enter` (ESLint
  `vuejs-accessibility/*` + a clickable-`<tr>` rule). No deprecated CSS keywords
  (e.g. use `overflow-wrap: anywhere`, not `word-break: break-word`).
- **No implicit object stringification:** never `String(v)` on `unknown`; narrow the type first.
- **Shell scripts:** errors go to stderr (`>&2`), positional parameters are copied into named
  `local` variables in functions, repeated literals become a function or variable, and
  credentials are never sent over plain HTTP except to the loopback interface.
- **Trivy suppressions** (`.trivyignore`) document why the advisory does not apply (with the
  `govulncheck` evidence) and carry an `exp:YYYY-MM-DD` date so they lapse and get re-reviewed.
- **CI actions:** third-party actions are pinned by commit SHA (a mutable tag of an outside
  maintainer is a supply-chain risk); GitHub's own actions (`actions/*`, `github/*`) may use
  their major tag, since they come from the platform that runs the workflow (decided
  2026-09-30). Downloaded tools are verified by checksum (e.g. `bufbuild/buf-action` with
  `checksum`), never `go install tool@version` in a workflow.
- **Contexts:** never replace an available `ctx` by `context.Background()`; to outlive a
  cancelled context keep its values with `context.WithoutCancel(ctx)`.
- A genuine false positive is fixed at the source (`.sonarcloud.properties`) or, for one line,
  annotated `// NOSONAR <rule>` with a one-line justification — never silenced without a reason.

## Testing & change discipline

- Keep changes scoped; follow existing package boundaries and patterns.
- Add/update tests for behavioral changes; prefer focused `go test ./path/...`.
  Pure unit tests cover pagination, kind validation, the migration parser, and
  authadapter. DB-touching behavior is covered by the env-gated integration tests in
  `pkg/integration` (see Commands) — extend those (or add a sibling package following
  the same pattern) when you touch SQL, migrations, or transaction invariants.
- Run `make lint` for Go or protobuf changes, and `make check` before handoff.
- Do not hand-edit generated files to make tests pass.
- Do not revert unrelated work in a dirty worktree.

## Design rules (from the domain spec §17 — keep honouring them)

Explicit domain model (no generic EAV); JSONB only for secondary data; critical
fields as typed columns; non-destructive deletes; every mutation audited; every
relationship typed and validated; boring, explicit, testable API; prefer clear
domain names over technical abstractions.

## Guiding principle — the spec is a floor, not a ceiling

The spec (active v2, `requirements/goeland_poc_domain_model_agent_v2.md`) is a starting
point, not a maximum. Several features intentionally go **beyond** it (typed proto
API, richer GED document, accent-insensitive search — see
`requirements/IMPLEMENTATION_STATUS.md` §3 "Decided enhancements 🚀").

- **Do not regress** capability or degrade UX just because something is "not in the
  spec". If you're tempted to remove/simplify a feature that isn't in the spec,
  check §3 first — if it's a decided enhancement, keep it.
- The bar is **the best user experience, never at the expense of security,
  readability, or maintainability.** When a spec item conflicts with that bar,
  reconcile it explicitly (record it in IMPLEMENTATION_STATUS.md §3) rather than
  silently dropping to the lesser option.
- When you add a betterment beyond the spec, record it under §3 with a 🚀 and a
  one-line rationale so the next agent knows it is deliberate.
