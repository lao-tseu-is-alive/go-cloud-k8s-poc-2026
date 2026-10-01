package legacyimport

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const (
	keyTimelineEntry = "TIMELINE_ENTRY"
	// Timeline values (timeline.EntryType, Status and Visibility, kept as numbers).
	entryTypeComment      = 1
	entryStatusDraft      = 1
	entryStatusValidated  = 2
	entryStatusLocked     = 3
	visibilityAllInvolved = 1
	// maxEntryBody is the POC limit on an entry body (timeline.MaxBodyLength).
	maxEntryBody = 20000
)

var (
	timelineEntryColumns = []string{"id", "case_id", "entry_type", "status", "body", "visibility", "occurred_at", "metadata", "created_at", "created_by"}
	entryLinkColumns     = []string{"timeline_entry_id", "document_id", "document_version_id", "created_at", "created_by"}
	entryFinalColumns    = []string{"id", "status", "at", "by"}
)

// importTimelineEntries writes the case follow-ups as COMMENT entries visible
// to everyone involved, first as drafts (the documents of an entry can only be
// cited while it is a draft): their final status is set by
// importTimelineFinalStatus once the documents are cited.
func (imp *Importer) importTimelineEntries(ctx context.Context, c *StageCounts) error {
	n, err := imp.copyFromSource(ctx, "case_timeline_entry", timelineEntryColumns, timelineEntriesSQL, func(rows pgx.Rows) ([]any, error) {
		var id, caseID int64
		var creator *int64
		var body string
		var occurred, created *time.Time
		var color string
		if err := rows.Scan(&id, &caseID, &creator, &body, &occurred, &created, &color); err != nil {
			return nil, err
		}
		c.Read++
		if strings.TrimSpace(body) == "" {
			c.skip("empty follow-up")
			return nil, nil
		}
		caseUUID := ID(string(core.SubjectKindCase), caseID)
		if !imp.known(caseUUID, core.SubjectKindCase) {
			c.skip("case not imported")
			return nil, nil
		}
		if len([]rune(body)) > maxEntryBody {
			body = string([]rune(body)[:maxEntryBody-1]) + "…"
			c.adjust("body longer than 20000 characters, shortened")
		}
		entryID := ID(keyTimelineEntry, id)
		imp.entries[entryID] = true
		metadata := map[string]any{}
		if color != "" {
			metadata["legacy_color"] = color
		}
		return []any{entryID, caseUUID, int16(entryTypeComment), int16(entryStatusDraft), body, int16(visibilityAllInvolved),
			firstTime(occurred, created, &imp.now), metadata, firstTime(created, &imp.now), imp.operator(creator)}, nil
	})
	c.Written = n
	return err
}

// importTimelineDocuments cites the documents linked to the follow-ups,
// pinning their single imported version.
func (imp *Importer) importTimelineDocuments(ctx context.Context, c *StageCounts) error {
	seen := map[[2]uuid.UUID]bool{}
	n, err := imp.copyFromSource(ctx, "timeline_document_link", entryLinkColumns, timelineDocumentsSQL, func(rows pgx.Rows) ([]any, error) {
		var entry, doc int64
		if err := rows.Scan(&entry, &doc); err != nil {
			return nil, err
		}
		c.Read++
		entryID, docID := ID(keyTimelineEntry, entry), ID(string(core.SubjectKindDocument), doc)
		if !imp.entries[entryID] || !imp.known(docID, core.SubjectKindDocument) {
			c.skip("entry or document not imported")
			return nil, nil
		}
		key := [2]uuid.UUID{entryID, docID}
		if seen[key] {
			c.skip("repeated citation")
			return nil, nil
		}
		seen[key] = true
		return []any{entryID, docID, ID(keyDocumentVersion, doc), imp.now, OperatorID}, nil
	})
	c.Written = n
	return err
}

