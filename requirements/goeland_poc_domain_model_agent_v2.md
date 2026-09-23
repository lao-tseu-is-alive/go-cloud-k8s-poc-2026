# Goéland POC — Modèle de domaine et trajectoire d'implémentation v2
## Spécification de référence pour agent de codage

**Statut :** référence active du POC  
**Version :** 2.0 révisée  
**Projet existant à faire évoluer :** `lao-tseu-is-alive/go-cloud-k8s-poc-2026`  
**Principe directeur :** faire évoluer l'existant sans casser les choix déjà sains ou supérieurs à la spécification initiale.

---

# 0. Mission de l'agent

Faire évoluer le POC existant vers un noyau moderne et durable de **case management administratif**, inspiré des invariants qui ont permis à Goéland de rester extensible pendant plus de vingt ans.

Le but n'est pas :

- de réécrire le repo depuis zéro ;
- de reproduire l'application historique écran par écran ;
- de construire immédiatement un moteur BPMN complet ;
- de transformer Goéland en GED, SIG ou outil BI ;
- de refactorer uniquement pour faire correspondre le code à une arborescence théorique.

Le but est de **conserver les bonnes fondations déjà implémentées**, puis de compléter et ajuster progressivement le domaine.

Le POC doit démontrer qu'un petit noyau de concepts peut couvrir durablement :

- affaires / dossiers métier ;
- documents ;
- versions documentaires ;
- contenus binaires dédupliqués ;
- objets métier et territoriaux ;
- acteurs externes ;
- relations typées entre sujets ;
- spécialisations métier ;
- suivis chronologiques ;
- tâches ;
- circulations ;
- décisions et validations ;
- droits ;
- non-destruction ;
- traçabilité probante ;
- conservation ;
- migration/provenance ;
- API et événements ;
- automatisation future, y compris agents IA avec validation humaine ;
- future exposition MCP.

---

# 1. Baseline : le repo existant est la fondation

Le projet actuel contient déjà plusieurs choix structurants conformes ou supérieurs à cette V2.

**Ne pas les supprimer ni les réécrire sans raison démontrable.**

Baseline actuelle à préserver :

```text
Proto-first API
ConnectRPC
gRPC / gRPC-Web
Vanguard REST transcoding
google.api.http / OpenAPI
protovalidate
pgx / SQL explicite
PostgreSQL / PostGIS
dbmate-compatible migrations
bundleable pkg/<domain> modules
embedded Vue 3 + Vuetify 4 UI
transactional audit
typed relationships
non-destructive lifecycle
authenticated operator attribution
request/correlation identifiers
integration tests PostgreSQL
CI / Trivy / reproducible container build
```

Le repo actuel a déjà mis en œuvre :

```text
subject_kind
subject_ref
record_metadata
audit_event
relationship_type
subject_relationship

document_type
document

actor
actor_contact
organization_category
```

et les domaines :

```text
Core
Document
Actor
```

Ces composants constituent la **baseline**, pas du code provisoire à jeter.

---

# 2. Principe "do not regress"

Lorsqu'une capacité déjà implémentée est meilleure que la première spécification, **la V2 ne doit pas provoquer de régression**.

Exemples à préserver :

- API proto-first plutôt que REST-first ;
- ConnectRPC + Vanguard ;
- protovalidate ;
- modules `pkg/<domain>` composables ;
- interface Web Vue/Vuetify embarquée ;
- recherche accent-insensitive déjà disponible sur Document/Actor ;
- audit écrit dans la même transaction que la mutation ;
- soft-delete/non-destruction ;
- opérateur dérivé de l'identité authentifiée ;
- `X-Request-ID` propagé dans l'audit ;
- tests d'intégration réels PostgreSQL ;
- contrôle de mutabilité transactionnel ;
- distinction ACTOR / USER ;
- rôles Actor portés par les relations et non par l'entité.

La règle est :

> **Adapter la spécification à une architecture réelle saine, pas dégrader l'architecture saine pour imiter un schéma théorique.**

---

# 3. Vision du produit

Goéland doit être considéré comme un système de **case management administratif territorial**.

Le cœur métier est :

```text
Une organisation reçoit une demande ou initie une activité,
ouvre une affaire,
associe des acteurs, documents et objets métier,
fait intervenir des personnes et unités organisationnelles,
crée des tâches, suivis, avis et décisions,
gère des délais et validations,
maintient une chronologie probante,
clôture le dossier,
puis applique les règles de conservation et de sort final.
```

Une affaire peut utiliser un workflow mais une affaire **n'est pas** une instance de workflow.

Invariant :

```text
CASE != WORKFLOW_INSTANCE
```

Une affaire doit pouvoir fonctionner :

- avec un workflow structuré ;
- avec quelques tâches ;
- avec une circulation ;
- seulement avec une timeline et des décisions ;
- avec une combinaison de ces mécanismes.

---

# 4. ADN conceptuel hérité de Goéland

Les grands sujets restent :

```text
CASE
DOCUMENT
THING
ACTOR
```

Les identités organisationnelles restent séparées :

```text
USER
ORG_UNIT
GROUP
PROCESS_ROLE
```

---

# 5. Définitions canoniques

## 5.1 CASE / AFFAIRE

