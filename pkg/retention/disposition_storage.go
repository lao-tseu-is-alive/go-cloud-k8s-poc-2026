package retention

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// eligibility is what eligibleCaseSQL reads about a case to put in a bordereau.
type eligibility struct {
	// RetentionID is the running retention.
	RetentionID uuid.UUID `db:"retention_id"`
	// FinalDisposition is the policy's disposition.
	FinalDisposition Disposition `db:"final_disposition"`
	// Due reports a retention ended on or before today.
	Due bool `db:"due"`
	// OnHold reports an active legal hold.
	OnHold bool `db:"on_hold"`
	// InOpenBordereau reports the case in another open bordereau.
	InOpenBordereau bool `db:"in_open_bordereau"`
	// ApproverSubjectID is the live owning unit, or the case.
	ApproverSubjectID uuid.UUID `db:"approver_subject_id"`
}

// check refuses a case that cannot go in a bordereau of disposition d.
func (e *eligibility) check(caseID uuid.UUID, d Disposition) error {
	switch {
	case !e.Due:
		return fmt.Errorf("%w: case %s is not due yet", core.ErrInvalidState, caseID)
	case e.OnHold:
		return fmt.Errorf("%w: case %s is on legal hold", core.ErrInvalidState, caseID)
	case e.InOpenBordereau:
		return fmt.Errorf("%w: case %s is already in an open bordereau", core.ErrConflict, caseID)
	case e.FinalDisposition != d && e.FinalDisposition != DispositionReview:
		return fmt.Errorf("%w: case %s is due for %s, not %s", core.ErrInvalidInput, caseID, e.FinalDisposition, d)
	}
	return nil
}

// CreateBordereau prepares a bordereau of due cases, each locked, checked and
// audited (RETENTION_DISPOSITION_PROPOSED), in one transaction.
func (r *PostgresRepository) CreateBordereau(ctx context.Context, in BordereauInput) (*Bordereau, error) {
	var id uuid.UUID
	err := core.InTx(ctx, r.pool, "create bordereau", func(tx pgx.Tx) error {
		b, err := collectBordereau(tx.Query(ctx, insertBordereauSQL, pgx.NamedArgs{
			"final_disposition": string(in.Disposition), "reason": in.Reason, "operator_id": in.OperatorID,
		}))
		if err != nil {
			return fmt.Errorf("insert bordereau: %w", err)
		}
		id = b.ID
		for _, caseID := range in.CaseIDs {
			if err := proposeCaseTx(ctx, tx, b, caseID, in.OperatorID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetBordereau(ctx, id)
}

// proposeCaseTx puts one case in the bordereau.
func proposeCaseTx(ctx context.Context, tx pgx.Tx, b *Bordereau, caseID uuid.UUID, operatorID string) error {
	if err := findCase(ctx, tx, lockCaseSQL, caseID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, eligibleCaseSQL, pgx.NamedArgs{"case_id": caseID})
	if err != nil {
		return fmt.Errorf("read case retention: %w", err)
	}
	e, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[eligibility])
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: case %s has no running retention", core.ErrInvalidState, caseID)
	}
	if err != nil {
		return fmt.Errorf("read case retention: %w", err)
	}
	if err := e.check(caseID, b.FinalDisposition); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, insertItemSQL, pgx.NamedArgs{
		"bordereau_id": b.ID, "case_id": caseID, "retention_id": e.RetentionID, "approver_subject_id": e.ApproverSubjectID,
	}); err != nil {
		return fmt.Errorf("insert bordereau item: %w", err)
	}
	_, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID: caseID, EventType: EventDispositionProposed, ActorUserID: operatorID, Reason: b.Reason,
		AfterState: map[string]any{"bordereau_id": b.ID, "final_disposition": string(b.FinalDisposition), "approver_subject_id": e.ApproverSubjectID},
	})
	return err
}

// GetBordereau reads a bordereau with its items.
func (r *PostgresRepository) GetBordereau(ctx context.Context, id uuid.UUID) (*Bordereau, error) {
	b, err := collectBordereau(r.pool.Query(ctx, getBordereauSQL, pgx.NamedArgs{"id": id}))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: bordereau %s", core.ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("read bordereau: %w", err)
	}
	if b.Items, err = listItems(ctx, r.pool, id); err != nil {
		return nil, err
	}
	return b, nil
}

