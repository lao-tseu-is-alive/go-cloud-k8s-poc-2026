# Legacy data import — mapping and runbook

Status: waves 1 and 2 (GLD-051, GLD-052, GLD-054), decided with the product owner on 2026-10-01.

This document describes how the POC is loaded from the legacy Goéland database (a read-only
PostgreSQL replica of the production MSSQL database): the rules, the decisions and how to run the
import again. It holds rules and orders of magnitude only — never a value of a real row.

## Principles

- **A transform, not a copy.** Each legacy row is mapped onto the POC model (subjects, typed
  relationships, grants); what has no place in the model is left out and counted.
- **One shot at an instant T, repeatable.** The target is a brand-new database rebuilt from scratch
  on every run; the instant T is the date of the replica. Running again on a refreshed replica
  gives the new state.
- **Deterministic identities.** A subject imported from legacy row `<id>` of kind `<KIND>` gets
  `UUIDv5(importNamespace, "<KIND>:<id>")`, so two runs give the same ids and relationships,
  grants and later waves can be computed without lookups.
- **Provenance** (GLD-027): every imported subject has a `subject_provenance` row (source system,
  table and id, import batch); the legacy ids are provenance, never the POC ids.
- **One import marker.** A run is one `import_batch` row (source, snapshot date, counts, rejects),
  not one audit event per row. The legacy creation dates and creators are kept on the subjects.
- **Real data stays local.** The import code and this document are versioned; the data, the
  databases and any listing of rows are not. The import prints counts only.

## Running an import

Prerequisites: the replica (`goeland`, read-only role) and a local PostgreSQL superuser to
recreate the target database.

```bash
# 1. Refresh the replica first if the instant T must move (outside this repository).
# 2. Rebuild the target from scratch and import (dry run without -apply):
GOELAND_IMPORT_SOURCE_URL='postgres://goeland_read:<password>@localhost/goeland' \
GOELAND_IMPORT_ADMIN_URL='postgres://postgres@127.0.0.1:5432/postgres' \
    scripts/import_rebuild.sh goeland_import
# 3. Run the POC on the imported data: same .env, another database
DB_NAME=goeland_import make run          # DATABASE_URL, when set, must name it too
```

`scripts/import_rebuild.sh` drops and recreates the target database (owned by the `.env`
application role, extensions enabled as the superuser), migrates it and runs
`go run ./cmd/goeland-import -apply`; `--dry-run` rolls the whole import back and only prints the
counts. The target name must start with `goeland_import` (the development database and the replica
can never be dropped by mistake), and the import refuses a target that already holds imported
data. The import ends with `ANALYZE`: right after a bulk load the planner has no statistics and the
first requests get very poor plans (a 10 s timeout was seen before it was added).

A run takes about 3.5 minutes on a workstation (peak memory about 1.5 GB). It prints, per stage,
the rows read, written, left out by reason and adjusted (renamed, defaulted, ended, ...); the same
counts are stored on the `import_batch` row.

To look at the POC as a given employee in dev mode, set `GOELAND_DEV_USER_ID` to that employee's
legacy id: the imported grants then apply exactly as in production.

## Mapping — wave 1

| Legacy | POC | Rules |
|---|---|---|
| `employe` (~10k, ~2.8k active) | `app_user` + USER subject | `user_id` = legacy `IdEmploye`; display name, e-mail, active; never birth date, private address or phones. Membership `USER_MEMBER_OF_ORG_UNIT` to the direct unit (`employe__org_unit`, level 0) |
| `org_unit` (~740) | `org_unit` | the GLD-041 rules (type, parent, abbreviation, dissolution) |
| `securite_groupe` (~190) + members | GROUP + `USER_MEMBER_OF_GROUP` | nested groups are flattened (a member of an inner group is a member of the outer one) |
| `type_affaire` (~325) | `case_type` | code `LEG_<IdTypeAffaire>`, label = name, active flag; default confidentiality 2 when more than 95% of its cases are confidential |
| `affaire` (~512k) | `case_file` + CASE subject | status: terminated → CLOSED, suspended → SUSPENDED, otherwise OPEN; confidential → level 2; created date and creator kept |
| `affaire_droit_emp_or_ou` (~2.4M) | `access_grant` | 1 Contrôle total → FULL_CONTROL, 2 Edition → MANAGE, 3 Ajout suivis/documents → CONTRIBUTE, 4 Consultation → READ; `E`/`e` → USER, `O` → ORG_UNIT, `G` → GROUP; 5 Aucun accès is not imported (no deny level, counted) |
| `acteur` (~85k) + `act_physique` / `act_moral` | `actor` | person: salutation, last and first name only; organization: legal name, category (by label), complement |
| `acteur_complement` (~165k) | `actor_contact` | typed by the legacy complement type; values failing the POC rules are counted and left out |
| `acteur_adresse_corresp_denormalise` | `address` + `actor_address` | one principal CORRESPONDENCE address per actor when street, postal code and locality are known |
| `acteur_role` on `Affaire` (~270k, 34 roles) | `CASE_HAS_ACTOR_<ROLE>` | existing types reused (requester, owner, ...), the others created |
| `affaire_employe` (~840k, 15 roles) | `CASE_HAS_USER_<ROLE>` | participation roles, distinct from grants |
| `affaire_org_unit` (~1.7M, 13 roles) | `CASE_HAS_ORG_UNIT_<ROLE>` | existing LEADER, MANAGER, PARTICIPANT reused |

## Mapping — wave 2