Une affaire est un dossier métier ayant :

- une identité stable ;
- une référence métier éventuelle ;
- un type métier ;
- un cycle de vie ;
- des participants ;
- des relations avec d'autres sujets ;
- une timeline ;
- éventuellement des tâches, circulations et workflows ;
- un historique ;
- des règles de droits, confidentialité et conservation.

---

## 5.2 DOCUMENT

Un document est un **objet documentaire métier logique**.

Il n'est pas équivalent à un fichier binaire.

Un document possède :

- une identité stable ;
- un type ;
- un titre ;
- des métadonnées ;
- une ou plusieurs versions ;
- des relations avec affaires, acteurs et objets ;
- un cycle de vie.

---

## 5.3 DOCUMENT_VERSION

Une version est un état documentaire identifié dans le temps.

Elle possède notamment :

- un document parent ;
- un numéro ou ordre de version ;
- une date ;
- un auteur technique / origine ;
- éventuellement un statut de validation ;
- un lien vers un contenu binaire ;
- éventuellement le statut de `record`.

Deux versions distinctes peuvent exceptionnellement pointer vers **le même contenu binaire**, si le même fichier est re-soumis à deux moments différents mais que l'événement de version a une signification métier.

---

## 5.4 CONTENT_BLOB

Le contenu binaire est une ressource physique/logique identifiée par son empreinte.

Invariant important :

```text
SHA-256 UNIQUE sur CONTENT_BLOB
```

Un contenu identique ne doit pas être stocké physiquement plusieurs fois.

Le même blob peut être référencé par :

- plusieurs versions du même document ;
- éventuellement plusieurs documents logiques si un cas métier futur le justifie.

Par défaut, lorsqu'un fichier identique est déjà connu, le système doit favoriser **la réutilisation du document ou du blob existant et la création de relations/contextes**, plutôt que la duplication silencieuse.

---

## 5.5 THING

Un `THING` est un objet métier, souvent territorial et éventuellement géoréférencé.

Exemples :

```text
BUILDING
PARCEL
STREET
TREE
ADDRESS
INFRASTRUCTURE
ADVERTISEMENT
SPORT_ZONE
```

---

## 5.6 ACTOR

Conserver impérativement la définition historique Goéland :

> **Un ACTOR est une personne physique ou morale externe à la Ville qui joue un rôle dans le système Goéland.**

Exemples :

```text
personne physique
entreprise
bureau d'architectes
propriétaire
mandataire
association
fournisseur
institution externe
```

Ne jamais utiliser `ACTOR` pour :

- un utilisateur interne ;
- une unité organisationnelle ;
- un groupe de sécurité ;
- un rôle BPMN ;
- une partie prenante projet.

Le domaine Actor déjà implémenté dans le repo respecte cette définition et doit être préservé.

---

## 5.7 USER / ORG_UNIT

`USER` représente une identité interne authentifiée.

`ORG_UNIT` représente une unité organisationnelle interne.

Ils ne sont pas des ACTOR.

---

# 6. Relations typées : concept métier de premier ordre

Les relations entre sujets sont une fondation centrale du produit.

Exemples :

```text
ACTOR -- authored --> DOCUMENT
ACTOR -- recipient_of --> DOCUMENT
ACTOR -- requester_in --> CASE
ACTOR -- mandatee_in --> CASE

CASE -- concerns --> THING
CASE -- has_document --> DOCUMENT
CASE -- related_to --> CASE

DOCUMENT -- represents --> THING

THING -- contains --> THING
```

Le repo actuel possède déjà :

```text
relationship_type
subject_relationship
```

avec :

```text
source_kind
target_kind
is_directed
inverse_label
role_detail
valid_from
valid_to
```

**Conserver ce modèle.**

Améliorations futures possibles, sans urgence :

```text
min_targets
max_targets
temporal constraints
security implications
validation rules
```

Ne pas rendre ces extensions obligatoires avant un cas métier concret.

---

# 7. Rôles métier portés par les relations

Conserver le principe déjà implémenté dans Actor :

> **Les rôles ne sont pas des attributs permanents de l'entité.**

Exemple :

```text
ACTOR A
  -- REQUESTER_IN --> CASE 1
  -- MANDATEE_IN --> CASE 2
  -- AUTHOR_OF --> DOCUMENT 3
```

Le rôle dépend du contexte de la relation.

Ce principe est confirmé par le modèle historique `ActeurRole`.

---

# 8. Identité technique et référence métier

Chaque sujet possède un UUID technique immutable.

Ajouter à `subject_ref` une référence métier optionnelle.

Proposition :

```sql
ALTER TABLE subject_ref
    ADD COLUMN business_ref TEXT,
    ADD COLUMN business_ref_namespace TEXT;
```

Ne pas imposer immédiatement une unicité globale sur `business_ref`.

Une contrainte utile peut être :

```text
(namespace, business_ref)
```

lorsque le namespace est connu.

Exemples :

```text
namespace = OPC
business_ref = 2026-001245
```

ou :

```text
business_ref = OPC-2026-001245
```

Le `display_label` / titre **n'est pas un identifiant** et n'a pas besoin d'être unique.

---

# 9. Types et spécialisations

Le concept de type est commun :

