# Goéland POC — Brief agent de codage

## Génération d’une UI Vue 3 + Vuetify à partir de `core.proto` et `document.proto`, avec fichier intermédiaire UI schema et support traduction/i18n

**Destinataire :** agent de codage Codex, Claude Code ou équivalent  
**Objectif :** produire une base frontend maintenable pour le POC Goéland à partir des contrats Protobuf existants.  
**Cible technique :** Vue 3, TypeScript strict, Vuetify, Vite, vue-i18n, client API généré depuis Protobuf/ConnectRPC.  
**Principe clé :** ne pas générer directement une UI depuis les `.proto`. Générer d’abord un fichier intermédiaire `*.ui.schema.json`, puis générer les composants Vue/Vuetify à partir de ce schéma.

---

## 1. Contexte métier

Le POC Goéland vise à reconstruire un noyau moderne de gestion d’affaires administratives. Le cœur du modèle est constitué de sujets métier durables :

- `CASE` : affaire / dossier métier ;
- `DOCUMENT` : pièce documentaire ;
- `THING` : objet métier ou territorial ;
- `ACTOR` : personne ou organisation externe ;
- `USER` : utilisateur interne ;
- `ORG_UNIT` : unité organisationnelle.

Le système n’est pas seulement une GED ni un workflow engine. Il doit représenter :

```text
Sujets métier durables
+ relations typées entre sujets
+ documents contextualisés
+ gouvernance / droits / confidentialité
+ non-destruction
+ audit append-only
+ verrouillage probant
```

Le frontend doit refléter cette vision. Il ne doit pas seulement afficher des formulaires CRUD génériques.

---

## 2. Fichiers d’entrée

L’agent doit utiliser prioritairement ces fichiers :

```text
core.proto
document.proto
```

### 2.1 `core.proto`

Ce fichier contient les éléments transversaux :

```text
SubjectKind
Permission
SubjectRef
RecordMetadata
AuditEvent
RelationshipType
SubjectRelationship
CoreService
```

Ces éléments doivent produire des composants UI transversaux réutilisables.

### 2.2 `document.proto`

Ce fichier contient le module document :

```text
DocumentStatus
DocumentType
Document
CreateDocumentRequest
GetDocumentRequest
UpdateDocumentMetadataRequest
FinalizeDocumentRequest
VerifyDocumentIntegrityRequest
SearchDocumentsRequest
LinkDocumentRequest
DeleteDocumentRequest
ListDocumentTypesRequest
DocumentService
```

Le module document est le premier module métier complet à implémenter côté UI.

---

## 3. Objectif de génération

Produire une base frontend permettant de :

1. Rechercher des documents.
2. Afficher une liste de documents paginée.
3. Créer / enregistrer un document.
4. Afficher le détail d’un document.
5. Modifier les métadonnées modifiables d’un document.
6. Finaliser un document avec confirmation.
7. Vérifier l’intégrité SHA-256 d’un document.
8. Afficher les relations du document avec d’autres sujets.
9. Lier un document à une affaire, un objet ou un autre sujet autorisé.
10. Afficher l’audit récent ou complet.
11. Respecter les états `is_final`, `is_record`, `record_metadata.is_locked`, `deleted_at`.
12. Supporter les traductions de l’interface, des enums, des messages et des erreurs.

---

## 4. Architecture frontend cible

Structure recommandée :

```text
frontend/
  package.json
  vite.config.ts
  tsconfig.json
  src/
    main.ts
    App.vue
    router/
      index.ts
    plugins/
      vuetify.ts
      i18n.ts
    api/
      client.ts
      documentClient.ts
      coreClient.ts
    generated/
      goeland/v1/...
    schemas/
      core.ui.schema.json
      document.ui.schema.json
    locales/
      fr-CH.json
      en.json
    components/
      core/
        SubjectIdentityCard.vue
        RecordMetadataPanel.vue
        AuditTimeline.vue
        RelationshipTable.vue
        RelationshipTypeSelect.vue
        LinkSubjectDialog.vue
      document/
        DocumentStatusChip.vue
        DocumentTypeSelect.vue
        DocumentMetadataForm.vue
        DocumentFinalizeDialog.vue
        DocumentIntegrityPanel.vue
        DocumentRelationshipsPanel.vue
        DocumentAuditPanel.vue
    pages/
      documents/
        DocumentListPage.vue
        DocumentDetailPage.vue
        DocumentCreatePage.vue
    composables/
      useDocumentApi.ts
      useCoreApi.ts
      useApiErrors.ts
      useI18nEnum.ts
    utils/
      protoValidationToVuetifyRules.ts
      formatters.ts
      visibility.ts
```

---

## 5. Règle fondamentale : proto → UI schema → Vuetify

Ne pas faire :

```text
.proto → composants Vue directement
```

Faire :

```text
.proto
  → analyse structurée
  → core.ui.schema.json + document.ui.schema.json
  → composants Vue/Vuetify
  → traductions i18n
  → revue automatique
  → revue humaine
```

Raison : les `.proto` décrivent le contrat API et le domaine, mais ne suffisent pas à définir une bonne ergonomie : ordre des champs, sections, priorités, actions critiques, affichage mobile, libellés métier, icônes, messages utilisateur.

---

## 6. Fichier intermédiaire `document.ui.schema.json`

