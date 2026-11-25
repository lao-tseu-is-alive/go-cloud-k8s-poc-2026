// Package s3store is the S3-compatible implementation of blobstore.Store
// (GLD-020), written against Garage and limited to the operations every
// S3-compatible store supports: PutObject (single or multipart), GetObject with
// ranges, HeadObject, DeleteObject, HeadBucket and ListObjects (tests only).
//
// Content bytes live in one bucket, under an optional key prefix, each object
// named by a random UUID plus the original, lower-cased extension and
// addressed by an s3://<bucket>/<key> reference. Like the filestore, the store
// is deliberately dumb: it does not know about documents, auth or the
// database. It streams the bytes, computes their SHA-256 while sending them,
// and reads or deletes them back, rejecting every reference outside its
// bucket and prefix.
package s3store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
)

// Scheme is the URI prefix of references owned by an S3 store.
const Scheme = "s3://"

// partSize is the size of a multipart upload part, and so the memory one
// upload holds at most: content up to this size is sent in a single
// PutObject, larger content in parts (10 000 parts of 16 MiB: 160 GiB at most).
const partSize = 16 << 20

// Config locates the bucket and carries the credentials of the store.
type Config struct {
	// Endpoint is the URL of the S3 API, e.g. https://s3.example.ch or
	// http://127.0.0.1:3900 (Garage). The scheme decides TLS; the caller
	// decides whether plain http is acceptable.
	Endpoint string
	// Region is the region the endpoint signs for (Garage: its s3_region).
	Region string
	// Bucket is the existing bucket holding the bytes.
	Bucket string
	// Prefix is an optional key prefix (e.g. "goeland/documents"), letting
	// several deployments share a bucket.
	Prefix string
	// AccessKeyID is the access key; a secret, never logged.
	AccessKeyID string
	// SecretAccessKey is the secret key; a secret, never logged.
	SecretAccessKey string
}

// Store writes and reads content bytes in one bucket. It is safe for
// concurrent use: every Put writes a new, uniquely named object.
type Store struct {
	client *minio.Client
	bucket string
	prefix string // "" or ends with "/"
}

// Store implements the blobstore contract.
var _ blobstore.Store = (*Store)(nil)

// New connects to the endpoint and checks that the bucket exists, so a wrong
// endpoint, credential or bucket stops the server at startup.
func New(ctx context.Context, cfg Config) (*Store, error) {
	store, err := newStore(cfg)
	if err != nil {
		return nil, err
	}
	exists, err := store.client.BucketExists(ctx, store.bucket)
	if err != nil {
		return nil, fmt.Errorf("s3store: check bucket %q: %w", store.bucket, err)
	}
	if !exists {
		return nil, fmt.Errorf("s3store: bucket %q does not exist", store.bucket)
	}
	return store, nil
}

// newStore validates cfg and builds the client without contacting the endpoint.
func newStore(cfg Config) (*Store, error) {
	endpoint, err := url.Parse(strings.TrimSpace(cfg.Endpoint))
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" ||
		strings.Trim(endpoint.Path, "/") != "" || endpoint.RawQuery != "" || endpoint.User != nil {
		return nil, errors.New("s3store: the endpoint must be an http(s) URL without path, query or credentials")
	}
	for name, value := range map[string]string{
		"region": cfg.Region, "bucket": cfg.Bucket, "access key id": cfg.AccessKeyID, "secret access key": cfg.SecretAccessKey,
	} {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("s3store: the %s is required", name)
		}
	}
	prefix, err := normalizePrefix(cfg.Prefix)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(endpoint.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure:       endpoint.Scheme == "https",
		Region:       strings.TrimSpace(cfg.Region),
		BucketLookup: minio.BucketLookupPath, // Garage serves virtual hosts only when configured
	})
	if err != nil {
		return nil, fmt.Errorf("s3store: build client: %w", err)
	}
	return &Store{client: client, bucket: strings.TrimSpace(cfg.Bucket), prefix: prefix}, nil
}

// normalizePrefix trims the slashes around prefix and ends it with one; its
// segments must be plain names.
func normalizePrefix(prefix string) (string, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return "", nil
	}
	for segment := range strings.SplitSeq(prefix, "/") {
		if !plainName(segment) {
			return "", fmt.Errorf("s3store: invalid key prefix %q", prefix)
		}
	}
	return prefix + "/", nil
}

