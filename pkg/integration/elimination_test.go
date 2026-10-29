package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/retention"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// TestEliminationErasesTheCase covers GLD-077: executing an approved DESTROY
// bordereau keeps a notice of the case and the proof, blanks its texts and the
// states of its audit, eliminates the documents only it held (files whose
// bytes no other document holds are destroyed) and keeps a document shared
// with a case that stays; the immutability guards hold outside an elimination.
func TestEliminationErasesTheCase(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	typeCode := dueNowCaseType(t, env, retention.DispositionDestroy)
	doomed, _, err := env.caseSvc.Create(ctx, casefile.CreateInput{CaseTypeCode: typeCode, Title: "Litige Dupont " + uniqueToken(), Description: "Données personnelles", OperatorID: testOperator})
	if err != nil {
		t.Fatal(err)
	}
	kept, _, err := env.caseSvc.Create(ctx, caseInput("Affaire qui reste "+uniqueToken(), 0))
	if err != nil {
		t.Fatal(err)
	}
	ownContent := "contenu propre " + uniqueToken()
	own := caseDocument(t, env, doomed.ID, ownContent, 0)
	shared := caseDocument(t, env, doomed.ID, "contenu partagé "+uniqueToken(), 0)
	link(t, env, kept.ID, core.LinkInput{TargetSubjectID: shared.ID, RelationshipTypeCode: "CASE_HAS_DOCUMENT"})
	entry := addEntry(t, env, timeline.CreateInput{CaseID: doomed.ID, Type: timeline.TypeComment, Body: "M. Dupont a appelé", OperatorID: testOperator})
	if _, _, err := env.timelineSvc.Validate(ctx, entry.ID, testOperator, ""); err != nil {
		t.Fatal(err)
	}
	transition(t, env, doomed.ID, casefile.StatusClosed, "jugement rendu")
	ownBlob := blobOf(t, env, own.ID)
	var ownRef string
	if err := env.pool.QueryRow(ctx, `SELECT storage_ref FROM content_blob WHERE id = $1`, ownBlob).Scan(&ownRef); err != nil {
		t.Fatal(err)
	}

	archivist := newUser(t, env, "Élise Archiviste")
	b, err := env.retentionSvc.CreateBordereau(ctx, retention.BordereauInput{
		Disposition: retention.DispositionDestroy, Reason: "élimination annuelle", CaseIDs: []uuid.UUID{doomed.ID}, OperatorID: archivist.UserID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.retentionSvc.Decide(ctx, retention.DecisionInput{ItemID: b.Items[0].ID, Approve: true, Reason: "ok", OperatorID: testOperator}); err != nil {
		t.Fatal(err)
	}
	assertDisabledEliminationSimulates(t, env, b.ID, archivist.UserID, doomed)
	done, err := env.retentionSvc.Execute(ctx, b.ID, archivist.UserID)
	if err != nil || done.Status != retention.BordereauExecuted || len(done.ProofSHA256) != 64 {
		t.Fatalf("execute elimination: %+v (%v)", done, err)
	}

	notice, err := env.caseSvc.Get(ctx, doomed.ID)
	if err != nil || notice.Title == doomed.Title || notice.Description != "" {
		t.Fatalf("the case becomes a notice: %+v (%v)", notice, err)
	}
	if got, _ := env.retentionSvc.CaseRetention(ctx, doomed.ID); got.Current == nil || *got.Current.DispositionStatus != retention.OutcomeDisposed {
		t.Fatalf("the retention is DISPOSED: %+v", got.Current)
	}
	var body string
	if err := env.pool.QueryRow(ctx, `SELECT body FROM case_timeline_entry WHERE id = $1`, entry.ID).Scan(&body); err != nil || body != "[éliminé]" {
		t.Fatalf("a validated timeline entry is blanked: %q (%v)", body, err)
	}
	assertAuditBlanked(t, env, doomed.ID)
	assertDocumentsAfterElimination(t, env, own.ID, shared.ID, ownBlob)
	if obj, err := env.store.Get(ctx, ownRef); err == nil {
		_ = obj.Close()
		t.Fatal("the destroyed file's bytes are gone from the store")
	}
	if again := ingest(t, env, ownContent); again.Blob.ID != ownBlob || again.Blob.StorageRef == "" || again.Reused {
		t.Fatalf("an identical upload stores the bytes again: %+v", again)
	}

	if _, err := env.pool.Exec(ctx, `UPDATE audit_event SET reason = 'x' WHERE subject_id = $1`, doomed.ID); err == nil {
		t.Fatal("outside an elimination the audit stays append-only")
	}
	if _, err := env.pool.Exec(ctx, `UPDATE case_timeline_entry SET body = 'x' WHERE id = $1`, entry.ID); err == nil {
		t.Fatal("outside an elimination a validated entry stays immutable")
	}
	if _, _, err := env.caseSvc.Transition(ctx, doomed.ID, casefile.TransitionInput{Target: casefile.StatusOpen, Reason: "x", OperatorID: testOperator}); !errors.Is(err, core.ErrLocked) {
		t.Fatalf("an eliminated case is frozen: %v", err)
	}
}

// assertDisabledEliminationSimulates covers a server where elimination is
// off (GOELAND_DISPOSAL_ENABLED unset): executing is refused, simulating
// reports what the case would lose (its own document and file, the shared
// document kept) and leaves everything as it was.
func assertDisabledEliminationSimulates(t *testing.T, env *testEnv, bordereauID uuid.UUID, operatorID string, c *casefile.Case) {
	t.Helper()
	repo, err := retention.NewPostgresRepository(env.pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := retention.NewService(repo.WithBlobStore(env.store), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := disabled.Execute(env.ctx, bordereauID, operatorID); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("elimination disabled: execution refused, got %v", err)
	}
	got, err := disabled.SimulateElimination(env.ctx, bordereauID, operatorID)
	want := retention.Elimination{CaseID: c.ID, DocumentsEliminated: 1, DocumentsKept: 1, FilesDestroyed: 1}
	if err != nil || len(got) != 1 || got[0] != want {
		t.Fatalf("simulation: want [%+v], got %+v (%v)", want, got, err)
	}
	same, err := env.caseSvc.Get(env.ctx, c.ID)
	if err != nil || same.Title != c.Title || same.Description != c.Description {
		t.Fatalf("a simulation changes nothing: %+v (%v)", same, err)
	}
	b, err := env.retentionSvc.GetBordereau(env.ctx, bordereauID)
	if err != nil || b.Status != retention.BordereauOpen {
		t.Fatalf("a simulated bordereau stays open: %+v (%v)", b, err)
	}
}

// blobOf returns the content blob of a document's current version.
func blobOf(t *testing.T, env *testEnv, docID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT v.content_blob_id FROM document d JOIN document_version v ON v.id = d.current_version_id WHERE d.id = $1`, docID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// assertAuditBlanked requires every audit event of the case but the last
// (RETENTION_DISPOSED) to carry only the marker.
func assertAuditBlanked(t *testing.T, env *testEnv, caseID uuid.UUID) {
	t.Helper()
	var total, marked, disposed int
	err := env.pool.QueryRow(env.ctx, `
SELECT count(*), count(*) FILTER (WHERE after_state = '{"eliminated": true}'::jsonb AND reason = '' AND before_state IS NULL),
       count(*) FILTER (WHERE event_type = 'RETENTION_DISPOSED')
FROM audit_event WHERE subject_id = $1`, caseID).Scan(&total, &marked, &disposed)
	if err != nil || total < 5 || disposed != 1 || marked != total-1 {
		t.Fatalf("audit states blanked, who and when kept: %d events, %d marked, %d disposed (%v)", total, marked, disposed, err)
	}
}

// assertDocumentsAfterElimination checks the eliminated and the kept documents
// and that an identical upload revives the destroyed file.
func assertDocumentsAfterElimination(t *testing.T, env *testEnv, own, shared, ownBlob uuid.UUID) {
	t.Helper()
	var title string
	var deleted bool
	if err := env.pool.QueryRow(env.ctx, `SELECT d.title, rm.deleted_at IS NOT NULL FROM document d JOIN record_metadata rm ON rm.subject_id = d.id WHERE d.id = $1`, own).Scan(&title, &deleted); err != nil || title != "Document éliminé" || !deleted {
		t.Fatalf("the case's own document is eliminated: %q deleted=%v (%v)", title, deleted, err)
	}
	if err := env.pool.QueryRow(env.ctx, `SELECT d.title, rm.deleted_at IS NOT NULL FROM document d JOIN record_metadata rm ON rm.subject_id = d.id WHERE d.id = $1`, shared).Scan(&title, &deleted); err != nil || title == "Document éliminé" || deleted {
		t.Fatalf("a document shared with a case that stays is kept: %q deleted=%v (%v)", title, deleted, err)
	}
	var ref string
	var disposed bool
	if err := env.pool.QueryRow(env.ctx, `SELECT storage_ref, disposed_at IS NOT NULL FROM content_blob WHERE id = $1`, ownBlob).Scan(&ref, &disposed); err != nil || ref != "" || !disposed {
		t.Fatalf("the file only the document held is destroyed: %q %v (%v)", ref, disposed, err)
	}
}

// TestRebuildTransferPackage covers rebuilding the failed package of a
// transferred case, once.
func TestRebuildTransferPackage(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := closedOwnedCase(t, env, dueNowCaseType(t, env, retention.DispositionArchive), nil)
	archivist := newUser(t, env, "Rémi Archiviste")
	b, err := env.retentionSvc.CreateBordereau(ctx, retention.BordereauInput{Disposition: retention.DispositionArchive, Reason: "versement", CaseIDs: []uuid.UUID{c.ID}, OperatorID: archivist.UserID})
	if err != nil {
		t.Fatal(err)
	}
	item := b.Items[0]
	if _, err := env.retentionSvc.Decide(ctx, retention.DecisionInput{ItemID: item.ID, Approve: true, Reason: "ok", OperatorID: testOperator}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.retentionSvc.RebuildTransfer(ctx, item.ID, archivist.UserID); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("before the execution nothing is rebuilt: %v", err)
	}
	done, err := env.retentionSvc.Execute(ctx, b.ID, archivist.UserID)
	if err != nil {
		t.Fatal(err)
	}
	first := *done.Items[0].ExportID
	if _, err := env.retentionSvc.RebuildTransfer(ctx, item.ID, archivist.UserID); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("a package still queued is not rebuilt: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE case_export SET status = 'FAILED', error = 'disque plein' WHERE id = $1`, first); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := env.retentionSvc.RebuildTransfer(ctx, item.ID, archivist.UserID)
	if err != nil || rebuilt.ExportID == nil || *rebuilt.ExportID == first || rebuilt.ExportStatus == nil || *rebuilt.ExportStatus != "QUEUED" {
		t.Fatalf("a failed package is queued again: %+v (%v)", rebuilt, err)
	}
}
