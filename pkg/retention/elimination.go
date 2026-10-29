package retention

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Elimination of cases (GLD-077): executing an approved DESTROY bordereau
// keeps a notice of each case and the bordereau's proof, and erases the rest.

// eliminateItemTx eliminates one approved case in the bordereau transaction
// (enableDisposalSQL already run). eliminated are all the cases of the
// bordereau being eliminated. It returns what the case lost and the storage
// references whose bytes must be deleted once the transaction is committed.
func eliminateItemTx(ctx context.Context, tx pgx.Tx, b *Bordereau, item *Item, eliminated []uuid.UUID, operatorID string) (Elimination, []string, error) {
	e := Elimination{CaseID: item.CaseID}
	if err := lockUnheldCaseTx(ctx, tx, item.CaseID); err != nil {
		return e, nil, err
	}
	docs, kept, err := doomedDocumentsTx(ctx, tx, item.CaseID, eliminated)
	if err != nil {
		return e, nil, err
	}
	if err := blankCaseTx(ctx, tx, item.CaseID); err != nil {
		return e, nil, err
	}
	refs, err := eliminateDocumentsTx(ctx, tx, docs, operatorID)
	if err != nil {
		return e, nil, err
	}
	subjects := append([]uuid.UUID{item.CaseID}, docs...)
	if _, err := tx.Exec(ctx, blankEdgesSQL, pgx.NamedArgs{"ids": subjects}); err != nil {
		return e, nil, fmt.Errorf("blank relationships: %w", err)
	}
	if _, err := tx.Exec(ctx, blankAuditSQL, pgx.NamedArgs{"ids": subjects}); err != nil {
		return e, nil, fmt.Errorf("blank audit: %w", err)
	}
	exports, err := collectRefs(tx.Query(ctx, purgeExportsSQL, pgx.NamedArgs{"id": item.CaseID}))
	if err != nil {
		return e, nil, fmt.Errorf("purge exports: %w", err)
	}
	e.DocumentsEliminated, e.DocumentsKept, e.FilesDestroyed, e.ExportsPurged = len(docs), kept, len(refs), len(exports)
	if err := recordOutcomeTx(ctx, tx, b, item, OutcomeDisposed, operatorID); err != nil {
		return e, nil, err
	}
	_, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID: item.CaseID, EventType: EventDisposed, ActorUserID: operatorID,
		AfterState: map[string]any{"bordereau_id": b.ID, "documents_eliminated": e.DocumentsEliminated, "files_destroyed": e.FilesDestroyed},
	})
	return e, append(refs, exports...), err
}

// lockUnheldCaseTx locks a case and refuses it while a legal hold, placed
// after its approval, is active.
func lockUnheldCaseTx(ctx context.Context, tx pgx.Tx, caseID uuid.UUID) error {
	if err := findCase(ctx, tx, lockCaseSQL, caseID); err != nil {
		return err
	}
	var held bool
	if err := tx.QueryRow(ctx, activeHoldSQL, pgx.NamedArgs{"case_id": caseID}).Scan(&held); err != nil {
		return fmt.Errorf("read hold: %w", err)
	}
	if held {
		return fmt.Errorf("%w: case %s was put on legal hold; reject it or release the hold", core.ErrInvalidState, caseID)
	}
	return nil
}

// doomedDocumentsTx returns the documents of a case that go with it (those
// not linked to a thing nor to a case that stays) and how many stay.
func doomedDocumentsTx(ctx context.Context, tx pgx.Tx, caseID uuid.UUID, eliminated []uuid.UUID) ([]uuid.UUID, int, error) {
	rows, err := tx.Query(ctx, caseDocumentsSQL, pgx.NamedArgs{"case_id": caseID, "eliminated": eliminated})
	if err != nil {
		return nil, 0, fmt.Errorf("list case documents: %w", err)
	}
	type candidate struct {
		ID   uuid.UUID `db:"id"`
		Kept bool      `db:"kept"`
	}
	all, err := pgx.CollectRows(rows, pgx.RowToStructByName[candidate])
	if err != nil {
		return nil, 0, fmt.Errorf("list case documents: %w", err)
	}
	var doomed []uuid.UUID
	for _, c := range all {
		if !c.Kept {
			doomed = append(doomed, c.ID)
		}
	}
	return doomed, len(all) - len(doomed), nil
}

