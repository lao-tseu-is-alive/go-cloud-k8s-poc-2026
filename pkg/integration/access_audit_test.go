package integration

import (
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

// subjectAccessLog lists the sensitive read audit of one subject, newest first,
// as seen by viewer.
func subjectAccessLog(t *testing.T, env *testEnv, subject uuid.UUID, viewer core.Viewer) []*core.AccessAuditEvent {
	t.Helper()
	res, err := env.coreSvc.ListAccessAudit(env.ctx, core.AccessAuditFilter{Viewer: viewer, SubjectID: &subject, Limit: core.MaxPageSize})
	if err != nil {
		t.Fatalf("list sensitive accesses: %v", err)
	}
	return res.Events
}

// TestSensitiveReadsAreAudited covers the sensitive read audit (GLD-033): a
// read of a confidential subject is recorded, granted or denied, a read of a
// public one is not, and the log hides labels from those who cannot read them.
func TestSensitiveReadsAreAudited(t *testing.T) {
	env := newTestEnv(t)
	x := newUser(t, env, "Xavier Lecture")
	secret, _, err := env.caseSvc.Create(env.ctx, caseInput("Dossier secret "+uniqueToken(), 3))
	if err != nil {
		t.Fatalf("create confidential case: %v", err)
	}
	public := openCase(t, env, "Dossier public "+uniqueToken())

	if _, err := core.AuditedAccessTx(env.ctx, env.pool, testOperator, public.ID, core.LevelRead, core.AccessReadSensitive); err != nil {
		t.Fatalf("read the public case: %v", err)
	}
	if got := subjectAccessLog(t, env, public.ID, viewerOf(t, env, testOperator)); len(got) != 0 {
		t.Fatalf("a public read is not audited: %d rows", len(got))
	}

	if _, err := core.AuditedAccessTx(env.ctx, env.pool, testOperator, secret.ID, core.LevelRead, core.AccessReadSensitive); err != nil {
		t.Fatalf("the creator reads its confidential case: %v", err)
	}
	_, err = core.AuditedAccessTx(env.ctx, env.pool, x.UserID, secret.ID, core.LevelRead, core.AccessReadSensitive)
	denied(t, "reading a confidential case without a grant", err)

	events := subjectAccessLog(t, env, secret.ID, viewerOf(t, env, testOperator))
	if len(events) != 2 {
		t.Fatalf("one granted and one denied read: %d rows", len(events))
	}
	refused, allowed := events[0], events[1] // newest first
	if refused.UserID != x.UserID || refused.Outcome != core.AccessDenied || refused.AccessLevel != core.LevelNone {
		t.Fatalf("the refusal is recorded: %+v", refused)
	}
	if allowed.UserID != testOperator || allowed.Outcome != core.AccessGranted || allowed.Action != core.AccessReadSensitive ||
		allowed.AccessSource != core.SourcePersonal || allowed.ConfidentialityLevel != 3 || allowed.SubjectKind != core.SubjectKindCase {
		t.Fatalf("the granted read is recorded: %+v", allowed)
	}
	if allowed.SubjectLabel != secret.Title {
		t.Fatalf("a reader of the subject sees its label: %q", allowed.SubjectLabel)
	}
	if hidden := subjectAccessLog(t, env, secret.ID, viewerOf(t, env, x.UserID)); hidden[0].SubjectLabel != "" {
		t.Fatalf("the log never discloses the label of an unreadable subject: %q", hidden[0].SubjectLabel)
	}

	byUser, err := env.coreSvc.ListAccessAudit(env.ctx, core.AccessAuditFilter{UserID: x.UserID, Outcome: core.AccessDenied})
	if err != nil || len(byUser.Events) != 1 || byUser.Events[0].SubjectID != secret.ID {
		t.Fatalf("the log filters by reader and outcome: %+v (%v)", byUser.Events, err)
	}
	if _, err := env.coreSvc.ListAccessAudit(env.ctx, core.AccessAuditFilter{OrderBy: "user_id"}); err == nil {
		t.Fatal("an unknown sort field is refused")
	}
}

// TestConfidentialDownloadsAreAudited covers DOWNLOAD (GLD-033): the version
// served is recorded, and a refused download is recorded too.
func TestConfidentialDownloadsAreAudited(t *testing.T) {
	env := newTestEnv(t)
	x := newUser(t, env, "Xavier Téléchargement")
	blob := ingest(t, env, "bytes "+uniqueToken()).Blob
	created, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Plan confidentiel " + uniqueToken(), ContentBlobID: &blob.ID, OperatorID: testOperator,
		Governance: core.CreateSubjectInput{ConfidentialityLevel: core.ConfidentialLevel},
	})
	if err != nil {
		t.Fatalf("create confidential document: %v", err)
	}
	doc := created.Document

	if _, err := env.docSvc.Content(env.ctx, testOperator, doc.ID, nil); err != nil {
		t.Fatalf("the creator downloads: %v", err)
	}
	_, err = env.docSvc.Content(env.ctx, x.UserID, doc.ID, nil)
	denied(t, "downloading without READ", err)

	events := subjectAccessLog(t, env, doc.ID, viewerOf(t, env, testOperator))
	if len(events) != 2 {
		t.Fatalf("one granted and one denied download: %d rows", len(events))
	}
	refused, allowed := events[0], events[1]
	if refused.Action != core.AccessDownload || refused.Outcome != core.AccessDenied || refused.DocumentVersionID != nil {
		t.Fatalf("the refused download is recorded without a version: %+v", refused)
	}
	if allowed.Action != core.AccessDownload || allowed.Outcome != core.AccessGranted ||
		allowed.DocumentVersionID == nil || *allowed.DocumentVersionID != *doc.CurrentVersionID {
		t.Fatalf("the granted download names the version served: %+v", allowed)
	}
}
