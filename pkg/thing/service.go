package thing

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const (
	// MaxNameLength is the maximum number of code points in a thing name.
	MaxNameLength = 300
	// MaxDescriptionLength is the maximum number of code points in a description.
	MaxDescriptionLength = 4000
	// MaxExternalRefLength is the maximum number of code points in an external reference.
	MaxExternalRefLength = 200
	// recentAuditLimit bounds the audit events returned with a thing.
	recentAuditLimit = 20
)

var egridPattern = regexp.MustCompile(`^CH[0-9A-Z]{12}$`)

// Service contains the transport-independent thing business logic.
type Service struct {
	repo    Repository
	coreSvc *core.Service
	log     *slog.Logger
}

// NewService constructs a Service backed by the thing repository and the core
// service (relationships and audit). A nil logger falls back to slog.Default.
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

// Create validates and registers a thing; the type-dependent rules (detail
// block, derived name, geometry type) are checked once the type is known.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Thing, *core.AuditEvent, error) {
	in.TypeCode = strings.TrimSpace(in.TypeCode)
	if in.TypeCode == "" {
		return nil, nil, fmt.Errorf("%w: thing_type_code is required", core.ErrInvalidInput)
	}
	if err := trimTexts(&in.Name, &in.Description, &in.ExternalRef, &in.GeometryGeoJSON); err != nil {
		return nil, nil, err
	}
	in.Governance.Kind = core.SubjectKindThing
	in.Governance.OperatorID = in.OperatorID
	if in.Governance.OwnerUserID == "" {
		in.Governance.OwnerUserID = in.OperatorID
	}
	t, ev, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("create thing: %w", err)
	}
	s.log.Info("created thing", "thing_id", t.ID, "type", in.TypeCode)
	return t, ev, nil
}

// Get loads a thing by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Thing, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: thing id is required", core.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// Relationships returns the thing's relationships in both directions.
func (s *Service) Relationships(ctx context.Context, id uuid.UUID) ([]*core.SubjectRelationship, error) {
	return s.coreSvc.SubjectRelationships(ctx, id)
}

// RecentAudit returns the most recent audit events of a thing, newest first.
func (s *Service) RecentAudit(ctx context.Context, id uuid.UUID) ([]*core.AuditEvent, error) {
	res, err := s.coreSvc.ListAuditEvents(ctx, core.AuditFilter{SubjectID: id, Limit: recentAuditLimit})
	if err != nil {
		return nil, err
	}
	return res.Events, nil
}

// Update replaces the editable fields of a thing (rejected when locked or deleted).
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Thing, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: thing id is required", core.ErrInvalidInput)
	}
	if err := trimTexts(&in.Name, &in.Description, &in.ExternalRef, &in.GeometryGeoJSON); err != nil {
		return nil, nil, err
	}
	if in.Name == "" {
		return nil, nil, fmt.Errorf("%w: name is required", core.ErrInvalidInput)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	t, ev, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return nil, nil, fmt.Errorf("update thing: %w", err)
	}
	s.log.Info("updated thing", "thing_id", id)
	return t, ev, nil
}

// Search runs the filtered thing search.
func (s *Service) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	limit, err := core.NormalizePageSize(filter.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	filter.Limit, filter.Offset = limit, max(filter.Offset, 0)
	filter.Query = strings.TrimSpace(filter.Query)
	filter.TypeCode = strings.TrimSpace(filter.TypeCode)
	return s.repo.Search(ctx, filter)
}

// SoftDelete logically deletes a thing.
func (s *Service) SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: thing id is required", core.ErrInvalidInput)
	}
	ev, err := s.repo.SoftDelete(ctx, id, operatorID, strings.TrimSpace(reason))
	if err != nil {
		return nil, fmt.Errorf("delete thing: %w", err)
	}
	s.log.Info("deleted thing", "thing_id", id)
	return ev, nil
}

// ListTypes returns the thing type catalogue.
func (s *Service) ListTypes(ctx context.Context, onlyActive bool) ([]*ThingType, error) {
	return s.repo.ListTypes(ctx, onlyActive)
}

