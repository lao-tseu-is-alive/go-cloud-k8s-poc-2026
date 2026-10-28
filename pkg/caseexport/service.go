package caseexport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/circulation"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// Sources are the read services an archive is assembled from.
type Sources struct {
	// Core lists relationships and the audit.
	Core *core.Service
	// Cases reads the case.
	Cases *casefile.Service
	// Timeline lists the case timeline.
	Timeline *timeline.Service
	// Tasks lists the case tasks.
	Tasks *task.Service
	// Circulations lists the case circulations.
	Circulations *circulation.Service
	// Documents reads the documents and their versions.
	Documents *document.Service
}

// Service queues, builds, serves and expires case exports.
type Service struct {
	pool    *pgxpool.Pool
	store   blobstore.Store
	sources Sources
	cfg     Config
	log     *slog.Logger
}

// NewService validates its dependencies and returns a Service.
func NewService(pool *pgxpool.Pool, store blobstore.Store, sources Sources, cfg Config, log *slog.Logger) (*Service, error) {
	if pool == nil || store == nil {
		return nil, errors.New("caseexport: pool and blob store are required")
	}
	if sources.Core == nil || sources.Cases == nil || sources.Timeline == nil || sources.Tasks == nil ||
		sources.Circulations == nil || sources.Documents == nil {
		return nil, errors.New("caseexport: every source service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{pool: pool, store: store, sources: sources, cfg: cfg.withDefaults(), log: log}, nil
}

// RequestExport queues an export of a case. It needs MANAGE on the case; the
// request is recorded as EXPORT in the sensitive read audit, granted or
// refused, whatever the case's confidentiality.
func (s *Service) RequestExport(ctx context.Context, in RequestInput) (*Export, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(in.Reason) > maxReasonLength {
		return nil, fmt.Errorf("%w: reason exceeds %d characters", core.ErrInvalidInput, maxReasonLength)
	}
	var created *Export
	var refusal error // committed with its DENIED row, returned after the transaction
	err := core.InTx(ctx, s.pool, "request export", func(tx pgx.Tx) error {
		access, err := core.EffectiveAccessTx(ctx, tx, in.OperatorID, in.CaseID)
		if err != nil {
			return err
		}
		if access.Kind != core.SubjectKindCase {
			return fmt.Errorf("%w: only a case can be exported", core.ErrInvalidInput)
		}
		if denied := access.Require(core.LevelManage); denied != nil {
			refusal = denied
			return core.RecordAccessTx(ctx, tx, in.OperatorID, access, core.AccessExport, core.AccessDenied)
		}
		if err := core.RecordAccessTx(ctx, tx, in.OperatorID, access, core.AccessExport, core.AccessGranted); err != nil {
			return err
		}
		created, err = collectOne(tx.Query(ctx, insertExportSQL, pgx.NamedArgs{
			"case_id": in.CaseID, "requested_by": in.OperatorID, "reason": in.Reason,
			"request_id": core.RequestIDFromContext(ctx),
		}))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("request export: %w", err)
	}
	if refusal != nil {
		return nil, refusal
	}
	s.log.Info("case export requested", "export_id", created.ID, "case_id", in.CaseID)
	return created, nil
}

// Get returns an export, readable by its requester or an administrator.
func (s *Service) Get(ctx context.Context, id uuid.UUID, operatorID string, admin bool) (*Export, error) {
	exp, err := collectOne(s.pool.Query(ctx, getExportSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	if exp.RequestedBy != operatorID && !admin {
		return nil, core.ErrNotFound // another user's export is not disclosed
	}
	return exp, nil
}

// List returns the exports of a case, newest first: the caller's own, or
// every requester's for an administrator.
func (s *Service) List(ctx context.Context, caseID uuid.UUID, operatorID string, admin bool) ([]*Export, error) {
	requestedBy := operatorID
	if admin {
		requestedBy = ""
	}
	rows, err := s.pool.Query(ctx, listExportsSQL, pgx.NamedArgs{"case_id": caseID, "requested_by": requestedBy, "limit": listLimit})
	if err != nil {
		return nil, fmt.Errorf("list exports: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Export])
}

// Archive is a built archive opened for download.
type Archive struct {
	// Export is the job.
	Export *Export
	// Object streams the archive bytes; the caller must Close it.
	Object blobstore.Object
	// Filename is the download name.
	Filename string
}

// OpenArchive opens a built archive for its requester, who must still hold
// MANAGE on the case, or a transfer package for a records manager (records).
// A missing, foreign, unfinished or expired export is not found.
func (s *Service) OpenArchive(ctx context.Context, id uuid.UUID, operatorID string, records bool) (*Archive, error) {
	exp, err := collectOne(s.pool.Query(ctx, getExportSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	transfer := exp.Purpose == PurposeTransfer
	if (transfer && !records) || (!transfer && exp.RequestedBy != operatorID) {
		return nil, core.ErrNotFound // another user's export is not disclosed
	}
	if exp.Status != StatusDone || exp.StorageRef == "" {
		return nil, fmt.Errorf("%w: the export is %s", core.ErrInvalidState, strings.ToLower(string(exp.Status)))
	}
	if !transfer {
		if err := core.EnsureAccessTx(ctx, s.pool, operatorID, exp.CaseID, core.LevelManage); err != nil {
			return nil, err
		}
	}
	obj, err := s.store.Get(ctx, exp.StorageRef)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	return &Archive{Export: exp, Object: obj, Filename: "case-export-" + exp.ID.String() + ".zip"}, nil
}

// collectOne scans exactly one export row.
func collectOne(rows pgx.Rows, err error) (*Export, error) {
	if err != nil {
		return nil, err
	}
	exp, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Export])
	if err != nil {
		return nil, core.MapDBError(err, nil)
	}
	return exp, nil
}