L’agent doit produire ce fichier avant de générer les composants.

### 6.1 Exemple cible

```json
{
  "resource": "Document",
  "protoMessage": "goeland.v1.Document",
  "service": "goeland.v1.DocumentService",
  "labelKey": "resources.document.label",
  "pluralLabelKey": "resources.document.plural",
  "primaryField": "title",
  "identityField": "subject_ref.id",
  "statusField": "status",
  "lockField": "record_metadata.is_locked",
  "pages": [
    {
      "name": "DocumentListPage",
      "route": "/documents",
      "kind": "list",
      "serviceMethod": "SearchDocuments",
      "titleKey": "pages.documents.list.title",
      "filters": [
        {
          "field": "query",
          "component": "v-text-field",
          "labelKey": "fields.search.query",
          "cols": { "xs": 12, "md": 4 }
        },
        {
          "field": "document_type_code",
          "component": "DocumentTypeSelect",
          "labelKey": "fields.document.document_type",
          "cols": { "xs": 12, "md": 3 }
        },
        {
          "field": "confidentiality_max",
          "component": "v-select",
          "labelKey": "fields.recordMetadata.confidentiality_level",
          "cols": { "xs": 12, "md": 2 }
        },
        {
          "field": "only_records",
          "component": "v-switch",
          "labelKey": "fields.document.only_records",
          "cols": { "xs": 6, "md": 1 }
        },
        {
          "field": "only_final",
          "component": "v-switch",
          "labelKey": "fields.document.only_final",
          "cols": { "xs": 6, "md": 1 }
        },
        {
          "field": "include_deleted",
          "component": "v-switch",
          "labelKey": "fields.document.include_deleted",
          "cols": { "xs": 12, "md": 1 }
        }
      ],
      "table": {
        "serverSide": true,
        "itemsField": "documents",
        "nextPageTokenField": "next_page_token",
        "totalSizeField": "total_size",
        "columns": [
          { "field": "title", "labelKey": "fields.document.title", "primary": true },
          { "field": "document_type.label", "labelKey": "fields.document.document_type" },
          { "field": "status", "labelKey": "fields.document.status", "component": "DocumentStatusChip" },
          { "field": "official_date", "labelKey": "fields.document.official_date", "format": "date" },
          { "field": "language", "labelKey": "fields.document.language" },
          { "field": "is_final", "labelKey": "fields.document.is_final", "component": "boolean-chip" },
          { "field": "is_record", "labelKey": "fields.document.is_record", "component": "boolean-chip" },
          { "field": "created_at", "labelKey": "fields.common.created_at", "format": "datetime" }
        ],
        "rowClickRoute": "/documents/:subject_ref.id"
      }
    },
    {
      "name": "DocumentDetailPage",
      "route": "/documents/:id",
      "kind": "detail",
      "serviceMethod": "GetDocument",
      "titleField": "document.title",
      "subtitleFields": ["document.document_type.label", "document.status"],
      "requestOptions": {
        "include_relationships": true,
        "include_audit": true
      },
      "sections": [
        {
          "id": "summary",
          "labelKey": "sections.document.summary",
          "layout": "two-column",
          "fields": [
            "title",
            "description",
            "document_type",
            "status",
            "official_date",
            "language",
            "is_final",
            "is_record"
          ]
        },
        {
          "id": "storage",
          "labelKey": "sections.document.storage",
          "layout": "two-column",
          "fields": [
            "storage_ref",
            "external_system",
            "external_id",
            "external_url",
            "mime_type",
            "file_size_bytes",
            "page_count"
          ]
        },
        {
          "id": "integrity",
          "labelKey": "sections.document.integrity",
          "component": "DocumentIntegrityPanel"
        },
        {
          "id": "relationships",
          "labelKey": "sections.document.relationships",
          "component": "DocumentRelationshipsPanel"
        },
        {
          "id": "governance",
          "labelKey": "sections.document.governance",
          "component": "RecordMetadataPanel"
        },
        {
          "id": "audit",
          "labelKey": "sections.document.audit",
          "component": "DocumentAuditPanel"
        }
      ],
      "actions": [
        {
          "id": "updateMetadata",
          "labelKey": "actions.document.updateMetadata",
          "serviceMethod": "UpdateDocumentMetadata",
          "kind": "primary",
          "visibleWhen": "!document.record_metadata.is_locked && !document.record_metadata.deleted_at"
        },
        {
          "id": "finalize",
          "labelKey": "actions.document.finalize",
          "serviceMethod": "FinalizeDocument",
          "kind": "critical",
          "confirmationDialog": "DocumentFinalizeDialog",
          "visibleWhen": "!document.is_final && !document.record_metadata.is_locked && !document.record_metadata.deleted_at"
        },
        {
          "id": "verifyIntegrity",
          "labelKey": "actions.document.verifyIntegrity",
          "serviceMethod": "VerifyDocumentIntegrity",
          "kind": "secondary",
          "visibleWhen": "!!document.storage_ref && !document.record_metadata.deleted_at"
        },
        {
          "id": "delete",
          "labelKey": "actions.document.delete",
          "serviceMethod": "DeleteDocument",
          "kind": "danger",
          "requiresReason": true,
          "visibleWhen": "!document.record_metadata.deleted_at && !document.record_metadata.is_locked"
        }
      ]
    }
  ]
}
```