```text
CaseType
DocumentType
ThingType
ActorType / OrganizationCategory
```

Mais la V2 **n'impose pas** une table `subject_type` universelle si cela rend le modèle moins lisible.

Le repo possède déjà `document_type` et `organization_category`.

Les futurs domaines peuvent conserver :

```text
case_type
thing_type
document_type
```

tant qu'ils suivent des principes cohérents :

- code stable ;
- libellé ;
- description ;
- actif/inactif ;
- attributs structurants typés ;
- possibilité de schémas/configuration secondaire.

Une généralisation physique vers `subject_type` ne sera faite que si plusieurs domaines partagent réellement des règles et des comportements communs.

> **Préférer une abstraction conceptuelle saine à une méta-table prématurée.**

---

# 10. Spécialisation : supertype + subtype

Conserver le pattern historique.

Exemple :

```text
THING
  |
  +-- PARCEL
  +-- BUILDING
  +-- TREE
```

SQL cible :

```sql
thing (...)

thing_parcel (
    thing_id UUID PRIMARY KEY REFERENCES thing(id),
    ...
)

thing_building (
    thing_id UUID PRIMARY KEY REFERENCES thing(id),
    ...
)
```

JSONB reste limité aux données secondaires ou instables.

Règle :

> Si un champ sert régulièrement à rechercher, filtrer, sécuriser, joindre, contraindre ou décider, il mérite probablement une vraie colonne.

Pas d'EAV généralisé.

---

# 11. Configuration métier != composant spécifique

La V2 **ne reprend pas** la prescription :

```text
toute spécificité métier = composant séparé
```

Elle est considérée comme contre-productive pour le noyau.

Le produit doit permettre une configuration métier contrôlée :

```text
types
attributs
formulaires
relations
règles
statuts
droits
workflows
```

sans fork applicatif.

Créer un composant séparé seulement lorsqu'une fonctionnalité :

- nécessite une logique véritablement spécifique ;
- dépend fortement d'un système externe ;
- ne peut raisonnablement pas être exprimée dans les abstractions du noyau.

---

# 12. Pas de multi-tenancy dans le POC

Ne pas implémenter de multi-tenancy.

Une affaire peut traverser de nombreuses unités organisationnelles.

Exemple OPC :

```text
une seule affaire
    |
    +-- plusieurs dizaines d'intervenants
    +-- jusqu'à de nombreuses UO/services
```

L'isolation logique par tenant risquerait de casser la transversalité qui constitue justement la valeur du produit.

Si une organisation différente doit disposer d'une instance séparée :

```text
autre namespace Kubernetes
autre cluster
autre déploiement
```

est suffisant à ce stade.

---

# 13. Architecture du code : conserver le repo actuel

Conserver l'organisation :

```text
pkg/core
pkg/document
pkg/actor
pkg/<future-domain>
```

et le modèle de modules composables.

Ne pas migrer vers :

```text
internal/domain
internal/application
internal/infrastructure
```

uniquement pour suivre une convention théorique.

Les frontières métier comptent davantage que l'arborescence.

Les futurs domaines devraient donc naturellement devenir :

```text
pkg/casefile
pkg/thing
pkg/timeline
pkg/task
pkg/circulation
pkg/security
pkg/retention
pkg/ai
```

selon ce qui reste lisible et cohérent avec le repo.

---

# 14. Proto-first / ConnectRPC : conserver sans régression

Conserver :

```text
Protocol Buffers
ConnectRPC
gRPC
gRPC-Web
Vanguard
google.api.http
OpenAPI
protovalidate
```

Cette surface est particulièrement adaptée à :

- UI Web ;
- intégrations institutionnelles ;
- workers ;
- moteurs de workflow ;
- automatisations ;
- futurs agents IA ;
- future interface MCP.

---

# 15. Document : refactoring V2 prioritaire

Le domaine Document actuel est la principale zone où la V2 demande une évolution structurelle.

Aujourd'hui, le repo stocke dans `document` :

```text
storage_ref
mime_type
file_size_bytes
sha256
version
previous_version_id
is_final
is_record
```

Ce modèle a été utile pour le premier vertical slice.

La V2 sépare désormais :

```text
DOCUMENT
    |
    +-- DOCUMENT_VERSION
            |
            +-- CONTENT_BLOB
```

---

# 16. Document logique cible

```sql
create table document (
    id uuid primary key,
    kind text not null default 'DOCUMENT',
    document_type_id uuid not null references document_type(id),
    title text not null,
    description text not null default '',
    official_date date,
    language text not null default '',
    status smallint not null,
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    created_by text not null default '',
    updated_at timestamptz not null default now(),

    constraint document_subject_fkey
        foreign key (id, kind) references subject_ref(id, kind),
    constraint document_kind_is_document check (kind = 'DOCUMENT')
);
```

---

# 17. CONTENT_BLOB : déduplication globale

Créer une table dédiée.

```sql
create table content_blob (
    id uuid primary key default gen_random_uuid(),
    sha256 char(64) not null unique,
    storage_ref text not null,
    mime_type text not null default '',
    file_size_bytes bigint not null,
    created_at timestamptz not null default now(),
    created_by text not null default '',
    verified_at timestamptz,

    constraint content_blob_size_non_negative
        check (file_size_bytes >= 0)
);
```

