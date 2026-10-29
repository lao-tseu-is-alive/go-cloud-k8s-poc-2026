# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This POC uses [Semantic Versioning](https://semver.org/); while pre-1.0, a breaking
change bumps the **minor** version and features/fixes bump the **patch** version.

## [Unreleased]

## [0.22.0] - 2026-10-29

Migrations separated from the runtime (GLD-046): the schema owner runs `goeland-server migrate`,
which also grants the server's role row privileges only; with `GOELAND_DB_MIGRATE=verify` the
server never migrates, so it can neither alter the schema nor drop the audit triggers. The default
(`auto`) is unchanged for local work. Deployment change: the k8s manifests add a `migrate` init
container and a runtime role `goeland_app` (Secret key `DB_APP_PASSWORD`); read
`docs/PRODUCTION_READINESS.md` §2.

### Security

- Migrations separated from the runtime (GLD-046): `goeland-server migrate` applies the
  migrations as the schema owner, then grants the runtime role named by `GOELAND_DB_APP_ROLE`
  only row privileges (`SELECT, INSERT, UPDATE`; `DELETE` on `case_type_default_grant` and
  `outbox_event` only). With `GOELAND_DB_MIGRATE=verify` the server never migrates, refuses to
  start while a migration is pending, and warns when its role owns the schema; `auto` (default)
  keeps migrating at startup for local work. The runtime role can no longer alter the schema nor
  drop or disable the immutability triggers. The k8s manifests run the migrations in an init
  container as `goeland` and the server as `goeland_app` (new Secret key `DB_APP_PASSWORD`); the
  end-to-end tests run their servers as such a role.

## [0.21.1] - 2026-10-29

Security patch: the JWT tokens are verified by Goéland itself (GLD-078), so the Echo framework,
flagged by two MEDIUM advisories without a fix, and `golang.org/x/crypto` leave the image. The
issuer of a token is now checked: `JWT_ISSUER_ID` must equal the auth service's.

### Security

- JWT verification without Echo (GLD-078): Goéland verifies the go-cloud-k8s-auth tokens itself
  (`pkg/authadapter/jwt_verifier.go`) instead of through `go-cloud-k8s-common-libs/pkg/goHttpEcho`,
  which linked the Echo framework flagged by CVE-2026-118651 and CVE-2026-118653 (unreachable here:
  no Echo server, proxy or rate limiter ran). The check is stricter: the issuer must be
  `JWT_ISSUER_ID` and an expiry is required; token content is never logged. `JWT_CONTEXT_KEY` and
  `JWT_DURATION_MINUTES` are no longer read. Echo, `golang.org/x/crypto` and the GO-2026-5932
  suppression are gone.

## [0.21.0] - 2026-10-29

Elimination of cases (GLD-077): an approved elimination bordereau keeps each case as a notice and
erases the rest, destroying the files only it held; a failed transfer package can be rebuilt. The
elimination is **off by default**: until operations set `GOELAND_DISPOSAL_ENABLED=true`, it can only
be simulated (what it would erase, nothing written). Migration `0037`. Read `docs/RETENTION.md`
§5b-5c before enabling it.

### Added

- Elimination of cases (GLD-077, migration `0037`): executing an approved elimination bordereau
  keeps each case as a notice (« Affaire éliminée <référence> », frozen, retention DISPOSED) and
  erases the rest: its texts, those of its timeline, tasks and circulations, the states and
  reasons of its audit events (who and when stay), its ordinary exports, and the documents only it
  held (blanked and logically deleted); their files are destroyed when no other document version
  references them. The audit, timeline and document version guards accept this erasure only inside
  the elimination transaction. The page asks for an explicit confirmation. A failed or expired
  transfer package is rebuilt from the bordereau page. See `docs/RETENTION.md` §5b.
- Elimination is off by default (GLD-077): only `GOELAND_DISPOSAL_ENABLED=true` (exactly) lets an
  elimination bordereau be executed; any other non-empty value except `false` stops the server,
  which logs its mode at startup. Otherwise an elimination can only be simulated: « Simuler
  l'élimination » (`SimulateElimination`, `POST /api/disposition-bordereaux/{id}/simulate`) runs it
  in a rolled-back transaction and lists, per case, the documents eliminated and kept, the files
  destroyed and the exports removed. `GET /config` exposes `disposalEnabled`. See
  `docs/RETENTION.md` §5c.

## [0.20.0] - 2026-10-28

Retention of closed cases and their transfer to the archives (GLD-076): a retention schedule
with a policy per case type, the retention computed at closing, legal holds, the deadlines, and
disposition bordereaux approved by the owning units, whose execution builds the complete
transfer package, freezes the case and keeps a verifiable proof. Nothing is destroyed yet
(that is the next task); the policies shipped are examples. Migrations `0035` and `0036`; new application role
Archiviste (`RECORDS_MANAGER`). Read `docs/RETENTION.md` before any use on real cases.

### Added