// importTimelineFinalStatus gives the follow-ups their final status: validated
// in the legacy → VALIDATED, locked (affaire_suivi_verrou) or belonging to a
// closed case → LOCKED; the others stay drafts.
func (imp *Importer) importTimelineFinalStatus(ctx context.Context, c *StageCounts) error {
	if _, err := imp.tx.Exec(ctx, createEntryFinalStagingSQL); err != nil {
		return fmt.Errorf("create entry status staging: %w", err)
	}
	n, err := imp.copyFromSource(ctx, "import_entry_final", entryFinalColumns, timelineStatusSQL, func(rows pgx.Rows) ([]any, error) {
		var id, caseID int64
		var validatedAt, lockedAt *time.Time
		var validator, locker *int64
		if err := rows.Scan(&id, &caseID, &validatedAt, &validator, &lockedAt, &locker); err != nil {
			return nil, err
		}
		c.Read++
		entryID := ID(keyTimelineEntry, id)
		if !imp.entries[entryID] {
			return nil, nil
		}
		return imp.finalStatusRow(entryID, ID(string(core.SubjectKindCase), caseID), validatedAt, validator, lockedAt, locker, c), nil
	})
	if err != nil {
		return err
	}
	if _, err := imp.tx.Exec(ctx, applyEntryFinalStatusSQL); err != nil {
		return fmt.Errorf("apply entry status: %w", err)
	}
	c.Written = n
	return nil
}

// finalStatusRow is the final status of one entry, or nil to keep it a draft.
func (imp *Importer) finalStatusRow(entryID, caseID uuid.UUID, validatedAt *time.Time, validator *int64, lockedAt *time.Time, locker *int64, c *StageCounts) []any {
	switch {
	case validatedAt != nil:
		c.adjust("validated")
		return []any{entryID, int16(entryStatusValidated), *validatedAt, imp.operator(validator)}
	case lockedAt != nil:
		c.adjust("locked")
		return []any{entryID, int16(entryStatusLocked), *lockedAt, imp.operator(locker)}
	}
	if closed, ok := imp.caseClosedAt[caseID]; ok {
		c.adjust("locked with its closed case")
		return []any{entryID, int16(entryStatusLocked), closed, OperatorID}
	}
	c.adjust("kept as a draft (open case)")
	return nil
}

// caseThings, caseLinks and the other wave 2 link tables.
var (
	caseThingLink    = link{typeCode: "CASE_CONCERNS_THING", sourceKind: core.SubjectKindCase, targetKind: core.SubjectKindThing, edgesSQL: caseThingsSQL}
	caseDocumentLink = link{typeCode: "CASE_HAS_DOCUMENT", sourceKind: core.SubjectKindCase, targetKind: core.SubjectKindDocument, edgesSQL: caseDocumentsSQL}
	thingDocLink     = link{typeCode: "DOCUMENT_REPRESENTS_THING", sourceKind: core.SubjectKindDocument, targetKind: core.SubjectKindThing, edgesSQL: thingDocumentsSQL}
	parentCaseLink   = link{typeCode: "CASE_PARENT_OF_CASE", sourceKind: core.SubjectKindCase, targetKind: core.SubjectKindCase, edgesSQL: parentCasesSQL}
	relatedCaseLink  = link{typeCode: "CASE_RELATED_TO_CASE", sourceKind: core.SubjectKindCase, targetKind: core.SubjectKindCase, edgesSQL: relatedCasesSQL}

	thingActorRoles = roleFamily{
		prefix: "THING_HAS_ACTOR_", sourceKind: core.SubjectKindThing, targetKind: core.SubjectKindActor,
		rolesSQL: thingActorRoleNamesSQL, edgesSQL: thingActorRolesSQL,
		existing: map[string]string{"Propriétaire": "THING_HAS_ACTOR_OWNER"},
		label:    "Objet a %s", inverse: "Acteur %s de l'objet",
	}
	documentActorRoles = roleFamily{
		prefix: "DOCUMENT_HAS_ACTOR_", sourceKind: core.SubjectKindDocument, targetKind: core.SubjectKindActor,
		rolesSQL: documentActorRoleNamesSQL, edgesSQL: documentActorRolesSQL,
		existing: map[string]string{"Auteur": "DOCUMENT_AUTHORED_BY_ACTOR"},
		label:    "Document a %s", inverse: "Acteur %s du document",
	}
)

// linkStage wraps a link table as a stage.
func (imp *Importer) linkStage(l link) func(context.Context, *StageCounts) error {
	return func(ctx context.Context, c *StageCounts) error { return imp.importLink(ctx, l, c) }
}

// roleStage wraps a role family as a stage.
func (imp *Importer) roleStage(f roleFamily) func(context.Context, *StageCounts) error {
	return func(ctx context.Context, c *StageCounts) error { return imp.importRoles(ctx, f, c) }
}
