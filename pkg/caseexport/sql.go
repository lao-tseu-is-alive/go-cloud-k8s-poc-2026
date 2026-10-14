package caseexport

// Raw SQL of the case export jobs (table case_export, alias x).

const exportColumns = `
x.id, x.case_id, x.requested_by, x.reason, x.status, x.requested_at, x.started_at, x.finished_at,
x.expires_at, x.attempts, x.storage_ref, x.sha256, x.size_bytes, x.error`

const insertExportSQL = `
INSERT INTO case_export AS x (case_id, requested_by, reason, request_id)
VALUES (@case_id, @requested_by, @reason, @request_id)
RETURNING ` + exportColumns + `;`

const getExportSQL = `SELECT ` + exportColumns + ` FROM case_export x WHERE x.id = @id;`

// listExportsSQL lists the exports of a case, newest first; an empty
// @requested_by lists every requester's (administrators).
const listExportsSQL = `
SELECT ` + exportColumns + `
FROM case_export x
WHERE x.case_id = @case_id AND (@requested_by = '' OR x.requested_by = @requested_by)
ORDER BY x.requested_at DESC, x.id DESC
LIMIT @limit;`

// claimExportSQL takes the oldest queued job, or a RUNNING one whose worker
// went silent for longer than @stale_after (with attempts left), and marks it
// RUNNING. SKIP LOCKED lets the workers of several replicas share the queue.
const claimExportSQL = `
UPDATE case_export AS x
SET status = 'RUNNING', started_at = now(), attempts = x.attempts + 1
WHERE x.id = (
    SELECT q.id FROM case_export q
    WHERE q.status = 'QUEUED'
       OR (q.status = 'RUNNING' AND q.attempts < @max_attempts AND q.started_at < now() - @stale_after::interval)
    ORDER BY q.requested_at, q.id
    LIMIT 1
    FOR UPDATE SKIP LOCKED)
RETURNING ` + exportColumns + `;`

// finishExportSQL records a built archive.
const finishExportSQL = `
UPDATE case_export
SET status = 'DONE', finished_at = now(), expires_at = now() + @ttl::interval,
    storage_ref = @storage_ref, sha256 = @sha256, size_bytes = @size_bytes, error = ''
WHERE id = @id AND status = 'RUNNING';`

// failExportSQL records a failed build.
const failExportSQL = `
UPDATE case_export
SET status = 'FAILED', finished_at = now(), error = @error
WHERE id = @id AND status = 'RUNNING';`

// requeueExportSQL puts back a job whose build was interrupted by a shutdown.
const requeueExportSQL = `
UPDATE case_export
SET status = 'QUEUED', started_at = NULL, attempts = greatest(attempts - 1, 0)
WHERE id = @id AND status = 'RUNNING';`

// failExhaustedSQL gives up on the jobs whose builds kept dying.
const failExhaustedSQL = `
UPDATE case_export
SET status = 'FAILED', finished_at = now(), error = 'the export was interrupted too many times'
WHERE status = 'RUNNING' AND attempts >= @max_attempts AND started_at < now() - @stale_after::interval;`

// expiredExportsSQL lists the built archives past their time to live.
const expiredExportsSQL = `
SELECT ` + exportColumns + `
FROM case_export x
WHERE x.status = 'DONE' AND x.expires_at <= now()
ORDER BY x.expires_at
LIMIT 100;`

// expireExportSQL marks an archive removed.
const expireExportSQL = `
UPDATE case_export
SET status = 'EXPIRED', storage_ref = ''
WHERE id = @id AND status = 'DONE';`

// countRelationshipsSQL counts the non-unlinked edges of a case in both
// directions, as core.ListRelationships sees them before the read filter.
const countRelationshipsSQL = `
SELECT count(*) FROM subject_relationship
WHERE deleted_at IS NULL AND (source_subject_id = @case_id OR target_subject_id = @case_id);`

// countTimelineSQL counts every entry of a case timeline, withdrawn included.
const countTimelineSQL = `SELECT count(*) FROM case_timeline_entry WHERE case_id = @case_id;`

// listProvenanceSQL reads the legacy origin of the exported subjects.
const listProvenanceSQL = `
SELECT subject_id, source_system, source_table, source_id, import_batch_id
FROM subject_provenance
WHERE subject_id = ANY(@ids::uuid[])
ORDER BY subject_id;`