Invariant :

```text
un SHA-256 donné correspond à un seul CONTENT_BLOB
```

Le SHA-256 est l'identité cryptographique du **contenu**, pas du document métier.

Ne pas supprimer le bénéfice de l'index unique actuel : **le déplacer au bon niveau conceptuel**.

---

# 18. DOCUMENT_VERSION

```sql
create table document_version (
    id uuid primary key default gen_random_uuid(),
    document_id uuid not null references document(id),
    version_no integer not null,
    content_blob_id uuid not null references content_blob(id),
    created_at timestamptz not null default now(),
    created_by text not null default '',
    validated_at timestamptz,
    validated_by text,
    is_final boolean not null default false,
    is_record boolean not null default false,
    metadata jsonb not null default '{}',

    unique(document_id, version_no)
);
```

Une version validée / `record` devient immutable.

---

# 19. Pourquoi Blob et Version sont distincts

Cas :

```text
Version 1 -> Blob AAA
Version 2 -> Blob BBB
Version 3 -> Blob AAA
```

Le système :

- ne stocke `AAA` qu'une seule fois ;
- conserve toutefois l'événement métier "Version 3 créée à telle date".

Le même contenu peut donc être réutilisé sans duplication physique tout en gardant une chronologie documentaire correcte.

---

# 20. Stratégie d'ingestion d'un fichier

Lors d'un upload :

```text
1. recevoir le flux
2. calculer SHA-256 en streaming
3. chercher content_blob.sha256
4. si présent :
      réutiliser le blob
   sinon :
      stocker les octets
      créer content_blob
5. déterminer si un DOCUMENT existant doit être réutilisé
6. créer si nécessaire DOCUMENT_VERSION
7. créer les relations métier appropriées
8. auditer
```

Important :

> **Un fichier identique utilisé dans deux affaires ne justifie pas à lui seul deux documents.**

Le contexte d'utilisation doit être porté par les relations :

```text
DOCUMENT D1
  -- CASE_HAS_DOCUMENT --> CASE A
  -- CASE_HAS_DOCUMENT --> CASE B
```

---

# 21. Migration depuis le modèle Document actuel

Ne pas casser le vertical slice Document existant.

Procéder par migration additive.

Approche recommandée :

```text
Phase A
    créer content_blob
    créer document_version

Phase B
    backfill :
      document.sha256/storage_ref/... -> content_blob
      document.version/... -> document_version

Phase C
    adapter repository/service/proto

Phase D
    faire passer l'UI sur la nouvelle API

Phase E
    supprimer/déprécier anciennes colonnes seulement après tests
```

Pendant la migration, conserver la compatibilité nécessaire pour que :

```text
CreateDocument
GetDocument
FinalizeDocument
VerifyDocumentIntegrity
SearchDocuments
LinkDocument
DeleteDocument
```

continuent à fonctionner ou soient remplacés par une évolution clairement versionnée.

---

# 22. SHA-256 : règles précises

Conserver :

```text
SHA-256 UNIQUE sur content_blob
```

Ne pas imposer :

```text
document.id == sha256
```

Ne pas considérer le hash comme :

- identifiant métier ;
- titre ;
- identifiant d'une affaire ;
- preuve suffisante de contexte.

Le hash prouve l'identité du contenu binaire.

Le document et ses relations portent le contexte métier.

---

# 23. Stockage documentaire

Conserver l'abstraction actuelle de stockage.

Le repo a déjà un filestore local et une logique :

```text
internal://<uuid>
```

Cette approche doit évoluer vers une interface substituable.

Exemple :

```go
type BlobStore interface {
    Put(ctx context.Context, r io.Reader, metadata BlobMetadata) (BlobRef, error)
    Get(ctx context.Context, ref BlobRef) (io.ReadCloser, error)
    Delete(ctx context.Context, ref BlobRef) error
}
```

Le stockage physique pourra devenir ultérieurement :

```text
filesystem
S3-compatible
GED institutionnelle
```

sans modifier le modèle Document.

---

# 24. CASE

Prochaine grande tranche métier après l'alignement V2 du core/document.

```sql
create table case_type (
    id uuid primary key default gen_random_uuid(),
    code text unique not null,
    label text not null,
    description text not null default '',
    is_active boolean not null default true
);
```

```sql
create table case_file (
    id uuid primary key,
    kind text not null default 'CASE',
    case_type_id uuid not null references case_type(id),
    title text not null,
    description text not null default '',
    status text not null default 'OPEN',
    opened_at timestamptz not null default now(),
    closed_at timestamptz,
    closure_reason text,
    metadata jsonb not null default '{}',

    foreign key (id, kind) references subject_ref(id, kind),
    check (kind = 'CASE')
);
```

Une affaire existe indépendamment de tout workflow.

---

# 25. THING

Après Case, construire Thing tôt car les relations Case ↔ Thing sont fondamentales.

```sql
create table thing_type (...);

create table thing (
    id uuid primary key,
    kind text not null default 'THING',
    thing_type_id uuid not null references thing_type(id),
    name text not null,
    description text not null default '',
    external_ref text,
    geom geometry,
    metadata jsonb not null default '{}',
    foreign key (id, kind) references subject_ref(id, kind),
    check (kind = 'THING')
);
```

