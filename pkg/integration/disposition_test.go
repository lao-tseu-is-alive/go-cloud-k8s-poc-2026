package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/caseexport"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/retention"
)

// dueNowCaseType creates a case type whose cases are due on their closing day.
func dueNowCaseType(t *testing.T, env *testEnv, d retention.Disposition) string {
	t.Helper()
	policy := referenceCode("ITZ")
	if _, _, err := env.retentionSvc.CreatePolicy(env.ctx, retention.PolicyInput{
		Code: policy, Label: "Échéance immédiate", DurationYears: 0, FinalDisposition: d, OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("create policy: %v", err)
	}
	return newRetentionCaseType(t, env, policy)
}

// closedOwnedCase creates a case of a type, owned by unit when not nil, and closes it.
func closedOwnedCase(t *testing.T, env *testEnv, typeCode string, unit *uuid.UUID) *casefile.Case {
	t.Helper()
	c, _, err := env.caseSvc.Create(env.ctx, casefile.CreateInput{
		CaseTypeCode: typeCode, Title: "Dossier échu " + uniqueToken(), OperatorID: testOperator,
		Governance: core.CreateSubjectInput{OwnerOrgID: unit},
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	transition(t, env, c.ID, casefile.StatusClosed, "terminé")
	return c
}

// TestDispositionBordereauArchive covers GLD-032 step B1: a records manager
// gathers due cases in a bordereau; each is decided by a holder of MANAGE on
// its owning unit (or on the case without one), never by the preparer; the
// execution transfers the approved cases (complete package, frozen case,
// ARCHIVED retention) and keeps the proof.
func TestDispositionBordereauArchive(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	typeCode := dueNowCaseType(t, env, retention.DispositionArchive)
	unit := newUnit(t, env, "SERVICE", "Service versant "+uniqueToken(), nil)
	owned := closedOwnedCase(t, env, typeCode, &unit.ID)
	orphan := closedOwnedCase(t, env, typeCode, nil)
	archivist := newUser(t, env, "Aline Archiviste")

	b, err := env.retentionSvc.CreateBordereau(ctx, retention.BordereauInput{
		Disposition: retention.DispositionArchive, Reason: "versement annuel", CaseIDs: []uuid.UUID{owned.ID, orphan.ID}, OperatorID: archivist.UserID,
	})
	if err != nil || b.Status != retention.BordereauOpen || len(b.Items) != 2 || b.PendingCount != 2 {
		t.Fatalf("create bordereau: %+v (%v)", b, err)
	}
	approvers := map[uuid.UUID]uuid.UUID{}
	items := map[uuid.UUID]*retention.Item{}
	for _, it := range b.Items {
		approvers[it.CaseID], items[it.CaseID] = it.ApproverSubjectID, it
	}
	if approvers[owned.ID] != unit.ID || approvers[orphan.ID] != orphan.ID {
		t.Fatalf("the owning unit approves, or the case without one: %v", approvers)
	}
	if _, err := env.retentionSvc.CreateBordereau(ctx, retention.BordereauInput{
		Disposition: retention.DispositionArchive, Reason: "doublon", CaseIDs: []uuid.UUID{owned.ID}, OperatorID: archivist.UserID,
	}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a case in an open bordereau: want ErrConflict, got %v", err)
	}

	decide := func(c *casefile.Case, approve bool, by string) error {
		_, err := env.retentionSvc.Decide(ctx, retention.DecisionInput{ItemID: items[c.ID].ID, Approve: approve, Reason: "vu", OperatorID: by})
		return err
	}
	if err := decide(owned, true, archivist.UserID); !errors.Is(err, core.ErrPermissionDenied) {
		t.Fatalf("the preparer does not approve: %v", err)
	}
	if _, err := env.retentionSvc.Execute(ctx, b.ID, archivist.UserID); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("executing with pending cases: want ErrInvalidState, got %v", err)
	}
	mine, err := env.retentionSvc.MyApprovals(ctx, testOperator)
	if err != nil || !hasApproval(mine, items[owned.ID].ID) || !hasApproval(mine, items[orphan.ID].ID) {
		t.Fatalf("the unit's and the case's manager sees both approvals: %v", err)
	}
	if err := decide(owned, true, testOperator); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := decide(orphan, false, testOperator); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err := decide(orphan, true, testOperator); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("deciding twice: want ErrInvalidState, got %v", err)
	}

	done, err := env.retentionSvc.Execute(ctx, b.ID, archivist.UserID)
	if err != nil || done.Status != retention.BordereauExecuted || len(done.ProofSHA256) != 64 || done.ApprovedCount != 1 {
		t.Fatalf("execute: %+v (%v)", done, err)
	}
	if sum := sha256.Sum256([]byte(done.ProofManifest)); hex.EncodeToString(sum[:]) != done.ProofSHA256 || !strings.Contains(done.ProofManifest, owned.ID.String()) {
		t.Fatalf("anyone recomputes the proof from the stored manifest: %s", done.ProofManifest)
	}
	got, _ := env.retentionSvc.CaseRetention(ctx, owned.ID)
	if got.Current == nil || got.Current.DispositionStatus == nil || *got.Current.DispositionStatus != retention.OutcomeArchived {
		t.Fatalf("the approved case is ARCHIVED: %+v", got.Current)
	}
	if _, _, err := env.caseSvc.Transition(ctx, owned.ID, casefile.TransitionInput{Target: casefile.StatusOpen, OperatorID: testOperator, Reason: "x"}); !errors.Is(err, core.ErrLocked) {
		t.Fatalf("an archived case is frozen: %v", err)
	}
	if rest, _ := env.retentionSvc.CaseRetention(ctx, orphan.ID); rest.Current == nil || rest.Current.DispositionStatus != nil {
		t.Fatalf("the rejected case waits again: %+v", rest.Current)
	}
	assertTransferPackage(t, env, done, owned.ID, archivist.UserID)
}

// assertTransferPackage builds the transfer package of the archived case and
// checks that only records managers download it.
func assertTransferPackage(t *testing.T, env *testEnv, b *retention.Bordereau, caseID uuid.UUID, archivist string) {
	t.Helper()
	var exportID uuid.UUID
	for _, it := range b.Items {
		if it.CaseID == caseID && it.ExportID != nil {
			exportID = *it.ExportID
		}
	}
	if exportID == uuid.Nil {
		t.Fatalf("the archived case has a transfer package: %+v", b.Items)
	}
	for range 20 {
		exp, err := env.exportSvc.Get(env.ctx, exportID, archivist, true)
		if err != nil {
			t.Fatal(err)
		}
		if exp.Status == caseexport.StatusDone {
			break
		}
		if _, err := env.exportSvc.ProcessNext(env.ctx); err != nil {
			t.Fatal(err)
		}
	}
	archive, err := env.exportSvc.OpenArchive(env.ctx, exportID, archivist, true)
	if err != nil || archive.Export.Purpose != caseexport.PurposeTransfer || archive.Export.SizeBytes == 0 {
		t.Fatalf("a records manager downloads the package: %+v (%v)", archive, err)
	}
	_ = archive.Object.Close()
	var downloads int
	if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM access_audit_event WHERE subject_id = $1 AND action = 'DOWNLOAD' AND user_id = $2`, caseID, archivist).Scan(&downloads); err != nil || downloads != 1 {
		t.Fatalf("the download of a transfer package is logged: %d (%v)", downloads, err)
	}
	if _, err := env.exportSvc.OpenArchive(env.ctx, exportID, archivist, false); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("without the records scope: want ErrNotFound, got %v", err)
	}
}

// TestDispositionBordereauRules covers what may not enter a bordereau and its cancellation.
func TestDispositionBordereauRules(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	archivist := newUser(t, env, "Bruno Archiviste")
	in := func(d retention.Disposition, ids ...uuid.UUID) retention.BordereauInput {
		return retention.BordereauInput{Disposition: d, Reason: "test", CaseIDs: ids, OperatorID: archivist.UserID}
	}
	notDue := closedCase(t, env, newRetentionCaseType(t, env, "EXAMPLE_ARCHIVE_10"))
	if _, err := env.retentionSvc.CreateBordereau(ctx, in(retention.DispositionArchive, notDue.ID)); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("a case not due: want ErrInvalidState, got %v", err)
	}
	destroy := closedOwnedCase(t, env, dueNowCaseType(t, env, retention.DispositionDestroy), nil)
	if _, err := env.retentionSvc.CreateBordereau(ctx, in(retention.DispositionArchive, destroy.ID)); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("a case due for destruction in a transfer: want ErrInvalidInput, got %v", err)
	}
	if _, _, err := env.retentionSvc.PlaceHold(ctx, retention.HoldInput{CaseID: destroy.ID, Reason: "enquête", OperatorID: archivist.UserID}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.retentionSvc.CreateBordereau(ctx, in(retention.DispositionDestroy, destroy.ID)); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("a held case: want ErrInvalidState, got %v", err)
	}
	if _, _, err := env.retentionSvc.ReleaseHold(ctx, retention.HoldInput{CaseID: destroy.ID, Reason: "close", OperatorID: archivist.UserID}); err != nil {
		t.Fatal(err)
	}
	b, err := env.retentionSvc.CreateBordereau(ctx, in(retention.DispositionDestroy, destroy.ID))
	if err != nil {
		t.Fatalf("create destroy bordereau: %v", err)
	}
	cancelled, err := env.retentionSvc.Cancel(ctx, b.ID, "reporté", archivist.UserID)
	if err != nil || cancelled.Status != retention.BordereauCancelled {
		t.Fatalf("cancel: %+v (%v)", cancelled, err)
	}
	if _, err := env.retentionSvc.CreateBordereau(ctx, in(retention.DispositionDestroy, destroy.ID)); err != nil {
		t.Fatalf("a case of a cancelled bordereau can be proposed again: %v", err)
	}
}

func hasApproval(list []retention.Approval, itemID uuid.UUID) bool {
	for _, a := range list {
		if a.Item.ID == itemID {
			return true
		}
	}
	return false
}
