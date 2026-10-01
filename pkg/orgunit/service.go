package orgunit

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const (
	// MaxAbbreviationLength is the maximum number of code points in an abbreviation.
	MaxAbbreviationLength = 50
	// MaxExternalRefLength is the maximum number of code points in an external reference.
	MaxExternalRefLength = 100
	// MaxLabelLength is the maximum number of code points in a unit label.
	MaxLabelLength = 200
	// MaxDescriptionLength is the maximum number of code points in a description.
	MaxDescriptionLength = 2000
	// recentAuditLimit bounds the audit events returned with a unit.
	recentAuditLimit = 20
)

// Service contains the transport-independent org unit business logic.
type Service struct {
	repo    Repository
	coreSvc *core.Service
	log     *slog.Logger
}

// NewService constructs a Service backed by the org unit repository and the
// core service (used to read relationships and audit). A nil logger falls back
// to slog.Default.
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

// Create validates and adds a unit.
func (s *Service) Create(ctx context.Context, in CreateInput) (*OrgUnit, *core.AuditEvent, error) {
	var err error
	if in.ExternalRef, err = normalizeToken("external_ref", in.ExternalRef, MaxExternalRefLength); err != nil {
		return nil, nil, err
	}
	if in.Input, err = normalizeInput(in.Input); err != nil {
		return nil, nil, err
	}
	u, ev, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("create org unit: %w", err)
	}
	s.log.Info("created org unit", "org_unit_id", u.ID)
	return u, ev, nil
}

// Get loads a unit with its ancestors and children.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Detail, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: org unit id is required", core.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// Relationships returns the unit's active relationships in both directions.
func (s *Service) Relationships(ctx context.Context, id uuid.UUID, viewer core.Viewer) ([]*core.SubjectRelationship, error) {
	return s.coreSvc.SubjectRelationships(ctx, id, viewer)
}

// RecentAudit returns the most recent audit events of a unit, newest first.
func (s *Service) RecentAudit(ctx context.Context, id uuid.UUID) ([]*core.AuditEvent, error) {
	res, err := s.coreSvc.ListAuditEvents(ctx, core.AuditFilter{SubjectID: id, Limit: recentAuditLimit})
	if err != nil {
		return nil, err
	}
	return res.Events, nil
}

// List returns the whole tree as a flat list.
func (s *Service) List(ctx context.Context, includeDissolved bool, viewer core.Viewer) ([]*Node, error) {
	return s.repo.List(ctx, includeDissolved, viewer)
}

// Search runs the filtered unit search.
func (s *Service) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	limit, err := core.NormalizePageSize(filter.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	filter.Limit = limit
	filter.Offset = max(filter.Offset, 0)
	filter.Query = strings.TrimSpace(filter.Query)
	return s.repo.Search(ctx, filter)
}

// Update validates and replaces the editable fields of a live unit.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (*OrgUnit, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: org unit id is required", core.ErrInvalidInput)
	}
	in, err := normalizeInput(in)
	if err != nil {
		return nil, nil, err
	}
	u, ev, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return nil, nil, fmt.Errorf("update org unit: %w", err)
	}
	s.log.Info("updated org unit", "org_unit_id", id)
	return u, ev, nil
}

// Dissolve dissolves a unit without live sub-units; a reason is required.
func (s *Service) Dissolve(ctx context.Context, id uuid.UUID, operatorID, reason string) (*OrgUnit, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: org unit id is required", core.ErrInvalidInput)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, nil, fmt.Errorf("%w: a reason is required to dissolve a unit", core.ErrInvalidInput)
	}
	u, ev, err := s.repo.Dissolve(ctx, id, operatorID, reason)
	if err != nil {
		return nil, nil, fmt.Errorf("dissolve org unit: %w", err)
	}
	s.log.Info("dissolved org unit", "org_unit_id", id)
	return u, ev, nil
}

// ListTypes returns the unit type catalogue.
func (s *Service) ListTypes(ctx context.Context, onlyActive bool) ([]*OrgUnitType, error) {
	return s.repo.ListTypes(ctx, onlyActive)
}

// normalizeToken trims an optional short identifier (abbreviation, external
// reference) and checks its length and characters (no control characters;
// spaces, accents and punctuation are allowed because real sigles use them).
func normalizeToken(field, value string, maxLength int) (string, error) {
	value = strings.TrimSpace(value)
	switch {
	case utf8.RuneCountInString(value) > maxLength:
		return "", fmt.Errorf("%w: %s exceeds %d characters", core.ErrInvalidInput, field, maxLength)
	case strings.ContainsFunc(value, unicode.IsControl):
		return "", fmt.Errorf("%w: %s contains control characters", core.ErrInvalidInput, field)
	}
	return value, nil
}

// normalizeInput trims and checks the editable fields of a unit.
func normalizeInput(in Input) (Input, error) {
	in.TypeCode = strings.TrimSpace(in.TypeCode)
	in.Label = strings.TrimSpace(in.Label)
	in.Description = strings.TrimSpace(in.Description)
	in.Email = strings.TrimSpace(in.Email)
	in.Reason = strings.TrimSpace(in.Reason)
	abbreviation, err := normalizeToken("abbreviation", in.Abbreviation, MaxAbbreviationLength)
	if err != nil {
		return in, err
	}
	in.Abbreviation = abbreviation
	switch {
	case in.TypeCode == "":
		return in, fmt.Errorf("%w: org_unit_type_code is required", core.ErrInvalidInput)
	case in.Label == "":
		return in, fmt.Errorf("%w: label is required", core.ErrInvalidInput)
	case utf8.RuneCountInString(in.Label) > MaxLabelLength:
		return in, fmt.Errorf("%w: label exceeds %d characters", core.ErrInvalidInput, MaxLabelLength)
	case utf8.RuneCountInString(in.Description) > MaxDescriptionLength:
		return in, fmt.Errorf("%w: description exceeds %d characters", core.ErrInvalidInput, MaxDescriptionLength)
	}
	if in.ParentID != nil && *in.ParentID == uuid.Nil {
		in.ParentID = nil
	}
	if in.Email == "" {
		return in, nil
	}
	email, err := core.NormalizeEmail(in.Email)
	if err != nil {
		return in, fmt.Errorf("%w: email: %v", core.ErrInvalidInput, err)
	}
	in.Email = email
	return in, nil
}