---

## 7. Fichier intermédiaire `core.ui.schema.json`

### 7.1 Objectif

Décrire les composants transversaux réutilisés par tous les modules.

### 7.2 Exemple cible

```json
{
  "resource": "Core",
  "components": [
    {
      "name": "SubjectIdentityCard",
      "protoMessage": "goeland.v1.SubjectRef",
      "fields": [
        "kind",
        "display_label",
        "canonical_url",
        "created_at"
      ]
    },
    {
      "name": "RecordMetadataPanel",
      "protoMessage": "goeland.v1.RecordMetadata",
      "sections": [
        {
          "id": "lifecycle",
          "fields": [
            "created_at",
            "created_by",
            "updated_at",
            "updated_by",
            "deleted_at",
            "deleted_by"
          ]
        },
        {
          "id": "ownership",
          "fields": [
            "owner_user_id",
            "owner_org_id",
            "confidentiality_level"
          ]
        },
        {
          "id": "locking",
          "fields": [
            "version",
            "is_locked",
            "locked_at",
            "locked_by"
          ]
        },
        {
          "id": "retention",
          "fields": [
            "retention_until",
            "sort_final"
          ]
        }
      ]
    },
    {
      "name": "AuditTimeline",
      "protoMessage": "goeland.v1.AuditEvent",
      "display": "timeline-or-table",
      "fields": [
        "occurred_at",
        "event_type",
        "actor_user_id",
        "reason",
        "correlation_id",
        "request_id"
      ]
    },
    {
      "name": "RelationshipTable",
      "protoMessage": "goeland.v1.SubjectRelationship",
      "fields": [
        "source.display_label",
        "relationship_type.label",
        "target.display_label",
        "role_detail",
        "valid_from",
        "valid_to",
        "created_at",
        "created_by",
        "deleted_at"
      ]
    }
  ]
}
```

---

## 8. Mapping proto → composants Vuetify

L’agent doit appliquer les règles suivantes.

| Élément Protobuf | Interprétation | Composant Vuetify / Vue recommandé |
|---|---|---|
| `string` court | titre, nom, email, code | `v-text-field` |
| `string` long | description, commentaire, reason | `v-textarea` |
| `string` avec `uuid` | identifiant | readonly text / hidden / route param |
| `string` avec URL | URL externe | `v-text-field` + bouton ouvrir |
| `string` avec pattern SHA-256 | empreinte | readonly monospace + validation pattern |
| `int32`, `int64` | nombre | `v-text-field type="number"` ou composant numérique |
| `bool` | option | `v-switch`, `v-checkbox`, `v-chip` readonly |
| `enum` | statut/type/droit | `v-select` ou chip traduit |
| `google.protobuf.Timestamp` | date/heure technique | format datetime, readonly si `OUTPUT_ONLY` |
| `string` ISO date | date métier | date picker ou champ date |
| `google.protobuf.Struct` | metadata libre | panneau JSON contrôlé ou sous-formulaire dynamique limité |
| `map<string,string>` | metadata clé/valeur | table clé/valeur éditable si autorisé |
| `repeated message` | liste liée | `v-data-table`, `v-list`, `v-expansion-panels` |
| `OUTPUT_ONLY` | champ serveur | readonly, jamais dans formulaire de création |
| `REQUIRED` | champ obligatoire | règle `required` + astérisque |
| `buf.validate max_len` | limite de saisie | compteur + règle de validation |
| `buf.validate pattern` | format contraint | règle pattern traduite |
| `record_metadata.is_locked` | verrouillage | désactiver édition/actions mutation |
| `deleted_at` non vide | soft-delete | afficher état supprimé, désactiver mutations normales |

---

## 9. Règles métier UI obligatoires

### 9.1 Verrouillage

Si :

```text
document.record_metadata.is_locked == true
```

Alors :

- aucun formulaire de modification ne doit être actif ;
- l’action `FinalizeDocument` doit être masquée ou désactivée ;
- l’action `DeleteDocument` doit être masquée ou désactivée ;
- afficher un `v-alert` ou un chip : “Document verrouillé”.

### 9.2 Finalisation

Si :

```text
document.is_final == true
```

Alors :

- afficher un chip “Final” ;
- masquer l’action “Finaliser” ;
- ne pas permettre de revenir en brouillon côté UI ;
- afficher l’événement d’audit de finalisation si disponible.

### 9.3 Suppression logique

Si :

```text
document.record_metadata.deleted_at != null
```

Alors :

- afficher un état “Supprimé” ;
- désactiver les actions de modification ;
- ne pas cacher le document si l’utilisateur a explicitement demandé `include_deleted` ;
- ne jamais proposer de suppression physique.

### 9.4 Audit

Les `AuditEvent` sont append-only. L’UI doit :

- afficher les événements en lecture seule ;
- ne jamais proposer modification ou suppression d’événements ;
- afficher au minimum `occurred_at`, `event_type`, `actor_user_id`, `reason` ;
- pouvoir afficher `before_state` / `after_state` dans un panneau avancé.

### 9.5 Relations typées

Pour créer une relation, l’UI doit :

