package timeline

// SQL fragments for the timeline repository. Column projections are the single
// source of truth for pgx named scanning (columns map to `db` tags); they are
// alias-prefixed, so INSERT statements alias their target (AS e).

const entryColumns = `
e.id, e.case_id, e.entry_type, e.status, e.title, e.body, e.visibility, e.occurred_at,
e.corrects_entry_id, e.metadata, e.created_at, e.created_by, e.updated_at, e.updated_by,
e.validated_at, e.validated_by, e.locked_at, e.locked_by,
e.withdrawn_at, e.withdrawn_by, e.withdrawal_reason`

// readEntryColumns adds the live correction, if any (served by the partial
// unique index on corrects_entry_id).
const readEntryColumns = entryColumns + `,
(SELECT c.id FROM case_timeline_entry c
 WHERE c.corrects_entry_id = e.id AND c.status <> 4) AS corrected_by_entry_id`

// insertEntrySQL creates a draft; a nil occurred_at means now.
const insertEntrySQL = `
INSERT INTO case_timeline_entry AS e
    (case_id, entry_type, title, body, visibility, occurred_at, corrects_entry_id, created_by)
VALUES (@case_id, @entry_type, @title, @body, @visibility,
        coalesce(@occurred_at::timestamptz, now()), @corrects_entry_id, @created_by)
RETURNING ` + entryColumns + `;`

// insertSystemEntrySQL records a server-written fact, born LOCKED.
const insertSystemEntrySQL = `
INSERT INTO case_timeline_entry AS e
    (case_id, entry_type, status, title, body, metadata, created_by, locked_at, locked_by)
VALUES (@case_id, 7, 3, @title, @body, @metadata, @operator_id, now(), @operator_id)
RETURNING ` + entryColumns + `;`

const getEntrySQL = `
SELECT ` + readEntryColumns + `
FROM case_timeline_entry e
WHERE e.id = @id;`

// getEntryForUpdateSQL locks the entry row for a check-then-mutate on its status.
const getEntryForUpdateSQL = `
SELECT ` + entryColumns + `
FROM case_timeline_entry e
WHERE e.id = @id
FOR UPDATE;`

// listEntriesSQL pages through one case timeline, most recent business date first.
const listEntriesSQL = `
SELECT ` + readEntryColumns + `,
COUNT(*) OVER() AS total_count
FROM case_timeline_entry e
WHERE e.case_id = @case_id
  AND (cardinality(@entry_types::smallint[]) = 0 OR e.entry_type = ANY(@entry_types::smallint[]))
  AND (@include_withdrawn OR e.status <> 4)
ORDER BY e.occurred_at DESC, e.created_at DESC, e.id
LIMIT @limit OFFSET @offset;`

// updateEntrySQL replaces a draft's content; a nil occurred_at keeps the current date.
const updateEntrySQL = `
UPDATE case_timeline_entry e
SET entry_type = @entry_type, title = @title, body = @body, visibility = @visibility,
    occurred_at = coalesce(@occurred_at::timestamptz, e.occurred_at),
    updated_at = now(), updated_by = @operator_id
WHERE e.id = @id
RETURNING ` + entryColumns + `;`

const validateEntrySQL = `
UPDATE case_timeline_entry e
SET status = 2, validated_at = now(), validated_by = @operator_id
WHERE e.id = @id
RETURNING ` + entryColumns + `;`

const lockEntrySQL = `
UPDATE case_timeline_entry e
SET status = 3, locked_at = now(), locked_by = @operator_id
WHERE e.id = @id
RETURNING ` + entryColumns + `;`

const withdrawEntrySQL = `
UPDATE case_timeline_entry e
SET status = 4, withdrawn_at = now(), withdrawn_by = @operator_id, withdrawal_reason = @reason
WHERE e.id = @id
RETURNING ` + entryColumns + `;`

// liveCorrectionExistsSQL tells whether an entry already has a live correction.
const liveCorrectionExistsSQL = `
SELECT EXISTS (
    SELECT 1 FROM case_timeline_entry
    WHERE corrects_entry_id = @id AND status <> 4
);`

const countDraftsSQL = `
SELECT count(*) FROM case_timeline_entry
WHERE case_id = @case_id AND status = 1;`

// caseStatusSQL reads the case status; callers first lock the case's
// governance row (core.EnsureMutableTx), which serializes status changes.
const caseStatusSQL = `
SELECT status FROM case_file
WHERE id = @id;`

// --- document links -----------------------------------------------------------------

// listLinksSQL returns the live links of several entries with the document
// label and the pinned version number, oldest first.
const listLinksSQL = `
SELECT l.id, l.timeline_entry_id, l.document_id, sr.display_label AS document_label,
       l.document_version_id, dv.version_no, l.created_at, l.created_by
FROM timeline_document_link l
JOIN subject_ref sr ON sr.id = l.document_id
LEFT JOIN document_version dv ON dv.id = l.document_version_id
WHERE l.timeline_entry_id = ANY(@entry_ids::uuid[]) AND l.removed_at IS NULL
ORDER BY l.created_at, l.id;`

const insertLinkSQL = `
INSERT INTO timeline_document_link (timeline_entry_id, document_id, created_by)
VALUES (@entry_id, @document_id, @operator_id)
RETURNING id;`

const removeLinkSQL = `
UPDATE timeline_document_link
SET removed_at = now(), removed_by = @operator_id
WHERE timeline_entry_id = @entry_id AND document_id = @document_id AND removed_at IS NULL
RETURNING id;`

// pinVersionsSQL pins the current version of every cited document; it runs
// while the entry is still a draft, just before it is validated or locked.
const pinVersionsSQL = `
UPDATE timeline_document_link l
SET document_version_id = d.current_version_id
FROM document d
WHERE d.id = l.document_id AND l.timeline_entry_id = @entry_id AND l.removed_at IS NULL;`

// caseHasDocumentSQL tells whether an open CASE_HAS_DOCUMENT edge links the
// case to the document (same openness rule as the relationship unique index).
const caseHasDocumentSQL = `
SELECT EXISTS (
    SELECT 1
    FROM subject_relationship r
    JOIN relationship_type rt ON rt.id = r.relationship_type_id
    WHERE rt.code = 'CASE_HAS_DOCUMENT'
      AND r.source_subject_id = @case_id AND r.target_subject_id = @document_id
      AND r.deleted_at IS NULL AND r.valid_to IS NULL
);`
