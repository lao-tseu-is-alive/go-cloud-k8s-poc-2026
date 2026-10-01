# Legacy data import — mapping and runbook

Status: wave 1 in progress (GLD-051, GLD-052), decided with the product owner on 2026-10-01.

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

Out of wave 1: the type-specific tables of 172 case types, the timeline (`affaire_suivi`, ~2.2M),
documents (~2.3M, metadata with an external reference later, never the bytes), things, links
between cases, tasks and circulations, access logs (GLD-033).

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

## Decisions

- Grants given to inactive employees are imported (no effect, history kept).
- "Aucun accès" (a few dozen rows) is not imported: the person may then reach the case through
  its unit; to be revisited before a shared environment.
- Employee identity: the POC user id is the legacy employee id; a shared environment will need the
  identity provider (or the production F5 JWT service) to issue that id.
