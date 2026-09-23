# Goéland POC Roadmap

Tracked version: **v0.4.3**.

This document is the source of truth for implementation order, scope and task
state. How the built system relates to the spec lives in
[`requirements/IMPLEMENTATION_STATUS.md`](../requirements/IMPLEMENTATION_STATUS.md);
what each release delivered lives in [`CHANGELOG.md`](../CHANGELOG.md). The
rules binding the three are in [DOCUMENTATION.md](DOCUMENTATION.md#roadmap-changelog-and-version-traceability).

## Conventions

- `[ ]` to do, `[~]` in progress, `[x]` done and verified.
- Every task has a unique, stable `GLD-NNN` ID; IDs are never reused or renumbered.
- A task is done only when its tests and acceptance criteria pass. A `[x]` task
  MUST be named in a dated changelog section, and a task named in a dated
  changelog section MUST be `[x]` here (`make release-traceability-check`).
- A change of scope or order is recorded here before it is implemented.
- Work delivered before this roadmap existed (v0.1.0 to v0.4.0: core, document,
  actor, embedded SPA) is recorded in the changelog without task IDs.

## Next action

Review the proposed v2 requirements with the product owner (GLD-002) before
starting the Case slice: the review may reorder the phases below.

## Cross-cutting quality

- [x] **GLD-001 — Documentation quality contract**: adopt the
  [normative contract](DOCUMENTATION.md) in five slices (checker and gates,
  atlas, GoDoc, Protobuf comments, roadmap and guarded release) so drift fails
  `make release-check`, CI and the release workflows.
- [ ] **GLD-002 — Requirements v2 review**: review
  `requirements/goeland_poc_domain_model_agent_v2_from_ChatGPT_20260923.md`
  with the product owner, decide what becomes normative and reconcile this
  roadmap and `IMPLEMENTATION_STATUS.md`.
- [ ] **GLD-003 — Duplicate digest error**: a second document with an existing
  `sha256` violates the unique index and surfaces as `INTERNAL`; map it to
  `ALREADY_EXISTS` and update the proto comment.
- [ ] **GLD-004 — Cross-kind actor update error**: non-empty fields of the
  other kind in `UpdateActor` violate a CHECK constraint and surface as
  `INTERNAL`; reject them as `INVALID_ARGUMENT` in the service.
- [ ] **GLD-005 — Partial actor rename**: `UpdateActorRequest.display_name`
  is required by validation, which makes the adapter's "empty means unchanged"
  branch unreachable; make it truly optional or document it as required.
- [ ] **GLD-006 — Document incoming relationships**: `GetDocument` returns only
  outgoing edges, so the `CASE_HAS_DOCUMENT` link to a case is not shown with
  the document; return both directions (proto, service, SPA panel).
- [ ] **GLD-007 — Non-destructive contact replacement**: `replace_contacts`
  physically deletes `actor_contact` rows and the audit event keeps only the
  display name; preserve the replaced contacts (soft delete or before/after
  state) as spec §17 requires.
- [ ] **GLD-008 — Generated-code reproducibility gate**: add a
  `generated-check` (regenerate, diff `gen/` and `api/openapi/`) to
  `make release-check`; needs network access for the remote OpenAPI plugin.
- [ ] **GLD-009 — Frontend tests**: add unit/component tests to the SPA; the
  gate is currently type-check, lint and build only.
- [ ] **GLD-010 — Observability**: Prometheus metrics and OpenTelemetry traces.

## Phase 1 — Case spine (spec §6.1, §8, §9)

- [ ] **GLD-011 — Case slice**: `case_type` + `case_file` + `CaseService`
  (proto-first, mirroring Document/Actor), the first `CASE`-source relationship
  types including expanded `CASE_HAS_ACTOR_*` roles, a Vue panel and a
  `pkg/integration` lifecycle test. Unlocks the demo scenario (spec §3.1).
- [ ] **GLD-012 — Timeline**: `case_timeline_entry` + `timeline_document_link`
  with validation and immutability (spec §17.8). Depends on GLD-011.
- [ ] **GLD-013 — Circulation**: `case_circulation` +
  `case_circulation_recipient`. Depends on GLD-011 and GLD-012.

Exit criteria: the spec §3.1 scenario runs end to end, covered by an
integration test.

## Phase 2 — Actor follow-ups (spec §6.4)

- [ ] **GLD-014 — Actor addresses**: `address` + M:N `actor_address` with
  `is_principal` (production `acteur_adresse` + `lien_acteur_adresse`), in the
  API and the Actor UI.
- [ ] **GLD-015 — Actor role vocabulary**: seed the remaining production roles
  (`dico_acteur_role`) into `relationship_type` as their target domains land;
  incremental with GLD-011 and GLD-016, not a standalone slice.

A richer CH-register person detail is out of scope unless a real need appears,
and then only through opaque references (no PII).

## Phase 3 — Thing (spec §6.3)

- [ ] **GLD-016 — Thing slice**: `thing` + `thing_type` with parcel and
  building specializations and PostGIS geometry (extension enabled since
  migration 0001); enables `CASE_CONCERNS_THING` and the land-rights actor
  roles (propriétaire, locataire, superficiaire, fermier, servitude).

## Phase 4 — Security (spec §10)

- [ ] **GLD-017 — Real authorization**: `access_grant`, per-subject grants and
  deny-by-default confidentiality (or Casbin/OpenFGA). Today every
  authenticated caller gets both `goeland:read` and `goeland:write`; this is the
  largest POC-to-production gap.

## Phase 5 — Real-data import (MSSQL replica → POC)

Loading the POC is a transform from the legacy-shape replica, not a copy.
Subject IDs are deterministic (`UUIDv5(namespace, "<kind>:<legacyId>")`) so
reruns are idempotent and relationships can be rebuilt later. Profiling stays
aggregates-only.

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

- [ ] **GLD-020 — Object storage**: replace the node-local blob store with
  MinIO/S3 (no proto change).
- [ ] **GLD-021 — Probative integrity verification**: stream the stored bytes,
  recompute SHA-256, set `sha256_verified_at` and write an audited
  verification event under the write scope.
