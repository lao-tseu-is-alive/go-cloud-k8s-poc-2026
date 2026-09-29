package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// newVersionedDocument creates a document whose version 1 holds fresh content.
func newVersionedDocument(t *testing.T, env *testEnv) *document.Document {
	t.Helper()
	blob := ingest(t, env, "timeline plan "+uniqueToken()).Blob
	created, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Plan cité " + uniqueToken(), ContentBlobID: &blob.ID, OperatorID: testOperator,
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	return created.Document
}

// addEntry creates a draft entry and fails the test on error.
func addEntry(t *testing.T, env *testEnv, in timeline.CreateInput) *timeline.Entry {
	t.Helper()
	in.OperatorID = testOperator
	e, ev, err := env.timelineSvc.Create(env.ctx, in)
	if err != nil {
		t.Fatalf("create timeline entry: %v", err)
	}
	if ev.EventType != "TIMELINE_ENTRY_ADDED" || ev.SubjectID != in.CaseID || ev.Metadata["timeline_entry_id"] != e.ID.String() {
		t.Fatalf("TIMELINE_ENTRY_ADDED must be audited on the case and name the entry, got %+v", ev)
	}
	return e
}

// TestTimelineLifecycle covers v2 §50 steps 19-21 and §26-27: add a follow-up,
// cite a document (linked to the case on the way), validate it (immutable, the
// document version pinned) and correct it with a new entry.
func TestTimelineLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Timeline "+uniqueToken())
	doc := newVersionedDocument(t, env)

	draft := addEntry(t, env, timeline.CreateInput{
		CaseID: c.ID, Type: timeline.TypeOpinion, Title: "Préavis", Body: "Préavis favorable sous conditions",
		DocumentIDs: []uuid.UUID{doc.ID},
	})
	if draft.Status != timeline.StatusDraft || len(draft.Documents) != 1 || draft.Documents[0].DocumentVersionID != nil {
		t.Fatalf("a new entry is a draft citing the document without a pinned version: %+v", draft)
	}
	assertCaseHasDocument(t, env, c.ID, doc.ID)

	updated, ev, err := env.timelineSvc.Update(ctx, draft.ID, timeline.UpdateInput{Type: timeline.TypeDecision, Body: "Décision : permis accordé", OperatorID: testOperator, Reason: "reformulé"})
	if err != nil || updated.Type != timeline.TypeDecision || updated.UpdatedAt == nil || ev.BeforeState["entry_type"] != "OPINION" {
		t.Fatalf("a draft is editable: %+v / %+v (%v)", updated, ev, err)
	}

	validated, ev, err := env.timelineSvc.Validate(ctx, draft.ID, testOperator, "")
	if err != nil || validated.Status != timeline.StatusValidated || validated.ValidatedBy != testOperator || ev.EventType != "TIMELINE_ENTRY_VALIDATED" {
		t.Fatalf("validate: %+v (%v)", validated, err)
	}
	pinned := validated.Documents[0]
	if pinned.DocumentVersionID == nil || *pinned.DocumentVersionID != *doc.CurrentVersionID || pinned.DocumentVersionNo == nil || *pinned.DocumentVersionNo != 1 {
		t.Fatalf("validation must pin the current document version, got %+v", pinned)
	}
	assertEntryImmutable(t, env, validated, doc.ID)
	assertCorrections(t, env, c, validated)
}

// assertCaseHasDocument checks that citing the document linked it to the case.
func assertCaseHasDocument(t *testing.T, env *testEnv, caseID, documentID uuid.UUID) {
	t.Helper()
	rels, err := env.caseSvc.Relationships(env.ctx, caseID)
	if err != nil {
		t.Fatalf("case relationships: %v", err)
	}
	for _, rel := range rels {
		if rel.RelationshipType.Code == "CASE_HAS_DOCUMENT" && rel.TargetSubjectID == documentID {
			return
		}
	}
	t.Fatalf("citing a document must link it to the case (CASE_HAS_DOCUMENT), got %d relationships", len(rels))
}