// blankCaseTx turns the case into its notice and blanks its content.
func blankCaseTx(ctx context.Context, tx pgx.Tx, caseID uuid.UUID) error {
	var label string
	if err := tx.QueryRow(ctx, caseLabelSQL, pgx.NamedArgs{"id": caseID, "prefix": eliminatedLabel}).Scan(&label); err != nil {
		return fmt.Errorf("case notice label: %w", err)
	}
	for _, step := range []struct {
		sql  string
		args pgx.NamedArgs
	}{
		{blankCaseSQL, pgx.NamedArgs{"id": caseID, "label": label}},
		{blankSubjectSQL, pgx.NamedArgs{"id": caseID, "label": label}},
		{blankCaseContentSQL, pgx.NamedArgs{"id": caseID, "text": eliminatedText}},
	} {
		if _, err := tx.Exec(ctx, step.sql, step.args); err != nil {
			return fmt.Errorf("blank case: %w", err)
		}
	}
	return nil
}

// eliminateDocumentsTx blanks and logically deletes the doomed documents and
// marks destroyed the files no other document version holds; it returns their
// storage references.
func eliminateDocumentsTx(ctx context.Context, tx pgx.Tx, docs []uuid.UUID, operatorID string) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	if _, err := tx.Exec(ctx, blankDocumentsSQL, pgx.NamedArgs{"ids": docs, "label": eliminatedDocumentLabel, "operator_id": operatorID}); err != nil {
		return nil, fmt.Errorf("blank documents: %w", err)
	}
	for _, id := range docs {
		if _, err := tx.Exec(ctx, blankSubjectSQL, pgx.NamedArgs{"id": id, "label": eliminatedDocumentLabel}); err != nil {
			return nil, fmt.Errorf("blank document subject: %w", err)
		}
	}
	refs, err := collectRefs(tx.Query(ctx, disposeBlobsSQL, pgx.NamedArgs{"ids": docs}))
	if err != nil {
		return nil, fmt.Errorf("dispose files: %w", err)
	}
	return refs, nil
}

// recordOutcomeTx freezes the case and records the outcome of its retention.
func recordOutcomeTx(ctx context.Context, tx pgx.Tx, b *Bordereau, item *Item, outcome, operatorID string) error {
	if _, err := core.LockRecordMetadataTx(ctx, tx, item.CaseID, operatorID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, executeRetentionSQL, pgx.NamedArgs{"id": item.RetentionID, "outcome": outcome, "bordereau_id": b.ID})
	if err != nil {
		return fmt.Errorf("record outcome: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: the retention of case %s is no longer running", core.ErrInvalidState, item.CaseID)
	}
	return nil
}

// collectRefs reads the non-empty storage references a statement returned.
func collectRefs(rows pgx.Rows, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	all, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	refs := all[:0]
	for _, r := range all {
		if r != "" {
			refs = append(refs, r)
		}
	}
	return refs, nil
}

// destroyFiles deletes the bytes of eliminated files and archives after the
// commit; a failure is logged (the database already records them destroyed)
// so an operator can remove the leftover by its reference.
func (r *PostgresRepository) destroyFiles(ctx context.Context, refs []string) {
	for _, ref := range refs {
		if err := r.store.Delete(context.WithoutCancel(ctx), ref); err != nil {
			r.log.Error("elimination: file bytes not deleted, remove them by hand", "storage_ref", ref, "error", err)
		}
	}
}
