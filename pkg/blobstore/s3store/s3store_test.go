package s3store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/blobstoretest"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/s3store/s3storetest"
)

// testConfig returns the test store settings, or skips the test.
func testConfig(t *testing.T) Config {
	t.Helper()
	env := s3storetest.Require(t, "S3 store tests")
	return Config{Endpoint: env.Endpoint, Region: env.Region, Bucket: env.Bucket, AccessKeyID: env.AccessKeyID, SecretAccessKey: env.SecretAccessKey}
}

// newTestStore returns a store on a fresh prefix of the test bucket, emptied
// when the test ends.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	cfg := testConfig(t)
	cfg.Prefix = "goeland-test/" + uuid.NewString()
	store, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context()) // t.Context is cancelled before the cleanups
		for _, key := range listKeys(ctx, t, store) {
			_ = store.client.RemoveObject(ctx, store.bucket, key, minio.RemoveObjectOptions{})
		}
	})
	return store
}

// listKeys lists the keys under the store's prefix.
func listKeys(ctx context.Context, t *testing.T, store *Store) []string {
	t.Helper()
	var keys []string
	for obj := range store.client.ListObjects(ctx, store.bucket, minio.ListObjectsOptions{Prefix: store.prefix, Recursive: true}) {
		if obj.Err != nil {
			t.Fatalf("list: %v", obj.Err)
		}
		keys = append(keys, obj.Key)
	}
	return keys
}

func TestConformance(t *testing.T) {
	blobstoretest.Run(t, func(t *testing.T) blobstore.Store { return newTestStore(t) })
}

// TestMultipartRoundTrip sends more than one part, then reads the object back
// whole and by range, as a download with a Range header does.
func TestMultipartRoundTrip(t *testing.T) {
	store := newTestStore(t)
	content := bytes.Repeat([]byte("0123456789abcdef"), (partSize+3<<20)/16) // 19 MiB: two parts
	stored, err := store.Put(t.Context(), bytes.NewReader(content), blobstore.Metadata{Filename: "scan.TIFF"})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	want := sha256.Sum256(content)
	if stored.SHA256 != hex.EncodeToString(want[:]) || stored.Size != int64(len(content)) || !strings.HasSuffix(stored.Ref, ".tiff") {
		t.Fatalf("stored %+v", stored)
	}
	obj, err := store.Get(t.Context(), stored.Ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer obj.Close()
	h := sha256.New()
	if n, err := io.Copy(h, obj); err != nil || n != int64(len(content)) || !bytes.Equal(h.Sum(nil), want[:]) {
		t.Fatalf("read back %d bytes (%v)", n, err)
	}
	seeker, ok := obj.(io.ReadSeeker)
	if !ok {
		t.Fatal("an S3 object must be seekable for range downloads")
	}
	if _, err := seeker.Seek(partSize-4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	tail := make([]byte, 8)
	if _, err := io.ReadFull(seeker, tail); err != nil || !bytes.Equal(tail, content[partSize-4:partSize+4]) {
		t.Fatalf("range read %q (%v)", tail, err)
	}
}

// TestFailedPutLeavesNoObject checks that a reader failing before or after the
// first part leaves nothing in the bucket.
func TestFailedPutLeavesNoObject(t *testing.T) {
	store := newTestStore(t)
	for _, after := range []int64{10, partSize + 1<<20} {
		src := io.MultiReader(io.LimitReader(zeros{}, after), failingReader{})
		if _, err := store.Put(t.Context(), src, blobstore.Metadata{}); err == nil {
			t.Fatalf("Put must fail when the reader fails after %d bytes", after)
		}
	}
	if keys := listKeys(t.Context(), t, store); len(keys) != 0 {
		t.Fatalf("a failed Put left %v", keys)
	}
}

func TestRefsOutsideBucketAndPrefixAreRejected(t *testing.T) {
	store, err := newStore(Config{Endpoint: "http://127.0.0.1:3900", Region: "garage", Bucket: "docs", Prefix: "/goeland/", AccessKeyID: "k", SecretAccessKey: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if key, err := store.resolve("s3://docs/goeland/a.pdf"); err != nil || key != "goeland/a.pdf" {
		t.Fatalf("own reference: %q %v", key, err)
	}
	for _, ref := range []string{
		"", "internal://a.pdf", "s3://other/goeland/a.pdf", "s3://docs/a.pdf", "s3://docs/goeland/",
		"s3://docs/goeland/sub/a.pdf", "s3://docs/goeland/..", "s3://docs/goeland/.", `s3://docs/goeland/a\b`,
		"s3://docs/goelandx/a.pdf",
	} {
		if _, err := store.Get(t.Context(), ref); !errors.Is(err, blobstore.ErrInvalidRef) {
			t.Errorf("Get(%q): want ErrInvalidRef, got %v", ref, err)
		}
		if err := store.Delete(t.Context(), ref); !errors.Is(err, blobstore.ErrInvalidRef) {
			t.Errorf("Delete(%q): want ErrInvalidRef, got %v", ref, err)
		}
	}
}

func TestConfigIsValidated(t *testing.T) {
	valid := Config{Endpoint: "https://s3.example.ch", Region: "garage", Bucket: "docs", AccessKeyID: "k", SecretAccessKey: "s"}
	if _, err := newStore(valid); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	for name, mutate := range map[string]func(*Config){
		"no endpoint":          func(c *Config) { c.Endpoint = "" },
		"no scheme":            func(c *Config) { c.Endpoint = "s3.example.ch:3900" },
		"endpoint with path":   func(c *Config) { c.Endpoint = "https://s3.example.ch/docs" },
		"endpoint credentials": func(c *Config) { c.Endpoint = "https://k:s@s3.example.ch" },
		"no region":            func(c *Config) { c.Region = " " },
		"no bucket":            func(c *Config) { c.Bucket = "" },
		"no access key":        func(c *Config) { c.AccessKeyID = "" },
		"no secret":            func(c *Config) { c.SecretAccessKey = "" },
		"traversing prefix":    func(c *Config) { c.Prefix = "goeland/../other" },
	} {
		cfg := valid
		mutate(&cfg)
		if _, err := newStore(cfg); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// zeros reads zero bytes forever.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// failingReader always fails.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failure") }
