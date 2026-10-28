package retention

// Raw SQL of the retention schedule (retention_policy), the retentions of the
// cases (subject_retention, alias r) and the legal holds (retention_hold, alias h).

// --- retention_policy ------------------------------------------------------------

const policyColumns = `id, code, label, description, legal_basis, duration_years, final_disposition, is_active`

const listPoliciesSQL = `
SELECT ` + policyColumns + `
FROM retention_policy
WHERE (NOT @only_active OR is_active = true)
ORDER BY label, code;`

const insertPolicySQL = `
INSERT INTO retention_policy (code, label, description, legal_basis, duration_years, final_disposition)
VALUES (@code, @label, @description, @legal_basis, @duration_years, @final_disposition)
RETURNING ` + policyColumns + `;`

const getPolicyForUpdateSQL = `
SELECT ` + policyColumns + `
FROM retention_policy
WHERE code = @code
FOR UPDATE;`

// updatePolicySQL replaces the fields given (NULL keeps the current value).
const updatePolicySQL = `
UPDATE retention_policy
SET label = coalesce(@label::text, label),
    description = coalesce(@description::text, description),
    legal_basis = coalesce(@legal_basis::text, legal_basis),
    duration_years = coalesce(@duration_years::smallint, duration_years),
    final_disposition = coalesce(@final_disposition::text, final_disposition),
    is_active = coalesce(@is_active::boolean, is_active)
WHERE code = @code
RETURNING ` + policyColumns + `;`

// policyActiveSQL reads whether a policy is active (no row when unknown).
const policyActiveSQL = `SELECT is_active FROM retention_policy WHERE code = @code;`

// casePolicySQL reads the default policy of a case's type (no row when none).
const casePolicySQL = `
SELECT p.id, p.code, p.label, p.description, p.legal_basis, p.duration_years, p.final_disposition, p.is_active
FROM case_file c
JOIN case_type t ON t.id = c.case_type_id
JOIN retention_policy p ON p.code = t.default_retention_policy_code
WHERE c.id = @case_id;`

// --- subject_retention -------------------------------------------------------------

const retentionColumns = `r.id, r.subject_id, r.policy_code,
coalesce((SELECT p.label FROM retention_policy p WHERE p.code = r.policy_code), '') AS policy_label,
r.duration_years, r.final_disposition, r.starts_on, r.until_on, r.created_at, r.created_by,
r.ended_at, r.ended_by, r.end_reason, r.disposition_status, r.disposed_at, r.bordereau_id`

// startRetentionSQL computes the retention of a closed case from its type's
// default policy: it starts on the closing day (Europe/Zurich) and ends the
// duration later. No row when the type has no policy or the case is not
// closed; a running retention is left as it is.
const startRetentionSQL = `
INSERT INTO subject_retention AS r (subject_id, policy_code, duration_years, final_disposition, starts_on, until_on, created_by)
SELECT c.id, p.code, p.duration_years, p.final_disposition, d.day, (d.day + make_interval(years => p.duration_years))::date, @operator_id
FROM case_file c
JOIN case_type t ON t.id = c.case_type_id
JOIN retention_policy p ON p.code = t.default_retention_policy_code
CROSS JOIN LATERAL (SELECT (c.closed_at AT TIME ZONE 'Europe/Zurich')::date AS day) d
WHERE c.id = @case_id AND c.status = 4 AND c.closed_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM subject_retention x WHERE x.subject_id = c.id AND x.ended_at IS NULL)
RETURNING ` + retentionColumns + `;`

// endRetentionSQL ends the running retention of a subject (no row when none).
const endRetentionSQL = `
UPDATE subject_retention AS r
SET ended_at = now(), ended_by = @operator_id, end_reason = @reason
WHERE r.subject_id = @subject_id AND r.ended_at IS NULL
RETURNING ` + retentionColumns + `;`

const listRetentionsSQL = `
SELECT ` + retentionColumns + `
FROM subject_retention r
WHERE r.subject_id = @subject_id
ORDER BY r.created_at DESC, r.id DESC;`

