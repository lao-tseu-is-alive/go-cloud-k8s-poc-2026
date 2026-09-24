// Package blobstoretest is the conformance suite of the blobstore.Store
// contract. Every implementation runs it from its own tests:
//
//	func TestConformance(t *testing.T) {
//		blobstoretest.Run(t, func(t *testing.T) blobstore.Store { return newStore(t) })
//	}
package blobstoretest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
)

// Run checks that stores built by newStore honour the blobstore.Store
// contract. newStore must return a fresh, empty store for each call.
func Run(t *testing.T, newStore func(t *testing.T) blobstore.Store) {
	t.Helper()
	t.Run("put computes digest and size", func(t *testing.T) { testPutDigest(t, newStore(t)) })
	t.Run("get round-trips the bytes", func(t *testing.T) { testRoundTrip(t, newStore(t)) })
	t.Run("delete removes and is idempotent", func(t *testing.T) { testDelete(t, newStore(t)) })
	t.Run("foreign references are rejected", func(t *testing.T) { testForeignRefs(t, newStore(t)) })
	t.Run("cancelled context fails", func(t *testing.T) { testCancelled(t, newStore(t)) })
	t.Run("concurrent puts get distinct references", func(t *testing.T) { testConcurrentPuts(t, newStore(t)) })
}

func put(t *testing.T, store blobstore.Store, content []byte) blobstore.Stored {
	t.Helper()
	stored, err := store.Put(t.Context(), bytes.NewReader(content), blobstore.Metadata{Filename: "plan.pdf", ContentType: "application/pdf"})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	return stored
}

func testPutDigest(t *testing.T, store blobstore.Store) {
	content := bytes.Repeat([]byte("goéland "), 128*1024) // ~1 MiB, streamed
	stored := put(t, store, content)
	want := sha256.Sum256(content)
	if stored.SHA256 != hex.EncodeToString(want[:]) || stored.Size != int64(len(content)) {
		t.Fatalf("got sha256 %s size %d, want %s size %d", stored.SHA256, stored.Size, hex.EncodeToString(want[:]), len(content))
	}
	if stored.Ref == "" || stored.Filename != "plan.pdf" {
		t.Fatalf("unexpected stored %+v", stored)
	}
}

func testRoundTrip(t *testing.T, store blobstore.Store) {
	stored := put(t, store, []byte("round trip"))
	obj, err := store.Get(t.Context(), stored.Ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer obj.Close()
	got, err := io.ReadAll(obj)
	if err != nil || string(got) != "round trip" || obj.Info().Size != int64(len(got)) {
		t.Fatalf("read %q (info %+v, err %v)", got, obj.Info(), err)
	}
}

func testDelete(t *testing.T, store blobstore.Store) {
	stored := put(t, store, []byte("to delete"))
	if err := store.Delete(t.Context(), stored.Ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(t.Context(), stored.Ref); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("Get after Delete: want ErrNotFound, got %v", err)
	}
	if err := store.Delete(t.Context(), stored.Ref); err != nil {
		t.Fatalf("deleting missing bytes must not fail: %v", err)
	}
}

func testForeignRefs(t *testing.T, store blobstore.Store) {
	for _, ref := range []string{"", "file:///etc/passwd", "/etc/passwd", "unknown-scheme://x"} {
		if _, err := store.Get(t.Context(), ref); !errors.Is(err, blobstore.ErrInvalidRef) {
			t.Errorf("Get(%q): want ErrInvalidRef, got %v", ref, err)
		}
		if err := store.Delete(t.Context(), ref); !errors.Is(err, blobstore.ErrInvalidRef) {
			t.Errorf("Delete(%q): want ErrInvalidRef, got %v", ref, err)
		}
	}
}

func testCancelled(t *testing.T, store blobstore.Store) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.Put(ctx, strings.NewReader("never stored"), blobstore.Metadata{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Put with a cancelled context: want context.Canceled, got %v", err)
	}
}

func testConcurrentPuts(t *testing.T, store blobstore.Store) {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		refs = map[string]bool{}
	)
	for range 8 {
		wg.Go(func() {
			stored, err := store.Put(t.Context(), strings.NewReader("same bytes"), blobstore.Metadata{})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Errorf("concurrent Put: %v", err)
				return
			}
			refs[stored.Ref] = true
		})
	}
	wg.Wait()
	if len(refs) != 8 {
		t.Fatalf("each Put must get its own reference, got %d distinct", len(refs))
	}
}
