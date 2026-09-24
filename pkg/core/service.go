package core

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
)

// recentAuditLimit bounds the number of audit events returned inline with a subject.
const recentAuditLimit = 10

// Service contains the transport-independent core business logic.
type Service struct {
	repo Repository
	log  *slog.Logger
}

// NewService constructs a Service backed by the given repository. A nil logger falls back to slog.Default.
func NewService(repo Repository, log *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, log: log}, nil
}

// Repo exposes the underlying repository so sibling domains (Document) can reuse core primitives.
func (s *Service) Repo() Repository { return s.repo }

// CreateSubjectRef validates the input and creates a subject identity + governance record.
func (s *Service) CreateSubjectRef(ctx context.Context, in CreateSubjectInput) (*SubjectRef, *RecordMetadata, *AuditEvent, error) {
	in.DisplayLabel = strings.TrimSpace(in.DisplayLabel)
	if !in.Kind.Valid() {
		return nil, nil, nil, fmt.Errorf("%w: unknown subject kind %q", ErrInvalidInput, in.Kind)
	}
	if in.DisplayLabel == "" {
		return nil, nil, nil, fmt.Errorf("%w: display_label is required", ErrInvalidInput)
	}
	if in.ConfidentialityLevel < 0 || in.ConfidentialityLevel > 5 {
		return nil, nil, nil, fmt.Errorf("%w: confidentiality_level must be between 0 and 5", ErrInvalidInput)
	}
	if !in.BusinessRef.IsZero() {
		req, err := in.BusinessRef.Normalized()
		if err != nil {
			return nil, nil, nil, err
		}
		in.BusinessRef = req
	}
	ref, md, ev, err := s.repo.CreateSubject(ctx, in)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create subject: %w", err)
	}
	s.log.Info("created subject", "subject_id", ref.ID, "kind", ref.Kind)
	return ref, md, ev, nil
}

// AssignBusinessRef gives an existing subject its business reference (explicit or
// allocated) and audits it. A subject keeps its first reference: a second
// assignment fails with ErrConflict, as does a (namespace, reference) pair
// already in use.
func (s *Service) AssignBusinessRef(ctx context.Context, subjectID uuid.UUID, req BusinessRefRequest, operatorID, reason string) (*SubjectRef, *AuditEvent, error) {
	if subjectID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: subject id is required", ErrInvalidInput)
	}
	req, err := req.Normalized()
	if err != nil {
		return nil, nil, err
	}
	ref, ev, err := s.repo.AssignBusinessRef(ctx, subjectID, req, operatorID, strings.TrimSpace(reason))
	if err != nil {
		return nil, nil, fmt.Errorf("assign business_ref: %w", err)
	}
	s.log.Info("assigned business reference", "subject_id", subjectID, "namespace", ref.BusinessRefNamespace, "allocated", req.Allocate)
	return ref, ev, nil
}

// LookupSubjects finds subjects by exact business reference (at most 50).
func (s *Service) LookupSubjects(ctx context.Context, filter LookupFilter) ([]*SubjectRef, error) {
	filter.BusinessRef = strings.TrimSpace(filter.BusinessRef)
	filter.Namespace = strings.TrimSpace(filter.Namespace)
	if filter.BusinessRef == "" {
		return nil, fmt.Errorf("%w: business_ref is required", ErrInvalidInput)
	}
	if filter.Kind != SubjectKindUnspecified && !filter.Kind.Valid() {
		return nil, fmt.Errorf("%w: unknown subject kind %q", ErrInvalidInput, filter.Kind)
	}
	return s.repo.LookupSubjects(ctx, filter, maxLookupResults)
}

// GetSubjectRef loads a subject and optionally its metadata and recent audit events.
func (s *Service) GetSubjectRef(ctx context.Context, id uuid.UUID, includeMetadata, includeAudit bool) (*SubjectRef, *RecordMetadata, []*AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, nil, fmt.Errorf("%w: subject id is required", ErrInvalidInput)
	}
	ref, err := s.repo.GetSubject(ctx, id)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get subject: %w", err)
	}
	var md *RecordMetadata
	if includeMetadata {
		md, err = s.repo.GetRecordMetadata(ctx, id)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("get record metadata: %w", err)
		}
	}
	var audit []*AuditEvent
	if includeAudit {
		res, err := s.repo.ListAuditEvents(ctx, AuditFilter{SubjectID: id, Limit: recentAuditLimit})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("get recent audit: %w", err)
		}
		audit = res.Events
	}
	return ref, md, audit, nil
}