| Legacy | POC | Rules |
|---|---|---|
| `type_thing` (~110) | `thing_type` | parcel, building, tree and advertisement to the seeded types, the others generic `LEG_<IdTypeThing>` |
| `thing` (~180k) + `thing_position` | `thing` | the legacy keeps an extent in LV95 centimetres: an exact point, or the centre of a wider extent (`metadata.legacy_location`); parcels get no geometry (their shape is unknown) |
| `parcelle` / `thi_building_egid` | `thing_parcel` / `thing_building` | OFS commune, parcel number, EGRID, surface; EGID; a repeated EGRID or EGID keeps its first owner |
| `acteur_role` on `Thing` | `THING_HAS_ACTOR_<ROLE>` | owner reused, the others created |
| `lien_thing_affaire` | `CASE_CONCERNS_THING` | |
| `lien_affaire_affaire` | `CASE_PARENT_OF_CASE`, `CASE_RELATED_TO_CASE` | "Parent": case 1 is the parent of case 2 (the older one in 86% of the rows), "Enfant" is its mirror (not imported); "Lien" is stored both ways (kept once), "Lien unidirectionnel" keeps its direction |
| `document` (~2.3M) | `document` + one `document_version` + `content_blob` | one generic type "Document Goéland" (the legacy type is a file format, which gives the media type); the current content is known by its SHA-256 only (no storage reference: the bytes stay in the legacy store); definitive → final; earlier versions not imported |
| `doclevelconfidential` | confidentiality and `access_grant` | see the decision below (levels 0-6 of the legacy UI) |
| `lien_affaire_document` / `lien_thing_document` | `CASE_HAS_DOCUMENT` / `DOCUMENT_REPRESENTS_THING` | |
| `acteur_role` on `Document` | `DOCUMENT_AUTHORED_BY_ACTOR`, `DOCUMENT_HAS_ACTOR_<ROLE>` | |
| `affaire_suivi` (~2.2M) | `case_timeline_entry` (COMMENT, visible to all involved) | validated → VALIDATED; locked (`affaire_suivi_verrou`) or of a closed case → LOCKED; otherwise a draft |
| `lien_affaire_suivi_document` (~1.5M) | `timeline_document_link` | the single imported version is pinned |

Not imported: the type-specific tables of 172 case types and of the things, the bytes of the
documents and their earlier versions, tasks and circulations (none in the legacy form), access
logs (GLD-033).

## First run (2026-10-01, replica of 2026-05-18)

Everything is loaded except, by design or because the POC rules refuse it:

- grants: the 63 "Aucun accès" rows, a handful pointing at unknown units or employees;
- group memberships: inactive employees, archived groups, and the 90 nested group links (not
  flattened in wave 1);
- contacts: about 3% fail the POC rules (malformed phones, IDE, VAT, e-mails, postal boxes) and
  the bank references (CCP, IBAN) are not imported;
- addresses: about 4% of the correspondence addresses lack a street, postal code or locality;
- roles: a few hundred participations of cases or actors that do not exist.

Adjustments: ~24k persons have no separate last name (their display name is used), 17 case types
are confidential by default, 6 inactive units stay live because they have live sub-units, and the
~346k open participations of dissolved units are imported as ended at the closing of their case
(or at the import for an open case). 59 relationship types are created from the legacy roles.

Observed at production volume and fixed in GLD-053: the unfiltered case search took ~2.7 s (an
exact total over ~490k readable cases) and now ~0.04 s with a total capped at 10 000; a text search
takes ~0.23 s, an actor search ~0.04 s; relationship panels page through up to ~125k incoming
relationships of a unit (~0.07 s a page) instead of stopping at 200.

## Wave 2 run (2026-10-01)

Loaded in about 28 minutes in all (peak memory about 1.5 GB): ~180k things (~47k located at the
centre of their extent, parcels without geometry; a few repeated EGID/EGRID left empty), ~94k
actor roles on things, ~607k case–thing links, ~5k parent and ~135k related case links, ~2.3M
documents (one version each, ~30k with earlier versions not imported), the readers of the
confidential documents (see Decisions), ~2M case–document and ~1.9M
thing–document links, ~715k document–actor roles, ~2.2M follow-ups (~125k validated, ~1.75M
locked, ~326k drafts of open cases) and ~1.5M cited documents.

At this volume (GLD-053): a case's documents ~20 ms, a thing's ~90 ms, a case timeline ~10 ms;
the unscoped document search counts within the 20 000 newest documents (a lower bound); with the
confirmed levels (~20% of the documents confidential) every document search takes 30–120 ms.
Measured with the first, provisional rule (81% confidential), a user without grants waited ~1.6 s:
a table where most rows are unreadable remains the worst case of the read filter.

## Decisions

- **Document confidentiality (confirmed 2026-10-02 from the legacy UI, `selNivConf`)**: the
  level decides, not the `docisconfidential` flag (kept in the metadata). 0 "Document public" →
  confidentiality 0; 1 "Interne Ville de Lausanne" (~78% of the documents) → 1, read by every
  employee; 2 "Limité aux employés de la direction", 3 "… du service", 4 "… de l'unité
  organisationnelle" → confidential (2) with READ to the poster's direction, service (its nearest
  ancestor of that type) or direct unit, covering their sub-units; 5 "Limité au(x) groupe(s) de
  sécurité autorisé(s)" and 6 "Limité aux employés autorisés" → confidential with READ to the
  access lists only. The poster gets FULL_CONTROL on a confidential document. The poster's unit
  is its current one (the legacy does not keep the unit at the time of posting). Access to a case
  does not open its confidential documents (no live inheritance, as in the legacy levels).

- Grants given to inactive employees are imported (no effect, history kept).
- "Aucun accès" (a few dozen rows) is not imported: the person may then reach the case through
  its unit; to be revisited before a shared environment.
- Employee identity: the POC user id is the legacy employee id; a shared environment will need the
  identity provider (or the production F5 JWT service) to issue that id.