Spécialisations POC initiales :

```text
thing_parcel
thing_building
```

PostGIS est déjà préparé dans le repo : conserver ce choix.

---

# 26. Timeline / Suivis

Le suivi est une abstraction centrale.

```sql
create table case_timeline_entry (
    id uuid primary key default gen_random_uuid(),
    case_id uuid not null references case_file(id),
    entry_type text not null,
    body text not null,
    visibility text not null default 'CASE_PARTICIPANTS',
    created_at timestamptz not null default now(),
    created_by text not null default '',
    updated_at timestamptz,
    updated_by text not null default '',
    validated_at timestamptz,
    validated_by text,
    locked_at timestamptz,
    locked_by text
);
```

Types initiaux :

```text
COMMENT
OPINION
DECISION
REQUEST
RESPONSE
VALIDATION
SYSTEM
AI_PROPOSAL
```

Invariant :

> Un suivi validé ou verrouillé devient immutable.

Une correction crée un nouvel élément plutôt que de réécrire silencieusement l'ancien.

---

# 27. Documents liés à la timeline

```sql
create table timeline_document_link (
    timeline_entry_id uuid not null references case_timeline_entry(id),
    document_id uuid not null references document(id),
    created_at timestamptz not null default now(),
    created_by text not null default '',
    primary key (timeline_entry_id, document_id)
);
```

Le lien porte sur le **document logique**.

Un besoin futur peut permettre de figer explicitement une `document_version_id` lorsqu'une décision doit référencer exactement la version probante utilisée.

---

# 28. TASK avant CIRCULATION

Introduire Task comme concept indépendant.

```sql
create table case_task (
    id uuid primary key default gen_random_uuid(),
    case_id uuid not null references case_file(id),
    task_type text not null,
    title text not null,
    description text,
    status text not null default 'OPEN',
    assignee_user_id text,
    assignee_org_unit_id uuid,
    due_at timestamptz,
    created_at timestamptz not null default now(),
    created_by text not null default '',
    completed_at timestamptz,
    completed_by text
);
```

Une tâche peut être :

- manuelle ;
- créée par une circulation ;
- créée plus tard par workflow ;
- proposée par IA ;
- attribuée à un USER ou une ORG_UNIT.

Historiser les réattributions.

---

# 29. CIRCULATION comme orchestration légère de tâches

Conceptuellement :

```text
Circulation
    |
    +-- task/recipient A
    +-- task/recipient B
    +-- task/recipient C
```

Le modèle peut garder des tables dédiées au POC mais doit tendre vers une composition cohérente de tâches.

Réponses initiales :

```text
FAVORABLE
UNFAVORABLE
COMMENT
NOT_CONCERNED
NEED_MORE_INFO
```

Une réponse métier significative doit pouvoir créer un suivi dans la timeline.

---

# 30. Workflow : optionnel et après Case/Task/Circulation

Ne pas construire d'abord un moteur BPMN.

Préparer seulement :

```text
ProcessDefinition
ProcessDefinitionVersion
ProcessInstance
```

Une instance de processus référence une CASE.

La CASE ne dépend pas obligatoirement d'une instance.

Fonctions futures :

```text
parallel branches
join
conditions
events
timers
escalations
suspension
resume
cancellation
human task
validation
subprocess
ad-hoc step
```

Question obligatoire avant intégration d'un moteur :

> Que devient une instance en cours lorsque la définition du processus change ?

Candidats open source à évaluer ultérieurement :

```text
Flowable
Temporal
autres
```

---

# 31. USER / ORG_UNIT / GROUP

Ajouter lorsque Case/Task/Circulation commencent à en avoir besoin.

Le repo contient déjà `USER` / `ORG_UNIT` dans `SubjectKind`, mais ces sujets ne doivent pas nécessairement utiliser exactement le même stockage métier que Case/Document/Thing/Actor.

Éviter une généralisation forcée.

---

# 32. Sécurité

L'autorisation actuelle basée sur scopes est une bonne étape POC mais reste incomplète.

Le modèle futur doit pouvoir prendre en compte :

```text
scope
role
group
org unit
ownership
participation in case
typed relationship
confidentiality
temporary exception
need-to-know
```

API métier conceptuelle :

```go
Can(ctx, user, action, subject) (bool, error)
```

Évaluer plus tard :

```text
Casbin
OpenFGA
```

sans coupler le domaine directement à une technologie.

---

# 33. Audit : préserver l'existant

Le repo actuel écrit déjà l'audit dans la même transaction que les mutations.

**Conserver strictement ce principe.**

Audit mutation :

```text
subject_id
event_type
authenticated operator
occurred_at
before_state
after_state
reason
correlation_id
request_id
metadata
```

Ajouter ultérieurement `causation_id` si nécessaire.

---

# 34. Audit des lectures sensibles

Ajouter un mécanisme distinct lorsque le domaine Sécurité sera traité.

```sql
create table access_audit_event (
    id uuid primary key default gen_random_uuid(),
    user_id text not null,
    subject_id uuid references subject_ref(id),
    action text not null,
    occurred_at timestamptz not null default now(),
    purpose text,
    correlation_id uuid,
    request_id text
);
```