// Bucket returns the bucket the bytes are stored in.
func (s *Store) Bucket() string { return s.bucket }

// Prefix returns the key prefix ("" or ending with "/").
func (s *Store) Prefix() string { return s.prefix }

// Put streams r to a new uniquely named object, computing its SHA-256 and size
// as it sends. Content up to partSize is buffered and sent in one PutObject;
// larger content goes in parts, and a failed multipart upload is aborted.
// meta.Filename only contributes an extension and is echoed back.
func (s *Store) Put(ctx context.Context, r io.Reader, meta blobstore.Metadata) (blobstore.Stored, error) {
	if err := ctx.Err(); err != nil {
		return blobstore.Stored{}, err
	}
	name := uuid.NewString() + blobstore.SafeExt(meta.Filename)
	hasher := sha256.New()
	counter := &countingWriter{}
	src := io.TeeReader(blobstore.ContextReader(ctx, r), io.MultiWriter(hasher, counter))

	var head bytes.Buffer
	n, err := io.CopyN(&head, src, partSize)
	if err != nil && !errors.Is(err, io.EOF) {
		return blobstore.Stored{}, fmt.Errorf("s3store: read content: %w", err)
	}
	opts := minio.PutObjectOptions{ContentType: meta.ContentType, PartSize: partSize}
	body, size := io.Reader(&head), n
	if n == partSize { // maybe more: stream the rest in parts
		body, size = io.MultiReader(&head, src), -1
	}
	if _, err := s.client.PutObject(ctx, s.bucket, s.prefix+name, body, size, opts); err != nil {
		return blobstore.Stored{}, fmt.Errorf("s3store: write blob: %w", err)
	}
	return blobstore.Stored{
		Ref:      Scheme + s.bucket + "/" + s.prefix + name,
		SHA256:   hex.EncodeToString(hasher.Sum(nil)),
		Size:     counter.n,
		Filename: filepath.Base(meta.Filename),
	}, nil
}

// Get opens the object behind an s3:// reference. It fails with
// blobstore.ErrInvalidRef for a reference outside this store's bucket and
// prefix, and blobstore.ErrNotFound when no object exists. The object is an
// io.Seeker, so HTTP handlers serve range requests from it.
func (s *Store) Get(ctx context.Context, ref string) (blobstore.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, err := s.resolve(ref)
	if err != nil {
		return nil, err
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3store: open blob: %w", err)
	}
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		if isNotFound(err) {
			return nil, blobstore.ErrNotFound
		}
		return nil, fmt.Errorf("s3store: stat blob: %w", err)
	}
	return &object{Object: obj, info: blobstore.ObjectInfo{Name: path.Base(key), Size: info.Size, ModTime: info.LastModified}}, nil
}

// Delete removes the object behind an s3:// reference, with the same
// reference validation as Get. A missing object is not an error.
func (s *Store) Delete(ctx context.Context, ref string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := s.resolve(ref)
	if err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil && !isNotFound(err) {
		return fmt.Errorf("s3store: remove blob: %w", err)
	}
	return nil
}

// resolve maps an s3:// reference to its object key, returning
// blobstore.ErrInvalidRef for any reference outside the bucket and prefix or
// naming anything but one plain object under the prefix.
func (s *Store) resolve(ref string) (string, error) {
	name, ok := strings.CutPrefix(ref, Scheme+s.bucket+"/"+s.prefix)
	if !ok || !plainName(name) {
		return "", blobstore.ErrInvalidRef
	}
	return s.prefix + name, nil
}

// plainName reports whether name is one path element that cannot traverse.
func plainName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

// isNotFound reports an S3 "no such key" answer.
func isNotFound(err error) bool {
	code := minio.ToErrorResponse(err).Code
	return code == minio.NoSuchKey || code == "NotFound"
}

// object is an open S3 object: reads, seeks and range reads go to the store.
type object struct {
	*minio.Object
	info blobstore.ObjectInfo
}

// Info describes the object.
func (o *object) Info() blobstore.ObjectInfo { return o.info }

// countingWriter counts the bytes written to it.
type countingWriter struct{ n int64 }

// Write counts p.
func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
