package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
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
	ref, md, ev, err := s.repo.CreateSubject(ctx, in)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create subject: %w", err)
	}
	s.log.Info("created subject", "subject_id", ref.ID, "kind", ref.Kind)
	return ref, md, ev, nil
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