Ne pas nécessairement journaliser toute lecture banale.

L'objectif est de pouvoir auditer :

```text
READ_SENSITIVE
DOWNLOAD
EXPORT
```

sur les périmètres sensibles.

---

# 35. Lifecycle : distinguer les concepts

Ne pas confondre :

```text
ACTIVE
CLOSED
ARCHIVED
LOGICALLY_DELETED
DISPOSED
```

`CLOSED` = fin opérationnelle.

`ARCHIVED` = dossier conservé/figé selon politique.

`LOGICALLY_DELETED` = masqué/inactif dans l'usage courant.

`DISPOSED` = élimination autorisée après procédure de conservation.

Aucune suppression physique silencieuse depuis les services métier.

---

# 36. Retention : faire évoluer sans casser

Le repo actuel possède dans `record_metadata` :

```text
retention_until TEXT
sort_final TEXT
```

Ne pas casser immédiatement cette structure.

Considérer ces champs comme **préparation V1**.

À terme, introduire :

```text
retention_policy
subject_retention
```

avec :

```text
policy
retention_start
retention_until
final_disposition
disposition_status
disposition_proof
```

Migration ultérieure seulement lorsque le cas métier est assez clair.

---

# 37. Provenance / migration

Ajouter tôt la provenance pour préparer la coexistence avec Goéland historique.

```sql
create table subject_provenance (
    subject_id uuid not null references subject_ref(id),
    source_system text not null,
    source_id text not null,
    source_type text,
    imported_at timestamptz not null default now(),
    import_batch_id uuid,
    metadata jsonb not null default '{}',
    primary key(subject_id, source_system)
);
```

Les anciens IDs sont des identifiants de provenance, pas les nouveaux UUID.

---

# 38. Outbox / événements

Ne pas passer au full event sourcing.

Conserver :

```text
tables d'état
+ audit append-only
+ outbox
```

Événements futurs :

```text
CaseCreated
CaseClosed
RelationshipCreated
RelationshipEnded
TimelineEntryAdded
TimelineEntryValidated
TaskAssigned
TaskCompleted
CirculationCreated
CirculationResponded
DocumentVersionAdded
DocumentVersionValidated
AIProposalCreated
```

Outbox transactionnelle :

```sql
create table outbox_event (
    id uuid primary key default gen_random_uuid(),
    aggregate_kind text not null,
    aggregate_id uuid not null,
    event_type text not null,
    payload jsonb not null,
    occurred_at timestamptz not null default now(),
    published_at timestamptz,
    correlation_id uuid
);
```

---

# 39. AI-ready / Human-in-the-loop

L'IA est une capacité structurante future.

Elle doit pouvoir :

- rechercher ;
- lire une affaire ;
- lire la timeline ;
- consulter les documents ;
- analyser les relations ;
- classifier ;
- extraire des métadonnées ;
- détecter des pièces manquantes ;
- proposer une action ;
- préparer une réponse ;
- proposer un suivi ;
- proposer une tâche ;
- demander une validation humaine.

Invariant :

```text
AI proposal
    |
    v
Human validation
    |
    v
Sensitive domain mutation
```

Ne jamais donner à un LLM :

```text
SQL libre
mutation silencieuse
autorité implicite
```

---

# 40. AI action trace

```sql
create table ai_action (
    id uuid primary key default gen_random_uuid(),
    case_id uuid references case_file(id),
    subject_id uuid references subject_ref(id),
    action_type text not null,
    model_provider text,
    model_id text,
    model_version text,
    prompt_template_id text,
    input_refs jsonb not null default '[]',
    output_json jsonb,
    status text not null default 'PROPOSED',
    created_at timestamptz not null default now(),
    validated_by text,
    validated_at timestamptz,
    rejected_by text,
    rejected_at timestamptz
);
```

---

# 41. MCP / Agent interface

Préparer une future interface MCP.

Tools possibles :

```text
get_case
search_cases
get_case_timeline
get_related_subjects
get_document_metadata
list_document_versions
list_case_documents
propose_timeline_entry
propose_task
propose_relationship
request_human_validation
```

Les mutations sensibles passent par la couche métier et ses contrôles.

---

# 42. Recherche

Conserver la recherche PostgreSQL déjà utile dans Document/Actor.

Ne pas introduire immédiatement Meilisearch si PostgreSQL suffit au POC.

Prévoir une interface remplaçable plus tard.

---

# 43. Open source

Le projet est volontairement libre.

Principes :

- dépendances essentielles open source ;
- licences connues ;
- pas de composant propriétaire obligatoire au runtime ;
- formats ouverts/documentés ;
- API documentées ;
- migrations versionnées ;
- possibilité de compiler/déployer indépendamment d'un fournisseur ;
- pas de SaaS obligatoire ;
- inventaire des licences directes ;
- SBOM ultérieurement.

---

# 44. Pas d'EAV généralisé

Interdit comme mécanisme universel :

```text
attribute
attribute_value
generic_object_property
```

Préférer :

```text
tables typées
tables de spécialisation
JSONB limité
```

---

# 45. Pas de full event sourcing

Ne pas reconstruire l'état courant en rejouant tous les événements.

Utiliser :

```text
state tables
+ audit
+ outbox
```

