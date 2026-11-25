package blobstore_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/blobstoretest"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/filestore"
)

func newFileStore(t *testing.T) *filestore.Store {
	t.Helper()
	store, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestChainConformance(t *testing.T) {
	blobstoretest.Run(t, func(t *testing.T) blobstore.Store {
		return blobstore.NewChain(newFileStore(t), ownsNothing{})
	})
}

// TestChainRoutesByOwner checks that new content goes to the primary store
// while a reference of the previous store is still read and deleted there.
func TestChainRoutesByOwner(t *testing.T) {
	previous := newFileStore(t)
	old, err := previous.Put(t.Context(), strings.NewReader("stored before the move"), blobstore.Metadata{})
	if err != nil {
		t.Fatal(err)
	}
	primary := &recordingStore{}
	chain := blobstore.NewChain(primary, previous)

	if _, err := chain.Put(t.Context(), strings.NewReader("new"), blobstore.Metadata{}); err != nil || primary.puts != 1 {
		t.Fatalf("Put goes to the primary store: puts=%d err=%v", primary.puts, err)
	}
	obj, err := chain.Get(t.Context(), old.Ref)
	if err != nil {
		t.Fatalf("Get of a previous-store reference: %v", err)
	}
	got, _ := io.ReadAll(obj)
	_ = obj.Close()
	if string(got) != "stored before the move" {
		t.Fatalf("read %q", got)
	}
	if err := chain.Delete(t.Context(), old.Ref); err != nil {
		t.Fatal(err)
	}
	if _, err := previous.Get(t.Context(), old.Ref); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("Delete reaches the previous store: %v", err)
	}
	if _, err := chain.Get(t.Context(), "nobody://x"); !errors.Is(err, blobstore.ErrInvalidRef) {
		t.Fatalf("a reference no store owns: %v", err)
	}
}

func TestSafeExt(t *testing.T) {
	for name, want := range map[string]string{
		"plan.PDF": ".pdf", "a/b\\scan.tiff": ".tiff", "archive.tar.gz": ".gz", "README": "", "trailing.": "",
		"évaluation.dOcX": ".docx", "x.p df": "", "x.é": "", "x." + strings.Repeat("a", 16): "", "..": "", "": "",
	} {
		if got := blobstore.SafeExt(name); got != want {
			t.Errorf("SafeExt(%q) = %q, want %q", name, got, want)
		}
	}
}

// ownsNothing is a store that owns no reference.
type ownsNothing struct{}

func (ownsNothing) Put(context.Context, io.Reader, blobstore.Metadata) (blobstore.Stored, error) {
	return blobstore.Stored{}, errors.New("not the primary store")
}
func (ownsNothing) Get(context.Context, string) (blobstore.Object, error) {
	return nil, blobstore.ErrInvalidRef
}
func (ownsNothing) Delete(context.Context, string) error { return blobstore.ErrInvalidRef }

// recordingStore counts its writes and owns no reference.
type recordingStore struct {
	ownsNothing
	puts int
}

func (r *recordingStore) Put(_ context.Context, src io.Reader, _ blobstore.Metadata) (blobstore.Stored, error) {
	r.puts++
	_, err := io.Copy(io.Discard, src)
	return blobstore.Stored{Ref: "recorded://x"}, err
}
