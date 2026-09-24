package filestore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/blobstoretest"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store
}

func TestConformance(t *testing.T) {
	blobstoretest.Run(t, func(t *testing.T) blobstore.Store { return newStore(t) })
}

func TestPutKeepsLowercasedExtension(t *testing.T) {
	stored, err := newStore(t).Put(t.Context(), strings.NewReader("x"), blobstore.Metadata{Filename: "plan.PDF"})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !strings.HasPrefix(stored.Ref, Scheme) || !strings.HasSuffix(stored.Ref, ".pdf") {
		t.Fatalf("ref %q should be %s<uuid>.pdf", stored.Ref, Scheme)
	}
}

func TestUnsafeRefsAreRejected(t *testing.T) {
	store := newStore(t)
	for _, ref := range []string{
		"internal://",
		"internal://../secret",
		"internal://sub/dir",
		"internal://" + filepath.Join("..", "escape"),
		"internal://.",
	} {
		if _, err := store.Get(t.Context(), ref); !errors.Is(err, blobstore.ErrInvalidRef) {
			t.Errorf("Get(%q): want ErrInvalidRef, got %v", ref, err)
		}
		if err := store.Delete(t.Context(), ref); !errors.Is(err, blobstore.ErrInvalidRef) {
			t.Errorf("Delete(%q): want ErrInvalidRef, got %v", ref, err)
		}
	}
}

func TestFailedPutLeavesNoFile(t *testing.T) {
	store := newStore(t)
	if _, err := store.Put(t.Context(), failingReader{}, blobstore.Metadata{}); err == nil {
		t.Fatal("Put must fail when the reader fails")
	}
	entries, err := os.ReadDir(store.Root())
	if err != nil || len(entries) != 0 {
		t.Fatalf("a failed Put must not leave a partial file, found %d (%v)", len(entries), err)
	}
}

// failingReader fails on the first read, simulating a broken upload.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }
