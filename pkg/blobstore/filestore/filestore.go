// Package filestore is the local-filesystem implementation of blobstore.Store.
//
// Content bytes live as files directly under one root directory, each
// addressed by an internal://<name> reference (a random UUID plus the
// original, lower-cased extension). The store is deliberately dumb: it does
// not know about documents, auth or the database. It only writes bytes,
// computes their SHA-256 while streaming, and reads or deletes them back,
// guarding every reference against path traversal.
package filestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
)

// Scheme is the URI prefix of references owned by this local store.
const Scheme = "internal://"

// Store writes and reads content bytes under a single root directory. It is
// safe for concurrent use: every Put writes a new, uniquely named file.
type Store struct {
	root string
}

// Store implements the blobstore contract.
var _ blobstore.Store = (*Store)(nil)

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

// Root returns the absolute directory the bytes are stored under.
func (s *Store) Root() string { return s.root }

// Put streams r to a new uniquely named file, computing its SHA-256 and size
// as it writes. meta.Filename only contributes a file extension and is echoed
// back for display; it never determines the stored path. A partial file is
// removed if the copy fails or ctx is cancelled.
func (s *Store) Put(ctx context.Context, r io.Reader, meta blobstore.Metadata) (blobstore.Stored, error) {
	if err := ctx.Err(); err != nil {
		return blobstore.Stored{}, err
	}
	name := uuid.NewString() + safeExt(meta.Filename)
	dst := filepath.Join(s.root, name)

	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return blobstore.Stored{}, fmt.Errorf("filestore: create blob: %w", err)
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(f, hasher), contextReader{ctx: ctx, r: r})
	closeErr := f.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = os.Remove(dst)
		return blobstore.Stored{}, fmt.Errorf("filestore: write blob: %w", err)
	}
	return blobstore.Stored{
		Ref:      Scheme + name,
		SHA256:   hex.EncodeToString(hasher.Sum(nil)),
		Size:     size,
		Filename: filepath.Base(meta.Filename),
	}, nil
}

// Get opens the file behind an internal:// reference. It fails with
// blobstore.ErrInvalidRef for a reference this store does not own or that
// escapes the root, and blobstore.ErrNotFound when no file exists.
func (s *Store) Get(ctx context.Context, ref string) (blobstore.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	full, err := s.resolve(ref)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil, blobstore.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("filestore: open blob: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("filestore: stat blob: %w", err)
	}
	return &object{File: f, info: blobstore.ObjectInfo{Name: info.Name(), Size: info.Size(), ModTime: info.ModTime()}}, nil
}

// Delete removes the file behind an internal:// reference, with the same
// reference validation as Get. A missing file is not an error.
func (s *Store) Delete(ctx context.Context, ref string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := s.resolve(ref)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("filestore: remove blob: %w", err)
	}
	return nil
}

// object is an open file; it is also an io.Seeker, which lets HTTP handlers
// serve range requests.
type object struct {
	*os.File
	info blobstore.ObjectInfo
}

// Info describes the open file.
func (o *object) Info() blobstore.ObjectInfo { return o.info }

// contextReader stops a copy as soon as ctx is cancelled.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

// Read reads from the wrapped reader unless the context is done.
func (c contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// resolve maps an internal:// reference to its absolute path, returning
// blobstore.ErrInvalidRef for any reference not owned by this backend or
// escaping root.
func (s *Store) resolve(ref string) (string, error) {
	name, ok := strings.CutPrefix(ref, Scheme)
	if !ok {
		return "", blobstore.ErrInvalidRef
	}
	// The name must be a single, plain path element: no directories, no
	// traversal, no absolute paths.
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || filepath.IsAbs(name) {
		return "", blobstore.ErrInvalidRef
	}
	full := filepath.Join(s.root, name)
	// Defence in depth: the resolved path must still live directly under root.
	if filepath.Dir(full) != s.root {
		return "", blobstore.ErrInvalidRef
	}
	return full, nil
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