// LinkSubjects validates and creates a typed relationship between two subjects.
func (s *Service) LinkSubjects(ctx context.Context, in LinkInput) (*SubjectRelationship, *AuditEvent, error) {
	if in.SourceSubjectID == uuid.Nil || in.TargetSubjectID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: source and target subject ids are required", ErrInvalidInput)
	}
	in.RelationshipTypeCode = strings.TrimSpace(in.RelationshipTypeCode)
	if in.RelationshipTypeCode == "" {
		return nil, nil, fmt.Errorf("%w: relationship_type_code is required", ErrInvalidInput)
	}
	if in.SourceSubjectID == in.TargetSubjectID {
		return nil, nil, fmt.Errorf("%w: a subject cannot be linked to itself", ErrInvalidInput)
	}
	rel, ev, err := s.repo.LinkSubjects(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("link subjects: %w", err)
	}
	s.log.Info("linked subjects", "relationship_id", rel.ID, "type", in.RelationshipTypeCode)
	return rel, ev, nil
}

// UnlinkSubjects soft-deletes an existing relationship.
func (s *Service) UnlinkSubjects(ctx context.Context, relationshipID uuid.UUID, operatorID, reason string) (*SubjectRelationship, *AuditEvent, error) {
	if relationshipID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: relationship id is required", ErrInvalidInput)
	}
	rel, ev, err := s.repo.UnlinkSubjects(ctx, relationshipID, operatorID, reason)
	if err != nil {
		return nil, nil, fmt.Errorf("unlink subjects: %w", err)
	}
	s.log.Info("unlinked subjects", "relationship_id", relationshipID)
	return rel, ev, nil
}

// EndRelationship records the business end of an open relationship; the edge
// stays listed as history. It fails with ErrNotFound for an unknown or unlinked
// edge, ErrInvalidState when the edge already has an end, and ErrInvalidInput
// when the end precedes the start of validity.
func (s *Service) EndRelationship(ctx context.Context, in EndInput) (*SubjectRelationship, *AuditEvent, error) {
	if in.RelationshipID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: relationship id is required", ErrInvalidInput)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	rel, ev, err := s.repo.EndRelationship(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("end relationship: %w", err)
	}
	s.log.Info("ended relationship", "relationship_id", in.RelationshipID)
	return rel, ev, nil
}

// ListRelationships returns a page of relationships for a subject.
func (s *Service) ListRelationships(ctx context.Context, filter RelationshipFilter) (RelationshipResult, error) {
	if filter.SubjectID == uuid.Nil {
		return RelationshipResult{}, fmt.Errorf("%w: subject id is required", ErrInvalidInput)
	}
	limit, err := NormalizePageSize(filter.Limit)
	if err != nil {
		return RelationshipResult{}, err
	}
	filter.Limit = limit
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	filter.RelationshipTypeCode = strings.TrimSpace(filter.RelationshipTypeCode)
	return s.repo.ListRelationships(ctx, filter)
}

// ListRelationshipTypes returns the catalogue of relationship types.
func (s *Service) ListRelationshipTypes(ctx context.Context, onlyActive bool, sourceKind, targetKind SubjectKind) ([]*RelationshipType, error) {
	return s.repo.ListRelationshipTypes(ctx, onlyActive, sourceKind, targetKind)
}

// ListAuditEvents returns a page of audit events for a subject.
func (s *Service) ListAuditEvents(ctx context.Context, filter AuditFilter) (AuditResult, error) {
	if filter.SubjectID == uuid.Nil {
		return AuditResult{}, fmt.Errorf("%w: subject id is required", ErrInvalidInput)
	}
	limit, err := NormalizePageSize(filter.Limit)
	if err != nil {
		return AuditResult{}, err
	}
	filter.Limit = limit
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	filter.EventType = strings.TrimSpace(filter.EventType)
	return s.repo.ListAuditEvents(ctx, filter)
}

// CurrentUser returns the recorded profile of the authenticated caller,
// recording it now when the verifier could not (recording is best effort there).
func (s *Service) CurrentUser(ctx context.Context, user *authadapter.AuthenticatedUser) (*AppUser, error) {
	profile := ProfileFromUser(user)
	if profile.UserID == "" {
		return nil, fmt.Errorf("%w: an authenticated user is required", ErrInvalidInput)
	}
	users, err := s.repo.GetUsers(ctx, []string{profile.UserID})
	if err != nil {
		return nil, fmt.Errorf("current user: %w", err)
	}
	if len(users) == 1 && profile.sameAs(users[0]) {
		return users[0], nil
	}
	recorded, err := s.repo.RecordUser(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("current user: %w", err)
	}
	return recorded, nil
}

// BatchGetUsers resolves operator ids to users; blank and repeated ids are
// ignored and unknown ids are absent from the result.
func (s *Service) BatchGetUsers(ctx context.Context, userIDs []string) ([]*AppUser, error) {
	ids := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id = strings.TrimSpace(id); id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	switch {
	case len(ids) == 0:
		return nil, fmt.Errorf("%w: at least one user id is required", ErrInvalidInput)
	case len(ids) > MaxBatchUsers:
		return nil, fmt.Errorf("%w: at most %d user ids", ErrInvalidInput, MaxBatchUsers)
	}
	return s.repo.GetUsers(ctx, ids)
}