// assertEntryImmutable checks that a validated entry refuses every change,
// through the service and directly in the database (trigger).
func assertEntryImmutable(t *testing.T, env *testEnv, e *timeline.Entry, documentID uuid.UUID) {
	t.Helper()
	ctx := env.ctx
	if _, _, err := env.timelineSvc.Update(ctx, e.ID, timeline.UpdateInput{Type: timeline.TypeComment, Body: "réécrit", OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("updating a validated entry: want ErrInvalidState, got %v", err)
	}
	if _, _, err := env.timelineSvc.Withdraw(ctx, e.ID, testOperator, "oups"); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("withdrawing a validated entry: want ErrInvalidState, got %v", err)
	}
	if _, _, err := env.timelineSvc.UnlinkDocument(ctx, e.ID, documentID, testOperator, ""); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("unlinking from a validated entry: want ErrInvalidState, got %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE case_timeline_entry SET body = 'forged' WHERE id = $1`, e.ID); err == nil {
		t.Fatal("the database must refuse to rewrite a validated entry")
	}
	if _, err := env.pool.Exec(ctx, `DELETE FROM case_timeline_entry WHERE id = $1`, e.ID); err == nil {
		t.Fatal("the database must refuse to delete a timeline entry")
	}
	if _, err := env.pool.Exec(ctx, `UPDATE timeline_document_link SET removed_at = now() WHERE timeline_entry_id = $1`, e.ID); err == nil {
		t.Fatal("the database must refuse to change the documents of a validated entry")
	}
}

// assertCorrections checks the correction rules: a new entry names the
// immutable entry it corrects, once, within the same case.
func assertCorrections(t *testing.T, env *testEnv, c *casefile.Case, original *timeline.Entry) {
	t.Helper()
	ctx := env.ctx
	correction := addEntry(t, env, timeline.CreateInput{CaseID: c.ID, Type: timeline.TypeDecision, Body: "Décision corrigée", CorrectsEntryID: &original.ID})
	if correction.CorrectsEntryID == nil || *correction.CorrectsEntryID != original.ID {
		t.Fatalf("a correction names the corrected entry, got %+v", correction)
	}
	reread, err := env.timelineSvc.Get(ctx, original.ID)
	if err != nil || reread.CorrectedByEntryID == nil || *reread.CorrectedByEntryID != correction.ID || reread.Body != original.Body {
		t.Fatalf("the original stays unchanged and points at its correction: %+v (%v)", reread, err)
	}
	second := timeline.CreateInput{CaseID: c.ID, Type: timeline.TypeDecision, Body: "Autre correction", CorrectsEntryID: &original.ID, OperatorID: testOperator}
	if _, _, err := env.timelineSvc.Create(ctx, second); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a second correction: want ErrConflict, got %v", err)
	}
	ofDraft := timeline.CreateInput{CaseID: c.ID, Type: timeline.TypeComment, Body: "Correction d'un brouillon", CorrectsEntryID: &correction.ID, OperatorID: testOperator}
	if _, _, err := env.timelineSvc.Create(ctx, ofDraft); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("correcting a draft: want ErrInvalidState, got %v", err)
	}
	other := openCase(t, env, "Autre affaire "+uniqueToken())
	elsewhere := timeline.CreateInput{CaseID: other.ID, Type: timeline.TypeComment, Body: "Mauvaise affaire", CorrectsEntryID: &original.ID, OperatorID: testOperator}
	if _, _, err := env.timelineSvc.Create(ctx, elsewhere); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("correcting an entry of another case: want ErrInvalidInput, got %v", err)
	}
	if _, _, err := env.timelineSvc.Withdraw(ctx, correction.ID, testOperator, "finalement inutile"); err != nil {
		t.Fatalf("withdraw the correction draft: %v", err)
	}
	// A withdrawn correction frees the corrected entry for a new one.
	addEntry(t, env, second)
}