---

# 46. RPO / RTO

Ne pas imposer de chiffre dans le domaine POC.

L'architecture doit simplement rester compatible avec :

- PostgreSQL WAL ;
- Point-In-Time Recovery ;
- sauvegarde continue ;
- réplication ;
- cohérence transactionnelle état + audit + outbox.

Les données probantes devront pouvoir viser plus tard un RPO très faible.

---

# 47. Stack technique recommandée = stack actuelle

Conserver :

```text
Go
PostgreSQL
PostGIS
Protocol Buffers
ConnectRPC
Vanguard
protovalidate
pgx
Vue 3
Vuetify 4
bun/Vite
```

Pas de refactoring technologique gratuit.

---

# 48. Ordre de travail recommandé à partir du repo actuel

## Phase 0 — Alignement V2 sans régression

1. conserver V1 comme archive historique ;
2. placer cette V2 comme référence active ;
3. mettre à jour `IMPLEMENTATION_STATUS.md` ;
4. ajouter `business_ref` / namespace ;
5. créer `content_blob` ;
6. créer `document_version` ;
7. migrer progressivement le Document actuel ;
8. conserver SHA-256 unique au niveau `content_blob` ;
9. maintenir tous les tests actuels ;
10. ajouter des tests de déduplication globale.

## Phase 1 — CASE

```text
case_type
case_file
CaseService
relations Case <-> Actor/Document
```

## Phase 2 — THING

```text
thing_type
thing
thing_parcel
thing_building
PostGIS
Case <-> Thing
Document <-> Thing
```

## Phase 3 — TIMELINE

```text
case_timeline_entry
validation
immutability
document links
```

## Phase 4 — TASK

```text
case_task
assignment
reassignment
history
deadlines
```

## Phase 5 — CIRCULATION

```text
parallel recipients
responses
deadline
completion
timeline integration
```

## Phase 6 — USER / ORG_UNIT / SECURITY

```text
internal identities
authorization
confidentiality
sensitive read audit
```

## Phase 7 — PROVENANCE / OUTBOX / EXPORT

## Phase 8 — AI proposal / Human validation / MCP readiness

## Phase 9 — Workflow abstraction / engine evaluation

---

# 49. Tests V2 à ajouter immédiatement

## Content blob

- upload nouveau contenu -> nouveau blob ;
- même SHA-256 -> blob existant réutilisé ;
- aucun stockage binaire en double ;
- collision logique taille/hash incohérente -> erreur forte ;
- vérification du contenu en streaming.

## Document version

- plusieurs versions pour un document ;
- deux versions peuvent réutiliser le même blob ;
- version validée immutable ;
- `is_record` immutable ;
- version courante déterminable.

## Document relations

- un document peut être lié à plusieurs affaires ;
- même document / même blob, contextes métier différents ;
- pas de duplication de document uniquement parce qu'il est lié à une seconde affaire.

---

# 50. Scénario POC principal V2

```text
1. Créer USER interne.
2. Créer deux ORG_UNIT.
3. Créer CASE OPC_DEMANDE_PC.
4. Attribuer business_ref.
5. Créer THING/PARCEL.
6. Créer THING/BUILDING.
7. Créer ACTOR personne.
8. Créer ACTOR organisation.
9. CASE -- concerns --> PARCEL.
10. CASE -- concerns --> BUILDING.
11. ACTOR personne -- requester_in --> CASE.
12. ACTOR organisation -- mandatee_in --> CASE.
13. Créer DOCUMENT "Plan de bâtiment".
14. Uploader fichier -> CONTENT_BLOB SHA-256 unique.
15. Créer DOCUMENT_VERSION 1 -> blob.
16. CASE A -- has_document --> DOCUMENT.
17. CASE B -- has_document --> le même DOCUMENT.
18. DOCUMENT -- represents --> BUILDING.
19. Ajouter suivi.
20. Lier DOCUMENT au suivi.
21. Valider suivi -> immutable.
22. Créer tâches.
23. Réassigner tâche -> historique.
24. Créer circulation vers deux UO.
25. Réponses -> timeline.
26. Créer AI_PROPOSAL simulée.
27. Validation humaine -> suivi métier.
28. Clôturer affaire.
29. Vérifier mutations interdites.
30. Consulter audit complet.
31. Exporter le graphe d'affaire.
```

---

# 51. ExportCase

Prévoir une exportation structurée :

```text
case
metadata
timeline
tasks
circulations
relationships
related subjects
documents
document versions
content blob metadata
hashes
audit summary
provenance
retention metadata
```

Format POC :

```text
JSON + blobs dans ZIP
```

Ne jamais dupliquer plusieurs fois le même blob dans l'archive si plusieurs références pointent vers lui.

---

# 52. Requirement mapping utile