1. charger les types de relations via `CoreService.ListRelationshipTypes` ;
2. filtrer par `source_kind` et `target_kind` si possible ;
3. afficher le label traduit ou métier ;
4. envoyer le `relationship_type_code`, jamais le label ;
5. afficher le résultat dans `RelationshipTable`.

---

## 10. Composants à générer

### 10.1 Pages

```text
DocumentListPage.vue
DocumentDetailPage.vue
DocumentCreatePage.vue
```

### 10.2 Composants document

```text
DocumentStatusChip.vue
DocumentTypeSelect.vue
DocumentMetadataForm.vue
DocumentFinalizeDialog.vue
DocumentIntegrityPanel.vue
DocumentRelationshipsPanel.vue
DocumentAuditPanel.vue
DocumentSearchFilters.vue
```

### 10.3 Composants core

```text
SubjectIdentityCard.vue
RecordMetadataPanel.vue
AuditTimeline.vue
RelationshipTable.vue
RelationshipTypeSelect.vue
LinkSubjectDialog.vue
```

### 10.4 Composables

```text
useDocumentApi.ts
useCoreApi.ts
useApiErrors.ts
useI18nEnum.ts
```

---

## 11. Détail des composants attendus

### 11.1 `DocumentListPage.vue`

Responsabilités :

- afficher les filtres de `SearchDocumentsRequest` ;
- appeler `DocumentService.SearchDocuments` ;
- afficher les résultats dans `v-data-table-server` ;
- gérer pagination via `page_token` / `next_page_token` ;
- permettre la navigation vers `DocumentDetailPage` ;
- utiliser les traductions pour colonnes, filtres, statuts.

Filtres minimaux :

```text
query
document_type_code
case_id
thing_id
confidentiality_max
only_records
only_final
include_deleted
```

### 11.2 `DocumentDetailPage.vue`

Responsabilités :

- appeler `GetDocument` avec :

```json
{
  "include_relationships": true,
  "include_audit": true
}
```

- afficher :

```text
SubjectIdentityCard
DocumentMetadataForm en readonly ou edit selon état
DocumentIntegrityPanel
DocumentRelationshipsPanel
RecordMetadataPanel
DocumentAuditPanel
```

- gérer les actions :

```text
UpdateDocumentMetadata
FinalizeDocument
VerifyDocumentIntegrity
DeleteDocument
```

### 11.3 `DocumentMetadataForm.vue`

Responsabilités :

- formulaire pour `title`, `description`, `official_date`, `language`, `metadata` ;
- respecter `buf.validate` ;
- ne pas inclure les champs `OUTPUT_ONLY` ;
- mode readonly si document verrouillé ;
- mode création si utilisé par `DocumentCreatePage`.

### 11.4 `DocumentFinalizeDialog.vue`

Responsabilités :

- confirmation explicite ;
- champ `reason` ;
- option `also_lock_governance` ;
- appel `FinalizeDocument` ;
- afficher erreur ou succès ;
- recharger document après succès.

### 11.5 `DocumentIntegrityPanel.vue`

Responsabilités :

- afficher `sha256` ;
- afficher `sha256_verified_at` ;
- afficher `storage_ref` ;
- bouton `VerifyDocumentIntegrity` ;
- état visuel : non vérifié, vérifié, erreur.

### 11.6 `RecordMetadataPanel.vue`

Responsabilités :

- afficher gouvernance : création, modification, suppression logique, propriétaire, org unit, confidentialité, version, verrouillage, conservation ;
- lecture seule par défaut ;
- afficher clairement `is_locked`, `deleted_at`, `confidentiality_level`.

### 11.7 `AuditTimeline.vue`

Responsabilités :

- afficher les événements d’audit ;
- permettre mode compact et mode détaillé ;
- afficher `before_state` et `after_state` dans un bloc JSON repliable ;
- ne jamais permettre modification.

---

## 12. Traduction / i18n

### 12.1 Principe

Ne jamais traduire les codes stockés.

On stocke et transmet :

```text
DOCUMENT_STATUS_FINAL
SUBJECT_KIND_DOCUMENT
PERMISSION_MANAGE
PLAN
CASE_HAS_DOCUMENT
DOCUMENT_REPRESENTS_THING
```

On affiche :

```text
Final
Document
Gérer
Plan
Document rattaché à l’affaire
Document représente un objet
```

### 12.2 Langues initiales

Produire au minimum :

```text
fr-CH.json
en.json
```

La langue par défaut du POC est `fr-CH`.

### 12.3 Convention des clés

Utiliser cette convention :

