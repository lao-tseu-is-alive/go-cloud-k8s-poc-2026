package retention

// Raw SQL of the disposition bordereaux (disposition_bordereau, alias b) and
// their items (disposition_item, alias i).

const bordereauColumns = `b.id, b.final_disposition, b.status, b.reason, b.prepared_by, b.prepared_at,
b.closed_at, b.closed_by, b.close_reason, b.proof_sha256,
(SELECT count(*) FROM disposition_item c WHERE c.bordereau_id = b.id)::int AS item_count,
(SELECT count(*) FROM disposition_item c WHERE c.bordereau_id = b.id AND c.decision = 'PENDING')::int AS pending_count,
(SELECT count(*) FROM disposition_item c WHERE c.bordereau_id = b.id AND c.decision = 'APPROVED')::int AS approved_count`

const itemColumns = `i.id, i.bordereau_id, i.case_id, i.retention_id,
s.display_label AS case_label, s.business_ref, t.code AS case_type_code,
r.until_on, r.starts_on, r.policy_code,
i.approver_subject_id, coalesce(a.display_label, '') AS approver_label,
i.decision, i.decided_at, i.decided_by, i.decision_reason, i.export_id,
x.status AS export_status, nullif(x.sha256, '') AS export_sha256`

const itemJoins = `
JOIN subject_ref s ON s.id = i.case_id
JOIN case_file c ON c.id = i.case_id
JOIN case_type t ON t.id = c.case_type_id
JOIN subject_retention r ON r.id = i.retention_id
LEFT JOIN subject_ref a ON a.id = i.approver_subject_id
LEFT JOIN case_export x ON x.id = i.export_id`

const insertBordereauSQL = `
INSERT INTO disposition_bordereau AS b (final_disposition, reason, prepared_by)
VALUES (@final_disposition, @reason, @operator_id)
RETURNING ` + bordereauColumns + `;`

const getBordereauSQL = `SELECT ` + bordereauColumns + ` FROM disposition_bordereau b WHERE b.id = @id;`

const lockBordereauSQL = `SELECT ` + bordereauColumns + ` FROM disposition_bordereau b WHERE b.id = @id FOR UPDATE OF b;`

const listBordereauxSQL = `
SELECT ` + bordereauColumns + `, count(*) OVER ()::int AS total_size
FROM disposition_bordereau b
WHERE (@status = '' OR b.status = @status)
ORDER BY b.prepared_at DESC, b.id DESC
LIMIT @limit OFFSET @offset;`

// eligibleCaseSQL reads, for a case to put in a bordereau, its running and
// unexecuted retention, the policy's disposition, whether it is due today
// (Europe/Zurich), on hold or in another open bordereau, and its approver: the
// live owning unit, or the case itself.
const eligibleCaseSQL = `
SELECT r.id AS retention_id, r.final_disposition,
       r.until_on <= (now() AT TIME ZONE 'Europe/Zurich')::date AS due,
       EXISTS (SELECT 1 FROM retention_hold h WHERE h.subject_id = r.subject_id AND h.released_at IS NULL) AS on_hold,
       EXISTS (SELECT 1 FROM disposition_item i JOIN disposition_bordereau b ON b.id = i.bordereau_id
               WHERE i.case_id = r.subject_id AND b.status = 'OPEN' AND i.decision <> 'REJECTED') AS in_open_bordereau,
       coalesce((SELECT u.id FROM org_unit u WHERE u.id = rm.owner_org_id AND u.dissolved_at IS NULL), r.subject_id) AS approver_subject_id
FROM subject_retention r
JOIN record_metadata rm ON rm.subject_id = r.subject_id
WHERE r.subject_id = @case_id AND r.ended_at IS NULL AND r.disposition_status IS NULL;`

const insertItemSQL = `
INSERT INTO disposition_item (bordereau_id, case_id, retention_id, approver_subject_id)
VALUES (@bordereau_id, @case_id, @retention_id, @approver_subject_id);`

const listItemsSQL = `
SELECT ` + itemColumns + `
FROM disposition_item i` + itemJoins + `
WHERE i.bordereau_id = @bordereau_id
ORDER BY s.business_ref, i.case_id;`

const lockItemSQL = `
SELECT ` + itemColumns + `
FROM disposition_item i` + itemJoins + `
WHERE i.id = @id
FOR UPDATE OF i;`

// pendingItemsSQL lists the items waiting for a decision in open bordereaux,
// oldest bordereau first, with the bordereau's preparer and disposition.
const pendingItemsSQL = `
SELECT ` + itemColumns + `, b.prepared_by, b.final_disposition
FROM disposition_item i` + itemJoins + `
JOIN disposition_bordereau b ON b.id = i.bordereau_id
WHERE i.decision = 'PENDING' AND b.status = 'OPEN'
ORDER BY b.prepared_at, s.business_ref
LIMIT 500;`

const decideItemSQL = `
UPDATE disposition_item
SET decision = @decision, decided_at = now(), decided_by = @operator_id, decision_reason = @reason
WHERE id = @id AND decision = 'PENDING';`

const setItemExportSQL = `UPDATE disposition_item SET export_id = @export_id WHERE id = @id;`

// queueTransferSQL queues the complete transfer package of a case (built by
// the export worker as the system).
const queueTransferSQL = `
INSERT INTO case_export (case_id, requested_by, reason, purpose)
VALUES (@case_id, @operator_id, @reason, 'TRANSFER')
RETURNING id;`

// executeRetentionSQL records the outcome of a retention executed by a bordereau.
const executeRetentionSQL = `
UPDATE subject_retention
SET disposition_status = @outcome, disposed_at = now(), bordereau_id = @bordereau_id
WHERE id = @id AND ended_at IS NULL AND disposition_status IS NULL;`

// closeBordereauSQL executes or cancels an open bordereau.
const closeBordereauSQL = `
UPDATE disposition_bordereau
SET status = @status, closed_at = now(), closed_by = @operator_id, close_reason = @reason,
    proof_sha256 = @proof_sha256, proof_manifest = @proof_manifest
WHERE id = @id AND status = 'OPEN';`

// activeHoldSQL reports an active legal hold on a case.
const activeHoldSQL = `SELECT EXISTS (SELECT 1 FROM retention_hold h WHERE h.subject_id = @case_id AND h.released_at IS NULL);`
