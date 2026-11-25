package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

// TestOrphanUploadCollection covers GLD-045: the bytes of an upload no version
// references are destroyed once the delay has passed since its last upload;
// the blob row stays, attaching it is refused, and an identical upload stores
// the bytes again. Fresh, attached, imported (no bytes held) and being-attached
// blobs are left alone. One test, steps in order: each Collect sees every
// candidate of the shared database.
func TestOrphanUploadCollection(t *testing.T) {
	env := newTestEnv(t)
	collector, err := document.NewOrphanCollector(env.pool, env.store, document.OrphanConfig{TTL: 24 * time.Hour}, nil)
	if err != nil {
		t.Fatal(err)
	}

	orphanText := "orphelin " + uniqueToken()
	orphan := ingest(t, env, orphanText).Blob
	fresh := ingest(t, env, "récent "+uniqueToken()).Blob
	attached := ingest(t, env, "rattaché "+uniqueToken()).Blob
	createDocumentOn(t, env, attached.ID)
	imported := importedBlob(t, env)
	backdate(t, env, orphan.ID, attached.ID)

	collect(t, collector)
	assertBlobHeld(t, env, fresh.ID, "a fresh upload")
	assertBlobHeld(t, env, attached.ID, "an attached upload")
	assertBlobDisposed(t, env, orphan)
	var importedDisposed bool
	if err := env.pool.QueryRow(env.ctx, `SELECT disposed_at IS NOT NULL FROM content_blob WHERE id = $1`, imported).Scan(&importedDisposed); err != nil || importedDisposed {
		t.Fatalf("a blob whose bytes Goéland does not hold is never collected: %v (%v)", importedDisposed, err)
	}

	_, err = env.docSvc.Create(env.ctx, document.CreateInput{DocumentTypeCode: "PLAN", Title: "Trop tard " + uniqueToken(), ContentBlobID: &orphan.ID, OperatorID: testOperator})
	if !errors.Is(err, core.ErrInvalidState) {
		t.Fatalf("attaching destroyed bytes is refused: %v", err)
	}
	assertSameUploadRestoresBytes(t, env, collector, orphan, orphanText)
}

// assertSameUploadRestoresBytes checks that an identical upload stores the
// destroyed bytes again, and that re-uploading an old orphan restarts its delay.
func assertSameUploadRestoresBytes(t *testing.T, env *testEnv, collector *document.OrphanCollector, orphan *document.ContentBlob, content string) {
	t.Helper()
	again := ingest(t, env, content)
	if again.Blob.ID != orphan.ID || again.Reused || again.Blob.StorageRef == "" || again.Blob.DisposedAt != nil {
		t.Fatalf("an identical upload stores the bytes again in the same blob: %+v", again)
	}
	backdate(t, env, orphan.ID)
	refreshed := ingest(t, env, content)
	if refreshed.Blob.ID != orphan.ID || refreshed.Blob.StorageRef == "" {
		t.Fatalf("a duplicate upload returns the held blob: %+v", refreshed)
	}
	collect(t, collector)
	assertBlobHeld(t, env, orphan.ID, "an upload repeated after the delay")
	createDocumentOn(t, env, orphan.ID)
}

// TestOrphanCollectionSkipsBlobBeingAttached covers the lock between the
// collection and CreateDocument: a blob held FOR SHARE by an attaching
// transaction is skipped, and attached once it commits.
func TestOrphanCollectionSkipsBlobBeingAttached(t *testing.T) {
	env := newTestEnv(t)
	collector, err := document.NewOrphanCollector(env.pool, env.store, document.OrphanConfig{TTL: 24 * time.Hour}, nil)
	if err != nil {
		t.Fatal(err)
	}
	blob := ingest(t, env, "en cours "+uniqueToken()).Blob
	backdate(t, env, blob.ID)
	tx, err := env.pool.Begin(env.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(env.ctx)) }()
	if _, err := tx.Exec(env.ctx, `SELECT 1 FROM content_blob WHERE id = $1 FOR SHARE`, blob.ID); err != nil {
		t.Fatal(err)
	}
	collect(t, collector)
	if err := tx.Rollback(env.ctx); err != nil {
		t.Fatal(err)
	}
	assertBlobHeld(t, env, blob.ID, "a blob being attached")
}

// importedBlob registers an old blob known only by its digest, like the legacy
// import does, and returns its id.
func importedBlob(t *testing.T, env *testEnv) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	digest := strings.Repeat("0", 32) + strings.ReplaceAll(uuid.NewString(), "-", "")
	err := env.pool.QueryRow(env.ctx, `
INSERT INTO content_blob (sha256, storage_ref, file_size_bytes, created_at) VALUES ($1, '', 1, now() - interval '30 days') RETURNING id`, digest).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// backdate moves the upload time of blobs two days back.
func backdate(t *testing.T, env *testEnv, ids ...uuid.UUID) {
	t.Helper()
	if _, err := env.pool.Exec(env.ctx, `UPDATE content_blob SET uploaded_at = now() - interval '2 days' WHERE id = ANY($1)`, ids); err != nil {
		t.Fatal(err)
	}
}

// collect runs one collection.
func collect(t *testing.T, collector *document.OrphanCollector) {
	t.Helper()
	if _, err := collector.Collect(context.Background()); err != nil {
		t.Fatalf("collect: %v", err)
	}
}

// assertBlobHeld requires the blob's bytes to be stored still.
func assertBlobHeld(t *testing.T, env *testEnv, id uuid.UUID, what string) {
	t.Helper()
	var ref string
	var disposed bool
	if err := env.pool.QueryRow(env.ctx, `SELECT storage_ref, disposed_at IS NOT NULL FROM content_blob WHERE id = $1`, id).Scan(&ref, &disposed); err != nil || ref == "" || disposed {
		t.Fatalf("%s is kept: ref=%q disposed=%v (%v)", what, ref, disposed, err)
	}
	obj, err := env.store.Get(env.ctx, ref)
	if err != nil {
		t.Fatalf("%s keeps its bytes: %v", what, err)
	}
	_ = obj.Close()
}

// assertBlobDisposed requires the blob row to be marked destroyed and its bytes gone.
func assertBlobDisposed(t *testing.T, env *testEnv, blob *document.ContentBlob) {
	t.Helper()
	var ref string
	var disposed bool
	if err := env.pool.QueryRow(env.ctx, `SELECT storage_ref, disposed_at IS NOT NULL FROM content_blob WHERE id = $1`, blob.ID).Scan(&ref, &disposed); err != nil || ref != "" || !disposed {
		t.Fatalf("an old orphan upload is marked destroyed: ref=%q disposed=%v (%v)", ref, disposed, err)
	}
	if _, err := env.store.Get(env.ctx, blob.StorageRef); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("its bytes are deleted: %v", err)
	}
}