```text
resources.document.label
resources.document.plural

pages.documents.list.title
pages.documents.detail.title
pages.documents.create.title

fields.document.title
fields.document.description
fields.document.official_date
fields.document.storage_ref
fields.document.external_system
fields.document.external_id
fields.document.external_url
fields.document.mime_type
fields.document.file_size_bytes
fields.document.sha256
fields.document.sha256_verified_at
fields.document.version
fields.document.previous_version_id
fields.document.is_final
fields.document.is_record
fields.document.language
fields.document.page_count
fields.document.status
fields.document.metadata

fields.recordMetadata.created_at
fields.recordMetadata.created_by
fields.recordMetadata.updated_at
fields.recordMetadata.updated_by
fields.recordMetadata.deleted_at
fields.recordMetadata.deleted_by
fields.recordMetadata.owner_user_id
fields.recordMetadata.owner_org_id
fields.recordMetadata.confidentiality_level
fields.recordMetadata.version
fields.recordMetadata.is_locked
fields.recordMetadata.locked_at
fields.recordMetadata.locked_by
fields.recordMetadata.retention_until
fields.recordMetadata.sort_final

sections.document.summary
sections.document.storage
sections.document.integrity
sections.document.relationships
sections.document.governance
sections.document.audit

actions.document.create
actions.document.updateMetadata
actions.document.finalize
actions.document.verifyIntegrity
actions.document.delete
actions.common.save
actions.common.cancel
actions.common.close

enums.DocumentStatus.DOCUMENT_STATUS_UNSPECIFIED
enums.DocumentStatus.DOCUMENT_STATUS_DRAFT
enums.DocumentStatus.DOCUMENT_STATUS_FINAL
enums.DocumentStatus.DOCUMENT_STATUS_SUPERSEDED
enums.DocumentStatus.DOCUMENT_STATUS_ARCHIVED

enums.SubjectKind.SUBJECT_KIND_CASE
enums.SubjectKind.SUBJECT_KIND_DOCUMENT
enums.SubjectKind.SUBJECT_KIND_THING
enums.SubjectKind.SUBJECT_KIND_ACTOR
enums.SubjectKind.SUBJECT_KIND_USER
enums.SubjectKind.SUBJECT_KIND_ORG_UNIT

enums.Permission.PERMISSION_NONE
enums.Permission.PERMISSION_READ
enums.Permission.PERMISSION_CONTRIBUTE
enums.Permission.PERMISSION_MANAGE
enums.Permission.PERMISSION_FULL_CONTROL

validation.required
validation.minLength
validation.maxLength
validation.uuid
validation.sha256
validation.pattern
validation.numberMin
validation.numberMax

messages.document.lockedCannotEdit
messages.document.finalizeConfirm
messages.document.finalizeSuccess
messages.document.integrityVerified
messages.document.integrityFailed
messages.document.deleteConfirm
messages.document.deleted
messages.common.loading
messages.common.error
messages.common.noData
```

### 12.4 Exemple `fr-CH.json`

```json
{
  "resources": {
    "document": {
      "label": "Document",
      "plural": "Documents"
    }
  },
  "pages": {
    "documents": {
      "list": {
        "title": "Documents"
      },
      "detail": {
        "title": "Détail du document"
      },
      "create": {
        "title": "Créer un document"
      }
    }
  },
  "sections": {
    "document": {
      "summary": "Résumé",
      "storage": "Stockage",
      "integrity": "Intégrité",
      "relationships": "Relations",
      "governance": "Gouvernance",
      "audit": "Audit"
    }
  },
  "fields": {
    "document": {
      "title": "Titre",
      "description": "Description",
      "official_date": "Date officielle",
      "document_type": "Type de document",
      "storage_ref": "Référence de stockage",
      "external_system": "Système externe",
      "external_id": "Identifiant externe",
      "external_url": "Lien externe",
      "mime_type": "Type MIME",
      "file_size_bytes": "Taille du fichier",
      "sha256": "Empreinte SHA-256",
      "sha256_verified_at": "Dernière vérification SHA-256",
      "version": "Version",
      "previous_version_id": "Version précédente",
      "is_final": "Finalisé",
      "is_record": "Document probant",
      "language": "Langue",
      "page_count": "Nombre de pages",
      "status": "Statut",
      "metadata": "Métadonnées",
      "only_records": "Seulement les documents probants",
      "only_final": "Seulement les documents finalisés",
      "include_deleted": "Inclure les supprimés"
    },
    "recordMetadata": {
      "created_at": "Créé le",
      "created_by": "Créé par",
      "updated_at": "Modifié le",
      "updated_by": "Modifié par",
      "deleted_at": "Supprimé le",
      "deleted_by": "Supprimé par",
      "owner_user_id": "Utilisateur propriétaire",
      "owner_org_id": "Unité propriétaire",
      "confidentiality_level": "Niveau de confidentialité",
      "version": "Version",
      "is_locked": "Verrouillé",
      "locked_at": "Verrouillé le",
      "locked_by": "Verrouillé par",
      "retention_until": "Conserver jusqu’au",
      "sort_final": "Sort final"
    },
    "search": {
      "query": "Recherche"
    },
    "common": {
      "created_at": "Créé le"
    }
  },
  "actions": {
    "document": {
      "create": "Créer le document",
      "updateMetadata": "Modifier les métadonnées",
      "finalize": "Finaliser le document",
      "verifyIntegrity": "Vérifier l’intégrité",
      "delete": "Supprimer le document"
    },
    "common": {
      "save": "Enregistrer",
      "cancel": "Annuler",
      "close": "Fermer",
      "open": "Ouvrir"
    }
  },
  "enums": {
    "DocumentStatus": {
      "DOCUMENT_STATUS_UNSPECIFIED": "Statut non précisé",
      "DOCUMENT_STATUS_DRAFT": "Brouillon",
      "DOCUMENT_STATUS_FINAL": "Final",
      "DOCUMENT_STATUS_SUPERSEDED": "Remplacé",
      "DOCUMENT_STATUS_ARCHIVED": "Archivé"
    },
    "SubjectKind": {
      "SUBJECT_KIND_UNSPECIFIED": "Type non précisé",
      "SUBJECT_KIND_CASE": "Affaire",
      "SUBJECT_KIND_DOCUMENT": "Document",
      "SUBJECT_KIND_THING": "Objet",
      "SUBJECT_KIND_ACTOR": "Acteur",
      "SUBJECT_KIND_USER": "Utilisateur",
      "SUBJECT_KIND_ORG_UNIT": "Unité organisationnelle"
    },
    "Permission": {
      "PERMISSION_UNSPECIFIED": "Permission non précisée",
      "PERMISSION_NONE": "Aucun accès",
      "PERMISSION_READ": "Lecture",
      "PERMISSION_CONTRIBUTE": "Contribution",
      "PERMISSION_MANAGE": "Gestion",
      "PERMISSION_FULL_CONTROL": "Contrôle complet"
    }
  },
  "validation": {
    "required": "Ce champ est obligatoire.",
    "minLength": "Ce champ doit contenir au moins {min} caractère.",
    "maxLength": "Ce champ ne peut pas dépasser {max} caractères.",
    "uuid": "Identifiant invalide.",
    "sha256": "L’empreinte SHA-256 doit contenir 64 caractères hexadécimaux.",
    "pattern": "Le format de ce champ est invalide.",
    "numberMin": "La valeur doit être supérieure ou égale à {min}.",
    "numberMax": "La valeur doit être inférieure ou égale à {max}."
  },
  "messages": {
    "document": {
      "lockedCannotEdit": "Ce document est verrouillé et ne peut plus être modifié.",
      "finalizeConfirm": "La finalisation rend le document probant. Confirmez-vous cette action ?",
      "finalizeSuccess": "Le document a été finalisé.",
      "integrityVerified": "L’intégrité du document a été vérifiée.",
      "integrityFailed": "L’intégrité du document n’a pas pu être vérifiée.",
      "deleteConfirm": "Le document sera supprimé logiquement. Il restera conservé dans l’historique.",
      "deleted": "Le document a été supprimé logiquement."
    },
    "common": {
      "loading": "Chargement…",
      "error": "Une erreur est survenue.",
      "noData": "Aucune donnée à afficher."
    }
  }
}
```

