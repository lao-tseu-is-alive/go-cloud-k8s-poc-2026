package actor

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
	// MaxDisplayNameLength is the maximum number of code points in an actor display name.
	MaxDisplayNameLength = 200
	// MaxContactValueLength is the maximum number of code points in a contact value.
	MaxContactValueLength = 400
)

// Service contains the transport-independent actor business logic.
type Service struct {
	repo    Repository
	coreSvc *core.Service
	log     *slog.Logger
}

// NewService constructs a Service backed by the actor repository and the core
// service (used to read relationships and audit). A nil logger falls back to slog.Default.
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

// Create validates and persists a new actor.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Actor, *core.AuditEvent, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if err := validateDisplayName(in.DisplayName); err != nil {
		return nil, nil, err
	}
	if !in.ActorKind.Valid() {
		return nil, nil, fmt.Errorf("%w: actor_kind must be PERSON or ORGANIZATION", core.ErrInvalidInput)
	}
	// Enforce that the specialization matches the kind (defence in depth on top of
	// the DB CHECK constraints), so person/organization columns never cross over.
	in.LegalName = strings.TrimSpace(in.LegalName)
	in.OrgComplement = strings.TrimSpace(in.OrgComplement)
	in.CategoryCode = strings.TrimSpace(in.CategoryCode)
	in.CHRegisterRef = strings.TrimSpace(in.CHRegisterRef)
	switch in.ActorKind {
	case KindOrganization:
		if in.LegalName == "" {
			return nil, nil, fmt.Errorf("%w: legal_name is required for an organization", core.ErrInvalidInput)
		}
		if utf8.RuneCountInString(in.LegalName) > MaxDisplayNameLength {
			return nil, nil, fmt.Errorf("%w: legal_name exceeds %d characters", core.ErrInvalidInput, MaxDisplayNameLength)
		}
		// Clear any person-only fields defensively.
		in.IsCHRegister = false
		in.CHRegisterRef = ""
	case KindPerson:
		// Clear any organization-only fields defensively.
		in.LegalName = ""
		in.OrgComplement = ""
		in.CategoryCode = ""
	}
	contacts, err := normalizeContacts(in.Contacts)
	if err != nil {
		return nil, nil, err
	}
	in.Contacts = contacts

	// Complete the governance/identity input consistently with the actor.
	in.Governance.Kind = core.SubjectKindActor
	in.Governance.DisplayLabel = in.DisplayName
	in.Governance.OperatorID = in.OperatorID
	if in.Governance.OwnerUserID == "" {
		in.Governance.OwnerUserID = in.OperatorID
	}

	act, ev, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("create actor: %w", err)
	}
	s.log.Info("created actor", "actor_id", act.ID, "kind", int16(in.ActorKind))
	return act, ev, nil
}

// Get loads an actor by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Actor, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: actor id is required", core.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// Relationships returns the relationships pointing at an actor (actors are the
// target of CASE_HAS_ACTOR_* / DOCUMENT_*_ACTOR edges), so list the incoming ones.
func (s *Service) Relationships(ctx context.Context, id uuid.UUID) ([]*core.SubjectRelationship, error) {
	res, err := s.coreSvc.ListRelationships(ctx, core.RelationshipFilter{
		SubjectID: id, Outgoing: false, Limit: core.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	return res.Relationships, nil
}

// RecentAudit returns the most recent audit events for an actor subject.
func (s *Service) RecentAudit(ctx context.Context, id uuid.UUID) ([]*core.AuditEvent, error) {
	res, err := s.coreSvc.ListAuditEvents(ctx, core.AuditFilter{SubjectID: id, Limit: 20})
	if err != nil {
		return nil, err
	}
	return res.Events, nil
}

// Update applies a partial update (rejected when the record is locked).
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Actor, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: actor id is required", core.ErrInvalidInput)
	}
	if in.DisplayName != nil {
		name := strings.TrimSpace(*in.DisplayName)
		if err := validateDisplayName(name); err != nil {
			return nil, nil, err
		}
		in.DisplayName = &name
	}
	if in.ReplaceContacts {
		contacts, err := normalizeContacts(in.Contacts)
		if err != nil {
			return nil, nil, err
		}
		in.Contacts = contacts
	}
	act, ev, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return nil, nil, fmt.Errorf("update actor: %w", err)
	}
	s.log.Info("updated actor", "actor_id", id)
	return act, ev, nil
}

// Search runs an accent-insensitive + filtered actor search.
func (s *Service) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	limit, err := core.NormalizePageSize(filter.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	filter.Limit = limit
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	filter.Query = strings.TrimSpace(filter.Query)
	filter.OrganizationCatCode = strings.TrimSpace(filter.OrganizationCatCode)
	return s.repo.Search(ctx, filter)
}

// SoftDelete logically deletes an actor.
func (s *Service) SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: actor id is required", core.ErrInvalidInput)
	}
	ev, err := s.repo.SoftDelete(ctx, id, operatorID, reason)
	if err != nil {
		return nil, fmt.Errorf("delete actor: %w", err)
	}
	s.log.Info("deleted actor", "actor_id", id)
	return ev, nil
}

// ListCategories returns the organization category catalogue.
func (s *Service) ListCategories(ctx context.Context, onlyActive bool) ([]*OrganizationCategory, error) {
	return s.repo.ListCategories(ctx, onlyActive)
}

// validateDisplayName enforces the non-empty + max-length rule on a display name.
func validateDisplayName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: display_name is required", core.ErrInvalidInput)
	}
	if utf8.RuneCountInString(name) > MaxDisplayNameLength {
		return fmt.Errorf("%w: display_name exceeds %d characters", core.ErrInvalidInput, MaxDisplayNameLength)
	}
	return nil
}

// normalizeContacts trims and validates each contact, rejecting unknown types,
// blank values and over-long values.
func normalizeContacts(in []ContactInput) ([]ContactInput, error) {
	out := make([]ContactInput, 0, len(in))
	for _, c := range in {
		if !c.ContactType.Valid() {
			return nil, fmt.Errorf("%w: invalid contact_type", core.ErrInvalidInput)
		}
		c.Value = strings.TrimSpace(c.Value)
		if c.Value == "" {
			return nil, fmt.Errorf("%w: contact value is required", core.ErrInvalidInput)
		}
		if utf8.RuneCountInString(c.Value) > MaxContactValueLength {
			return nil, fmt.Errorf("%w: contact value exceeds %d characters", core.ErrInvalidInput, MaxContactValueLength)
		}
		c.Label = strings.TrimSpace(c.Label)
		out = append(out, c)
	}
	return out, nil
}
