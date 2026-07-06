package document

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
	// MaxTitleLength is the maximum number of code points allowed in a document title.
	MaxTitleLength = 500
	// MaxDescriptionLength is the maximum number of code points allowed in a description.
	MaxDescriptionLength = 4000
)

// Service contains the transport-independent document business logic.
type Service struct {
	repo    Repository
	coreSvc *core.Service
	log     *slog.Logger
}

// NewService constructs a Service backed by the document repository and the core
// service (used to read relationships and audit for a document). A nil logger falls back to slog.Default.
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

// Create validates and persists a new document.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Document, *core.AuditEvent, *core.SubjectRelationship, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.DocumentTypeCode = strings.TrimSpace(in.DocumentTypeCode)
	if in.Title == "" {
		return nil, nil, nil, fmt.Errorf("%w: title is required", core.ErrInvalidInput)
	}
	if utf8.RuneCountInString(in.Title) > MaxTitleLength {
		return nil, nil, nil, fmt.Errorf("%w: title exceeds %d characters", core.ErrInvalidInput, MaxTitleLength)
	}
	if utf8.RuneCountInString(in.Description) > MaxDescriptionLength {
		return nil, nil, nil, fmt.Errorf("%w: description exceeds %d characters", core.ErrInvalidInput, MaxDescriptionLength)
	}
	if in.DocumentTypeCode == "" {
		return nil, nil, nil, fmt.Errorf("%w: document_type_code is required", core.ErrInvalidInput)
	}
	// Complete the governance/identity input consistently with the document.
	in.Governance.Kind = core.SubjectKindDocument
	in.Governance.DisplayLabel = in.Title
	in.Governance.CanonicalURL = in.ExternalURL
	in.Governance.ActorUserID = in.ActorUserID
	if in.Governance.OwnerUserID == "" {
		in.Governance.OwnerUserID = in.ActorUserID
	}
	doc, ev, rel, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create document: %w", err)
	}
	s.log.Info("created document", "document_id", doc.ID, "type", in.DocumentTypeCode)
	return doc, ev, rel, nil
}

// Get loads a document by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Document, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// Relationships returns the outgoing relationships for a document subject.
func (s *Service) Relationships(ctx context.Context, id uuid.UUID) ([]*core.SubjectRelationship, error) {
	res, err := s.coreSvc.ListRelationships(ctx, core.RelationshipFilter{
		SubjectID: id, Outgoing: true, Limit: core.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	return res.Relationships, nil
}

// RecentAudit returns the most recent audit events for a document subject.
func (s *Service) RecentAudit(ctx context.Context, id uuid.UUID) ([]*core.AuditEvent, error) {
	res, err := s.coreSvc.ListAuditEvents(ctx, core.AuditFilter{SubjectID: id, Limit: 20})
	if err != nil {
		return nil, err
	}
	return res.Events, nil
}

// UpdateMetadata updates mutable metadata (rejected when the record is locked).
func (s *Service) UpdateMetadata(ctx context.Context, id uuid.UUID, in UpdateInput) (*Document, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, nil, fmt.Errorf("%w: title is required", core.ErrInvalidInput)
	}
	if utf8.RuneCountInString(in.Title) > MaxTitleLength {
		return nil, nil, fmt.Errorf("%w: title exceeds %d characters", core.ErrInvalidInput, MaxTitleLength)
	}
	doc, ev, err := s.repo.UpdateMetadata(ctx, id, in)
	if err != nil {
		return nil, nil, fmt.Errorf("update document metadata: %w", err)
	}
	s.log.Info("updated document metadata", "document_id", id)
	return doc, ev, nil
}

// Finalize marks a document final and optionally locks its governance record.
func (s *Service) Finalize(ctx context.Context, id uuid.UUID, actorUserID, reason string, alsoLock bool) (*Document, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	doc, ev, err := s.repo.Finalize(ctx, id, actorUserID, reason, alsoLock)
	if err != nil {
		return nil, nil, fmt.Errorf("finalize document: %w", err)
	}
	s.log.Info("finalized document", "document_id", id, "locked", alsoLock)
	return doc, ev, nil
}

// Verify checks a document's integrity hash and records the verification timestamp.
func (s *Service) Verify(ctx context.Context, id uuid.UUID, expectedSHA256 string) (*Document, bool, error) {
	if id == uuid.Nil {
		return nil, false, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	return s.repo.Verify(ctx, id, strings.TrimSpace(expectedSHA256))
}

// Search runs a full-text + filtered document search.
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
	filter.DocumentTypeCode = strings.TrimSpace(filter.DocumentTypeCode)
	// A confidentiality_max of 0 means "no cap" (return everything up to the max level).
	if filter.ConfidentialityMax <= 0 {
		filter.ConfidentialityMax = 5
	}
	return s.repo.Search(ctx, filter)
}

// Link creates a typed relationship from a document to another subject.
func (s *Service) Link(ctx context.Context, in core.LinkInput) (*core.SubjectRelationship, *core.AuditEvent, error) {
	if in.SourceSubjectID == uuid.Nil || in.TargetSubjectID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: document id and target id are required", core.ErrInvalidInput)
	}
	in.RelationshipTypeCode = strings.TrimSpace(in.RelationshipTypeCode)
	if in.RelationshipTypeCode == "" {
		return nil, nil, fmt.Errorf("%w: relationship_type_code is required", core.ErrInvalidInput)
	}
	rel, ev, err := s.repo.Link(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("link document: %w", err)
	}
	s.log.Info("linked document", "document_id", in.SourceSubjectID, "type", in.RelationshipTypeCode)
	return rel, ev, nil
}

// SoftDelete logically deletes a document.
func (s *Service) SoftDelete(ctx context.Context, id uuid.UUID, actorUserID, reason string) (*core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	ev, err := s.repo.SoftDelete(ctx, id, actorUserID, reason)
	if err != nil {
		return nil, fmt.Errorf("delete document: %w", err)
	}
	s.log.Info("deleted document", "document_id", id)
	return ev, nil
}

// ListTypes returns the document type catalogue.
func (s *Service) ListTypes(ctx context.Context, onlyActive bool) ([]*DocumentType, error) {
	return s.repo.ListTypes(ctx, onlyActive)
}