### 12.5 Exemple `en.json`

```json
{
  "resources": {
    "document": {
      "label": "Document",
      "plural": "Documents"
    }
  },
  "pages": {
    "documents": {
      "list": {
        "title": "Documents"
      },
      "detail": {
        "title": "Document details"
      },
      "create": {
        "title": "Create document"
      }
    }
  },
  "sections": {
    "document": {
      "summary": "Summary",
      "storage": "Storage",
      "integrity": "Integrity",
      "relationships": "Relationships",
      "governance": "Governance",
      "audit": "Audit"
    }
  },
  "fields": {
    "document": {
      "title": "Title",
      "description": "Description",
      "official_date": "Official date",
      "document_type": "Document type",
      "storage_ref": "Storage reference",
      "external_system": "External system",
      "external_id": "External identifier",
      "external_url": "External link",
      "mime_type": "MIME type",
      "file_size_bytes": "File size",
      "sha256": "SHA-256 fingerprint",
      "sha256_verified_at": "Last SHA-256 verification",
      "version": "Version",
      "previous_version_id": "Previous version",
      "is_final": "Finalized",
      "is_record": "Record",
      "language": "Language",
      "page_count": "Page count",
      "status": "Status",
      "metadata": "Metadata",
      "only_records": "Records only",
      "only_final": "Final documents only",
      "include_deleted": "Include deleted"
    }
  },
  "actions": {
    "document": {
      "create": "Create document",
      "updateMetadata": "Update metadata",
      "finalize": "Finalize document",
      "verifyIntegrity": "Verify integrity",
      "delete": "Delete document"
    },
    "common": {
      "save": "Save",
      "cancel": "Cancel",
      "close": "Close",
      "open": "Open"
    }
  },
  "enums": {
    "DocumentStatus": {
      "DOCUMENT_STATUS_UNSPECIFIED": "Unspecified status",
      "DOCUMENT_STATUS_DRAFT": "Draft",
      "DOCUMENT_STATUS_FINAL": "Final",
      "DOCUMENT_STATUS_SUPERSEDED": "Superseded",
      "DOCUMENT_STATUS_ARCHIVED": "Archived"
    },
    "SubjectKind": {
      "SUBJECT_KIND_UNSPECIFIED": "Unspecified kind",
      "SUBJECT_KIND_CASE": "Case",
      "SUBJECT_KIND_DOCUMENT": "Document",
      "SUBJECT_KIND_THING": "Thing",
      "SUBJECT_KIND_ACTOR": "Actor",
      "SUBJECT_KIND_USER": "User",
      "SUBJECT_KIND_ORG_UNIT": "Organizational unit"
    }
  },
  "validation": {
    "required": "This field is required.",
    "minLength": "This field must contain at least {min} character.",
    "maxLength": "This field must not exceed {max} characters.",
    "uuid": "Invalid identifier.",
    "sha256": "The SHA-256 fingerprint must contain 64 hexadecimal characters.",
    "pattern": "Invalid field format.",
    "numberMin": "The value must be greater than or equal to {min}.",
    "numberMax": "The value must be less than or equal to {max}."
  }
}
```

