package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/filestore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/s3store"
)

// Blob store backends (GOELAND_BLOB_STORE).
const (
	blobStoreFile = "file"
	blobStoreS3   = "s3"
)

// readBlobStoreSettings reads GOELAND_BLOB_STORE and, for the s3 backend, the
// GOELAND_S3_* settings (GLD-020). The access keys are secrets: they are never
// logged. Plain http carries them only to a loopback endpoint, or anywhere when
// GOELAND_S3_ALLOW_PLAIN_HTTP=true (e.g. a service-mesh encrypted cluster).
func (c *serverConfig) readBlobStoreSettings() error {
	c.BlobStore = strings.ToLower(envOrDefault("GOELAND_BLOB_STORE", blobStoreFile))
	switch c.BlobStore {
	case blobStoreFile:
		return nil
	case blobStoreS3:
	default:
		return fmt.Errorf("GOELAND_BLOB_STORE must be %s or %s", blobStoreFile, blobStoreS3)
	}
	c.S3 = s3store.Config{
		Endpoint:        strings.TrimSpace(os.Getenv("GOELAND_S3_ENDPOINT")),
		Region:          strings.TrimSpace(os.Getenv("GOELAND_S3_REGION")),
		Bucket:          strings.TrimSpace(os.Getenv("GOELAND_S3_BUCKET")),
		Prefix:          strings.TrimSpace(os.Getenv("GOELAND_S3_PREFIX")),
		AccessKeyID:     os.Getenv("GOELAND_S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("GOELAND_S3_SECRET_ACCESS_KEY"),
	}
	for name, value := range map[string]string{
		"GOELAND_S3_ENDPOINT": c.S3.Endpoint, "GOELAND_S3_REGION": c.S3.Region, "GOELAND_S3_BUCKET": c.S3.Bucket,
		"GOELAND_S3_ACCESS_KEY_ID": c.S3.AccessKeyID, "GOELAND_S3_SECRET_ACCESS_KEY": c.S3.SecretAccessKey,
	} {
		if value == "" {
			return fmt.Errorf("%s is required when GOELAND_BLOB_STORE=s3", name)
		}
	}
	endpoint, err := url.Parse(c.S3.Endpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return fmt.Errorf("GOELAND_S3_ENDPOINT must be an http(s) URL")
	}
	if endpoint.Scheme == "https" || isLoopbackHost(endpoint.Hostname()) {
		return nil
	}
	allowPlain, err := envBool("GOELAND_S3_ALLOW_PLAIN_HTTP", false)
	if err != nil {
		return err
	}
	if !allowPlain {
		return fmt.Errorf("GOELAND_S3_ENDPOINT must use https outside loopback (the requests are signed with the access key); set GOELAND_S3_ALLOW_PLAIN_HTTP=true to accept http")
	}
	return nil
}

// buildBlobStore opens the configured blob store. The s3 backend writes to the
// bucket and still reads and deletes, through a chain, the bytes stored before
// in GOELAND_DOCUMENT_PATH (internal:// references), so a move to S3 needs no
// immediate copy.
func buildBlobStore(ctx context.Context, config serverConfig, log *slog.Logger) (blobstore.Store, error) {
	files, err := filestore.New(config.DocumentPath)
	if err != nil {
		return nil, fmt.Errorf("document blob store: %w", err)
	}
	if config.BlobStore != blobStoreS3 {
		log.Info("document blob store ready", "backend", blobStoreFile, "path", files.Root(), "max_upload_bytes", config.MaxUploadBytes)
		return files, nil
	}
	bucket, err := s3store.New(ctx, config.S3)
	if err != nil {
		return nil, fmt.Errorf("document blob store: %w", err)
	}
	log.Info("document blob store ready", "backend", blobStoreS3, "endpoint", config.S3.Endpoint, "bucket", bucket.Bucket(),
		"prefix", bucket.Prefix(), "previous_files_path", files.Root(), "max_upload_bytes", config.MaxUploadBytes)
	return blobstore.NewChain(bucket, files), nil
}
