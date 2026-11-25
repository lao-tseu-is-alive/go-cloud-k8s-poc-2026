package document

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Default settings of the orphan upload collection (GLD-045).
const (
	// DefaultOrphanUploadTTL is how long uploaded bytes wait to be attached to
	// a document version before they are destroyed.
	DefaultOrphanUploadTTL = 24 * time.Hour
	// DefaultOrphanScanInterval is how often the collector looks for them.
	DefaultOrphanScanInterval = time.Hour
	// orphanBatchSize bounds the blobs locked by one collection transaction.
	orphanBatchSize = 100
)

// claimOrphanBlobsSQL locks the oldest blobs whose bytes Goéland holds, that no
// version references and that waited longer than @ttl since their last upload.
// SKIP LOCKED leaves a blob being attached (locked FOR SHARE by CreateDocument
// or AddDocumentVersion) and the batches of another replica alone.
const claimOrphanBlobsSQL = `
SELECT b.id FROM content_blob b
WHERE b.disposed_at IS NULL AND b.storage_ref <> ''
  AND coalesce(b.uploaded_at, b.created_at) < now() - @ttl::interval
  AND NOT EXISTS (SELECT 1 FROM document_version v WHERE v.content_blob_id = b.id)
ORDER BY coalesce(b.uploaded_at, b.created_at)
LIMIT @limit
FOR UPDATE OF b SKIP LOCKED;`

// disposeOrphanBlobsSQL marks destroyed the claimed blobs that are still
// orphans and returns where their bytes are. It is a second statement on
// purpose: under READ COMMITTED it sees the versions committed while the claim
// waited, and no new version can reference a blob locked FOR UPDATE.
const disposeOrphanBlobsSQL = `
WITH doomed AS (
    SELECT b.id, b.storage_ref FROM content_blob b
    WHERE b.id = ANY(@ids::uuid[])
      AND NOT EXISTS (SELECT 1 FROM document_version v WHERE v.content_blob_id = b.id)
)
UPDATE content_blob b SET disposed_at = now(), storage_ref = ''
FROM doomed WHERE b.id = doomed.id
RETURNING doomed.storage_ref;`

// OrphanConfig tunes the orphan upload collection.
type OrphanConfig struct {
	// TTL is how long uploaded bytes may stay unattached
	// (GOELAND_ORPHAN_UPLOAD_TTL_HOURS); zero means DefaultOrphanUploadTTL.
	TTL time.Duration
	// ScanInterval is how often the collector runs
	// (GOELAND_ORPHAN_SCAN_MINUTES); zero means DefaultOrphanScanInterval.
	ScanInterval time.Duration
}

// OrphanCollector destroys the bytes of uploads never attached to a document
// version (GLD-045). The blob row stays, marked disposed_at, like a blob
// destroyed by an elimination: an identical upload stores the bytes again.
// Blobs whose bytes Goéland does not hold (the imported ones) are never
// candidates. It is safe to run on every replica.
type OrphanCollector struct {
	pool  *pgxpool.Pool
	store blobstore.Store
	cfg   OrphanConfig
	log   *slog.Logger
}

// NewOrphanCollector builds a collector; the pool and the store are required,
// a nil logger falls back to slog.Default.
func NewOrphanCollector(pool *pgxpool.Pool, store blobstore.Store, cfg OrphanConfig, log *slog.Logger) (*OrphanCollector, error) {
	if pool == nil || store == nil {
		return nil, fmt.Errorf("%w: the orphan collector needs a pool and a blob store", core.ErrInvalidInput)
	}
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultOrphanUploadTTL
	}
	if cfg.ScanInterval <= 0 {
		cfg.ScanInterval = DefaultOrphanScanInterval
	}
	if log == nil {
		log = slog.Default()
	}
	return &OrphanCollector{pool: pool, store: store, cfg: cfg, log: log}, nil
}

// Run collects at start, then every ScanInterval, until ctx is cancelled.
func (c *OrphanCollector) Run(ctx context.Context) {
	c.log.Info("orphan upload collector started", "ttl", c.cfg.TTL.String(), "scan_interval", c.cfg.ScanInterval.String())
	ticker := time.NewTicker(c.cfg.ScanInterval)
	defer ticker.Stop()
	for {
		if n, err := c.Collect(ctx); err != nil {
			c.log.Error("orphan upload collection", "error", err, "collected", n)
		} else if n > 0 {
			c.log.Info("orphan uploads collected", "collected", n)
		}
		select {
		case <-ctx.Done():
			c.log.Info("orphan upload collector stopped")
			return
		case <-ticker.C:
		}
	}
}

// Collect destroys every orphan upload older than the TTL, by batches, and
// returns how many blobs it marked destroyed.
func (c *OrphanCollector) Collect(ctx context.Context) (int, error) {
	total := 0
	for ctx.Err() == nil {
		n, claimed, err := c.collectBatch(ctx)
		total += n
		if err != nil || claimed < orphanBatchSize {
			return total, err
		}
	}
	return total, nil
}

// collectBatch marks one batch destroyed, then deletes its bytes after the
// commit: a failed transaction leaves every byte in place. It returns the
// blobs marked and the blobs claimed.
func (c *OrphanCollector) collectBatch(ctx context.Context) (int, int, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin orphan collection: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, claimOrphanBlobsSQL, pgx.NamedArgs{"ttl": c.cfg.TTL, "limit": orphanBatchSize})
	if err != nil {
		return 0, 0, fmt.Errorf("claim orphan uploads: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil || len(ids) == 0 {
		return 0, 0, err
	}
	rows, err = tx.Query(ctx, disposeOrphanBlobsSQL, pgx.NamedArgs{"ids": ids})
	if err != nil {
		return 0, len(ids), fmt.Errorf("dispose orphan uploads: %w", err)
	}
	refs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return 0, len(ids), fmt.Errorf("dispose orphan uploads: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, len(ids), fmt.Errorf("commit orphan collection: %w", err)
	}
	for _, ref := range refs {
		// The rows already say destroyed: finish even when ctx is cancelled.
		if err := c.store.Delete(context.WithoutCancel(ctx), ref); err != nil {
			c.log.Error("orphan upload: bytes not deleted, remove them by hand", "storage_ref", ref, "error", err)
		}
	}
	return len(refs), len(ids), nil
}
