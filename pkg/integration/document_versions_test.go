package integration

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

// ingest uploads bytes through the service, as the upload endpoint does.
func ingest(t *testing.T, env *testEnv, content string) document.IngestResult {
	t.Helper()
	res, err := env.docSvc.IngestContent(env.ctx, strings.NewReader(content), "plan.pdf", "application/pdf", testOperator)
	if err != nil {
		t.Fatalf("ingest content: %v", err)
	}
	return res
}

// blobFiles counts the files physically stored for the test.
func blobFiles(t *testing.T, env *testEnv) int {
	t.Helper()
	entries, err := os.ReadDir(env.blobDir)
	if err != nil {
		t.Fatalf("read blob dir: %v", err)
	}
	return len(entries)
}

// newCase creates a bare CASE subject to link documents to.
func newCase(t *testing.T, env *testEnv, label string) uuid.UUID {
	t.Helper()
	ref, _, err := createCase(t, env, label, core.BusinessRefRequest{})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	return ref.ID
}

// TestContentDeduplication covers v2 §49 "content blob": new content creates a
// blob, identical content reuses it without storing the bytes twice, including
// under concurrent uploads, and a digest/size mismatch is a hard error.
func TestContentDeduplication(t *testing.T) {
	env := newTestEnv(t)
	content := "dedup " + uniqueToken()

	first := ingest(t, env, content)
	if first.Reused || first.Blob.SHA256 == "" || first.Blob.FileSizeBytes != int64(len(content)) || first.Blob.MimeType != "application/pdf" {
		t.Fatalf("first upload must register new content, got %+v / %+v", first, first.Blob)
	}
	second := ingest(t, env, content)
	if !second.Reused || second.Blob.ID != first.Blob.ID {
		t.Fatalf("identical content must reuse blob %s, got %+v", first.Blob.ID, second)
	}
	if n := blobFiles(t, env); n != 1 {
		t.Fatalf("identical content must be stored once, found %d files", n)
	}

	concurrent := "race " + uniqueToken()
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		ids = map[uuid.UUID]bool{}
	)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := env.docSvc.IngestContent(env.ctx, strings.NewReader(concurrent), "same.pdf", "application/pdf", testOperator)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Errorf("concurrent ingest: %v", err)
				return
			}
			ids[res.Blob.ID] = true
		}()
	}
	wg.Wait()
	if len(ids) != 1 {
		t.Fatalf("concurrent identical uploads must converge on one blob, got %d", len(ids))
	}
	if n := blobFiles(t, env); n != 2 {
		t.Fatalf("expected exactly 2 stored files (two distinct contents), found %d", n)
	}

	repo, err := document.NewPostgresRepository(env.pool, nil)
	if err != nil {
		t.Fatalf("repository: %v", err)
	}
	if _, _, err := repo.RegisterBlob(env.ctx, document.ContentBlob{
		SHA256: first.Blob.SHA256, StorageRef: "internal://other", FileSizeBytes: first.Blob.FileSizeBytes + 1,
	}); err == nil || !strings.Contains(err.Error(), "does not match registered size") {
		t.Fatalf("same digest with another size must be a hard error, got %v", err)
	}
}

// TestDocumentReuseAcrossCases covers v2 §20 and §49 "document relations": the
// same content used in two cases is one document linked to both.
func TestDocumentReuseAcrossCases(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	blob := ingest(t, env, "shared plan "+uniqueToken()).Blob
	caseA, caseB := newCase(t, env, "Case A"), newCase(t, env, "Case B")

	created, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Plan de bâtiment", ContentBlobID: &blob.ID,
		OperatorID: testOperator, LinkToCaseID: &caseA,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	v1 := created.Document.CurrentVersion
	if created.Reused || v1 == nil || v1.VersionNo != 1 || v1.Content == nil || v1.Content.ID != blob.ID {
		t.Fatalf("new document must get version 1 with the blob, got %+v", created)
	}

	reused, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Same plan, other case", ContentBlobID: &blob.ID,
		OperatorID: testOperator, LinkToCaseID: &caseB,
	})
	if err != nil {
		t.Fatalf("create with known content: %v", err)
	}
	if !reused.Reused || reused.Document.ID != created.Document.ID || reused.Event.EventType != "DOCUMENT_REUSED" || reused.Relationship == nil {
		t.Fatalf("known content must reuse document %s and link case B, got %+v", created.Document.ID, reused)
	}
	if reused.Document.Title != "Plan de bâtiment" {
		t.Fatalf("reuse must not overwrite the existing title, got %q", reused.Document.Title)
	}
	for name, id := range map[string]uuid.UUID{"A": caseA, "B": caseB} {
		res, err := env.docSvc.Search(ctx, document.SearchFilter{CaseID: &id})
		if err != nil || !containsDocID(res.Documents, created.Document.ID) || len(res.Documents) != 1 {
			t.Fatalf("case %s must list exactly the shared document: %+v (%v)", name, res.Documents, err)
		}
	}

	// Reusing again for a case already linked keeps the existing edge.
	again, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "x", ContentBlobID: &blob.ID, OperatorID: testOperator, LinkToCaseID: &caseA,
	})
	if err != nil || !again.Reused || again.Relationship != nil {
		t.Fatalf("reuse with an existing link must succeed without a new edge, got %+v (%v)", again, err)
	}

	// A soft-deleted document is never brought back by reuse.
	if _, err := env.docSvc.SoftDelete(ctx, created.Document.ID, testOperator, "retired"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	fresh, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "After deletion", ContentBlobID: &blob.ID, OperatorID: testOperator,
	})
	if err != nil || fresh.Reused || fresh.Document.ID == created.Document.ID {
		t.Fatalf("content of a deleted document must yield a new document, got %+v (%v)", fresh, err)
	}
}