// ListBordereaux pages the bordereaux, newest first.
func (r *PostgresRepository) ListBordereaux(ctx context.Context, filter BordereauFilter) (BordereauList, error) {
	rows, err := r.pool.Query(ctx, listBordereauxSQL, pgx.NamedArgs{"status": string(filter.Status), "limit": filter.Limit, "offset": filter.Offset})
	if err != nil {
		return BordereauList{}, fmt.Errorf("list bordereaux: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Bordereau])
	if err != nil {
		return BordereauList{}, fmt.Errorf("list bordereaux: %w", err)
	}
	res := BordereauList{Bordereaux: list}
	if len(list) > 0 {
		res.TotalSize = list[0].TotalSize
	}
	return res, nil
}

// pendingItem is a pending item with its bordereau's preparer and disposition.
type pendingItem struct {
	Item
	// PreparedBy is the bordereau's preparer.
	PreparedBy string `db:"prepared_by"`
	// FinalDisposition is the bordereau's disposition.
	FinalDisposition Disposition `db:"final_disposition"`
}

// MyApprovals lists the pending items whose approver subject operatorID
// manages, except in the bordereaux it prepared.
func (r *PostgresRepository) MyApprovals(ctx context.Context, operatorID string) ([]Approval, error) {
	rows, err := r.pool.Query(ctx, pendingItemsSQL)
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}
	pending, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[pendingItem])
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}
	manages := map[uuid.UUID]bool{}
	var out []Approval
	for _, p := range pending {
		if p.PreparedBy == operatorID {
			continue
		}
		ok, seen := manages[p.ApproverSubjectID]
		if !seen {
			access, err := core.EffectiveAccessTx(ctx, r.pool, operatorID, p.ApproverSubjectID)
			if err != nil {
				return nil, err
			}
			ok = access.Level >= core.LevelManage
			manages[p.ApproverSubjectID] = ok
		}
		if ok {
			out = append(out, Approval{Item: &p.Item, Disposition: p.FinalDisposition})
		}
	}
	return out, nil
}