```text
EXFL.101-105  -> CASE + CaseType + business_ref
EXFL.106      -> confidentiality
EXFL.107-108  -> retention/disposition
EXFL.109-110  -> typed fields / specialization
EXFL.111-120  -> lifecycle + relationships + audit
EXFL.123-125  -> immutability / controlled disposition

EXFL.201-218  -> THING + specializations + geometry + relationships
EXFL.220-221  -> controlled disposition

EXFL.301-310  -> DOCUMENT + DocumentType
EXFL.311      -> content_blob SHA-256 deduplication
EXFL.312-320  -> DocumentVersion / validation / integrity
EXFL.325-326  -> controlled disposition / record

EXFL.401-417  -> ACTOR + typed relationships

EXFL.501-505  -> search

EXFL.601-610  -> TASK + CIRCULATION + optional workflow

EXFL.701-713  -> import/export + provenance

EXFL.805-806  -> mutation audit + sensitive read audit

EXFL.001-010  -> USER / ORG_UNIT / authorization

EXTR.01-05    -> provenance + migration + coexistence
```

Ne pas reprendre mécaniquement :

```text
multi-tenancy obligatoire
toute spécificité métier = composant séparé
limite arbitraire de 30 versions
RPO 4h comme contrainte du modèle
```

---

# 53. Critères d'acceptation fonctionnels

La V2 est correctement implémentée si :

- l'existant Document/Actor/Core ne régresse pas ;
- ACTOR conserve strictement son sens Goéland ;
- `subject_ref` reste l'identité canonique ;
- `business_ref` est disponible ;
- les relations typées restent une primitive centrale ;
- le même contenu binaire n'est stocké qu'une fois ;
- SHA-256 reste globalement unique au niveau ContentBlob ;
- un Document est distinct de ses versions ;
- une version est distincte de son contenu binaire ;
- un Document peut jouer plusieurs rôles/contextes via des relations ;
- aucun workflow n'est obligatoire pour créer une affaire ;
- les suivis validés sont immutables ;
- les tâches existent indépendamment du workflow ;
- les circulations peuvent agréger plusieurs réponses ;
- chaque mutation reste auditée transactionnellement ;
- la provenance peut être conservée ;
- les propositions IA sensibles requièrent validation humaine ;
- aucune multi-tenancy n'est requise ;
- aucune dépendance propriétaire obligatoire n'est introduite.

---

# 54. Critères techniques

Conserver les standards actuels :

- `go test ./...` vert ;
- tests d'intégration PostgreSQL ;
- migrations idempotentes/rejouables ;
- protovalidate ;
- SQL explicite ;
- contraintes PostgreSQL utiles ;
- transactions mutation + audit + outbox ;
- opérateur dérivé de l'authentification ;
- `request_id` / correlation propagés ;
- timestamps UTC ;
- SHA-256 calculé serveur ;
- pas de suppression physique métier ;
- Docker build reproductible ;
- Trivy/CI non régressifs ;
- UI existante encore fonctionnelle après migration.

---

# 55. Ce qui est hors scope initial

- multi-tenancy ;
- moteur BPMN complet ;
- CMMN complet ;
- DMN ;
- client mobile natif ;
- SAE complet ;
- facturation ;
- signature électronique ;
- BI avancé ;
- vraie intégration LLM ;
- infrastructure HA/DRP complète ;
- Meilisearch obligatoire ;
- microservices séparés par domaine.

---

# 56. Guiding principles

```text
Preserve what already works.
Domain first.
Case is not workflow.
Actor keeps the historical Goéland meaning.
Relationships are first-class.
Roles live in context.
Document != DocumentVersion != ContentBlob.
Identical content is globally deduplicated.
SHA-256 belongs to content identity.
History must be explainable.
Validated information becomes immutable.
Deletion is governed, not silent.
Configuration is not custom code.
No premature multi-tenancy.
No premature microservices.
No generalized EAV.
No full event sourcing.
No proprietary mandatory runtime.
AI proposes; humans validate sensitive actions.
```

---

# 57. Definition of Done de la phase d'alignement V2

Avant d'attaquer massivement Case/Thing/Timeline, l'alignement V2 est terminé lorsque :

```text
[ ] la V2 est la spec active
[ ] IMPLEMENTATION_STATUS reflète la V2
[ ] business_ref existe
[ ] content_blob existe
[ ] SHA-256 est UNIQUE sur content_blob
[ ] document_version existe
[ ] Document actuel est migré sans perte
[ ] le filestore existant fonctionne toujours
[ ] les APIs sont compatibles ou versionnées proprement
[ ] la UI Document fonctionne
[ ] les tests Actor restent verts
[ ] les tests Document restent verts
[ ] déduplication globale testée
[ ] même Document lié à plusieurs affaires possible
[ ] aucune régression audit / auth / sécurité / CI
```

---

# 58. Intention architecturale finale

La force recherchée n'est pas un framework générique capable de tout faire par configuration arbitraire.

La force recherchée est un petit nombre d'abstractions métier durables :

```text
SUBJECT
TYPE
SPECIALIZATION
RELATIONSHIP

CASE
TIMELINE
TASK
CIRCULATION

DOCUMENT
DOCUMENT_VERSION
CONTENT_BLOB

GOVERNANCE
AUDIT
RETENTION
PROVENANCE
```

Ces abstractions doivent permettre :

- une UX simple ;
- un domaine puissant ;
- une évolution sur le long terme ;
- des workflows optionnels ;
- des intégrations ouvertes ;
- des agents IA contrôlés ;
- une migration progressive depuis Goéland historique.

Le projet existant est déjà sur cette trajectoire.

La V2 doit donc **le faire converger**, pas le recommencer.