// pendingApplySQL lists closed, live cases of a type without a running
// retention, oldest closing first, to give them the type's policy.
const pendingApplySQL = `
SELECT c.id
FROM case_file c
JOIN case_type t ON t.id = c.case_type_id
JOIN record_metadata rm ON rm.subject_id = c.id AND rm.deleted_at IS NULL
WHERE t.code = @case_type_code AND c.status = 4 AND c.closed_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM subject_retention x WHERE x.subject_id = c.id AND x.ended_at IS NULL)
ORDER BY c.closed_at, c.id
LIMIT @limit;`

// countPendingApplySQL counts what pendingApplySQL would list without a limit.
const countPendingApplySQL = `
SELECT count(*)
FROM case_file c
JOIN case_type t ON t.id = c.case_type_id
JOIN record_metadata rm ON rm.subject_id = c.id AND rm.deleted_at IS NULL
WHERE t.code = @case_type_code AND c.status = 4 AND c.closed_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM subject_retention x WHERE x.subject_id = c.id AND x.ended_at IS NULL);`

// caseTypePolicySQL reads the default policy code of a case type (empty when none;
// no row for an unknown type).
const caseTypePolicySQL = `
SELECT coalesce(default_retention_policy_code, '') FROM case_type WHERE code = @code;`

// dueRetentionsSQL pages the running retentions ending on or before @due_on
// (today in Europe/Zurich when empty), oldest deadline first, with their case;
// held cases only when asked.
const dueRetentionsSQL = `
SELECT ` + retentionColumns + `,
       s.display_label AS case_label, s.business_ref, t.code AS case_type_code,
       EXISTS (SELECT 1 FROM retention_hold h WHERE h.subject_id = r.subject_id AND h.released_at IS NULL) AS on_hold,
       count(*) OVER ()::int AS total_size
FROM subject_retention r
JOIN subject_ref s ON s.id = r.subject_id
JOIN case_file c ON c.id = r.subject_id
JOIN case_type t ON t.id = c.case_type_id
WHERE r.ended_at IS NULL AND r.disposition_status IS NULL
  AND r.until_on <= coalesce(nullif(@due_on, '')::date, (now() AT TIME ZONE 'Europe/Zurich')::date)
  AND (@disposition = '' OR r.final_disposition = @disposition)
  AND (@include_held OR NOT EXISTS (SELECT 1 FROM retention_hold h WHERE h.subject_id = r.subject_id AND h.released_at IS NULL))
ORDER BY r.until_on, r.subject_id
LIMIT @limit OFFSET @offset;`

// --- retention_hold ----------------------------------------------------------------

const holdColumns = `h.id, h.subject_id, h.reason, h.placed_at, h.placed_by, h.released_at, h.released_by, h.release_reason`

// placeHoldSQL places a hold; the unique index rejects a second active one.
const placeHoldSQL = `
INSERT INTO retention_hold AS h (subject_id, reason, placed_by)
VALUES (@subject_id, @reason, @operator_id)
RETURNING ` + holdColumns + `;`

// releaseHoldSQL releases the active hold of a subject (no row when none).
const releaseHoldSQL = `
UPDATE retention_hold AS h
SET released_at = now(), released_by = @operator_id, release_reason = @reason
WHERE h.subject_id = @subject_id AND h.released_at IS NULL
RETURNING ` + holdColumns + `;`

const listHoldsSQL = `
SELECT ` + holdColumns + `
FROM retention_hold h
WHERE h.subject_id = @subject_id
ORDER BY h.placed_at DESC, h.id DESC;`

// liveCaseSQL finds a live case (no row for a missing, deleted or non-case subject).
const liveCaseSQL = `
SELECT s.id
FROM subject_ref s
JOIN record_metadata rm ON rm.subject_id = s.id AND rm.deleted_at IS NULL
WHERE s.id = @case_id AND s.kind = 'CASE'`

// lockCaseSQL locks a live case's subject row so retention changes of one
// case are serialized.
const lockCaseSQL = liveCaseSQL + `
FOR UPDATE OF s;`