---

## 13. Gestion de la langue du document

Attention : le champ suivant dans `Document` :

```text
language
```

ne représente pas la langue de l’interface. Il représente la langue du document lui-même.

Exemples :

```text
Interface utilisateur : fr-CH
Document.language : de
```

Cela signifie : l’utilisateur travaille en français, mais le document est en allemand.

L’agent doit donc distinguer :

```text
locale UI              → fr-CH, en, de-CH, etc.
Document.language      → fr, de, it, en, etc.
langue de contenu OCR  → future extension
```

---

## 14. Traduction des catalogues métier

Les enums sont traduits via fichiers i18n.

Les catalogues métier comme `DocumentType` et `RelationshipType` ont déjà :

```text
code
label
description
```

Pour le POC :

- afficher `label` depuis l’API si disponible ;
- sinon utiliser une clé i18n dérivée du code ;
- ne jamais envoyer le label à l’API ;
- toujours envoyer le `code`.

Convention de fallback :

```text
catalog.documentType.PLAN.label
catalog.documentType.INCOMING_LETTER.label
catalog.relationshipType.CASE_HAS_DOCUMENT.label
catalog.relationshipType.DOCUMENT_REPRESENTS_THING.label
```

À moyen terme, prévoir des tables de traduction côté backend :

```text
document_type_translation
relationship_type_translation
case_type_translation
thing_type_translation
```

---

## 15. Gestion des erreurs API et validations

### 15.1 Validation côté client

Utiliser les annotations `buf.validate` pour générer des règles Vuetify :

```text
min_len     → minLengthRule
max_len     → maxLengthRule
uuid        → uuidRule
pattern     → patternRule
int32.gte   → numberMinRule
int32.lte   → numberMaxRule
```

### 15.2 Validation côté serveur

Ne jamais afficher directement les messages techniques de Protobuf/protovalidate.

L’agent doit créer un mapping :

```text
violation.required  → validation.required
violation.min_len   → validation.minLength
violation.max_len   → validation.maxLength
violation.uuid      → validation.uuid
violation.pattern   → validation.pattern
```

### 15.3 Erreurs métier

Prévoir des messages propres pour :

```text
permission denied
record locked
not found
invalid relationship type
duplicate active relationship
integrity check failed
soft-deleted record
```

---

## 16. Règles de génération TypeScript

### 16.1 Contraintes générales

- TypeScript strict.
- Pas de `any` sauf justification locale.
- Pas de logique API dans les composants visuels.
- Les appels API passent par des composables.
- Les composants reçoivent des props typées.
- Les composants émettent des événements explicites.
- Les textes visibles passent par `t(...)`.
- Les enums sont affichés via une fonction i18n, jamais directement.

### 16.2 Exemple utilitaire enum

```ts
export function useI18nEnum() {
  const { t } = useI18n()

  function enumLabel(enumName: string, value: string): string {
    const key = `enums.${enumName}.${value}`
    const translated = t(key)
    return translated === key ? value : translated
  }

  return { enumLabel }
}
```

### 16.3 Exemple règle readonly

```ts
export function isDocumentEditable(document: Document): boolean {
  return !document.isFinal &&
    !document.recordMetadata?.isLocked &&
    !document.recordMetadata?.deletedAt
}
```

Adapter les noms exacts selon le générateur TypeScript utilisé.

---

## 17. Responsive design Vuetify

L’UI doit être responsive dès le départ.

### 17.1 Desktop

- liste documents en table ;
- détail en sections/tabs ;
- panneaux latéraux possibles pour gouvernance/audit.

### 17.2 Mobile

- éviter les tables trop larges ;
- afficher les documents en cards si nécessaire ;
- utiliser `v-expansion-panels` pour les sections ;
- garder les actions critiques visibles mais protégées par dialog.

### 17.3 Grille recommandée

```vue
<v-container fluid>
  <v-row>
    <v-col cols="12" md="8">
      <!-- contenu principal -->
    </v-col>
    <v-col cols="12" md="4">
      <!-- gouvernance / audit compact -->
    </v-col>
  </v-row>
</v-container>
```

---

## 18. Qualité UX attendue

Interface :

- sobre ;
- administrative ;
- lisible ;
- responsive ;
- cohérente ;
- pas “gadget” ;
- pas de design exotique ;
- priorité à la compréhension métier.

Les états métier doivent être visibles :

```text
Brouillon
Final
Archivé
Remplacé
Verrouillé
Supprimé logiquement
Document probant
Confidentiel
```

Les actions critiques doivent être protégées :

```text
Finaliser
Supprimer logiquement
Délier une relation
```

---

## 19. Livrables attendus

L’agent doit produire :

```text
1. src/schemas/core.ui.schema.json
2. src/schemas/document.ui.schema.json
3. src/locales/fr-CH.json
4. src/locales/en.json
5. src/plugins/i18n.ts
6. src/plugins/vuetify.ts
7. src/api/client.ts
8. src/api/documentClient.ts
9. src/api/coreClient.ts
10. src/composables/useDocumentApi.ts
11. src/composables/useCoreApi.ts
12. src/composables/useApiErrors.ts
13. src/composables/useI18nEnum.ts
14. src/components/core/*
15. src/components/document/*
16. src/pages/documents/*
17. src/router/index.ts
18. tests minimaux ou stories de démonstration
```

