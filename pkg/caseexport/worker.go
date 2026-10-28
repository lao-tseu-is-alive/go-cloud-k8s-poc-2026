package caseexport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// requeueTimeout bounds putting back a job interrupted by a shutdown.
const requeueTimeout = 5 * time.Second

// Run builds queued exports and removes expired archives every PollInterval
// until ctx is cancelled. Every replica may run it: jobs are claimed with
// FOR UPDATE SKIP LOCKED.
func (s *Service) Run(ctx context.Context) {
	s.log.Info("case export worker started", "poll_interval", s.cfg.PollInterval.String(), "ttl", s.cfg.TTL.String())
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	for {
		s.tick(ctx)
		select {
		case <-ctx.Done():
			s.log.Info("case export worker stopped")
			return
		case <-ticker.C:
		}
	}
}

// tick builds every queued export, then removes the expired archives.
func (s *Service) tick(ctx context.Context) {
	for ctx.Err() == nil {
		processed, err := s.ProcessNext(ctx)
		if err != nil {
			s.log.Error("case export worker", "error", err)
			break
		}
		if !processed {
			break
		}
	}
	if ctx.Err() != nil {
		return
	}
	if _, err := s.ExpireArchives(ctx); err != nil {
		s.log.Error("case export expiry", "error", err)
	}
}

// ProcessNext claims the oldest queued export and builds it. It reports
// whether a job was claimed; a failed build marks the job FAILED and is not
// an error of ProcessNext.
func (s *Service) ProcessNext(ctx context.Context) (bool, error) {
	args := pgx.NamedArgs{"max_attempts": maxAttempts, "stale_after": s.cfg.StaleAfter}
	if _, err := s.pool.Exec(ctx, failExhaustedSQL, args); err != nil {
		return false, fmt.Errorf("give up interrupted exports: %w", err)
	}
	exp, err := collectOne(s.pool.Query(ctx, claimExportSQL, args))
	if errors.Is(err, core.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim export: %w", err)
	}
	buildErr := s.build(ctx, exp)
	switch {
	case buildErr == nil:
		s.log.Info("case export built", "export_id", exp.ID, "case_id", exp.CaseID)
	case ctx.Err() != nil:
		requeueCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), requeueTimeout)
		defer cancel()
		if _, err := s.pool.Exec(requeueCtx, requeueExportSQL, pgx.NamedArgs{"id": exp.ID}); err != nil {
			return true, fmt.Errorf("requeue interrupted export %s: %w", exp.ID, err)
		}
	default:
		s.log.Warn("case export failed", "export_id", exp.ID, "case_id", exp.CaseID, "error", buildErr)
		if _, err := s.pool.Exec(ctx, failExportSQL, pgx.NamedArgs{"id": exp.ID, "error": publicError(buildErr)}); err != nil {
			return true, fmt.Errorf("record failed export %s: %w", exp.ID, err)
		}
	}
	return true, nil
}

// build assembles the archive as the requester, who must still hold MANAGE on
// the case (or, for a transfer package, as the system reading everything),
// stores it and marks the job DONE, recording EXPORT for each confidential
// document of an ordinary export.
func (s *Service) build(ctx context.Context, exp *Export) error {
	viewer, level, err := s.builder(ctx, exp)
	if err != nil {
		return err
	}
	a, err := s.assemble(ctx, exp, viewer, level)
	if err != nil {
		return err
	}
	stored, err := s.storeArchive(ctx, exp, a)
	if err != nil {
		return err
	}
	if err := s.finish(ctx, exp, a, stored); err != nil {
		_ = s.store.Delete(context.WithoutCancel(ctx), stored.Ref)
		return err
	}
	return nil
}

// builder returns who reads the case for the archive and at which level: the
// system for a transfer package (the bordereau execution recorded the EXPORT),
// otherwise the requester, who must still hold MANAGE.
func (s *Service) builder(ctx context.Context, exp *Export) (core.Viewer, core.Level, error) {
	if exp.Purpose == PurposeTransfer {
		return core.SystemViewer(exp.RequestedBy), core.LevelFullControl, nil
	}
	access, err := core.EffectiveAccessTx(ctx, s.pool, exp.RequestedBy, exp.CaseID)
	if err != nil {
		return core.Viewer{}, 0, err
	}
	if err := access.Require(core.LevelManage); err != nil {
		return core.Viewer{}, 0, err
	}
	viewer, err := core.ViewerTx(ctx, s.pool, exp.RequestedBy)
	return viewer, access.Level, err
}

// finish marks the job DONE and records EXPORT for each confidential document
// of an ordinary archive, in one transaction.
func (s *Service) finish(ctx context.Context, exp *Export, a *assembly, stored blobstore.Stored) error {
	ttl := s.cfg.TTL
	if exp.Purpose == PurposeTransfer {
		ttl = TransferTTL
	}
	return core.InTx(ctx, s.pool, "finish export", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, finishExportSQL, pgx.NamedArgs{
			"id": exp.ID, "ttl": ttl, "storage_ref": stored.Ref, "sha256": stored.SHA256, "size_bytes": stored.Size,
		})
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("the export is no longer running (taken over by another worker)")
		}
		if exp.Purpose == PurposeTransfer {
			return nil
		}
		for _, id := range a.documents {
			access, err := core.EffectiveAccessTx(ctx, tx, exp.RequestedBy, id)
			if err != nil {
				return err
			}
			if err := core.RecordGrantedAccessTx(ctx, tx, exp.RequestedBy, access, core.AccessExport, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// ExpireArchives removes the archives past their time to live and marks
// their jobs EXPIRED; it returns how many it removed.
func (s *Service) ExpireArchives(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, expiredExportsSQL)
	if err != nil {
		return 0, fmt.Errorf("list expired exports: %w", err)
	}
	expired, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Export])
	if err != nil {
		return 0, fmt.Errorf("read expired exports: %w", err)
	}
	for _, exp := range expired {
		if err := s.store.Delete(ctx, exp.StorageRef); err != nil {
			return 0, fmt.Errorf("remove archive of %s: %w", exp.ID, err)
		}
		if _, err := s.pool.Exec(ctx, expireExportSQL, pgx.NamedArgs{"id": exp.ID}); err != nil {
			return 0, fmt.Errorf("expire %s: %w", exp.ID, err)
		}
	}
	return len(expired), nil
}

// publicError is the failure shown to the requester: the reason when it is
// theirs to act on, a pointer to the server log otherwise (internal details
// such as storage paths stay there).
func publicError(err error) string {
	if errors.Is(err, core.ErrPermissionDenied) {
		return "the requester no longer holds MANAGE on the case"
	}
	return "the archive could not be built; the server log has the details"
}

// QueueCounts returns how many export jobs are queued and running, for the
// metrics (GLD-010).
func (s *Service) QueueCounts(ctx context.Context) (queued, running int64, err error) {
	err = s.pool.QueryRow(ctx, queueCountsSQL).Scan(&queued, &running)
	return queued, running, err
}
