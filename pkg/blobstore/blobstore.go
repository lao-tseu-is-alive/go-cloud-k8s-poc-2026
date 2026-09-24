// Package blobstore defines the storage contract for binary content (spec v2
// §23): the domain registers content by digest and keeps only an opaque
// reference, while a Store holds the bytes. Implementations are substitutable
// (local filesystem today, S3-compatible or an institutional GED later)
// without touching the document model; package blobstoretest provides the
// conformance suite every implementation must pass.
package blobstore

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned by Get and Delete when a reference is well-formed but
// no bytes exist behind it.
var ErrNotFound = errors.New("blob not found")

// ErrInvalidRef is returned for a reference that this store does not own or
// that could escape its storage area (e.g. path traversal).
var ErrInvalidRef = errors.New("invalid blob reference")

// Store holds content bytes. Implementations must be safe for concurrent use.
type Store interface {
	// Put streams r to new, uniquely referenced storage and returns the
	// reference with the SHA-256 and size computed while writing. The bytes
	// are never read into memory at once. On error nothing is left behind.
	Put(ctx context.Context, r io.Reader, meta Metadata) (Stored, error)
	// Get opens the bytes behind ref. The caller must Close the object.
	Get(ctx context.Context, ref string) (Object, error)
	// Delete removes the bytes behind ref; deleting missing bytes is not an
	// error. Domain services only use it for bytes they never registered (a
	// duplicate of known content); governed disposition is a separate process.
	Delete(ctx context.Context, ref string) error
}

// Metadata describes content handed to Put.
type Metadata struct {
	// Filename is the original client filename; it may shape the reference
	// (e.g. keep an extension) but never determines where bytes are stored.
	Filename string
	// ContentType is the media type, when the backend can record it.
	ContentType string
}

// Stored describes content written by Put.
type Stored struct {
	// Ref is the opaque reference to persist (e.g. internal://<name>).
	Ref string
	// SHA256 is the lower-case, 64-character hex digest of the bytes.
	SHA256 string
	// Size is the number of bytes written.
	Size int64
	// Filename is the original client filename, informational only.
	Filename string
}

// Object is readable content returned by Get.
type Object interface {
	io.ReadCloser
	// Info describes the object.
	Info() ObjectInfo
}

// ObjectInfo describes an object returned by Get.
type ObjectInfo struct {
	// Name is a display name suitable for downloads (may be empty).
	Name string
	// Size is the number of bytes.
	Size int64
	// ModTime is when the bytes were written; zero when unknown.
	ModTime time.Time
}
