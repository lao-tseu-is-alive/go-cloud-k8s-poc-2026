package circulation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// responseTaskType is the seeded type of the recipients' tasks.
const responseTaskType = "CIRCULATION_RESPONSE"

// PostgresRepository implements Repository with pgx, composing the core,
// task and timeline primitives in one transaction per change.
type PostgresRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewPostgresRepository builds a PostgresRepository from a connection pool. A
// nil logger falls back to slog.Default.
func NewPostgresRepository(pool *pgxpool.Pool, log *slog.Logger) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &PostgresRepository{pool: pool, log: log}, nil
}

// inTx runs fn in a transaction on the repository's pool (core.InTx).
func (r *PostgresRepository) inTx(ctx context.Context, op string, fn func(pgx.Tx) error) error {
	return core.InTx(ctx, r.pool, op, fn)
}

// Create sends an open case to its recipients: the circulation, its
// recipients, the first step's tasks, a SYSTEM timeline entry and
// CIRCULATION_CREATED, in one transaction. Steps are already normalized.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Circulation, *core.AuditEvent, error) {
	var id uuid.UUID
	var ev *core.AuditEvent
	err := r.inTx(ctx, "create circulation", func(tx pgx.Tx) error {
		if err := core.EnsureOpenCaseTx(ctx, tx, in.CaseID); err != nil {
			return err
		}
		c, err := collectCirculation(tx.Query(ctx, insertCirculationSQL, pgx.NamedArgs{
			"case_id": in.CaseID, "title": in.Title, "message": in.Message, "due_at": in.DueAt,
			"step_count": stepCount(in.Recipients), "operator_id": in.OperatorID,
		}))
		if err != nil {
			return err
		}
		id = c.ID
		if err := insertRecipientsTx(ctx, tx, c.ID, in.Recipients); err != nil {
			return err
		}
		if err := openStepTx(ctx, tx, c, in.OperatorID); err != nil {
			return err
		}
		body := fmt.Sprintf("Circulation envoyée : %s\n%d destinataire(s), %d étape(s)", c.Title, len(in.Recipients), c.StepCount)
		if err := systemEntryTx(ctx, tx, c, "Circulation envoyée", body, "CIRCULATION_CREATED", in.OperatorID, nil); err != nil {
			return err
		}
		ev, err = auditTx(ctx, tx, c, "CIRCULATION_CREATED", in.OperatorID, "", map[string]any{
			"title": c.Title, "recipients": len(in.Recipients), "steps": c.StepCount,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	c, err := r.Get(ctx, id)
	return c, ev, err
}

// Respond records the answer of a recipient whose step is open: its task is
// completed, a locked RESPONSE timeline entry written and CIRCULATION_RESPONDED
// audited; the next step opens or the circulation completes when the step is
// fully answered.
func (r *PostgresRepository) Respond(ctx context.Context, in RespondInput) (*Circulation, *core.AuditEvent, error) {
	var id uuid.UUID
	var ev *core.AuditEvent
	err := r.inTx(ctx, "respond to circulation", func(tx pgx.Tx) error {
		c, rec, err := lockAwaitingTx(ctx, tx, in.RecipientID)
		if err != nil {
			return err
		}
		id = c.ID
		entry, err := responseEntryTx(ctx, tx, c, rec, in)
		if err != nil {
			return err
		}
		if _, _, err := task.MoveTx(ctx, tx, *rec.TaskID, task.MoveComplete, in.OperatorID, in.Text,
			task.MoveOptions{ByOrigin: true, SkipTimeline: true}); err != nil {
			return fmt.Errorf("complete recipient task: %w", err)
		}
		if _, err := tx.Exec(ctx, recordResponseSQL, pgx.NamedArgs{
			"id": rec.ID, "response": int16(in.Response), "text": in.Text, "operator_id": in.OperatorID, "entry_id": entry.ID,
		}); err != nil {
			return fmt.Errorf("record response: %w", err)
		}
		ev, err = auditTx(ctx, tx, c, "CIRCULATION_RESPONDED", in.OperatorID, in.Text, map[string]any{
			"recipient_id": rec.ID.String(), "recipient": rec.AssigneeLabel, "response": in.Response.String(),
		})
		if err != nil {
			return err
		}
		return advanceTx(ctx, tx, c, in.OperatorID)
	})
	if err != nil {
		return nil, nil, err
	}
	c, err := r.Get(ctx, id)
	return c, ev, err
}

// Cancel stops an open circulation: its open tasks are cancelled, a SYSTEM
// timeline entry written and CIRCULATION_CANCELLED audited.
func (r *PostgresRepository) Cancel(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Circulation, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "cancel circulation", func(tx pgx.Tx) error {
		c, err := lockOpenTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := cancelPendingTasksTx(ctx, tx, c.ID, operatorID, reason); err != nil {
			return err
		}
		c, err = collectCirculation(tx.Query(ctx, cancelCirculationSQL, pgx.NamedArgs{"id": id, "operator_id": operatorID, "reason": reason}))
		if err != nil {
			return err
		}
		body := fmt.Sprintf("Circulation annulée : %s\nMotif : %s", c.Title, reason)
		if err := systemEntryTx(ctx, tx, c, "Circulation annulée", body, "CIRCULATION_CANCELLED", operatorID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		ev, err = auditTx(ctx, tx, c, "CIRCULATION_CANCELLED", operatorID, reason, map[string]any{"status": "CANCELLED"})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	c, err := r.Get(ctx, id)
	return c, ev, err
}

// Get loads a circulation with its recipients.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Circulation, error) {
	c, err := collectCirculation(r.pool.Query(ctx, getCirculationSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	return c, r.hydrate(ctx, []*Circulation{c})
}

// ListCase returns the circulations of a case, newest first, with their recipients.
func (r *PostgresRepository) ListCase(ctx context.Context, caseID uuid.UUID) ([]*Circulation, error) {
	if _, err := core.GetRecordMetadataTx(ctx, r.pool, caseID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, listCaseCirculationsSQL, pgx.NamedArgs{"case_id": caseID})
	if err != nil {
		return nil, fmt.Errorf("list circulations: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Circulation])
	if err != nil {
		return nil, fmt.Errorf("read circulations: %w", err)
	}
	return list, r.hydrate(ctx, list)
}

// hydrate loads the recipients of circulations in one query.
func (r *PostgresRepository) hydrate(ctx context.Context, list []*Circulation) error {
	if len(list) == 0 {
		return nil
	}
	byID := make(map[uuid.UUID]*Circulation, len(list))
	ids := make([]uuid.UUID, 0, len(list))
	for _, c := range list {
		byID[c.ID] = c
		ids = append(ids, c.ID)
	}
	rows, err := r.pool.Query(ctx, listRecipientsSQL, pgx.NamedArgs{"ids": ids})
	if err != nil {
		return fmt.Errorf("list recipients: %w", err)
	}
	recipients, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Recipient])
	if err != nil {
		return fmt.Errorf("read recipients: %w", err)
	}
	for _, rec := range recipients {
		c := byID[rec.CirculationID]
		c.Recipients = append(c.Recipients, rec)
	}
	return nil
}

// EnsureNoOpenCirculationsTx fails with core.ErrInvalidState when the case still
// has open circulations. The case lifecycle calls it before closing a case.
func EnsureNoOpenCirculationsTx(ctx context.Context, q core.Querier, caseID uuid.UUID) error {
	var n int
	if err := q.QueryRow(ctx, countOpenCirculationsSQL, pgx.NamedArgs{"case_id": caseID}).Scan(&n); err != nil {
		return fmt.Errorf("count open circulations: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("%w: the case has %d open circulations; wait for the answers or cancel them before closing it", core.ErrInvalidState, n)
	}
	return nil
}

// insertRecipientsTx checks and inserts the recipients of a new circulation
// (every one now, so a later step cannot fail on an unknown recipient).
func insertRecipientsTx(ctx context.Context, q core.Querier, circulationID uuid.UUID, recipients []RecipientInput) error {
	for _, rec := range recipients {
		if err := task.CheckAssigneeTx(ctx, q, rec.Assignee); err != nil {
			return err
		}
		_, err := q.Exec(ctx, insertRecipientSQL, pgx.NamedArgs{
			"circulation_id": circulationID, "step": rec.Step,
			"assignee_user_id": rec.Assignee.UserID, "assignee_org_unit_id": rec.Assignee.OrgUnitID,
		})
		if err != nil {
			return fmt.Errorf("insert recipient: %w", mapDBError(err))
		}
	}
	return nil
}

// openStepTx creates the tasks of the circulation's current step (one per
// recipient, managed by the circulation).
func openStepTx(ctx context.Context, q core.Querier, c *Circulation, operatorID string) error {
	rows, err := q.Query(ctx, stepRecipientsSQL, pgx.NamedArgs{"circulation_id": c.ID, "step": c.CurrentStep})
	if err != nil {
		return fmt.Errorf("list step recipients: %w", err)
	}
	recipients, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Recipient])
	if err != nil {
		return fmt.Errorf("read step recipients: %w", err)
	}
	for _, rec := range recipients {
		created, _, err := task.CreateTx(ctx, q, task.CreateInput{
			CaseID:     c.CaseID,
			Content:    task.Content{TypeCode: responseTaskType, Title: "Circulation : " + c.Title, Description: c.Message, DueAt: c.DueAt},
			Assignee:   task.Assignee{UserID: rec.AssigneeUserID, OrgUnitID: rec.AssigneeOrgUnitID},
			OperatorID: operatorID,
		}, task.OriginCirculation, c.ID.String())
		if err != nil {
			return fmt.Errorf("create recipient task: %w", err)
		}
		if _, err := q.Exec(ctx, setRecipientTaskSQL, pgx.NamedArgs{"id": rec.ID, "task_id": created.ID}); err != nil {
			return fmt.Errorf("link recipient task: %w", err)
		}
	}
	return nil
}

// advanceTx opens the next step once the current one is fully answered, or
// completes the circulation after the last step.
func advanceTx(ctx context.Context, q core.Querier, c *Circulation, operatorID string) error {
	var unanswered int
	if err := q.QueryRow(ctx, countUnansweredSQL, pgx.NamedArgs{"circulation_id": c.ID, "step": c.CurrentStep}).Scan(&unanswered); err != nil {
		return fmt.Errorf("count unanswered recipients: %w", err)
	}
	if unanswered > 0 {
		return nil
	}
	if c.CurrentStep < c.StepCount {
		next, err := collectCirculation(q.Query(ctx, openNextStepSQL, pgx.NamedArgs{"id": c.ID}))
		if err != nil {
			return err
		}
		if err := openStepTx(ctx, q, next, operatorID); err != nil {
			return err
		}
		_, err = auditTx(ctx, q, next, "CIRCULATION_STEP_OPENED", operatorID, "", map[string]any{"step": next.CurrentStep})
		return err
	}
	return completeTx(ctx, q, c, operatorID)
}

// completeTx completes a fully answered circulation with a SYSTEM summary entry.
func completeTx(ctx context.Context, q core.Querier, c *Circulation, operatorID string) error {
	done, err := collectCirculation(q.Query(ctx, completeCirculationSQL, pgx.NamedArgs{"id": c.ID}))
	if err != nil {
		return err
	}
	counts, summary, err := responseSummaryTx(ctx, q, c.ID)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("Circulation terminée : %s\n%s", done.Title, summary)
	if err := systemEntryTx(ctx, q, done, "Circulation terminée", body, "CIRCULATION_COMPLETED", operatorID, map[string]any{"counts": counts}); err != nil {
		return err
	}
	_, err = auditTx(ctx, q, done, "CIRCULATION_COMPLETED", operatorID, "", map[string]any{"counts": counts})
	return err
}

// responseSummaryTx counts the answers by response: the structured counts and
// a French line ("Favorable : 2, Défavorable : 1").
func responseSummaryTx(ctx context.Context, q core.Querier, circulationID uuid.UUID) (map[string]any, string, error) {
	rows, err := q.Query(ctx, responseCountsSQL, pgx.NamedArgs{"circulation_id": circulationID})
	if err != nil {
		return nil, "", fmt.Errorf("count responses: %w", err)
	}
	defer rows.Close()
	counts := map[string]any{}
	var parts []string
	for rows.Next() {
		var response Response
		var n int
		if err := rows.Scan(&response, &n); err != nil {
			return nil, "", fmt.Errorf("read response count: %w", err)
		}
		counts[response.String()] = n
		parts = append(parts, fmt.Sprintf("%s : %d", labelsFR[response], n))
	}
	return counts, strings.Join(parts, ", "), rows.Err()
}

// cancelPendingTasksTx cancels the open tasks of a circulation.
func cancelPendingTasksTx(ctx context.Context, q core.Querier, circulationID uuid.UUID, operatorID, reason string) error {
	rows, err := q.Query(ctx, pendingTasksSQL, pgx.NamedArgs{"circulation_id": circulationID})
	if err != nil {
		return fmt.Errorf("list open circulation tasks: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return fmt.Errorf("read open circulation tasks: %w", err)
	}
	for _, id := range ids {
		if _, _, err := task.MoveTx(ctx, q, id, task.MoveCancel, operatorID, reason, task.MoveOptions{ByOrigin: true, SkipTimeline: true}); err != nil {
			return fmt.Errorf("cancel circulation task: %w", err)
		}
	}
	return nil
}

// lockOpenTx locks the circulation's case (core.EnsureOpenCaseTx), then the
// circulation, and requires it to be open. The case is locked first, as on
// creation, to avoid lock inversions.
func lockOpenTx(ctx context.Context, q core.Querier, id uuid.UUID) (*Circulation, error) {
	current, err := collectCirculation(q.Query(ctx, getCirculationSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	if err := core.EnsureOpenCaseTx(ctx, q, current.CaseID); err != nil {
		return nil, err
	}
	c, err := collectCirculation(q.Query(ctx, getCirculationForUpdateSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	if c.Status != StatusOpen {
		return nil, fmt.Errorf("%w: the circulation is no longer open", core.ErrInvalidState)
	}
	return c, nil
}

// lockAwaitingTx locks the recipient's circulation (see lockOpenTx), then the
// recipient, and requires it to be awaited (its step open, no answer yet).
func lockAwaitingTx(ctx context.Context, q core.Querier, recipientID uuid.UUID) (*Circulation, *Recipient, error) {
	current, err := collectRecipient(q.Query(ctx, getRecipientSQL, pgx.NamedArgs{"id": recipientID}))
	if err != nil {
		return nil, nil, err
	}
	c, err := lockOpenTx(ctx, q, current.CirculationID)
	if err != nil {
		return nil, nil, err
	}
	rec, err := collectRecipient(q.Query(ctx, getRecipientForUpdateSQL, pgx.NamedArgs{"id": recipientID}))
	if err != nil {
		return nil, nil, err
	}
	switch {
	case rec.Response != nil:
		return nil, nil, fmt.Errorf("%w: the recipient has already answered", core.ErrInvalidState)
	case rec.Step != c.CurrentStep || rec.TaskID == nil:
		return nil, nil, fmt.Errorf("%w: the recipient's step (%d) is not open yet (current step %d)", core.ErrInvalidState, rec.Step, c.CurrentStep)
	}
	rec.AssigneeLabel = current.AssigneeLabel
	return c, rec, nil
}

// responseEntryTx writes the answer as a locked RESPONSE timeline entry,
// authored by the operator recording it.
func responseEntryTx(ctx context.Context, q core.Querier, c *Circulation, rec *Recipient, in RespondInput) (*timeline.Entry, error) {
	body := in.Text
	if body == "" {
		body = labelsFR[in.Response]
	}
	return timeline.RecordSystemEntryTx(ctx, q, timeline.SystemEntry{
		Type:   timeline.TypeResponse,
		CaseID: c.CaseID,
		Title:  fmt.Sprintf("Réponse : %s — %s", labelsFR[in.Response], rec.AssigneeLabel),
		Body:   body,
		Metadata: map[string]any{
			"event": "CIRCULATION_RESPONSE", "circulation_id": c.ID.String(), "circulation_title": c.Title,
			"recipient_id": rec.ID.String(), "recipient": rec.AssigneeLabel, "response": in.Response.String(),
		},
		OperatorID: in.OperatorID,
	})
}

// systemEntryTx writes a SYSTEM timeline entry about a circulation.
func systemEntryTx(ctx context.Context, q core.Querier, c *Circulation, title, body, event, operatorID string, extra map[string]any) error {
	meta := map[string]any{"event": event, "circulation_id": c.ID.String(), "circulation_title": c.Title}
	for k, v := range extra {
		meta[k] = v
	}
	_, err := timeline.RecordSystemEntryTx(ctx, q, timeline.SystemEntry{CaseID: c.CaseID, Title: title, Body: body, Metadata: meta, OperatorID: operatorID})
	return err
}

// auditTx writes a circulation audit event on the CASE subject, naming the
// circulation in metadata.
func auditTx(ctx context.Context, q core.Querier, c *Circulation, eventType, operatorID, reason string, after map[string]any) (*core.AuditEvent, error) {
	ev, err := core.InsertAuditEventTx(ctx, q, core.AuditEvent{
		SubjectID: c.CaseID, EventType: eventType, ActorUserID: operatorID, Reason: reason, AfterState: after,
		Metadata: map[string]any{"circulation_id": c.ID.String()},
	})
	if err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	return ev, nil
}

// stepCount is the number of distinct (normalized) steps.
func stepCount(recipients []RecipientInput) int32 {
	var n int32
	for _, rec := range recipients {
		n = max(n, rec.Step)
	}
	return n
}

// collectCirculation reads exactly one circulation row.
func collectCirculation(rows pgx.Rows, err error) (*Circulation, error) {
	if err != nil {
		return nil, fmt.Errorf("query circulation: %w", err)
	}
	c, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Circulation])
	if err != nil {
		return nil, mapDBError(err)
	}
	return c, nil
}

// collectRecipient reads exactly one recipient row.
func collectRecipient(rows pgx.Rows, err error) (*Recipient, error) {
	if err != nil {
		return nil, fmt.Errorf("query recipient: %w", err)
	}
	rec, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Recipient])
	if err != nil {
		return nil, mapDBError(err)
	}
	return rec, nil
}

// mapDBError translates pgx.ErrNoRows to core.ErrNotFound, a repeated recipient
// to core.ErrInvalidInput and an unknown user or unit to core.ErrInvalidInput.
func mapDBError(err error) error {
	return core.MapDBError(err, func(pgErr *pgconn.PgError) error {
		switch pgErr.Code {
		case core.PgUniqueViolation:
			return fmt.Errorf("%w: a recipient appears twice in the circulation", core.ErrInvalidInput)
		case core.PgForeignKeyViolation:
			return fmt.Errorf("%w: unknown recipient (%s)", core.ErrInvalidInput, pgErr.ConstraintName)
		}
		return nil
	})
}
