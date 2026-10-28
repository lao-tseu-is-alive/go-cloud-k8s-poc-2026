package retention

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository is the persistence of the retention domain.
type Repository interface {
	ListPolicies(ctx context.Context, onlyActive bool) ([]*Policy, error)
	CreatePolicy(ctx context.Context, in PolicyInput) (*Policy, *core.ReferenceChange, error)
	UpdatePolicy(ctx context.Context, code string, in PolicyUpdate) (*Policy, *core.ReferenceChange, error)
	CaseRetention(ctx context.Context, caseID uuid.UUID) (*CaseRetention, error)
	PlaceHold(ctx context.Context, in HoldInput) (*Hold, *core.AuditEvent, error)
	ReleaseHold(ctx context.Context, in HoldInput) (*Hold, *core.AuditEvent, error)
	ListDue(ctx context.Context, filter DueFilter) (DueResult, error)
	ApplyCaseType(ctx context.Context, in ApplyInput) (ApplyResult, error)
}

// Service validates and runs the retention use cases.
type Service struct {
	repo Repository
	log  *slog.Logger
}

// NewService builds a Service. A nil logger falls back to slog.Default.
func NewService(repo Repository, log *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, log: log}, nil
}

// ListPolicies returns the retention schedule.
func (s *Service) ListPolicies(ctx context.Context, onlyActive bool) ([]*Policy, error) {
	return s.repo.ListPolicies(ctx, onlyActive)
}

// CreatePolicy validates and adds a policy.
func (s *Service) CreatePolicy(ctx context.Context, in PolicyInput) (*Policy, *core.ReferenceChange, error) {
	var err error
	in.Code = strings.TrimSpace(in.Code)
	if err = core.ValidateReferenceCode(in.Code); err != nil {
		return nil, nil, err
	}
	if in.Label, err = core.NormalizeReferenceText("label", in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if in.Description, err = core.NormalizeReferenceText("description", in.Description, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	if in.LegalBasis, err = core.NormalizeReferenceText("legal_basis", in.LegalBasis, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	if err = validDuration(in.DurationYears); err != nil {
		return nil, nil, err
	}
	if !in.FinalDisposition.Valid() {
		return nil, nil, fmt.Errorf("%w: final_disposition must be ARCHIVE, DESTROY or REVIEW", core.ErrInvalidInput)
	}
	return s.repo.CreatePolicy(ctx, in)
}

// UpdatePolicy validates and applies a policy change; running retentions keep
// their snapshot.
func (s *Service) UpdatePolicy(ctx context.Context, code string, in PolicyUpdate) (*Policy, *core.ReferenceChange, error) {
	if err := core.NormalizeOptionalReferenceText("label", &in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("description", &in.Description, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("legal_basis", &in.LegalBasis, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	if in.DurationYears != nil {
		if err := validDuration(*in.DurationYears); err != nil {
			return nil, nil, err
		}
	}
	if in.FinalDisposition != nil && !in.FinalDisposition.Valid() {
		return nil, nil, fmt.Errorf("%w: final_disposition must be ARCHIVE, DESTROY or REVIEW", core.ErrInvalidInput)
	}
	return s.repo.UpdatePolicy(ctx, strings.TrimSpace(code), in)
}

// CaseRetention describes the retention of a case.
func (s *Service) CaseRetention(ctx context.Context, caseID uuid.UUID) (*CaseRetention, error) {
	if caseID == uuid.Nil {
		return nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	return s.repo.CaseRetention(ctx, caseID)
}

// PlaceHold places a legal hold on a case.
func (s *Service) PlaceHold(ctx context.Context, in HoldInput) (*Hold, *core.AuditEvent, error) {
	in, err := normalizeHold(in)
	if err != nil {
		return nil, nil, err
	}
	h, ev, err := s.repo.PlaceHold(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	s.log.Info("retention hold placed", "case_id", in.CaseID, "operator", in.OperatorID)
	return h, ev, nil
}

// ReleaseHold releases the legal hold of a case.
func (s *Service) ReleaseHold(ctx context.Context, in HoldInput) (*Hold, *core.AuditEvent, error) {
	in, err := normalizeHold(in)
	if err != nil {
		return nil, nil, err
	}
	h, ev, err := s.repo.ReleaseHold(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	s.log.Info("retention hold released", "case_id", in.CaseID, "operator", in.OperatorID)
	return h, ev, nil
}

// ListDue pages the running retentions ending on or before a day.
func (s *Service) ListDue(ctx context.Context, filter DueFilter) (DueResult, error) {
	limit, err := core.NormalizePageSize(filter.Limit)
	if err != nil {
		return DueResult{}, err
	}
	filter.Limit, filter.Offset = limit, max(filter.Offset, 0)
	if filter.DueOn = strings.TrimSpace(filter.DueOn); filter.DueOn != "" {
		if _, err := time.Parse(time.DateOnly, filter.DueOn); err != nil {
			return DueResult{}, fmt.Errorf("%w: due_on must be a YYYY-MM-DD day", core.ErrInvalidInput)
		}
	}
	if filter.Disposition != "" && !filter.Disposition.Valid() {
		return DueResult{}, fmt.Errorf("%w: unknown final disposition %q", core.ErrInvalidInput, filter.Disposition)
	}
	return s.repo.ListDue(ctx, filter)
}

// ApplyCaseType gives a case type's policy to at most MaxApplyBatch of its
// closed cases without retention.
func (s *Service) ApplyCaseType(ctx context.Context, in ApplyInput) (ApplyResult, error) {
	in.CaseTypeCode, in.Reason = strings.TrimSpace(in.CaseTypeCode), strings.TrimSpace(in.Reason)
	if in.CaseTypeCode == "" {
		return ApplyResult{}, fmt.Errorf("%w: case type code is required", core.ErrInvalidInput)
	}
	if err := validReason(in.Reason); err != nil {
		return ApplyResult{}, err
	}
	res, err := s.repo.ApplyCaseType(ctx, in)
	if err != nil {
		return ApplyResult{}, err
	}
	s.log.Info("retention policy applied", "case_type", in.CaseTypeCode, "applied", res.Applied, "remaining", res.Remaining, "operator", in.OperatorID)
	return res, nil
}

// normalizeHold checks a hold change and trims its reason.
func normalizeHold(in HoldInput) (HoldInput, error) {
	if in.CaseID == uuid.Nil {
		return in, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	return in, validReason(in.Reason)
}

// validReason requires a reason of at most MaxReasonLength code points.
func validReason(reason string) error {
	switch {
	case reason == "":
		return fmt.Errorf("%w: a reason is required", core.ErrInvalidInput)
	case utf8.RuneCountInString(reason) > MaxReasonLength:
		return fmt.Errorf("%w: the reason exceeds %d characters", core.ErrInvalidInput, MaxReasonLength)
	}
	return nil
}

// validDuration checks a policy duration (0-MaxDurationYears).
func validDuration(years int32) error {
	if years < 0 || years > MaxDurationYears {
		return fmt.Errorf("%w: duration_years must be between 0 and %d", core.ErrInvalidInput, MaxDurationYears)
	}
	return nil
}
