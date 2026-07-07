package filestore

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveComputesHashAndSize(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	payload := []byte("hello goéland")
	blob, err := store.Save(strings.NewReader(string(payload)), "plan.PDF")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if blob.FileSizeBytes != int64(len(payload)) {
		t.Errorf("size = %d, want %d", blob.FileSizeBytes, len(payload))
	}
	want := sha256.Sum256(payload)
	if blob.SHA256 != hex.EncodeToString(want[:]) {
		t.Errorf("sha256 = %s, want %s", blob.SHA256, hex.EncodeToString(want[:]))
	}
	if !strings.HasPrefix(blob.StorageRef, Scheme) {
		t.Errorf("storage_ref %q missing scheme %q", blob.StorageRef, Scheme)
	}
	if !strings.HasSuffix(blob.StorageRef, ".pdf") {
		t.Errorf("storage_ref %q should preserve lowercased extension", blob.StorageRef)
	}
}

func TestSaveRoundTripsThroughOpen(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	blob, err := store.Save(strings.NewReader("round trip"), "note.txt")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	f, err := store.Open(blob.StorageRef)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "round trip" {
		t.Errorf("content = %q, want %q", got, "round trip")
	}
}

func TestOpenRejectsUnsafeRefs(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, ref := range []string{
		"",
		"internal://",
		"internal://../secret",
		"internal://sub/dir",
		"internal://" + filepath.Join("..", "escape"),
		"file:///etc/passwd",
		"/etc/passwd",
		"internal://.",
	} {
		if _, err := store.Open(ref); err == nil {
			t.Errorf("Open(%q) succeeded, want error", ref)
		}
	}
}
