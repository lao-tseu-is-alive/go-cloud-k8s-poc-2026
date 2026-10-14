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
// the case, stores it and marks the job DONE, recording EXPORT for each
// confidential document it holds.
func (s *Service) build(ctx context.Context, exp *Export) error {
	access, err := core.EffectiveAccessTx(ctx, s.pool, exp.RequestedBy, exp.CaseID)
	if err != nil {
		return err
	}
	if err := access.Require(core.LevelManage); err != nil {
		return err
	}
	viewer, err := core.ViewerTx(ctx, s.pool, exp.RequestedBy)
	if err != nil {
		return err
	}
	a, err := s.assemble(ctx, exp, viewer, access.Level)
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

// finish marks the job DONE and records EXPORT for each confidential document
// of the archive, in one transaction.
func (s *Service) finish(ctx context.Context, exp *Export, a *assembly, stored blobstore.Stored) error {
	return core.InTx(ctx, s.pool, "finish export", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, finishExportSQL, pgx.NamedArgs{
			"id": exp.ID, "ttl": s.cfg.TTL, "storage_ref": stored.Ref, "sha256": stored.SHA256, "size_bytes": stored.Size,
		})
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("the export is no longer running (taken over by another worker)")
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
