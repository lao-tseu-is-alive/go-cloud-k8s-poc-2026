# Goéland POC — Modèle de domaine OOA modernisé pour agent de codage

> ℹ️ **Ce document est la spécification de départ (intention d'origine) — il n'est
> volontairement PAS mis à jour au fil de l'implémentation.** Pour l'état réel du
> projet (ce qui est fait / à faire) et les écarts assumés par rapport à cette
> spec, voir le suivi vivant : [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).

## 0. Objectif du document

Ce document sert de spécification de départ pour un agent de codage chargé de créer un POC moderne inspiré de l'analyse orientée objet historique de Goéland.

Le but n'est pas de recoder Goéland à l'identique. Le but est de reconstruire un noyau propre, simple et durable permettant de gérer des dossiers métier / affaires administratives avec :

- sujets métier stables ;
- spécialisations contrôlées ;
- relations typées entre sujets ;
- suivis chronologiques ;
- documents contextualisés ;
- objets territoriaux ;
- acteurs ;
- droits ;
- non-destruction ;
- traçabilité probante ;
- possibilité future d'intégrer workflow, IA, GED, recherche et cartographie.

Le POC doit rester volontairement petit mais conceptuellement solide.

---

## 1. Contexte métier

Goéland est historiquement un système de gestion d'affaires administratives territoriales. Il ne doit pas être compris comme une simple GED, un SIG ou un outil de reporting.

Le cœur fonctionnel à préserver est le suivant :

```text
Une organisation reçoit une demande,
ouvre une affaire,
relie des documents, des acteurs et des objets métier,
fait intervenir plusieurs personnes ou services,
collecte des suivis, avis ou validations,
gère des délais,
prend une décision,
clôture le dossier,
et conserve une trace probante de ce qui s'est passé.
```

Les anciens grands sujets de Goéland restent pertinents :

```text
AFFAIRE
DOCUMENT
THING / OBJET géoréférencé
ACTEUR
EMPLOYÉ / USER
UNITÉ ORGANISATIONNELLE
```

Le POC doit conserver cet ADN.

---

## 2. Principes d'architecture

### 2.1 Ne pas commencer par un moteur de workflow

Le POC doit commencer par le modèle de domaine :

```text
Sujet métier
+ type
+ spécialisation
+ relation typée
+ suivi
+ document
+ droits
+ audit
```

Un moteur BPMN / CMMN / Temporal pourra être ajouté plus tard. Il ne doit pas être la fondation cognitive du modèle.

### 2.2 Ne pas faire un modèle EAV généralisé

Éviter un design du type :

```text
object
attribute
attribute_value
relation
relation_type
meta_type
```

Ce design paraît séduisant mais devient rapidement difficile à maintenir, sécuriser, requêter et optimiser.

Le bon compromis :

```text
Tables explicites pour les sujets principaux
+ tables de spécialisation si nécessaire
+ relations typées
+ JSONB seulement pour extensions secondaires
+ audit transversal
```

### 2.3 Garder les invariants historiques

Les invariants à préserver :

1. Chaque grand sujet possède une identité unique, stable et pérenne.
2. Chaque sujet peut avoir des spécialisations.
3. Les sujets doivent pouvoir être liés entre eux avec des rôles métier précis.
4. Certains attributs et comportements concernent tous les sujets : création, propriétaire, droits, non-destruction, traçabilité.
5. Les suivis forment le déroulement chronologique d'une affaire.
6. Une validation verrouille un suivi et ses documents liés.

---

## 3. Périmètre du POC

Le POC doit couvrir seulement quatre sujets métier au départ :

```text
CASE       = affaire / dossier métier
DOCUMENT   = pièce documentaire
THING      = objet métier ou territorial
ACTOR      = personne ou organisation externe
```

Les utilisateurs et unités organisationnelles peuvent être simulés dans une première étape, mais le modèle doit prévoir leur intégration.

### 3.1 Cas métier minimal à démontrer

Le POC doit démontrer le scénario suivant :

```text
1. Créer une affaire de type OPC_DEMANDE_PC.
2. Créer ou rattacher une parcelle comme THING.
3. Créer ou rattacher un bâtiment comme THING.
4. Créer un acteur externe de type ORGANIZATION ou PERSON.
5. Lier l'acteur à l'affaire dans le rôle REQUESTER ou MANDATAIRE.
6. Lier l'affaire à la parcelle dans le rôle CONCERNS.
7. Ajouter un document de type PLAN.
8. Lier le document à l'affaire.
9. Lier le document au bâtiment dans le rôle REPRESENTS.
10. Ajouter un suivi à l'affaire.
11. Attacher le document au suivi.
12. Valider le suivi.
13. Vérifier que le suivi validé ne peut plus être modifié.
14. Créer une circulation vers deux services ou utilisateurs.
15. Enregistrer une réponse de circulation sous forme de suivi.
16. Consulter l'audit de l'affaire.
```

---

## 4. Modèle conceptuel cible

### 4.1 Vue d'ensemble

```text
subject_ref
  ├── case_file
  ├── document
  ├── thing
  └── actor

subject_relationship
  ├── CASE -> DOCUMENT
  ├── CASE -> THING
  ├── CASE -> ACTOR
  ├── DOCUMENT -> THING
  ├── THING -> THING
  └── CASE -> CASE

case_file
  ├── case_timeline_entry
  ├── case_circulation
  └── case_task

record_metadata
  └── common governance data for every subject

audit_event
  └── append-only event log
```

### 4.2 Sujet métier générique

Un `subject_ref` représente l'identité canonique d'un sujet métier.

Tous les grands objets du domaine doivent avoir une entrée dans `subject_ref`.

```text
CASE, DOCUMENT, THING, ACTOR, USER, ORG_UNIT
```

Cela permet d'implémenter proprement des relations typées entre des objets de nature différente.

---

## 5. Schéma SQL initial PostgreSQL

Le POC doit cibler PostgreSQL. Si possible, activer PostGIS dès le début pour préparer les objets territoriaux.

### 5.1 Extensions

```sql
create extension if not exists pgcrypto;
create extension if not exists postgis;
```

### 5.2 Types simples

```sql
create table subject_kind (
    code text primary key
);

insert into subject_kind(code) values
    ('CASE'),
    ('DOCUMENT'),
    ('THING'),
    ('ACTOR'),
    ('USER'),
    ('ORG_UNIT');
```

### 5.3 Référence canonique des sujets

```sql
create table subject_ref (
    id uuid primary key default gen_random_uuid(),
    kind text not null references subject_kind(code),
    display_label text not null,
    canonical_url text,
    created_at timestamptz not null default now(),
    unique(id, kind)
);

create index idx_subject_ref_kind on subject_ref(kind);
create index idx_subject_ref_display_label on subject_ref using gin (to_tsvector('simple', display_label));
```

### 5.4 Métadonnées transversales

```sql
create table record_metadata (
    subject_id uuid primary key references subject_ref(id),
    created_at timestamptz not null default now(),
    created_by uuid,
    updated_at timestamptz,
    updated_by uuid,
    deleted_at timestamptz,
    deleted_by uuid,
    owner_user_id uuid,
    owner_org_id uuid,
    confidentiality_level int not null default 0,
    version int not null default 1,
    is_locked boolean not null default false,
    locked_at timestamptz,
    locked_by uuid
);

create index idx_record_metadata_owner_user on record_metadata(owner_user_id);
create index idx_record_metadata_owner_org on record_metadata(owner_org_id);
create index idx_record_metadata_deleted_at on record_metadata(deleted_at);
```

Rules:

- Do not physically delete domain records during normal operations.
- Use `deleted_at` / `deleted_by` for logical deletion.
- Use `is_locked` for immutable records when needed.

### 5.5 Audit append-only

```sql
create table audit_event (
    id uuid primary key default gen_random_uuid(),
    subject_id uuid not null references subject_ref(id),
    event_type text not null,
    actor_user_id uuid,
    occurred_at timestamptz not null default now(),
    before_state jsonb,
    after_state jsonb,
    reason text,
    correlation_id uuid,
    request_id text
);

create index idx_audit_event_subject on audit_event(subject_id, occurred_at desc);
create index idx_audit_event_type on audit_event(event_type);
create index idx_audit_event_correlation on audit_event(correlation_id);
```

Rules:

- Every domain mutation must write an audit event.
- Audit events must be append-only.
- Do not update or delete audit events in application code.

---

## 6. Modèle des sujets principaux

### 6.1 Affaires / Case

```sql
create table case_type (
    id uuid primary key default gen_random_uuid(),
    code text unique not null,
    label text not null,
    description text,
    schema_json jsonb not null default '{}',
    ui_schema_json jsonb not null default '{}',
    is_active boolean not null default true
);

create table case_file (
    id uuid primary key references subject_ref(id),
    case_type_id uuid not null references case_type(id),
    title text not null,
    description text,
    status text not null default 'OPEN',
    opened_at timestamptz not null default now(),
    closed_at timestamptz,
    metadata jsonb not null default '{}'
);

create index idx_case_file_type on case_file(case_type_id);
create index idx_case_file_status on case_file(status);
create index idx_case_file_title on case_file using gin (to_tsvector('simple', title));
```

Expected statuses for the POC:

```text
OPEN
IN_PROGRESS
SUSPENDED
CLOSED
ARCHIVED
```

### 6.2 Documents

```sql
create table document_type (
    id uuid primary key default gen_random_uuid(),
    code text unique not null,
    label text not null,
    description text,
    is_active boolean not null default true
);

create table document (
    id uuid primary key references subject_ref(id),
    document_type_id uuid not null references document_type(id),
    title text not null,
    description text,
    official_date date,
    storage_ref text,
    mime_type text,
    file_size_bytes bigint,
    sha256 char(64),
    version int not null default 1,
    is_final boolean not null default false,
    metadata jsonb not null default '{}'
);

create index idx_document_type on document(document_type_id);
create index idx_document_title on document using gin (to_tsvector('simple', title));
create unique index idx_document_sha256_not_null on document(sha256) where sha256 is not null;
```

### 6.3 Things / objets métier ou territoriaux

```sql
create table thing_type (
    id uuid primary key default gen_random_uuid(),
    code text unique not null,
    label text not null,
    description text,
    specialization_kind text not null default 'GENERIC',
    schema_json jsonb not null default '{}',
    ui_schema_json jsonb not null default '{}',
    is_active boolean not null default true
);

create table thing (
    id uuid primary key references subject_ref(id),
    thing_type_id uuid not null references thing_type(id),
    name text not null,
    description text,
    external_ref text,
    geom geometry,
    metadata jsonb not null default '{}'
);

create index idx_thing_type on thing(thing_type_id);
create index idx_thing_name on thing using gin (to_tsvector('simple', name));
create index idx_thing_geom on thing using gist(geom);
```

Optional specialization tables for the POC:

```sql
create table thing_parcel (
    thing_id uuid primary key references thing(id),
    commune_code text,
    parcel_number text not null,
    surface_m2 numeric,
    unique(commune_code, parcel_number)
);

create table thing_building (
    thing_id uuid primary key references thing(id),
    egid text,
    eca_number text,
    construction_year int,
    building_status text
);
```

### 6.4 Actors

```sql
create table actor (
    id uuid primary key references subject_ref(id),
    actor_kind text not null, -- PERSON / ORGANIZATION
    name text not null,
    external_ref text,
    email text,
    phone text,
    metadata jsonb not null default '{}'
);

create index idx_actor_kind on actor(actor_kind);
create index idx_actor_name on actor using gin (to_tsvector('simple', name));
```

---

## 7. Relations typées entre sujets

### 7.1 Types de relations

```sql
create table relationship_type (
    id uuid primary key default gen_random_uuid(),
    code text unique not null,
    label text not null,
    source_kind text not null references subject_kind(code),
    target_kind text not null references subject_kind(code),
    is_directed boolean not null default true,
    inverse_label text,
    is_active boolean not null default true
);
```

Initial relation types:

```sql
insert into relationship_type(code, label, source_kind, target_kind, inverse_label) values
    ('CASE_HAS_DOCUMENT', 'Affaire contient document', 'CASE', 'DOCUMENT', 'Document appartient à affaire'),
    ('CASE_CONCERNS_THING', 'Affaire concerne objet', 'CASE', 'THING', 'Objet concerné par affaire'),
    ('CASE_HAS_ACTOR_REQUESTER', 'Affaire a demandeur', 'CASE', 'ACTOR', 'Acteur demandeur dans affaire'),
    ('CASE_HAS_ACTOR_MANDATEE', 'Affaire a mandataire', 'CASE', 'ACTOR', 'Acteur mandataire dans affaire'),
    ('DOCUMENT_REPRESENTS_THING', 'Document représente objet', 'DOCUMENT', 'THING', 'Objet représenté par document'),
    ('DOCUMENT_AUTHORED_BY_ACTOR', 'Document a auteur acteur', 'DOCUMENT', 'ACTOR', 'Acteur auteur de document'),
    ('DOCUMENT_SENT_TO_ACTOR', 'Document envoyé à acteur', 'DOCUMENT', 'ACTOR', 'Acteur destinataire de document'),
    ('THING_CONTAINS_THING', 'Objet contient objet', 'THING', 'THING', 'Objet contenu par objet'),
    ('CASE_PARENT_OF_CASE', 'Affaire parente affaire', 'CASE', 'CASE', 'Affaire enfant de affaire');
```

### 7.2 Relations effectives

```sql
create table subject_relationship (
    id uuid primary key default gen_random_uuid(),
    source_subject_id uuid not null references subject_ref(id),
    target_subject_id uuid not null references subject_ref(id),
    relationship_type_id uuid not null references relationship_type(id),
    role_detail text,
    valid_from timestamptz,
    valid_to timestamptz,
    created_at timestamptz not null default now(),
    created_by uuid,
    deleted_at timestamptz,
    deleted_by uuid,
    unique(source_subject_id, target_subject_id, relationship_type_id)
);

create index idx_subject_relationship_source on subject_relationship(source_subject_id);
create index idx_subject_relationship_target on subject_relationship(target_subject_id);
create index idx_subject_relationship_type on subject_relationship(relationship_type_id);
```

Application rule:

Before creating a relationship, the service must check that:

- source subject exists ;
- target subject exists ;
- relationship type exists ;
- source kind matches `relationship_type.source_kind` ;
- target kind matches `relationship_type.target_kind` ;
- current user has permission to create this relation ;
- relation does not already exist as an active relation.

---

## 8. Suivis / timeline d'affaire

### 8.1 Timeline entry

```sql
create table case_timeline_entry (
    id uuid primary key default gen_random_uuid(),
    case_id uuid not null references case_file(id),
    entry_type text not null default 'COMMENT',
    body text not null,
    visibility text not null default 'CASE_PARTICIPANTS',
    created_at timestamptz not null default now(),
    created_by uuid,
    updated_at timestamptz,
    updated_by uuid,
    validated_at timestamptz,
    validated_by uuid,
    locked_at timestamptz,
    locked_by uuid
);

create index idx_case_timeline_case on case_timeline_entry(case_id, created_at desc);
create index idx_case_timeline_type on case_timeline_entry(entry_type);
create index idx_case_timeline_body on case_timeline_entry using gin (to_tsvector('simple', body));
```

Expected entry types:

```text
COMMENT
OPINION
DECISION
REQUEST
RESPONSE
VALIDATION
SYSTEM
```

Rules:

- Timeline entries are ordered chronologically.
- A validated or locked entry cannot be modified.
- A timeline entry can be linked to one or more documents.
- Validation must create an audit event.

### 8.2 Documents linked to timeline entries

```sql
create table timeline_document_link (
    timeline_entry_id uuid not null references case_timeline_entry(id),
    document_id uuid not null references document(id),
    created_at timestamptz not null default now(),
    created_by uuid,
    primary key (timeline_entry_id, document_id)
);
```

---

## 9. Circulation simple

### 9.1 Circulation

```sql
create table case_circulation (
    id uuid primary key default gen_random_uuid(),
    case_id uuid not null references case_file(id),
    title text not null,
    message text,
    due_at timestamptz,
    status text not null default 'OPEN',
    created_at timestamptz not null default now(),
    created_by uuid,
    completed_at timestamptz,
    expired_at timestamptz
);

create index idx_case_circulation_case on case_circulation(case_id);
create index idx_case_circulation_status on case_circulation(status);
create index idx_case_circulation_due on case_circulation(due_at);
```

Expected circulation statuses:

```text
OPEN
COMPLETED
EXPIRED
CANCELLED
```

### 9.2 Recipients

```sql
create table case_circulation_recipient (
    id uuid primary key default gen_random_uuid(),
    circulation_id uuid not null references case_circulation(id),
    recipient_kind text not null, -- USER / ORG_UNIT
    recipient_id uuid not null,
    response_status text,
    response_text text,
    response_timeline_entry_id uuid references case_timeline_entry(id),
    responded_at timestamptz
);

create index idx_circulation_recipient_circulation on case_circulation_recipient(circulation_id);
create index idx_circulation_recipient_recipient on case_circulation_recipient(recipient_kind, recipient_id);
```

Expected response statuses:

```text
FAVORABLE
UNFAVORABLE
COMMENT
NOT_CONCERNED
NEED_MORE_INFO
```

Rules:

- A circulation is completed when all recipients have responded.
- A response should create or link a timeline entry.
- If due date is exceeded and not all recipients responded, status becomes EXPIRED.
- Expiration must create an audit event.

---

## 10. Droits et confidentialité

For the POC, implement simple permission levels:

```text
NONE
READ
CONTRIBUTE      -- add timeline entries and documents
MANAGE          -- edit metadata, tasks, participants, relations
FULL_CONTROL
```

### 10.1 Access grants

```sql
create table access_grant (
    id uuid primary key default gen_random_uuid(),
    subject_id uuid not null references subject_ref(id),
    grantee_kind text not null, -- USER / ORG_UNIT / GROUP
    grantee_id uuid not null,
    permission text not null,
    created_at timestamptz not null default now(),
    created_by uuid,
    valid_from timestamptz,
    valid_to timestamptz,
    unique(subject_id, grantee_kind, grantee_id, permission)
);

create index idx_access_grant_subject on access_grant(subject_id);
create index idx_access_grant_grantee on access_grant(grantee_kind, grantee_id);
```

Rules:

- Every read/write operation must check access.
- Confidentiality level must be considered in access checks.
- Deny by default if the subject is confidential and no explicit grant exists.
- The POC can start with application-level checks before integrating Casbin/OpenFGA.

---

## 11. Go package structure

Use a modular monolith first. Do not split into microservices at this stage.

Recommended structure:

```text
/internal/domain/subject
/internal/domain/casefile
/internal/domain/document
/internal/domain/thing
/internal/domain/actor
/internal/domain/relationship
/internal/domain/timeline
/internal/domain/circulation
/internal/domain/security
/internal/domain/audit

/internal/app/commands
/internal/app/queries

/internal/infra/postgres
/internal/infra/http
/internal/infra/auth

/cmd/goeland-poc-server
/migrations
```

### 11.1 Domain types

```go
type SubjectKind string

const (
    SubjectCase     SubjectKind = "CASE"
    SubjectDocument SubjectKind = "DOCUMENT"
    SubjectThing    SubjectKind = "THING"
    SubjectActor    SubjectKind = "ACTOR"
    SubjectUser     SubjectKind = "USER"
    SubjectOrgUnit  SubjectKind = "ORG_UNIT"
)

type SubjectID string

type SubjectRef struct {
    ID    SubjectID
    Kind  SubjectKind
    Label string
}
```

### 11.2 Relationship model

```go
type RelationshipType struct {
    Code       string
    Label      string
    SourceKind SubjectKind
    TargetKind SubjectKind
}

type SubjectRelationship struct {
    ID                   string
    Source               SubjectRef
    Target               SubjectRef
    RelationshipTypeCode string
    RoleDetail           string
    CreatedBy            string
    CreatedAt            time.Time
}
```

### 11.3 Link command

```go
type LinkSubjectsCommand struct {
    SourceSubjectID string
    TargetSubjectID string
    RelationshipTypeCode string
    RoleDetail string
    ActorUserID string
}
```

The command handler must:

1. Load source subject.
2. Load target subject.
3. Load relationship type.
4. Validate source/target kind compatibility.
5. Check permission.
6. Insert relationship.
7. Insert audit event.
8. Return created relationship.

---

## 12. Application services to implement

### 12.1 SubjectService

Responsibilities:

- create subject reference ;
- get subject reference ;
- update display label ;
- logical delete subject ;
- create record metadata.

### 12.2 CaseService

Responsibilities:

- create case ;
- update case metadata ;
- close case ;
- suspend/reactivate case ;
- list case timeline ;
- list linked subjects ;
- write audit events.

### 12.3 DocumentService

Responsibilities:

- create document metadata ;
- register file reference ;
- compute/store SHA256 if file available ;
- link document to case ;
- link document to timeline entry ;
- mark document final.

### 12.4 ThingService

Responsibilities:

- create generic thing ;
- create parcel thing ;
- create building thing ;
- update geometry if present ;
- link thing to case or document.

### 12.5 ActorService

Responsibilities:

- create person actor ;
- create organization actor ;
- link actor to case ;
- link actor to document with role.

### 12.6 RelationshipService

Responsibilities:

- create typed relation ;
- list outgoing relations ;
- list incoming relations ;
- logical delete relation ;
- validate type compatibility.

### 12.7 TimelineService

Responsibilities:

- add timeline entry ;
- update timeline entry if not locked ;
- validate timeline entry ;
- lock timeline entry ;
- attach document ;
- prevent modification after validation.

### 12.8 CirculationService

Responsibilities:

- create circulation ;
- add recipients ;
- respond to circulation ;
- create response timeline entry ;
- complete circulation when all recipients responded ;
- mark expired circulations.

### 12.9 AuditService

Responsibilities:

- append audit event ;
- list audit events by subject ;
- list audit events by correlation id.

### 12.10 SecurityService

Responsibilities:

- check permission ;
- grant permission ;
- revoke permission ;
- apply confidentiality rules ;
- deny by default.

---

## 13. API endpoints for POC

REST is acceptable for the first POC. connect-rpc can be introduced later.

### 13.1 Cases

```text
POST   /api/cases
GET    /api/cases/{id}
PATCH  /api/cases/{id}
POST   /api/cases/{id}/close
GET    /api/cases/{id}/timeline
GET    /api/cases/{id}/relationships
GET    /api/cases/{id}/audit
```

### 13.2 Timeline

```text
POST   /api/cases/{id}/timeline
PATCH  /api/timeline/{entryId}
POST   /api/timeline/{entryId}/validate
POST   /api/timeline/{entryId}/documents/{documentId}
```

### 13.3 Documents

```text
POST   /api/documents
GET    /api/documents/{id}
POST   /api/cases/{caseId}/documents/{documentId}
```

### 13.4 Things

```text
POST   /api/things
GET    /api/things/{id}
POST   /api/things/parcels
POST   /api/things/buildings
```

### 13.5 Actors

```text
POST   /api/actors
GET    /api/actors/{id}
```

### 13.6 Relationships

```text
POST   /api/relationships
GET    /api/subjects/{subjectId}/relationships/outgoing
GET    /api/subjects/{subjectId}/relationships/incoming
DELETE /api/relationships/{relationshipId}
```

### 13.7 Circulations

```text
POST   /api/cases/{caseId}/circulations
GET    /api/circulations/{id}
POST   /api/circulations/{id}/recipients/{recipientId}/response
POST   /api/circulations/{id}/expire
```

---

## 14. Seed data

The POC should include seed data:

### 14.1 Case types

```text
OPC_DEMANDE_PC = Demande de permis de construire
GENERIC_REQUEST = Demande générique
```

### 14.2 Document types

```text
INCOMING_LETTER
OUTGOING_LETTER
PLAN
PHOTO
FORM
REPORT
```

### 14.3 Thing types

```text
PARCEL
BUILDING
STREET
ADDRESS
GENERIC_OBJECT
```

### 14.4 Relationship types

Use the relation types defined above.

### 14.5 Test users

```text
user_manager
user_opc
user_mobility
user_heritage
```

### 14.6 Test org units

```text
OPC
MOBILITY
HERITAGE
CADASTRE
```

---

## 15. Implementation order for the coding agent

### Phase 1 — Database skeleton

1. Create migrations for `subject_kind`, `subject_ref`, `record_metadata`, `audit_event`.
2. Create migrations for `case_file`, `document`, `thing`, `actor`.
3. Create migrations for `relationship_type`, `subject_relationship`.
4. Create migrations for `case_timeline_entry`, `timeline_document_link`.
5. Create migrations for `case_circulation`, `case_circulation_recipient`.
6. Create seed data.

Acceptance criteria:

- `go test ./...` passes.
- migrations run on an empty PostgreSQL database.
- seed data inserts without error.

### Phase 2 — Domain services

1. Implement SubjectService.
2. Implement AuditService.
3. Implement RelationshipService.
4. Implement CaseService.
5. Implement TimelineService.
6. Implement DocumentService.
7. Implement ThingService.
8. Implement ActorService.
9. Implement CirculationService.

Acceptance criteria:

- Unit tests cover happy paths and invalid relation types.
- Every mutation writes an audit event.
- Validated timeline entries cannot be modified.

### Phase 3 — HTTP API

1. Implement REST handlers.
2. Add request validation.
3. Add basic error responses.
4. Add integration tests for the full scenario.

Acceptance criteria:

- Full scenario can be executed through HTTP calls.
- Invalid subject relationship is rejected.
- Permission denied returns 403.
- Not found returns 404.
- Validation errors return 400.

### Phase 4 — Minimal UI or API demo

Optional for the first coding pass.

If UI is requested:

- create a case detail page ;
- show case metadata ;
- show linked actors, documents and things ;
- show chronological timeline ;
- allow adding a timeline entry ;
- allow validating a timeline entry ;
- show audit events.

---

## 16. Tests to create

### 16.1 Relationship tests

- CASE can be linked to THING with CASE_CONCERNS_THING.
- CASE cannot be linked to ACTOR with CASE_CONCERNS_THING.
- DOCUMENT can be linked to THING with DOCUMENT_REPRESENTS_THING.
- Duplicate active relationship is rejected.
- Logical deletion allows later recreation if explicitly desired.

### 16.2 Timeline tests

- Timeline entry can be created.
- Document can be attached to timeline entry.
- Timeline entry can be validated.
- Validated timeline entry cannot be updated.
- Validation creates audit event.

### 16.3 Circulation tests

- Circulation can be created with two recipients.
- First recipient response creates a timeline entry.
- Circulation remains OPEN after partial response.
- Second recipient response completes circulation.
- Expired circulation creates audit event.

### 16.4 Audit tests

- Creating a case writes audit event.
- Creating a relationship writes audit event.
- Validating a timeline entry writes audit event.
- Closing a case writes audit event.

### 16.5 Security tests

- User without access cannot read confidential case.
- User with READ can read but not contribute.
- User with CONTRIBUTE can add timeline entry.
- User with MANAGE can add relations.
- User with FULL_CONTROL can close case.

---

## 17. Design rules for the agent

The coding agent must respect these rules:

1. Keep the domain model explicit.
2. Do not introduce a generic EAV model.
3. Use JSONB only for secondary extension data.
4. Put critical fields in real typed columns.
5. Do not physically delete domain records.
6. Every mutation must create an audit event.
7. Every relationship must be typed and validated.
8. Timeline is the primary representation of case history.
9. Workflow engine integration is out of scope for the first POC.
10. Keep the API boring, explicit and testable.
11. Prefer clear domain names over technical abstractions.
12. Optimize for maintainability and explainability, not cleverness.

---

## 18. Non-goals for this POC

Do not implement yet:

- BPMN engine ;
- Flowable integration ;
- Temporal integration ;
- complex dynamic form builder ;
- full GED ;
- advanced cartographic viewer ;
- complete RBAC/ReBAC engine ;
- AI assistant ;
- advanced reporting ;
- document preview ;
- archive/legal retention workflow ;
- multi-tenant SaaS.

These are future extensions.

---

## 19. Future extension points

### 19.1 Workflow / Case engine

Later, Flowable or another BPMN/CMMN engine can drive:

- complex circulations ;
- deadlines ;
- formal human tasks ;
- process versioning ;
- service orchestration.

But `case_file`, `case_timeline_entry`, `document`, `thing`, `actor` and `subject_relationship` must remain the domain source of truth.

### 19.2 AI assistant

Later, an AI assistant can help with:

- document classification ;
- suggested timeline summaries ;
- draft responses ;
- completeness checks ;
- risk detection ;
- extraction of actors, parcels, buildings and dates.

AI outputs must always be stored as proposals unless validated by a human.

### 19.3 Search

Later, Meilisearch or PostgreSQL full-text search can index:

- case titles ;
- case descriptions ;
- timeline entries ;
- document metadata ;
- actor names ;
- thing names ;
- external references.

### 19.4 Authorization engine

Later, Casbin or OpenFGA can replace the simple permission service.

### 19.5 Geospatial

Later, PostGIS and OpenLayers can support:

- geometry editing ;
- map search ;
- case localization ;
- thing visualization ;
- spatial queries.

---

## 20. Core architectural thesis

The POC must prove this thesis:

```text
The administrative world is not primarily a set of forms,
a document library,
a workflow diagram,
or a BI dataset.

It is a graph of durable business subjects,
linked by typed roles,
animated by chronological case history,
protected by rights,
and made trustworthy by auditability.
```

This is the conceptual heart of Goéland and must remain the heart of the new design.