// TestTimelineAndCaseClosure covers the case/timeline interplay: SYSTEM entries
// record status changes, drafts block closure, a closed case accepts no entry,
// and the whole history is in the case audit trail.
func TestTimelineAndCaseClosure(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Clôture timeline "+uniqueToken())

	system := timeline.CreateInput{CaseID: c.ID, Type: timeline.TypeSystem, Body: "forgé", OperatorID: testOperator}
	if _, _, err := env.timelineSvc.Create(ctx, system); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("operators cannot write SYSTEM entries: got %v", err)
	}
	draft := addEntry(t, env, timeline.CreateInput{CaseID: c.ID, Type: timeline.TypeRequest, Body: "Merci de fournir le plan de situation"})
	if res, err := env.timelineSvc.List(ctx, timeline.ListFilter{CaseID: c.ID, Types: []timeline.EntryType{timeline.TypeSystem}}); err != nil || res.DraftCount != 1 {
		t.Fatalf("the draft count ignores the filters: got %d (%v)", res.DraftCount, err)
	}
	closing := casefile.TransitionInput{Target: casefile.StatusClosed, OperatorID: testOperator, Reason: "permis délivré"}
	if _, _, err := env.caseSvc.Transition(ctx, c.ID, closing); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("closing a case with drafts: want ErrInvalidState, got %v", err)
	}
	if _, _, err := env.timelineSvc.Lock(ctx, draft.ID, testOperator, "envoyé"); err != nil {
		t.Fatalf("lock the draft: %v", err)
	}
	if _, _, err := env.caseSvc.Transition(ctx, c.ID, closing); err != nil {
		t.Fatalf("close once no draft remains: %v", err)
	}
	late := timeline.CreateInput{CaseID: c.ID, Type: timeline.TypeComment, Body: "trop tard", OperatorID: testOperator}
	if _, _, err := env.timelineSvc.Create(ctx, late); !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("a closed case accepts no entry: got %v", err)
	}

	res, err := env.timelineSvc.List(ctx, timeline.ListFilter{CaseID: c.ID, Types: []timeline.EntryType{timeline.TypeSystem}})
	if err != nil || len(res.Entries) != 1 {
		t.Fatalf("closing must record one SYSTEM entry, got %d (%v)", len(res.Entries), err)
	}
	sys := res.Entries[0]
	if sys.Status != timeline.StatusLocked || sys.Metadata["event"] != "CASE_STATUS_CHANGED" || sys.Metadata["to"] != "CLOSED" || sys.Metadata["reason"] != "permis délivré" {
		t.Fatalf("the SYSTEM entry is locked and structured, got %+v", sys)
	}
	all, err := env.timelineSvc.List(ctx, timeline.ListFilter{CaseID: c.ID})
	if err != nil || all.TotalSize != 2 || all.Entries[0].ID != sys.ID {
		t.Fatalf("the timeline lists the closure first, then the request: %+v (%v)", all, err)
	}
	assertTimelineAudit(t, env, c.ID)
}

// assertTimelineAudit checks that the timeline mutations are in the case trail.
func assertTimelineAudit(t *testing.T, env *testEnv, caseID uuid.UUID) {
	t.Helper()
	res, err := env.coreSvc.ListAuditEvents(env.ctx, core.AuditFilter{SubjectID: caseID, Limit: 50})
	if err != nil {
		t.Fatalf("case audit: %v", err)
	}
	seen := map[string]int{}
	for _, ev := range res.Events {
		seen[ev.EventType]++
	}
	if seen["TIMELINE_ENTRY_ADDED"] != 2 || seen["TIMELINE_ENTRY_LOCKED"] != 1 || seen["CASE_STATUS_CHANGED"] != 1 {
		t.Fatalf("unexpected case audit trail %v", seen)
	}
}
