package casefile

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const (
	// MaxTitleLength is the maximum number of code points in a case title.
	MaxTitleLength = 500
	// MaxDescriptionLength is the maximum number of code points in a case description.
	MaxDescriptionLength = 4000
	// recentAuditLimit bounds the audit events returned with a case.
	recentAuditLimit = 20
)

// Service contains the transport-independent case business logic.
type Service struct {
	repo    Repository
	coreSvc *core.Service
	log     *slog.Logger
}

// NewService constructs a Service backed by the case repository and the core
// service (used to read relationships and audit). A nil logger falls back to
// slog.Default.
func NewService(repo Repository, coreSvc *core.Service, log *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", core.ErrInvalidInput)
	}
	if coreSvc == nil {
		return nil, fmt.Errorf("%w: core service is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, coreSvc: coreSvc, log: log}, nil
}

// Create validates and opens a new case (status OPEN), allocating its business
// reference in the case type's namespace unless one is given explicitly.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Case, *core.AuditEvent, error) {
	in.CaseTypeCode = strings.TrimSpace(in.CaseTypeCode)
	if in.CaseTypeCode == "" {
		return nil, nil, fmt.Errorf("%w: case_type_code is required", core.ErrInvalidInput)
	}
	title, err := validateText(in.Title, in.Description)
	if err != nil {
		return nil, nil, err
	}
	in.Title = title
	if !in.BusinessRef.IsZero() {
		if in.BusinessRef, err = in.BusinessRef.Normalized(); err != nil {
			return nil, nil, err
		}
	}
	// Complete the governance/identity input consistently with the case.
	in.Governance.Kind = core.SubjectKindCase
	in.Governance.DisplayLabel = in.Title
	in.Governance.OperatorID = in.OperatorID
	if in.Governance.OwnerUserID == "" {
		in.Governance.OwnerUserID = in.OperatorID
	}
	c, ev, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("create case: %w", err)
	}
	s.log.Info("opened case", "case_id", c.ID, "type", in.CaseTypeCode)
	return c, ev, nil
}

// Get loads a case by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Case, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// Relationships returns the case's active relationships in both directions:
// outgoing (actors, documents, things, child cases) then incoming (parent cases).
func (s *Service) Relationships(ctx context.Context, id uuid.UUID) ([]*core.SubjectRelationship, error) {
	return s.coreSvc.SubjectRelationships(ctx, id)
}

// RecentAudit returns the most recent audit events of a case, newest first.
func (s *Service) RecentAudit(ctx context.Context, id uuid.UUID) ([]*core.AuditEvent, error) {
	res, err := s.coreSvc.ListAuditEvents(ctx, core.AuditFilter{SubjectID: id, Limit: recentAuditLimit})
	if err != nil {
		return nil, err
	}
	return res.Events, nil
}

// Update replaces the editable metadata of a case (rejected when closed,
// locked or deleted).
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Case, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	title, err := validateText(in.Title, in.Description)
	if err != nil {
		return nil, nil, err
	}
	in.Title = title
	c, ev, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return nil, nil, fmt.Errorf("update case: %w", err)
	}
	s.log.Info("updated case", "case_id", id)
	return c, ev, nil
}

// Transition moves a case to another status; see CanTransition and
// RequiresReason for the rules.
func (s *Service) Transition(ctx context.Context, id uuid.UUID, in TransitionInput) (*Case, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	if !in.Target.Valid() {
		return nil, nil, fmt.Errorf("%w: unknown target status %d", core.ErrInvalidInput, in.Target)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	c, ev, err := s.repo.Transition(ctx, id, in)
	if err != nil {
		return nil, nil, fmt.Errorf("transition case: %w", err)
	}
	s.log.Info("changed case status", "case_id", id, "status", c.Status.String())
	return c, ev, nil
}

// Search runs the filtered case search.
func (s *Service) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	limit, err := core.NormalizePageSize(filter.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	filter.Limit = limit
	filter.Offset = max(filter.Offset, 0)
	filter.Query = strings.TrimSpace(filter.Query)
	filter.CaseTypeCode = strings.TrimSpace(filter.CaseTypeCode)
	if filter.Status != StatusUnspecified && !filter.Status.Valid() {
		return SearchResult{}, fmt.Errorf("%w: unknown status %d", core.ErrInvalidInput, filter.Status)
	}
	return s.repo.Search(ctx, filter)
}

// SoftDelete logically deletes a case.
func (s *Service) SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	ev, err := s.repo.SoftDelete(ctx, id, operatorID, strings.TrimSpace(reason))
	if err != nil {
		return nil, fmt.Errorf("delete case: %w", err)
	}
	s.log.Info("deleted case", "case_id", id)
	return ev, nil
}

// ListTypes returns the case type catalogue.
func (s *Service) ListTypes(ctx context.Context, onlyActive bool) ([]*CaseType, error) {
	return s.repo.ListTypes(ctx, onlyActive)
}

// validateText trims and checks a title and description, returning the trimmed title.
func validateText(title, description string) (string, error) {
	title = strings.TrimSpace(title)
	switch {
	case title == "":
		return "", fmt.Errorf("%w: title is required", core.ErrInvalidInput)
	case utf8.RuneCountInString(title) > MaxTitleLength:
		return "", fmt.Errorf("%w: title exceeds %d characters", core.ErrInvalidInput, MaxTitleLength)
	case utf8.RuneCountInString(description) > MaxDescriptionLength:
		return "", fmt.Errorf("%w: description exceeds %d characters", core.ErrInvalidInput, MaxDescriptionLength)
	}
	return title, nil
}
