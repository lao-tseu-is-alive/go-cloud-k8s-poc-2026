package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

// TestContentVerification covers GLD-021: rereading the stored bytes sets the
// content's verified_at and audits DOCUMENT_CONTENT_VERIFIED; altered or
// missing bytes clear it and audit DOCUMENT_CONTENT_VERIFICATION_FAILED; a
// document whose bytes Goéland does not hold reports NOT_HELD and writes nothing.
func TestContentVerification(t *testing.T) {
	env := newTestEnv(t)
	blob := ingest(t, env, "plan cadastral "+uniqueToken()).Blob
	doc := createDocumentOn(t, env, blob.ID)

	res := verifyContent(t, env, doc, document.OutcomeVerified)
	if res.VerifiedAt == nil || res.Event == nil || res.Event.EventType != "DOCUMENT_CONTENT_VERIFIED" || res.ComputedSHA256 != blob.SHA256 {
		t.Fatalf("intact bytes are verified and audited: %+v", res)
	}
	assertVerifiedAt(t, env, blob.ID, true)

	path := filepath.Join(env.blobDir, strings.TrimPrefix(blob.StorageRef, "internal://"))
	if err := os.WriteFile(path, []byte("altéré"), 0o600); err != nil {
		t.Fatal(err)
	}
	res = verifyContent(t, env, doc, document.OutcomeMismatch)
	if res.VerifiedAt != nil || res.Event.EventType != "DOCUMENT_CONTENT_VERIFICATION_FAILED" || res.Event.AfterState["computed_sha256"] != res.ComputedSHA256 {
		t.Fatalf("altered bytes clear the verification and are audited: %+v", res)
	}
	assertVerifiedAt(t, env, blob.ID, false)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if res = verifyContent(t, env, doc, document.OutcomeMissing); res.Event == nil || res.Event.AfterState["outcome"] != "MISSING" {
		t.Fatalf("missing bytes are audited: %+v", res)
	}

	imported := importedBlob(t, env)
	legacy := createDocumentOn(t, env, imported)
	before := auditCount(t, env, legacy)
	if res = verifyContent(t, env, legacy, document.OutcomeNotHeld); res.Event != nil || auditCount(t, env, legacy) != before {
		t.Fatalf("content known only by its digest writes nothing: %+v", res)
	}
}

// createDocumentOn creates a document on a blob and returns its id.
func createDocumentOn(t *testing.T, env *testEnv, blobID uuid.UUID) uuid.UUID {
	t.Helper()
	created, err := env.docSvc.Create(env.ctx, document.CreateInput{DocumentTypeCode: "PLAN", Title: "Pièce " + uniqueToken(), ContentBlobID: &blobID, OperatorID: testOperator})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	return created.Document.ID
}

// verifyContent verifies the current version of a document and requires an outcome.
func verifyContent(t *testing.T, env *testEnv, docID uuid.UUID, want document.VerificationOutcome) document.ContentVerification {
	t.Helper()
	res, err := env.docSvc.VerifyContent(env.ctx, docID, nil, testOperator)
	if err != nil || res.Outcome != want {
		t.Fatalf("verification outcome %q, want %q (%v)", res.Outcome, want, err)
	}
	return res
}

// assertVerifiedAt requires the blob's verified_at to be set or cleared.
func assertVerifiedAt(t *testing.T, env *testEnv, blobID uuid.UUID, set bool) {
	t.Helper()
	var got bool
	if err := env.pool.QueryRow(env.ctx, `SELECT verified_at IS NOT NULL FROM content_blob WHERE id = $1`, blobID).Scan(&got); err != nil || got != set {
		t.Fatalf("verified_at set = %v, want %v (%v)", got, set, err)
	}
}

// auditCount counts the audit events of a subject.
func auditCount(t *testing.T, env *testEnv, subjectID uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM audit_event WHERE subject_id = $1`, subjectID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