- Retention of closed cases (GLD-076, migration `0035`): a `RetentionService` with the
  retention schedule (`retention_policy`, administered like the other catalogues; three example
  policies until the City archives give the real one), a default policy per case type, the
  retention computed when a case is closed (a snapshot: start on the closing day in Europe/Zurich,
  end the policy's duration later, its final disposition) and ended when it is reopened, legal
  holds that keep a case off the due list, the due list and the application of a type's policy to
  its already closed cases (1000 per call, each audited). A new application role, RECORDS_MANAGER
  (Archiviste), gives the `goeland:records` scope; the SPA shows the retention on the case page
  (Gouvernance) and a Conservation page with the deadlines.
- Disposition bordereaux (GLD-076, migration `0036`): a records manager gathers due cases
  (not on hold) of one final disposition in a bordereau; each case is approved or rejected by a
  holder of MANAGE on its owning unit, or on the case without one, never by the preparer (the
  approvals appear on "Mes tâches"); executing a transfer bordereau builds each approved case's
  complete package (the export format of GLD-029, built by the system without read filter,
  downloadable by records managers for 90 days, every download logged), records it as EXPORT,
  freezes the case and keeps the bordereau's manifest and its SHA-256 as proof. Executing an
  elimination comes next. Reference for archivists, data protection and operators:
  `docs/RETENTION.md`.

### Changed

- The end-to-end scenario counts only its own operators' audit events when it checks their outbox
  events: another test package sharing the database purges its own events meanwhile.

## [0.19.0] - 2026-10-28

Observability and front-end quality: Prometheus metrics on a separate listener and OpenTelemetry
traces (both off by default), the SPA's API types checked against the protos, the icons bundled
one by one, and a dependency security update. `/health` no longer shows the database pool
statistics. No migration, no API change.

### Security

- `golang.org/x/net` 0.62.0 fixes GO-2026-6866 and GO-2026-6860 (govulncheck found them in a
  module the server requires, unreached by its code). GO-2026-5932 (`x/crypto/openpgp`, no fix
  upstream) stays suppressed, re-verified unreachable.

### Added

- Prometheus metrics (GLD-010, first step): `GOELAND_METRICS_ADDRESS` opens a separate listener
  serving `GET /metrics` with the requests and their durations per RPC or route pattern, the
  database pool, the outbox and case export backlogs, the build and the Go runtime. Off by
  default; the k8s smoke manifests serve it on port 9464, outside the Service.
- OpenTelemetry traces (GLD-010, second step): with `OTEL_EXPORTER_OTLP_ENDPOINT` set, each
  request becomes a span named after its RPC (continuing an incoming `traceparent`), each of its
  SQL queries a child span (statement only, never the arguments), exported over OTLP/HTTP; the
  logs written for a request carry its `trace_id`. Off by default.

### Changed

- `/health` answers `{"status":"ok"}` only: the database pool statistics it showed publicly are
  now metrics.

- The interface draws its icons as inline SVG and bundles only the 102 it uses instead of the
  whole Material Design Icons font (GLD-073): the built assets shrink from 8.9 to 5.2 MB and the
  main stylesheet from 696 to 373 KB.
- The SPA's hand-written API types are checked against the protos at compile time (GLD-072):
  `buf generate` also writes proto3-JSON TypeScript types (protobuf-es 2.16.0, type-only) that
  `make check` compares with `src/api/types.ts`, and `make generated-check` keeps them
  reproducible. The check tightened the `Struct` fields to JSON objects and gave the group,
  membership and "my tasks" answers their own types.

## [0.18.1] - 2026-10-27

The app bar works on phones and tablets, plus quick fixes (dates in the interface language, list
hints and links) and quality clean-ups. No migration, no API change.

### Fixed

- The app bar works on a phone or a tablet: below the large breakpoint the modules move to a
  drawer behind a menu button and the global search opens on a second row of the bar (it was
  hidden, and the module buttons too). On a large screen the secondary modules show their icon
  only, so the title no longer ends in an ellipsis.
- Dates follow the interface language (FR/EN switch) instead of the browser's, day first in English
  too.
- « 1 actives » on the relationship groups: the count takes the singular.
- The search hints of the case and object lists (reference, parcel number, EGRID, EGID) show again
  under the field; GLD-074 had reduced them to a tooltip.

### Changed

- The name in a list row is a real link (keyboard, middle click, new tab) rather than a row with a
  link role.
- Quality: SonarCloud no longer applies the cognitive complexity rule to Go tests (the local gate
  never did), plus small clean-ups; the k8s smoke manifests set ephemeral storage requests and
  limits, and the PostGIS pod no longer mounts a service account token.

## [0.18.0] - 2026-10-27

Transactional outbox (GLD-028): every audit event is announced to the outbox consumers, with an
administration view and replay. Migration `0034`; new RPCs `CoreService.GetOutboxStatus` and
`ReplayOutboxEvents`; new settings `GOELAND_OUTBOX_RETENTION_DAYS`, `GOELAND_OUTBOX_POLL_SECONDS`,
`GOELAND_OUTBOX_LOG`.

### Added

- **GLD-028** — Transactional outbox: every audit event writes an outbox event in the same
  transaction (`core.InsertAuditEventTx`), so a committed change is always announced and a
  rolled-back one never is. A worker on every replica delivers the events to the registered
  consumers at least once (woken by `pg_notify`, polling every `GOELAND_OUTBOX_POLL_SECONDS`), retries
  a refusal with a growing delay, gives up after 10 attempts and deletes the delivered events after
  `GOELAND_OUTBOX_RETENTION_DAYS` (default 7). Administration gets an « Outbox » tab: pending, given-up
  and kept events, the consumers, the latest failures and « Rejouer » (new RPCs
  `CoreService.GetOutboxStatus`, `GET /api/outbox`, and `ReplayOutboxEvents`,
  `POST /api/outbox:replay`). Migration `0034`. `GOELAND_OUTBOX_LOG=true` logs every event; the
  search indexer and the notifiers will be the first business consumers.

## [0.17.1] - 2026-10-27

The document page follows the information hierarchy of the other detail pages (GLD-075). No
migration, no API change.

### Changed

- **GLD-075** — The document page follows the information hierarchy of the actor, case and object
  pages: a header with the state, the key facts (official date, version, file, language, external
  system, creation and creator) and the download as the main action, jump links with counts, then
  the cases, actors and objects the document belongs to, grouped by role, beside its versions; the
  technical panels (identity, access, integrity, governance, audit, raw relationships) move to a
  « Gouvernance et historique » tab.

## [0.17.0] - 2026-10-27

Global search over every kind of subject (GLD-064) and list filters by period and creator, shown in
the search results by kind (GLD-074). Migrations `0032` (subject search index, about a minute on the
imported data) and `0033` (creator indexes); new RPC `CoreService.SearchSubjects`; new optional
filter fields on the case, document, actor and thing searches.

### Added

- **GLD-064** — Global search: a field in the app bar (« / » focuses it) searches cases, actors,
  documents and things at once by title, name and business reference, accent-insensitive, from the
  start of every word; suggestions are grouped by kind with their count (capped at 500), Enter opens
  every result (`/search`) and each group links to its list filtered by the query (the list pages
  read `?q=`). Only what the caller may read is found or counted. New RPC
  `CoreService.SearchSubjects` (`GET /api/subjects:search`) over `subject_ref`, indexed per kind by
  migration `0032` (about a minute on 3.1M subjects; it replaces an unused label index).
- **GLD-074** — List filters by period and creator: the search results page shows one tab per kind
  (cases, actors, documents, things) with its count and the kind's full list, filters included. The
  case, document, actor and thing searches take `created_from`, `created_to` (days in
  Europe/Zurich) and `created_by`; cases also `opened_from` / `opened_to`, documents
  `official_from` / `official_to`. The lists show them behind « Plus de filtres » and keep every
  filter in the URL (shareable, back button). Migration `0033` indexes `created_by` (a creator's
  list in a few milliseconds on the imported data).

### Changed

- **GLD-064** — The case, document, actor, thing and org unit searches match the start of every word
  (« rhod » finds « Rhodanie »), through the shared `core.PrefixTSQuery`.

## [0.16.2] - 2026-10-27

Security patch for the Trivy findings on the v0.16.1 image. No migration, no API change.

### Security

- Echo 4.16.2 (indirect, via `go-cloud-k8s-common-libs`), fixing GHSA-cxf3-9h37-7rr8 (rejected
  WebSocket upgrades bypassing route-level access controls), GHSA-9969-6676-49h5 (WebSocket proxy
  retaining connections), GHSA-f48p-c9v4-h7xv (unbounded BodyDump) and GHSA-h7wr-284f-5qvc
  (unbounded gzip expansion in Decompress). `make vuln-check` reports no reachable vulnerability.

## [0.16.1] - 2026-10-20

Security patch: labels of subjects the caller may not read are no longer shown (GLD-070); Go 1.27.3
and the dependency updates of 2026-10-20. No migration, no API shape change.

### Security

- **GLD-070** — Labels of unreadable subjects: a document cited by a timeline entry keeps its
  citation but not its title for a caller who may not read it (the SPA shows « Document
  restreint »); on an org unit page an unreadable ancestor keeps its place in the path without its
  name and unreadable sub-units are left out, as in the tree (`core.Authorizer.Unreadable`). No
  migration; the API now leaves `document_label` and an ancestor's `label` empty in that case.

### Changed

- Dependency updates of 2026-10-20: Go 1.27.3 (`go.mod` and the digest-pinned `golang` builder
  image), Connect 1.21.1, googleapis (`buf.lock`; the regenerated descriptors encode
  `field_behavior` packed, the OpenAPI is unchanged), the modules linked into the binary (Echo
  4.16, puddle 2.2.3, `x/time`, `x/exp`, isatty, colorable) and the `gocognit` 1.3 / `govulncheck`
  1.9 tools; Vue 3.5.44, Vuetify 4.3, vue-i18n 11.4.16, OpenLayers 10.11 and proj4 2.22 (still
  pinned exactly; two stricter typings adapted, the City's map checked in a browser), ESLint 10.13,
  Vitest 5.0.5, npm-run-all2 9 and unplugin-fonts 2 (same build output). Still left out:
  Vanguard 0.5 / validate 0.9 (they need Connect v2, planned in the roadmap), TypeScript 7 and
  @types/node 26; the `bun` and `postgis` images were already current.

### Fixed

- **GLD-071** — The SPA type-check no longer warns about typed routes: the unused vue-router Volar
  plugins (file-based routing, which the SPA does not use) were removed.

## [0.16.0] - 2026-10-15

The case and object pages follow the actor page's information hierarchy (GLD-068, GLD-069); the object
page shows the object on the City of Lausanne's maps. No migration, no API change.

### Added

- **GLD-068** — The case page follows the actor page's model: a header (reference, status, type,
  dates, what is ongoing, a status menu, jump links) and three tabs, « Déroulement » (timeline,
  tasks, circulations), « Intervenants et pièces » (roles by kind then role) and « Gouvernance et
  historique » (access, export, governance, audit, raw relationship table). Section titles of the
  roles panel depend on the subject on screen ("Intervenants", "Pièces du dossier" on a case).
- **GLD-069** — The object page takes the same header and tabs and shows the object on the City of
  Lausanne's maps (plan, cadastral background, 2024 and 2016 orthophotos in LV95, OpenLayers 10.6.1
  and proj4 2.19.10, adapted from the City's carte_mn95 application), with a link to the City's map
  application; the SVG outline remains outside the City's tiles. The CSP allows
  `https://tilesmn95.lausanne.ch` for images and connections.

## [0.15.0] - 2026-10-15

Information hierarchy piloted on the actor page (GLD-063) and import fixes found with it (GLD-066, GLD-067);
strict JSON on path-parameter routes (GLD-065). Migration `0031`; new RPC `CoreService.SummarizeRelationships`.
The import fixes take effect on a re-import.

### Added

- **GLD-063** — Actor page pilot (information hierarchy): relationship types name the role one end
  plays (migration `0031`: `role_label`, `role_label_plural`, `role_end`, `role_preposition`; seeded
  types filled, imported roles from their legacy name; editable in Administration), and groups are
  titled from the subject on screen ("Client des affaires suivantes", "Clients").
  `CoreService.SummarizeRelationships` (`GET /api/subjects/{subject_id}/relationship-summary`)
  counts the groups as the caller may read them, with the open cases and the split by case or
  document type; `ListRelationships` filters on the other end (`other_case_status`,
  `other_type_code`, `other_year_from`/`to`, `other_query`: text in the label or business reference)
  and describes it (`include_details`). The actor page
  has a header (IDE linked to the Swiss register, ABACUS debtor linked, main contacts with the
  Swiss phone directory, jump links with counts), its roles by kind then role (alphabetical; past 10
  edges a text filter, a period for cases and documents and a type filter; documents unfold by
  family and objects by type) and a "Gouvernance et historique" tab; the app bar shows
  the server version and the user menu reaches tasks, groups, administration and the version.

### Fixed

- **GLD-067** — The legacy import types each document by its family (`doc_family`, 114 families:
  "Photo (objet)", "Plan d'objet", ...) as document types `LEG_DOC_<IdDocFamily>`, instead of one
  generic "Document Goéland" type for all 2.3M documents. Takes effect on the next re-import.
- **GLD-066** — The legacy import keeps a phone or fax number followed by a note
  ("021 315 22 22 (secrétariat)", about 1,460 values left out until now): the number is imported and
  the note becomes the contact label. Takes effect on the next re-import.
- **GLD-065** — A request body with an unknown field answers 400 INVALID_ARGUMENT on routes with path
  parameters too (`PATCH /api/cases/{id}`, `POST /api/cases/{id}/transition`, ...), not HTTP 500.

### Changed

- `scripts/import_rebuild.sh` reads the replica and superuser connections from the environment or
  from `.env.import` (Git-ignored, `.env.import.sample`), never from `.env`, which the Makefile
  exports to the server; a placeholder password left in place is refused.
- `go-cloud-k8s-common-libs` v0.6.4, relicensed by its author from GPL-3.0 to MIT, so every
  dependency of the binary and the image is now under a permissive license (the GPL-3.0 entry
  found by the GLD-059 license inventory is gone).

## [0.14.1] - 2026-10-14

Reproducible generated code (GLD-008) and the triaged dependency updates. No migration, no API change.

### Added

- **GLD-008** — Generated-code reproducibility gate: `protoc-gen-go` and `protoc-gen-connect-go`
  run as `go.mod` tools at the module versions, the remote OpenAPI plugin is pinned (v2.31.0), and
  `make generated-check` (part of `make release-check`, hence CI) regenerates into a temporary
  directory and fails when the committed `gen/` or `api/openapi/` differ. The generated Go files
  were regenerated (only their `protoc-gen-go` version stamp changes).

### Changed

- Dependency updates (Dependabot triage of 2026-10-14): pgx 5.11, Connect 1.21, protovalidate;
  Vue 3.5.43, Vuetify 4.2, vue-router 5.4, vue-i18n 11.4, Pinia 4, Vite 8.3, ESLint 10 and the
  other SPA minors; GitHub's own actions (checkout 7, setup-go 7, codeql 4) and the pinned
  third-party ones at release tags (setup-trivy 0.3.1, docker actions, action-gh-release 3).
  Left out: Vanguard 0.5 / validate 0.9 (Connect v2, a planned roadmap task), TypeScript 7 (not supported by
  vue-tsc), @types/node 26 (the SPA targets Node 22) and an hours-old untagged trivy-action
  commit; Dependabot now ignores the first three.

## [0.14.0] - 2026-10-14

Case export (GLD-029) and immutable build inputs with SBOM (GLD-059). Migration `0030` (`case_export`);
new `CaseExportService`.

### Added

- **GLD-029** — Case export (v2 §51): a holder of MANAGE asks for a ZIP of a case
  (`CaseExportService.RequestCaseExport`, `POST /api/cases/{case_id}/exports`); a background
  worker (every replica, `FOR UPDATE SKIP LOCKED`, migration `0030` `case_export`) builds it as the
  requester: `manifest.json` (`CaseExportManifest`: case and governance, timeline, tasks,
  circulations, relationships, documents with all their versions, audit, provenance, withheld
  counts) and each content blob once under `blobs/<sha256>`, digests checked. Only what the
  requester may read is exported. Every request is recorded as EXPORT in the sensitive read audit,
  granted or refused, and so is each confidential document included. The requester downloads it
  from `GET /api/case-exports/{id}/content` until it expires (`GOELAND_EXPORT_TTL_HOURS`, default
  24; `GOELAND_EXPORT_POLL_SECONDS`, default 5). SPA: "Export de l'affaire" panel on the case page.
  The §50 scenario now plays step 31.

- **GLD-059** — Immutable build inputs and SBOM: the Dockerfile base images (`oven/bun:1.4.3`,
  `golang:1.27.2`) and the `postgis/postgis:17-3.5` image of CI and the smoke manifests are pinned
  by digest; `make build-inputs-check` (in `make check`) refuses an image without digest, a
  third-party action without commit SHA and a Go builder older than `go.mod`. Dependabot
  (`.github/dependabot.yml`) proposes weekly grouped updates. Each GitHub release carries a
  CycloneDX SBOM with licenses and a license inventory (`scripts/sbom.sh`, `make sbom`, Trivy), and
  the published image SBOM and provenance attestations.

## [0.13.1] - 2026-10-14

Actor correctness batch. Migration `0029` (contact history, relationship kind trigger); no new RPC.

### Fixed

- **GLD-004** — `UpdateActor` with details of the other kind (person details on an organization,
  organization details on a person) answers `INVALID_ARGUMENT` instead of `INTERNAL`.
- **GLD-005** — `UpdateActorRequest.display_name` is truly optional: empty keeps the current name
  (it was required by validation, so the "unchanged" case was unreachable).
- **GLD-007** — Replacing an actor's contacts no longer deletes them: the contacts that left the
  list are ended (`actor_contact.ended_at`, `ended_by`, migration `0029`) and kept. Contacts and
  address links identical to the new list stay untouched, so saving an actor without touching
  them writes nothing; `ACTOR_UPDATED` records `contacts_ended` / `contacts_added` and
  `addresses_ended` / `addresses_added` when they change (replacing `contacts_replaced` and
  `addresses_replaced`).

### Added

- **GLD-060** — The database checks that a relationship's source and target kinds match its type
  (trigger `trg_subject_relationship_kinds`, migration `0029`), so the legacy import and future
  workers that write edges directly cannot create an invalid one.

## [0.13.0] - 2026-10-14

Sensitive read audit (GLD-033). Migration `0028` (new append-only table); new RPC
`CoreService.ListAccessAuditEvents`.

### Added

- **GLD-033** — Sensitive read audit: reading a confidential subject (its detail, or a
  confidential case's timeline) or downloading a confidential document, and every refusal of
  either, writes an `access_audit_event` row (migration `0028`, append-only) before the answer; the
  read fails when the row cannot be written. Ordinary reads and list results are not logged.
  `CoreService.ListAccessAuditEvents` (`GET /api/access-audit-events`, filters by subject, reader,
  action, outcome and period) serves administrators, and holders of FULL_CONTROL for their
  subject; a subject's label is shown only to who may read it. SPA: Administration → Accès
  sensibles, and "Journal des accès sensibles" in each subject's Accès panel.

### Changed

- The `AccessSource` enum moved from `access.proto` to `core.proto` (same package and values, no
  wire change) so the audit rows reuse it.

## [0.12.2] - 2026-10-13

Security patch: Go 1.27.2 and the `golang.org/x/*` updates fix the Trivy findings on the v0.12.1
image. No migration, no API change.

### Security

- **Go 1.27.2** (`go.mod`; the `golang:1.27-alpine` builder sets `GOTOOLCHAIN=local`, so an older
  image now fails the build instead of shipping 1.27.1): standard library fixes for the Trivy
  findings on the v0.12.1 image (CVE-2026-78667, CVE-2026-78669, CVE-2026-97031, CVE-2026-56866,
  CVE-2026-78663, CVE-2026-94439, CVE-2026-94448, CVE-2026-97032, CVE-2026-56857, CVE-2026-78659,
  CVE-2026-78660, CVE-2026-94440, CVE-2026-97030: `net/http`, HTTP/2, `crypto/tls`,
  `html/template`, `mime/multipart`, `os`).
- `golang.org/x/net` v0.59.0 → v0.61.0 (CVE-2026-78669, HIGH, and CVE-2026-78663, CVE-2026-97032,
  CVE-2026-78659, CVE-2026-78660: `x/net/http2`); `x/crypto` v0.58.0, `x/sys`, `x/text`, `x/sync`,
  `x/tools` and `x/telemetry` follow. `.trivyignore` `GO-2026-5932` re-verified against
  x/crypto v0.58.0 (still no openpgp package linked).

## [0.12.1] - 2026-10-08

A maintenance release after the review of 2026-10-05: `GetDocument` lists the cases holding a
document (**GLD-006**), and the operator and contributor documentation matches the code again,
with new checks that keep it so (**GLD-058**). No migration and no breaking change.

### Fixed

- **GLD-006** — `GetDocument` with `includeRelationships` returns the document's relationships
  in both directions, including the `CASE_HAS_DOCUMENT` edges of every case holding it (outgoing
  edges only before).

### Changed

- **GLD-058** — Follow-ups of the 2026-10-05 review: the production readiness limitations, the
  README, the implementation status and several package docs match the code again, and
  `make docs-check` now fails when the latest migration or the download route drifts from the
  documents, or when the removed raw download returns to them.

## [0.12.0] - 2026-10-08

This release shows the POC on production data: a one-shot local import of the legacy Goéland
data (**GLD-051**, **GLD-052**, **GLD-054**; real data never leaves the machine), searches and
relationship lists that hold at that volume (**GLD-053**), and the UI remarks made on it —
sortable lists and detail tables (**GLD-055**, **GLD-056**) and type pickers ordered by label
and searchable (**GLD-057**). Migrations `0025` to `0027` apply automatically at startup.
Search and relationship totals are now counted up to 10 000 (`totalSizeCapped` tells a client
that the real total is higher).

### Added

- **GLD-051** — Legacy data import framework: `cmd/goeland-import` loads a brand-new local
  database from the read-only legacy replica in one transaction (dry run by default, the full
  run rolled back), set-based (COPY), with deterministic UUIDv5 ids, provenance and one import
  marker per run (migration `0025`: `import_batch`, `subject_provenance`; GLD-027), counts-only
  reports, and `scripts/import_rebuild.sh` to rebuild and rerun. Rules and runbook in
  `docs/IMPORT_MAPPING.md`.
- **GLD-052** — Import wave 1: employees as users with their unit, security groups and
  members, case types (default confidentiality), cases (status, confidentiality, legacy number
  as business reference `GOELAND`), grants, actors with checked contacts and a correspondence
  address, and the actor, employee and unit roles on cases.

- **GLD-053** — Production volume: search and relationship totals are counted up to 10 000
  (`totalSizeCapped` on `SearchCases`, `SearchDocuments`, `SearchActors`, `SearchThings` and
  `ListRelationships`; the SPA shows "10 000+"), indexes on the sort keys (migration `0026`), and
  detail pages load relationships page by page ("Charger plus"). Document searches scoped to a
  case or a thing start from its links, and the unscoped one counts within the newest 20 000
  documents.
- **GLD-054** — Import wave 2: things (approximate LV95 location, parcel and building
  details), their actor roles and cases, links between cases (parent, related), documents as
  metadata with their current content known by its SHA-256 (the bytes stay in the legacy store),
  the readers of the confidential documents, their cases, things and actors, and the case
  follow-ups as timeline entries with their cited documents and final status.
- **GLD-055** — Sortable lists: a click on a column header sorts the case, document, actor,
  thing, task and group lists (`orderBy` on `SearchCases`, `SearchDocuments`, `SearchActors`,
  `SearchThings`, `ListMyTasks` and `ListCaseTasks`: `"<field>"` or `"<field> desc"`, an unknown
  field answers 400); indexes on the sorted columns (migration `0027`).
- **GLD-056** — Sortable detail tables: the relationship table of the detail pages is one list
  in both directions sorted by its headers (`bothDirections` and `orderBy` on
  `ListRelationships`), the reference change log sorts on the server (`orderBy` on
  `ListReferenceChanges`), and the group members, document versions, reference catalogues and
  role holders sort in the browser; unit members are listed by name.
- **GLD-057** — Ordered type pickers: the catalogues (case, document, thing, task, org unit and
  relationship types, organization categories) come ordered by label, code as tie-break, and the
  case, document, thing, relationship type and organization category pickers filter as you
  type, ignoring accents and case.

### Changed

- Searches read their matches in index order and stop at the count limit, the server turns JIT
  off and forces custom plans on its connections: on ~512k cases the unfiltered case search went
  from 2.7 s to 0.04 s (GLD-053).
- `cmd/goeland-import-orgunits` shares its unit mapping with the new import (`pkg/legacyimport`).

## [0.11.0] - 2026-10-01

This release completes real authorization (**GLD-017**): searches, lists, downloads and the
timeline follow read rights (**GLD-049**), and case types carry default access (**GLD-050**).
Migration `0024` applies automatically at startup. **Breaking:** document bytes are downloaded
through `GET /api/documents/{id}/content` (the raw `?ref=` endpoint is removed), and request
bodies with an unknown field answer 400. The identity provider fixes go with it
(go-cloud-k8s-auth `0a829b5`: e-mail linking only when verified, user directory for
administrators).

### Added

- **GLD-049** — Searches and lists show only what the caller may read: cases, documents, actors,
  things, org units (search and tree), groups, business-reference lookups and relationship lists
  (an edge whose other end is unreadable is left out, also in `includeRelationships`). The filter
  runs inside the SQL query (`core.ReadableSQL`, `core.Viewer`), so pages and totals stay exact.
  Timeline entries follow their visibility: participants with READ on the case, internal with
  CONTRIBUTE, restricted with MANAGE, and always their author; writing for an audience needs the
  same level.
- **GLD-050** — Default access per case type: a minimum confidentiality
  (`defaultConfidentialityLevel` on `CreateCaseType` / `UpdateCaseType`) and default grants
  (`SetCaseTypeDefaultGrants`, `POST /api/case-types/{code}/default-grants`; users, groups, units
  and `GRANTEE_KIND_CREATOR_UNITS`, the creator's direct units), copied once onto every new case
  of the type; migration `0024`. SPA: Administration → Types d'affaire → "Accès par défaut".

### Changed

- **Breaking:** document bytes are downloaded with `GET /api/documents/{id}/content`
  (`?versionId=` for another version than the current one), which needs READ on the document;
  `GET /api/documents/download?ref=` is removed. The SPA uses the new endpoint.
- The automatic reuse of a document with identical content only picks a document the caller may
  read; otherwise a new document is created on the same content.
- **Breaking:** a request body naming an unknown field now answers 400 INVALID_ARGUMENT, on the
  REST bindings and on the Connect path, instead of being applied without it.

## [0.10.0] - 2026-10-01

This release makes authorization real (Phase 6, first two steps): application roles stored in
Goéland (**GLD-047**) and grants with security groups enforced on every subject (**GLD-048**).
Migrations `0022` and `0023` apply automatically at startup; current administrators keep their
role and every subject's creator gets FULL_CONTROL. **Breaking:** `goeland:admin` no longer comes
from the token, and a mutation now needs a level on its subject. Searches and lists are not
filtered yet.

### Added

- **GLD-047** — Application roles stored in Goéland, the first step of real
  authorization : migration `0022` (`app_role` with ADMIN, `app_user_role` history),
  `CoreService.ListAppRoles`, `ListRoleHolders`, `ListUserRoles`, `GrantUserRole` and
  `RevokeUserRole` (REST under `/api/app-roles` and `/api/users/{user_id}/roles`); grants and
  revocations need a reason, are audited on the user (`USER_ROLE_GRANTED` / `USER_ROLE_REVOKED`)
  and kept as history; the last administrator cannot be revoked. SPA: Administration → Rôles.
- **GLD-048** — Grants and security groups: `AccessService` (`GetMyAccess`,
  `ListGrants`, `SetGrant`, `RevokeGrant`, `ListGroups`, `GetGroup`, `CreateGroup`, `UpdateGroup`,
  `ArchiveGroup`, `AddGroupMember`, `RemoveGroupMember`) over migration `0023` (`access_grant` with
  history, `security_group`, `USER_MEMBER_OF_GROUP`, kind-wide roles ACTOR_MANAGER and
  THING_MANAGER). The effective level is the most specific grant (personal, groups, nearest unit,
  role, baseline READ) and confidential subjects (`confidentiality_level` >= 2) need an explicit
  grant, with no administrator bypass. It is enforced on every mutation and single read of
  cases, documents, actors, things, org units and groups, on timeline entries, tasks,
  circulations and relationships (memberships managed from the group or unit); creators get
  FULL_CONTROL, owning units MANAGE (backfilled for existing subjects); a document deposited from
  a case copies its grants and confidentiality once. SPA: "Accès" panel on every detail page
  (actions hidden by level) and the "Groupes" pages.

### Changed

- **Breaking:** mutations now need a level on the subject: a caller who is neither its creator,
  nor granted, nor an administrator only reads non-confidential subjects (403 otherwise).
- **Breaking:** `goeland:admin` now comes from the ADMIN role stored in Goéland; the auth server's
  `IsAdmin` flag is ignored. Current administrators keep the role (migrated from the last recorded
  flag); new ones come from `GOELAND_BOOTSTRAP_ADMINS` (user ids) or an administrator.
  `GOELAND_DEV_USER_ADMIN=true` bootstraps the dev user. `User.roles` lists a user's roles.

### Fixed

- The embedded SPA dropped the build chunks whose names start with `_` (Vite's
  `_plugin-vue_export-helper`): `//go:embed all:goeland-front/dist` keeps them, and a test checks
  that every built file is embedded.
- The SPA ignored the numeric error code of REST errors, so refusals showed the server's English
  text; 401, 403 and 404 now show a translated message.

## [0.9.2] - 2026-09-30

This release applies the second review of 2026-09-30 (**GLD-044**): the database integration and
end-to-end tests now run in CI, and the audit trail is append-only in the database. Migration
`0021` applies automatically at startup. **Breaking for deployments:** `AUTH_SERVER_URL` must use
HTTPS unless its host is loopback or `GOELAND_ALLOW_INSECURE_AUTH_URL=true` is set.

### Added

- **GLD-044** — Second review hardening: the database integration tests, the §50
  scenario and the API surface test run in CI against a PostGIS service (`GOELAND_REQUIRE_DB_TESTS`
  makes a missing database fail instead of skip); migration `0021` makes `audit_event` and
  `reference_change` refuse UPDATE, DELETE and TRUNCATE; the PAT cache is bounded (1024 entries,
  expired ones swept); `AUTH_SERVER_URL` must use HTTPS outside loopback unless
  `GOELAND_ALLOW_INSECURE_AUTH_URL=true`; a client `X-Request-ID` is kept only when short and made
  of safe characters; server configuration tests; `make test` and `make lint` build the frontend
  on a clean checkout; stale README, PRODUCTION_READINESS and IMPLEMENTATION_STATUS passages
  corrected.

## [0.9.1] - 2026-09-30

This release applies the post-audit hardening of 2026-09-29 (**GLD-042**, **GLD-043**): browser
security headers, batched search hydration, shared database plumbing, an API surface test
reaching every RPC, the first SPA unit tests, a local Kubernetes smoke deployment, and a server
that waits for its database at startup. No migration and no API change; REST calls with an
unknown query parameter now answer 400 instead of 500.

### Added

- End-to-end test of the spec v2 §50 scenario over HTTP against the real server
  (`cmd/goeland-server/scenario_test.go`: authentication, scopes, validation, error codes).
- Local Kubernetes smoke deployment: `deployments/k8s/` manifests (disposable PostGIS, hardened
  Deployment with probes) and `scripts/k8s_smoke_test.sh`, documented in `deployments/k8s/README.md`
  (dev token, browsing, reset, troubleshooting) and a README section.
- **GLD-042** — Post-audit hardening: browser security headers on every response
  (CSP with the auth server origin in `jwt` mode, nosniff, no framing, referrer, opener and
  permissions policies); a shared `core.InTx` and `core.MapDBError` base (SQLSTATE constants,
  `PgErrorWithCode`) replacing the per-domain copies while each domain keeps its messages;
  the case, actor, document, thing and org unit searches hydrate a page in a fixed number of
  queries (`core` batch loaders) instead of three to five per row; an API surface test calls
  every RPC the §50 scenario does not reach over REST (Connect adapters from 13-56 % to
  72-79 % covered, 74.8 % overall); Vitest unit tests of the SPA's pure modules in
  `make front-check`, the contact rules checked against the server's own cases
  (`pkg/actor/testdata/contact_values.json`, the first frontend tests);
  `AGENTS.md` frontend entry brought up to date.

### Changed

- At startup the server waits for the database, retrying with a growing delay for
  `GOELAND_DB_CONNECT_TIMEOUT_SECONDS` (default 60), instead of exiting on the first failure: a
  pod started before its database no longer crash-loops. The startup budget is that wait plus
  60 s for migrations (it was a fixed 15 s), and SIGTERM now also stops a startup in progress.

### Fixed

- **GLD-043** — A REST call with a query parameter that names no request field (a typo such as
  `?querry=`) answered 500 `UNKNOWN` from the Vanguard transcoder; it now answers 400
  `INVALID_ARGUMENT` naming the parameter (`cmd/goeland-server/queryparams.go`).
- The published container image reported `revision` and `build` as `unknown`: the
  docker-publish workflow now passes `APP_REVISION` and `BUILD_STAMP` like the local build script.

## [0.9.0] - 2026-09-29

This release completes the case spine of spec v2 (Phases 3 to 5): the case timeline
(**GLD-012**), organizational units (**GLD-041**), tasks (**GLD-026**) and circulations
(**GLD-013**). The 16-step demo scenario of the spec now runs end to end. Migrations `0017` to
`0020` apply automatically at startup. **Breaking:** `RecordMetadata.owner_org_id` becomes a
typed reference to an org unit (earlier free-text values are dropped).

### Added

- **GLD-012** — Case timeline ("suivis", spec v2 §26-27): `TimelineService` (`goeland.v1`,
  REST under `/api/cases/{case_id}/timeline` and `/api/timeline-entries`) over migration
  `0017`: `case_timeline_entry` (COMMENT, OPINION, DECISION, REQUEST, RESPONSE, VALIDATION,
  SYSTEM, AI_PROPOSAL) with a business date, an intended visibility (not enforced before
  real authorization) and the DRAFT → VALIDATED / LOCKED / WITHDRAWN lifecycle; only drafts change
  (also enforced by database triggers) and a correction is a new entry naming the corrected
  one. `timeline_document_link` cites logical documents, links them to the case when needed
  and pins the current version on validation. Every timeline mutation is audited on the case.
  SPA "Suivis" panel in the case detail: filters, create / edit / validate / lock / withdraw /
  correct, cited documents with their pinned version.

- **GLD-041** — Organizational units: `OrgUnitService` (`goeland.v1`, REST under
  `/api/org-units` and `/api/org-unit-types`) over migration `0018`: administrable
  `org_unit_type` (seven production types) and `org_unit`, an ORG_UNIT subject in one tree
  without cycles, with a non-unique abbreviation, a label unique among live siblings, an
  immutable external reference and dissolution instead of deletion. Mutations require
  `goeland:admin`. Relationship types `CASE_HAS_ORG_UNIT_LEADER`, `_MANAGER` and
  `_PARTICIPANT`. `cmd/goeland-import-orgunits` optionally imports the real tree (structure
  only) from a read-only replica. SPA "Unités" tree and detail pages, org units in the
  subject picker and the unit type catalogue in the administration page.

- **GLD-026** — Case tasks: `TaskService` (`goeland.v1`, REST under `/api/cases/{case_id}/tasks`,
  `/api/tasks` and `/api/task-types`) over migration `0019`: administrable `task_type`,
  `case_task` (OPEN → IN_PROGRESS → DONE / CANCELLED, reopen with a reason, deadline, origin)
  assigned to one internal user or org unit with a `case_task_assignment` history, "my tasks"
  (mine and my units' through the new `USER_MEMBER_OF_ORG_UNIT` relationship), SYSTEM timeline
  entries on completion and cancellation. `CoreService.SearchUsers` (`/api/users/search`).
  SPA: tasks panel in the case, "Mes tâches" page, unit members, users in the subject picker.

- **GLD-013** — Case circulations: `CirculationService` (`goeland.v1`, REST under
  `/api/cases/{case_id}/circulations`, `/api/circulations` and `/api/circulation-recipients`)
  over migration `0020`: a circulation sends a case to users and org units in ordered steps;
  each recipient gets a task managed by the circulation, answers (FAVORABLE, UNFAVORABLE,
  COMMENT, NOT_CONCERNED, NEED_MORE_INFO) are recorded as locked RESPONSE timeline entries, the
  next step opens once a step has fully answered and the last answer completes the circulation
  with a summary. SPA: circulations panel in the case, answering from "Mes tâches".

### Changed

- Tasks created by a circulation can only be started directly (`task.MoveTx` with
  `MoveOptions`); `timeline.RecordSystemEntryTx` can also write other locked entry types.
- A case with an open circulation can no longer be closed.
- A case with open tasks can no longer be closed (FAILED_PRECONDITION; the SPA explains why);
  the "is this case open" guard is shared as `core.EnsureOpenCaseTx`.
- SPA: a SYSTEM timeline entry no longer offers a correction.
- **Breaking:** `RecordMetadata.owner_org_id` is now a typed reference to an existing, not
  dissolved org unit (UUID); earlier free-text values are dropped by migration `0018`. The
  governance panel shows the owning unit by name. A dissolved unit can no longer be linked.
- E-mail normalization moved to `core.NormalizeEmail` (shared by actor contacts and units).
- A case status change now also records a locked SYSTEM timeline entry, and a case with
  draft timeline entries can no longer be closed (FAILED_PRECONDITION; the SPA explains why).
- The subject picker emits the picked subject so callers can show its label.

## [0.8.0] - 2026-09-24

This release adds the Thing slice (**GLD-016**, spec v2 Phase 2): parcels, buildings and other
business objects with an EPSG:2056 (LV95) PostGIS geometry, linked to cases, documents and
actors. Migration `0016` applies automatically at startup; no breaking change for existing
`goeland.v1` clients. It also fixes the size of small buttons in card titles across the SPA.

### Added

- **GLD-016** — Thing slice: `ThingService` (`goeland.v1`, REST under `/api/things` and
  `/api/thing-types`) over migration `0016`: `thing_type` (generic, parcel or building
  specialization; seeded PARCEL, BUILDING, STREET, TREE, INFRASTRUCTURE, ADVERTISEMENT,
  SPORT_ZONE; administrable), `thing` with an EPSG:2056 (LV95) geometry — GIST-indexed, checked
  valid, inside Switzerland (WGS84 degrees are refused) and of a type fitting the
  specialization — exchanged as GeoJSON with its computed area and an anchor point,
  `thing_parcel` (commune OFS number + unique parcel number, EGRID) and `thing_building`
  (unique EGID, ECA number, construction year, RegBL status). Search by text, parcel number,
  EGRID, EGID, type and LV95 extent (`bbox`). Land-rights relationship types
  `THING_HAS_ACTOR_OWNER`, `_TENANT`, `_SURFACE_RIGHT_HOLDER`, `_FARMER`, `_EASEMENT_HOLDER`.
  SPA things list, creation and detail pages with an SVG geometry preview and a
  map.geo.admin.ch link; the subject picker now finds things (e.g. "case concerns thing").

### Changed

- `core.ApplyInitialGovernance` replaces the governance-copy block of each Connect adapter, and
  the server wires the actor, case and thing modules through one helper.

### Fixed

- SPA: small buttons in card titles (e.g. "Link to a subject") rendered at the title's size:
  lazily loaded component CSS could precede the main stylesheet and invert the Vuetify cascade
  layers; `index.html` now declares the layer order before any stylesheet.

## [0.7.0] - 2026-09-24

This release completes Phase 1b, "usable Actor and Case", from the review of v0.6.0: local SSO
documentation and sign-in hints (**GLD-035**), navigable relationships (**GLD-036**), a subject
picker instead of UUIDs (**GLD-037**), internal users shown by name with a visible admin flag
(**GLD-025**), typed and validated actor complements (**GLD-038**), a person's minimal identity
(**GLD-039**), typed addresses with branches and contact persons (**GLD-014**) and reference data
administration (**GLD-040**). Migrations `0012`–`0015` apply automatically at startup.
**Behavior change** for API clients: malformed actor complements, a person without last name
and a Swiss postal code without four digits are now rejected with INVALID_ARGUMENT.

### Added

- **GLD-035** — README "Running locally with SSO (jwt auth)": go-cloud-k8s-auth redirect
  allowlist and CORS origins, shared JWT settings, and why the SPA must be opened on
  `localhost` rather than `127.0.0.1`; the sign-in panel detects that loopback mismatch and
  offers the right link, and names the origin the auth service must allow.
- **GLD-036** — Relationship rows link to the related case, document or actor page (real
  links: keyboard and middle-click work); detail pages reload when following such a link.
- **GLD-037** — The link dialog searches the subjects of the chosen relationship type's
  target kind (actors, cases, documents; accent-insensitive, by name, title or reference)
  instead of asking for a UUID.

- **GLD-025** — Internal users: every verified caller is recorded in `app_user` (migration
  `0012`) as a USER subject — created on first sight (`USER_REGISTERED`), updated when its name
  or admin flag changes (`USER_PROFILE_UPDATED`) — by a `core.RecordingVerifier` wrapping the
  token verifier (best effort, never fails authentication). New `CoreService.GetCurrentUser`
  (`GET /api/me`) and `BatchGetUsers` (`GET /api/users:batchGet`). The SPA shows names instead
  of numeric ids in governance and audit, and the signed-in user's e-mail, rights and an
  administrator badge. `GOELAND_DEV_USER_ADMIN=true` makes the dev user an administrator.

- **GLD-038** — Actor complements ("contacts") are validated and stored normalized per type,
  in the API and mirrored in the SPA: phones and fax in E.164 (`021 315 22 22` →
  `+41213152222`), bare e-mail addresses, http(s) websites, `Case postale <n>`, IDE with its
  modulo-11 check digit (`CHE-123.456.788`), VAT (IDE + MWST/TVA/IVA), ABACUS debtor digits,
  commercial register identifier or URL; OTHER needs a label. The actor form explains usual
  name versus legal name (RC) and the name complement, calls the list "Complements (phone,
  e-mail, identifiers)" with a per-type placeholder, note field and message, and the detail
  page shows formatted values with tel:/mailto:/web links. Editing an actor now validates the
  form before saving. **Behavior change:** malformed complement values are rejected with
  INVALID_ARGUMENT (no production data yet).

- **GLD-039** — Person minimal identity: `PersonDetails` gains `salutation` (new `Salutation`
  enum: Madame, Monsieur, neutral, unspecified), `last_name` (required when creating or editing
  a person) and `first_name` (migration `0013`, person-only by a CHECK constraint). A person's
  `display_name` may be omitted and is then derived as "<first> <last>"; search also matches
  first and last names (accent-insensitive); actor audit events record the identity. The SPA
  person form has salutation, first and last name, and proposes the usual name from them.
  No birth date, AVS number or civil-registry data is stored.

- **GLD-014** — Actor addresses: `ActorAddress` in `Actor`, `CreateActorRequest.addresses` and
  `UpdateActorRequest.replace_addresses` + `addresses` (migration `0014`: `address` rows linked M:N
  by `actor_address` with a role — head office, branch, correspondence, billing, residence,
  other — and at most one principal; the first becomes principal when none is; a Swiss postal
  code has four digits). Replacing addresses ends the previous links instead of deleting them,
  and is audited. New relationship types `ACTOR_BRANCH_OF_ACTOR` and
  `ACTOR_CONTACT_PERSON_OF_ACTOR`; `GetActor` now returns relationships in both directions. The
  SPA edits addresses (principal star, per-role fields, CH postal code check), shows them as
  cards with a map.geo.admin.ch link, and the actor page can link, end and unlink subjects.

- **GLD-040** — Reference data administration for `goeland:admin` users: `CreateCaseType` /
  `UpdateCaseType`, `CreateRelationshipType` / `UpdateRelationshipType`,
  `CreateOrganizationCategory` / `UpdateOrganizationCategory`, `CreateDocumentType` /
  `UpdateDocumentType` (REST `POST` / `PATCH` on the existing collections) and
  `CoreService.ListReferenceChanges` (`GET /api/reference-changes`). Codes are immutable
  (`^[A-Z][A-Z0-9_]{1,99}$`), relationship kinds and direction too, entries are deactivated
  rather than deleted, and every change is written with its before/after state, operator and
  reason to the append-only `reference_change` log (migration `0015`) in the same transaction.
  The SPA has an Administration page (admins only) with one tab per catalogue and the log.

### Changed

- The SPA opens on the case list (`/`) instead of the document list.
- The audit timeline labels the operator "By" (fr-CH "Par") instead of "Actor", which
  suggested an external ACTOR.

### Fixed

- SPA dev mode: a wrong dev token was accepted by the SPA, which then failed every call;
  the token is now checked with `GET /api/me` before signing in and a rejection is shown.

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
