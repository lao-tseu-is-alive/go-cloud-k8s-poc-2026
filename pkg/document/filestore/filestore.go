// Package filestore persists uploaded document blobs on the local filesystem.
//
// Documents in the Goéland POC are metadata-first: the document record only
// carries a storage_ref URI, never the bytes themselves. This store owns the
// bytes for the "internal" backend, addressing each blob with an
// internal://<name> URI that a document's storage_ref can point to.
//
// The store is deliberately dumb: it does not know about documents, auth, or
// the database. It only writes bytes, computes their SHA-256 while streaming,
// and reads them back — guarding against path traversal on the way out.
package filestore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Scheme is the URI prefix used for blobs owned by this local store.
const Scheme = "internal://"

// ErrInvalidRef is returned when a storage_ref does not resolve to a blob
// safely contained within the store root.
var ErrInvalidRef = errors.New("invalid internal storage reference")

// Store writes and reads document blobs under a single root directory.
type Store struct {
	root string
}

// Blob describes a persisted upload. The values map directly onto the
// document metadata fields the client later sends to CreateDocument.
type Blob struct {
	StorageRef    string // internal://<name>
	SHA256        string // lowercase hex, 64 chars
	FileSizeBytes int64
	Filename      string // original client filename (informational)
}

// New resolves root to an absolute path and creates it if missing.
func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("filestore: root path is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("filestore: resolve root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("filestore: create root %q: %w", abs, err)
	}
	return &Store{root: abs}, nil
}

// Root returns the absolute directory blobs are stored under.
func (s *Store) Root() string { return s.root }

// Save streams r to a new uniquely named file, computing its SHA-256 and size
// as it writes. The original filename is only used to preserve a file
// extension and is echoed back for display; it never determines the stored
// path. A partial file is removed if the copy fails.
func (s *Store) Save(r io.Reader, originalName string) (Blob, error) {
	name := uuid.NewString() + safeExt(originalName)
	dst := filepath.Join(s.root, name)

	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return Blob{}, fmt.Errorf("filestore: create blob: %w", err)
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(f, hasher), r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return Blob{}, fmt.Errorf("filestore: write blob: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return Blob{}, fmt.Errorf("filestore: close blob: %w", closeErr)
	}
	return Blob{
		StorageRef:    Scheme + name,
		SHA256:        hex.EncodeToString(hasher.Sum(nil)),
		FileSizeBytes: size,
		Filename:      filepath.Base(originalName),
	}, nil
}

// Open resolves an internal:// storage_ref to a readable file, rejecting any
// reference that escapes the store root or is not owned by this backend.
func (s *Store) Open(storageRef string) (*os.File, error) {
	name, ok := strings.CutPrefix(storageRef, Scheme)
	if !ok {
		return nil, ErrInvalidRef
	}
	// The name must be a single, plain path element: no directories, no
	// traversal, no absolute paths.
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || filepath.IsAbs(name) {
		return nil, ErrInvalidRef
	}
	full := filepath.Join(s.root, name)
	// Defence in depth: the resolved path must still live directly under root.
	if filepath.Dir(full) != s.root {
		return nil, ErrInvalidRef
	}
	f, err := os.Open(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrInvalidRef
		}
		return nil, fmt.Errorf("filestore: open blob: %w", err)
	}
	return f, nil
}

// safeExt returns the (lowercased) extension of name if it is short and free
// of path separators, otherwise the empty string.
func safeExt(name string) string {
	ext := filepath.Ext(filepath.Base(name))
	if len(ext) > 16 || strings.ContainsAny(ext, `/\`) {
		return ""
	}
	return strings.ToLower(ext)
}