// TestDocumentVersions covers v2 §49 "document version": several versions, a
// blob shared by two versions, an explicit current version, immutability of
// final and record versions, and the lock guard.
func TestDocumentVersions(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	blobA := ingest(t, env, "version A "+uniqueToken()).Blob
	blobB := ingest(t, env, "version B "+uniqueToken()).Blob

	created, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Versioned " + uniqueToken(), ContentBlobID: &blobA.ID, OperatorID: testOperator,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	docID := created.Document.ID

	doc, v2, ev, err := env.docSvc.AddVersion(ctx, docID, document.VersionInput{ContentBlobID: &blobB.ID, OperatorID: testOperator, Reason: "revised"})
	if err != nil || v2.VersionNo != 2 || ev.EventType != "DOCUMENT_VERSION_ADDED" || *doc.CurrentVersionID != v2.ID {
		t.Fatalf("add version 2: %+v / %+v (%v)", v2, ev, err)
	}
	// v2 §19: version 3 re-submits the content of version 1.
	doc, v3, _, err := env.docSvc.AddVersion(ctx, docID, document.VersionInput{ContentBlobID: &blobA.ID, OperatorID: testOperator})
	if err != nil || v3.VersionNo != 3 || v3.Content == nil || v3.Content.ID != blobA.ID || doc.CurrentVersion.ID != v3.ID {
		t.Fatalf("version 3 must reuse blob A and become current: %+v (%v)", v3, err)
	}
	versions, err := env.docSvc.ListVersions(ctx, docID)
	if err != nil || len(versions) != 3 || versions[0].VersionNo != 3 || versions[2].VersionNo != 1 {
		t.Fatalf("expected versions 3,2,1, got %+v (%v)", versions, err)
	}
	if n := blobFiles(t, env); n != 2 {
		t.Fatalf("three versions over two contents must store two files, found %d", n)
	}

	// Finalizing validates the current version, which then becomes immutable.
	final, _, err := env.docSvc.Finalize(ctx, docID, testOperator, "approved", false)
	if err != nil || !final.CurrentVersion.IsFinal || final.Status != document.StatusFinal {
		t.Fatalf("finalize: %+v (%v)", final.CurrentVersion, err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE document_version SET page_count = 9 WHERE id = $1`, v3.ID); err == nil {
		t.Fatal("a validated version must be immutable (database trigger)")
	}
	if _, err := env.pool.Exec(ctx, `DELETE FROM document_version WHERE id = $1`, v2.ID); err == nil {
		t.Fatal("versions must never be deleted (database trigger)")
	}
	// A new draft version reopens the document as DRAFT; the final one stays intact.
	doc, v4, _, err := env.docSvc.AddVersion(ctx, docID, document.VersionInput{OperatorID: testOperator})
	if err != nil || v4.VersionNo != 4 || v4.Content != nil || doc.Status != document.StatusDraft {
		t.Fatalf("metadata-only version 4: %+v status %d (%v)", v4, doc.Status, err)
	}

	// A record is final from its creation.
	record, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "DECISION", Title: "Record " + uniqueToken(), IsRecord: true, OperatorID: testOperator,
	})
	if err != nil || !record.Document.CurrentVersion.IsRecord || !record.Document.CurrentVersion.IsFinal || record.Document.Status != document.StatusFinal {
		t.Fatalf("a record version must be final at creation: %+v (%v)", record.Document.CurrentVersion, err)
	}

	// Locking the document forbids new versions.
	if _, _, err := env.docSvc.Finalize(ctx, docID, testOperator, "lock", true); err != nil {
		t.Fatalf("finalize and lock: %v", err)
	}
	if _, _, _, err := env.docSvc.AddVersion(ctx, docID, document.VersionInput{OperatorID: testOperator}); !errors.Is(err, core.ErrLocked) {
		t.Fatalf("adding a version to a locked document must fail with ErrLocked, got %v", err)
	}
}