---

## 20. Ordre d’exécution demandé à l’agent

L’agent doit travailler dans cet ordre strict :

```text
Étape 1 — Lire core.proto et document.proto.
Étape 2 — Résumer les messages, enums, services et champs importants.
Étape 3 — Générer core.ui.schema.json.
Étape 4 — Générer document.ui.schema.json.
Étape 5 — Générer les clés i18n fr-CH et en.
Étape 6 — Générer les clients/composables API.
Étape 7 — Générer les composants core.
Étape 8 — Générer les composants document.
Étape 9 — Générer les pages document.
Étape 10 — Ajouter la gestion des erreurs et validations.
Étape 11 — Ajouter tests ou stories minimales.
Étape 12 — Produire une revue critique de l’UI générée.
```

Ne pas passer à l’étape 6 tant que les fichiers `*.ui.schema.json` ne sont pas produits.

---

## 21. Prompt principal à donner à Codex ou Claude

```text
Tu es un agent de codage senior spécialisé Vue 3, TypeScript, Vuetify, Protobuf, ConnectRPC et applications métier administratives.

Contexte :
Nous construisons le frontend d’un POC Goéland. Goéland est un système de gestion d’affaires administratives fondé sur des sujets métier durables, des relations typées, des documents probants, des droits, de la non-destruction, du verrouillage et un audit append-only.

Entrées :
- core.proto
- document.proto

Objectif :
Créer une base frontend Vue 3 + Vuetify + TypeScript strict pour le module Document et les composants transversaux Core.

Règles impératives :
1. Ne génère pas directement les composants depuis les proto.
2. Commence par produire :
   - src/schemas/core.ui.schema.json
   - src/schemas/document.ui.schema.json
3. Utilise ces schémas intermédiaires pour générer les composants Vue/Vuetify.
4. Tous les textes visibles doivent passer par vue-i18n.
5. Produis au minimum :
   - src/locales/fr-CH.json
   - src/locales/en.json
6. Ne traduis jamais les codes stockés ou transmis à l’API.
7. Traduis uniquement l’affichage des enums, labels, actions, sections, validations et messages.
8. Respecte :
   - google.api.field_behavior = REQUIRED
   - google.api.field_behavior = OUTPUT_ONLY
   - buf.validate
   - record_metadata.is_locked
   - record_metadata.deleted_at
   - document.is_final
   - document.is_record
9. Les actions critiques doivent utiliser un dialog de confirmation.
10. Ne mets pas de logique API dans les composants visuels.
11. Utilise des composables API.
12. L’UI doit être responsive, sobre, administrative et lisible.
13. Les AuditEvent sont en lecture seule.
14. La suppression est logique, jamais physique côté UI.
15. Les relations doivent utiliser relationship_type_code, pas le label.

Livrables :
- UI schemas JSON
- traductions fr-CH et en
- composants Core
- composants Document
- pages DocumentListPage, DocumentDetailPage, DocumentCreatePage
- composables API
- utilitaires de validation
- gestion d’erreurs
- tests ou stories minimales

Commence par analyser les proto et produire les deux fichiers ui.schema.json. Ensuite seulement, génère le code.
```

---

## 22. Checklist de revue automatique

Après génération, l’agent doit répondre à cette checklist :

```text
[ ] Les composants ne contiennent aucun texte utilisateur hardcodé.
[ ] Les enums sont affichés via i18n.
[ ] Les codes métier sont transmis tels quels à l’API.
[ ] Les champs REQUIRED sont obligatoires dans les formulaires.
[ ] Les champs OUTPUT_ONLY ne sont pas éditables.
[ ] Les contraintes buf.validate sont reprises côté UI.
[ ] Les documents verrouillés ne sont pas modifiables.
[ ] Les documents finalisés ne proposent plus l’action de finalisation.
[ ] Les documents supprimés logiquement sont affichés comme tels.
[ ] Les actions critiques passent par confirmation.
[ ] AuditEvent est strictement en lecture seule.
[ ] RelationshipType utilise code pour l’API et label pour l’affichage.
[ ] La liste des documents est responsive.
[ ] Le détail document est utilisable sur mobile.
[ ] Les erreurs API sont traduites ou humanisées.
[ ] La langue du document est distincte de la langue de l’interface.
```

---

## 23. Non-objectifs

Ne pas implémenter maintenant :

```text
éditeur cartographique complet
GED complète avec upload binaire
preview PDF avancée
workflow BPMN
IA documentaire
OpenFGA/Casbin complet
moteur de recherche avancé
édition fine des catalogues métier
traduction automatique du contenu documentaire
```

Le but est de poser une base propre, pas de tout résoudre.

---

## 24. Résultat attendu

À la fin, on doit pouvoir lancer une application Vue/Vuetify qui démontre :

```text
Recherche de documents
Création d’un document
Affichage d’un document
Modification de métadonnées autorisées
Finalisation d’un document
Vérification SHA-256
Affichage de relations
Affichage de l’audit
Respect du verrouillage et de la non-destruction
Support fr-CH / en
```

La finalité n’est pas seulement de produire une UI. La finalité est de démontrer que les contrats Protobuf Goéland peuvent alimenter une interface métier propre, traduisible, contrôlée, responsive et durable.

