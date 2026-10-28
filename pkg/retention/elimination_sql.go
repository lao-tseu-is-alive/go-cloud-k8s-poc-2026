package retention

// Raw SQL of the elimination of cases (GLD-077). Every statement runs in the
// transaction of an approved DESTROY bordereau, after enableDisposalSQL: the
// immutability triggers then accept the blanking of content (migration 0037).

// enableDisposalSQL declares the elimination for the rest of the transaction only.
const enableDisposalSQL = `SELECT set_config('goeland.disposal', 'on', true);`

// eliminatedLabel prefixes the notice label of an eliminated case.
const eliminatedLabel = "Affaire éliminée"

// eliminatedText replaces a required text of an eliminated record.
const eliminatedText = "[éliminé]"

// eliminatedDocumentLabel is the label of an eliminated document.
const eliminatedDocumentLabel = "Document éliminé"

// caseDocumentsSQL lists the documents linked to a case by a live edge, with
// whether each must be kept: still linked to a thing, or to a case that is
// neither being eliminated (@eliminated) nor already eliminated.
const caseDocumentsSQL = `
SELECT d.id,
       EXISTS (
           SELECT 1 FROM subject_relationship o
           JOIN subject_ref other ON other.id = CASE WHEN o.source_subject_id = d.id THEN o.target_subject_id ELSE o.source_subject_id END
           WHERE (o.source_subject_id = d.id OR o.target_subject_id = d.id) AND o.deleted_at IS NULL
             AND (other.kind = 'THING'
                  OR (other.kind = 'CASE' AND other.id <> ALL(@eliminated::uuid[])
                      AND NOT EXISTS (SELECT 1 FROM subject_retention x WHERE x.subject_id = other.id AND x.disposition_status = 'DISPOSED')))
       ) AS kept
FROM subject_relationship r
JOIN subject_ref d ON d.kind = 'DOCUMENT' AND d.id = CASE WHEN r.source_subject_id = @case_id THEN r.target_subject_id ELSE r.source_subject_id END
WHERE (r.source_subject_id = @case_id OR r.target_subject_id = @case_id) AND r.deleted_at IS NULL
GROUP BY d.id;`

// blankCaseSQL turns the case into its notice; the label keeps the reference.
const blankCaseSQL = `
UPDATE case_file SET title = @label, description = '', closure_reason = '', metadata = '{}'::jsonb WHERE id = @id;`

// blankSubjectSQL renames a subject and blanks its governance metadata.
const blankSubjectSQL = `
WITH s AS (UPDATE subject_ref SET display_label = @label, canonical_url = '' WHERE id = @id RETURNING id)
UPDATE record_metadata SET metadata = '{}'::jsonb WHERE subject_id IN (SELECT id FROM s);`

// caseLabelSQL builds the notice label of a case from its reference.
const caseLabelSQL = `
SELECT CASE WHEN business_ref <> '' THEN @prefix || ' ' || business_ref ELSE @prefix END FROM subject_ref WHERE id = @id;`

// blankCaseContentSQL blanks the timeline, tasks, assignments and circulations of a case.
const blankCaseContentSQL = `
WITH t AS (
    UPDATE case_timeline_entry SET title = '', body = @text, metadata = '{}'::jsonb, withdrawal_reason = '' WHERE case_id = @id RETURNING 1
), k AS (
    UPDATE case_task SET title = @text, description = '', completion_note = '', cancellation_reason = '' WHERE case_id = @id RETURNING id
), a AS (
    UPDATE case_task_assignment SET reason = '' WHERE task_id IN (SELECT id FROM case_task WHERE case_id = @id) RETURNING 1
), c AS (
    UPDATE case_circulation SET title = @text, message = '', cancellation_reason = '' WHERE case_id = @id RETURNING id
)
UPDATE case_circulation_recipient SET response_text = ''
WHERE circulation_id IN (SELECT id FROM case_circulation WHERE case_id = @id);`

// blankEdgesSQL blanks the role details of the edges of the given subjects.
const blankEdgesSQL = `
UPDATE subject_relationship SET role_detail = ''
WHERE source_subject_id = ANY(@ids::uuid[]) OR target_subject_id = ANY(@ids::uuid[]);`

// blankAuditSQL replaces the states and reasons of the audit events of the
// given subjects by a marker; who did what and when stays.
const blankAuditSQL = `
UPDATE audit_event SET before_state = NULL, after_state = '{"eliminated": true}'::jsonb, reason = ''
WHERE subject_id = ANY(@ids::uuid[]);`

// blankDocumentsSQL blanks the documents only the case held and logically
// deletes them (they leave every list); their versions' metadata too.
const blankDocumentsSQL = `
WITH d AS (
    UPDATE document SET title = @label, description = '', metadata = '{}'::jsonb,
        external_system = '', external_id = '', external_url = ''
    WHERE id = ANY(@ids::uuid[]) RETURNING id
), v AS (
    UPDATE document_version SET metadata = '{}'::jsonb WHERE document_id IN (SELECT id FROM d) RETURNING 1
)
UPDATE record_metadata SET deleted_at = coalesce(deleted_at, now()), deleted_by = CASE WHEN deleted_at IS NULL THEN @operator_id ELSE deleted_by END
WHERE subject_id IN (SELECT id FROM d);`

// disposeBlobsSQL marks destroyed the blobs of the eliminated documents that
// no other document version references, and returns where their bytes are.
const disposeBlobsSQL = `
WITH doomed AS (
    SELECT b.id, b.storage_ref FROM content_blob b
    WHERE b.disposed_at IS NULL
      AND b.id IN (SELECT v.content_blob_id FROM document_version v WHERE v.document_id = ANY(@ids::uuid[]))
      AND NOT EXISTS (SELECT 1 FROM document_version o WHERE o.content_blob_id = b.id AND o.document_id <> ALL(@ids::uuid[]))
    FOR UPDATE OF b
)
UPDATE content_blob b SET disposed_at = now(), storage_ref = ''
FROM doomed WHERE b.id = doomed.id
RETURNING doomed.storage_ref;`

// purgeExportsSQL ends the ordinary exports of an eliminated case and returns
// where their archives are.
const purgeExportsSQL = `
WITH old AS (
    SELECT id, storage_ref, status FROM case_export WHERE case_id = @id AND status IN ('QUEUED', 'RUNNING', 'DONE') FOR UPDATE
)
UPDATE case_export x
SET status = CASE WHEN old.status = 'DONE' THEN 'EXPIRED' ELSE 'FAILED' END,
    storage_ref = '', error = CASE WHEN old.status = 'DONE' THEN x.error ELSE 'the case was eliminated' END,
    finished_at = coalesce(x.finished_at, now())
FROM old WHERE x.id = old.id
RETURNING old.storage_ref;`