// Decide approves or rejects a pending item of an open bordereau: the approver
// holds MANAGE on the item's approver subject and did not prepare the
// bordereau. The decision is audited on the case.
func (r *PostgresRepository) Decide(ctx context.Context, in DecisionInput) (*Item, error) {
	var bordereauID uuid.UUID
	err := core.InTx(ctx, r.pool, "decide disposition", func(tx pgx.Tx) error {
		item, err := collectItem(tx.Query(ctx, lockItemSQL, pgx.NamedArgs{"id": in.ItemID}))
		if err != nil {
			return err
		}
		bordereauID = item.BordereauID
		b, err := lockOpenBordereauTx(ctx, tx, item.BordereauID)
		if err != nil {
			return err
		}
		if err := checkDeciderTx(ctx, tx, b, item, in.OperatorID); err != nil {
			return err
		}
		decision, event := DecisionRejected, EventDispositionRejected
		if in.Approve {
			decision, event = DecisionApproved, EventDispositionApproved
		}
		if _, err := tx.Exec(ctx, decideItemSQL, pgx.NamedArgs{"id": item.ID, "decision": string(decision), "operator_id": in.OperatorID, "reason": in.Reason}); err != nil {
			return fmt.Errorf("decide item: %w", err)
		}
		_, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: item.CaseID, EventType: event, ActorUserID: in.OperatorID, Reason: in.Reason,
			AfterState: map[string]any{"bordereau_id": b.ID, "final_disposition": string(b.FinalDisposition)},
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.findItem(ctx, bordereauID, in.ItemID)
}

// checkDeciderTx requires a pending item, a decider other than the preparer
// and MANAGE on the approver subject.
func checkDeciderTx(ctx context.Context, tx pgx.Tx, b *Bordereau, item *Item, operatorID string) error {
	if item.Decision != DecisionPending {
		return fmt.Errorf("%w: the case is already %s", core.ErrInvalidState, item.Decision)
	}
	if b.PreparedBy == operatorID {
		return fmt.Errorf("%w: the preparer of a bordereau does not approve it", core.ErrPermissionDenied)
	}
	return core.EnsureAccessTx(ctx, tx, operatorID, item.ApproverSubjectID, core.LevelManage)
}

// Execute handles the approved cases of a fully decided bordereau and keeps
// its proof. ARCHIVE queues each case's transfer package, records the EXPORT,
// freezes the case and marks its retention ARCHIVED; DESTROY is step B2.
func (r *PostgresRepository) Execute(ctx context.Context, id uuid.UUID, operatorID string) (*Bordereau, error) {
	var refs []string
	err := core.InTx(ctx, r.pool, "execute bordereau", func(tx pgx.Tx) error {
		var err error
		refs, err = r.executeTx(ctx, tx, id, operatorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	r.destroyFiles(ctx, refs)
	return r.GetBordereau(ctx, id)
}

// executeTx runs Execute in tx and returns the storage references of the
// files an elimination destroyed, to delete after the commit.
func (r *PostgresRepository) executeTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, operatorID string) ([]string, error) {
	b, err := lockOpenBordereauTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := r.executable(b); err != nil {
		return nil, err
	}
	items, err := listItems(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	approved := approvedCases(items)
	if b.FinalDisposition == DispositionDestroy {
		if _, err := tx.Exec(ctx, enableDisposalSQL); err != nil {
			return nil, fmt.Errorf("declare elimination: %w", err)
		}
	}
	var refs []string
	for _, item := range items {
		if item.Decision != DecisionApproved {
			continue
		}
		destroyed, err := handleItemTx(ctx, tx, b, item, approved, operatorID)
		if err != nil {
			return nil, err
		}
		refs = append(refs, destroyed...)
	}
	return refs, closeWithProofTx(ctx, tx, b, items, operatorID)
}

// handleItemTx transfers or eliminates one approved case.
func handleItemTx(ctx context.Context, tx pgx.Tx, b *Bordereau, item *Item, approved []uuid.UUID, operatorID string) ([]string, error) {
	if b.FinalDisposition == DispositionDestroy {
		return eliminateItemTx(ctx, tx, b, item, approved, operatorID)
	}
	return nil, archiveItemTx(ctx, tx, b, item, operatorID)
}

// approvedCases lists the cases of the approved items.
func approvedCases(items []*Item) []uuid.UUID {
	var ids []uuid.UUID
	for _, it := range items {
		if it.Decision == DecisionApproved {
			ids = append(ids, it.CaseID)
		}
	}
	return ids
}

// executable refuses a bordereau with undecided cases, or an elimination
// without the store holding the bytes to destroy.
func (r *PostgresRepository) executable(b *Bordereau) error {
	if b.PendingCount > 0 {
		return fmt.Errorf("%w: %d case(s) still wait for a decision", core.ErrInvalidState, b.PendingCount)
	}
	if b.FinalDisposition == DispositionDestroy && r.store == nil {
		return fmt.Errorf("%w: no content store is configured to destroy the files", core.ErrInvalidState)
	}
	return nil
}

// archiveItemTx transfers one approved case: no legal hold may have been
// placed since the approval.
func archiveItemTx(ctx context.Context, tx pgx.Tx, b *Bordereau, item *Item, operatorID string) error {
	if err := lockUnheldCaseTx(ctx, tx, item.CaseID); err != nil {
		return err
	}
	exportID, err := queueTransferTx(ctx, tx, item, b.Reason, operatorID)
	if err != nil {
		return err
	}
	if err := recordOutcomeTx(ctx, tx, b, item, OutcomeArchived, operatorID); err != nil {
		return err
	}
	_, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID: item.CaseID, EventType: EventArchived, ActorUserID: operatorID, Reason: b.Reason,
		AfterState: map[string]any{"bordereau_id": b.ID, "export_id": exportID},
	})
	return err
}

// queueTransferTx queues the complete transfer package of an item's case,
// links it to the item and records the EXPORT.
func queueTransferTx(ctx context.Context, tx pgx.Tx, item *Item, reason, operatorID string) (uuid.UUID, error) {
	var exportID uuid.UUID
	if err := tx.QueryRow(ctx, queueTransferSQL, pgx.NamedArgs{"case_id": item.CaseID, "operator_id": operatorID, "reason": reason}).Scan(&exportID); err != nil {
		return uuid.Nil, fmt.Errorf("queue transfer package: %w", err)
	}
	item.ExportID = &exportID
	if _, err := tx.Exec(ctx, setItemExportSQL, pgx.NamedArgs{"id": item.ID, "export_id": exportID}); err != nil {
		return uuid.Nil, fmt.Errorf("link transfer package: %w", err)
	}
	access, err := core.EffectiveAccessTx(ctx, tx, operatorID, item.CaseID)
	if err != nil {
		return uuid.Nil, err
	}
	return exportID, core.RecordAccessTx(ctx, tx, operatorID, access, core.AccessExport, core.AccessGranted)
}

// RebuildTransfer queues again the transfer package of a transferred case
// whose package failed or expired, records the EXPORT and audits it.
func (r *PostgresRepository) RebuildTransfer(ctx context.Context, itemID uuid.UUID, operatorID string) (*Item, error) {
	var bordereauID uuid.UUID
	err := core.InTx(ctx, r.pool, "rebuild transfer package", func(tx pgx.Tx) error {
		item, err := collectItem(tx.Query(ctx, lockItemSQL, pgx.NamedArgs{"id": itemID}))
		if err != nil {
			return err
		}
		bordereauID = item.BordereauID
		var id uuid.UUID
		if err := tx.QueryRow(ctx, rebuildableItemSQL, pgx.NamedArgs{"id": itemID}).Scan(&id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: only a transferred case whose package failed or expired is rebuilt", core.ErrInvalidState)
			}
			return fmt.Errorf("read item: %w", err)
		}
		exportID, err := queueTransferTx(ctx, tx, item, "rebuilt transfer package", operatorID)
		if err != nil {
			return err
		}
		_, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: item.CaseID, EventType: EventTransferRebuilt, ActorUserID: operatorID,
			AfterState: map[string]any{"bordereau_id": item.BordereauID, "export_id": exportID},
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.findItem(ctx, bordereauID, itemID)
}

// findItem reads one item of a bordereau with its labels.
func (r *PostgresRepository) findItem(ctx context.Context, bordereauID, itemID uuid.UUID) (*Item, error) {
	items, err := listItems(ctx, r.pool, bordereauID)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if it.ID == itemID {
			return it, nil
		}
	}
	return nil, fmt.Errorf("%w: item %s", core.ErrNotFound, itemID)
}

// closeWithProofTx executes the bordereau with the SHA-256 of its manifest.
func closeWithProofTx(ctx context.Context, tx pgx.Tx, b *Bordereau, items []*Item, operatorID string) error {
	manifest := proofManifest{
		BordereauID: b.ID, FinalDisposition: b.FinalDisposition, Reason: b.Reason, PreparedBy: b.PreparedBy,
		PreparedAt: b.PreparedAt.UTC(), ExecutedBy: operatorID, ExecutedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	for _, it := range items {
		manifest.Items = append(manifest.Items, proofItem{
			CaseID: it.CaseID, BusinessRef: it.BusinessRef, CaseTypeCode: it.CaseTypeCode, PolicyCode: it.PolicyCode,
			StartsOn: it.StartsOn.Format(time.DateOnly), UntilOn: it.UntilOn.Format(time.DateOnly),
			Decision: it.Decision, DecidedBy: deref(it.DecidedBy), DecidedAt: it.DecidedAt, DecisionReason: it.DecisionReason,
			ExportID: it.ExportID,
		})
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("proof manifest: %w", err)
	}
	sum := sha256.Sum256(raw)
	return closeBordereauTx(ctx, tx, b.ID, BordereauExecuted, operatorID, "", hex.EncodeToString(sum[:]), raw)
}

// Cancel abandons an open bordereau; its cases leave it (audited).
func (r *PostgresRepository) Cancel(ctx context.Context, id uuid.UUID, reason, operatorID string) (*Bordereau, error) {
	err := core.InTx(ctx, r.pool, "cancel bordereau", func(tx pgx.Tx) error {
		if _, err := lockOpenBordereauTx(ctx, tx, id); err != nil {
			return err
		}
		items, err := listItems(ctx, tx, id)
		if err != nil {
			return err
		}
		for _, it := range items {
			if _, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
				SubjectID: it.CaseID, EventType: EventDispositionWithdrawn, ActorUserID: operatorID, Reason: reason,
				AfterState: map[string]any{"bordereau_id": id},
			}); err != nil {
				return err
			}
		}
		return closeBordereauTx(ctx, tx, id, BordereauCancelled, operatorID, reason, "", nil)
	})
	if err != nil {
		return nil, err
	}
	return r.GetBordereau(ctx, id)
}

func closeBordereauTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status BordereauStatus, operatorID, reason, proof string, manifest []byte) error {
	tag, err := tx.Exec(ctx, closeBordereauSQL, pgx.NamedArgs{
		"id": id, "status": string(status), "operator_id": operatorID, "reason": reason, "proof_sha256": proof, "proof_manifest": nullableText(manifest),
	})
	if err != nil {
		return fmt.Errorf("close bordereau: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: the bordereau is no longer open", core.ErrInvalidState)
	}
	return nil
}

// lockOpenBordereauTx locks a bordereau and requires it open.
func lockOpenBordereauTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Bordereau, error) {
	b, err := collectBordereau(tx.Query(ctx, lockBordereauSQL, pgx.NamedArgs{"id": id}))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: bordereau %s", core.ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("read bordereau: %w", err)
	}
	if b.Status != BordereauOpen {
		return nil, fmt.Errorf("%w: the bordereau is %s", core.ErrInvalidState, b.Status)
	}
	return b, nil
}

func listItems(ctx context.Context, q core.Querier, bordereauID uuid.UUID) ([]*Item, error) {
	rows, err := q.Query(ctx, listItemsSQL, pgx.NamedArgs{"bordereau_id": bordereauID})
	if err != nil {
		return nil, fmt.Errorf("list bordereau items: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Item])
	if err != nil {
		return nil, fmt.Errorf("list bordereau items: %w", err)
	}
	return items, nil
}

func collectBordereau(rows pgx.Rows, err error) (*Bordereau, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Bordereau])
}

func collectItem(rows pgx.Rows, err error) (*Item, error) {
	if err != nil {
		return nil, err
	}
	item, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Item])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	return item, err
}

// nullableText is the manifest as text, NULL when absent (a cancellation).
func nullableText(b []byte) *string {
	if b == nil {
		return nil
	}
	s := string(b)
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