// trimTexts trims the free-text fields and checks their lengths (the geometry
// is only trimmed; its size is bounded by the API contract).
func trimTexts(name, description, externalRef, geometry *string) error {
	*name, *description, *externalRef, *geometry = strings.TrimSpace(*name), strings.TrimSpace(*description),
		strings.TrimSpace(*externalRef), strings.TrimSpace(*geometry)
	switch {
	case utf8.RuneCountInString(*name) > MaxNameLength:
		return fmt.Errorf("%w: name exceeds %d characters", core.ErrInvalidInput, MaxNameLength)
	case utf8.RuneCountInString(*description) > MaxDescriptionLength:
		return fmt.Errorf("%w: description exceeds %d characters", core.ErrInvalidInput, MaxDescriptionLength)
	case utf8.RuneCountInString(*externalRef) > MaxExternalRefLength:
		return fmt.Errorf("%w: external_ref exceeds %d characters", core.ErrInvalidInput, MaxExternalRefLength)
	}
	return nil
}

// prepareCreate checks the detail block against the type and derives the
// name of a parcel or building when it is blank.
func prepareCreate(in *CreateInput, spec Specialization) error {
	if err := checkDetails(spec, in.Parcel, in.Building, true); err != nil {
		return err
	}
	if in.Name == "" {
		in.Name = derivedName(spec, in.Parcel, in.Building)
	}
	if in.Name == "" {
		return fmt.Errorf("%w: name is required", core.ErrInvalidInput)
	}
	return nil
}

// derivedName names a parcel "Parcelle <number>" and a building with an EGID
// "Bâtiment EGID <egid>"; other things get no derived name.
func derivedName(spec Specialization, parcel *Parcel, building *Building) string {
	switch {
	case spec == SpecializationParcel && parcel != nil:
		return "Parcelle " + parcel.ParcelNumber
	case spec == SpecializationBuilding && building != nil && building.EGID != nil:
		return "Bâtiment EGID " + strconv.Itoa(int(*building.EGID))
	}
	return ""
}

// checkDetails requires the detail block to match the specialization (a parcel
// block is mandatory on create) and validates its identifiers.
func checkDetails(spec Specialization, parcel *Parcel, building *Building, creating bool) error {
	switch {
	case parcel != nil && spec != SpecializationParcel, building != nil && spec != SpecializationBuilding:
		return fmt.Errorf("%w: the detail block does not match the thing type", core.ErrInvalidInput)
	case spec == SpecializationParcel && parcel == nil && creating:
		return fmt.Errorf("%w: a parcel needs its commune and parcel number", core.ErrInvalidInput)
	case parcel != nil:
		return checkParcel(parcel)
	case building != nil:
		return checkBuilding(building)
	}
	return nil
}

// checkParcel normalizes and validates the land-register identifiers.
func checkParcel(p *Parcel) error {
	p.ParcelNumber = strings.TrimSpace(p.ParcelNumber)
	p.EGRID = strings.ToUpper(strings.TrimSpace(p.EGRID))
	switch {
	case p.CommuneOFS < 1 || p.CommuneOFS > 9999:
		return fmt.Errorf("%w: commune_ofs must be an OFS commune number (1-9999)", core.ErrInvalidInput)
	case p.ParcelNumber == "" || utf8.RuneCountInString(p.ParcelNumber) > 20:
		return fmt.Errorf("%w: parcel_number is required (at most 20 characters)", core.ErrInvalidInput)
	case p.EGRID != "" && !egridPattern.MatchString(p.EGRID):
		return fmt.Errorf("%w: egrid must be CH followed by 12 letters or digits", core.ErrInvalidInput)
	case p.SurfaceM2 != nil && *p.SurfaceM2 <= 0:
		return fmt.Errorf("%w: surface_m2 must be positive", core.ErrInvalidInput)
	}
	return nil
}

// checkBuilding normalizes and validates the building identifiers.
func checkBuilding(b *Building) error {
	b.ECANumber = strings.TrimSpace(b.ECANumber)
	switch {
	case b.EGID != nil && *b.EGID <= 0:
		return fmt.Errorf("%w: egid must be positive", core.ErrInvalidInput)
	case utf8.RuneCountInString(b.ECANumber) > 20:
		return fmt.Errorf("%w: eca_number exceeds 20 characters", core.ErrInvalidInput)
	case b.ConstructionYear != nil && (*b.ConstructionYear < 1000 || *b.ConstructionYear > 2100):
		return fmt.Errorf("%w: construction_year must be between 1000 and 2100", core.ErrInvalidInput)
	case !b.Status.Valid():
		return fmt.Errorf("%w: invalid building_status", core.ErrInvalidInput)
	}
	return nil
}
