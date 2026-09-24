package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
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
	store   blobstore.Store
	log     *slog.Logger
}

// ErrNoContentStore is returned by IngestContent when the service was built
// without a blob store.
var ErrNoContentStore = errors.New("document service has no content store")

// NewService constructs a Service backed by the document repository, the core
// service (used to read relationships and audit for a document) and the store
// holding content bytes. store may be nil when the caller never ingests bytes
// (IngestContent then fails with ErrNoContentStore). A nil logger falls back to
// slog.Default.
func NewService(repo Repository, coreSvc *core.Service, store blobstore.Store, log *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", core.ErrInvalidInput)
	}
	if coreSvc == nil {
		return nil, fmt.Errorf("%w: core service is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, coreSvc: coreSvc, store: store, log: log}, nil
}

// IngestContent stores uploaded bytes and registers them as a content blob,
// with global deduplication (spec v2 §17, §20): when identical content is
// already known, the new bytes are removed and the existing blob is returned
// with Reused set. The digest and size are always computed by the server.
func (s *Service) IngestContent(ctx context.Context, r io.Reader, filename, mimeType, operatorID string) (IngestResult, error) {
	if s.store == nil {
		return IngestResult{}, ErrNoContentStore
	}
	saved, err := s.store.Put(ctx, r, blobstore.Metadata{Filename: filename, ContentType: mimeType})
	if err != nil {
		return IngestResult{}, fmt.Errorf("store content: %w", err)
	}
	blob, reused, err := s.repo.RegisterBlob(ctx, ContentBlob{
		SHA256:        saved.SHA256,
		StorageRef:    saved.Ref,
		MimeType:      mimeType,
		FileSizeBytes: saved.Size,
		CreatedBy:     operatorID,
	})
	if err != nil || reused {
		// Unregistered bytes (failure or duplicate) must not linger in storage;
		// the cleanup must run even when the request context was cancelled.
		if rmErr := s.store.Delete(context.WithoutCancel(ctx), saved.Ref); rmErr != nil {
			s.log.Error("remove unregistered content", "storage_ref", saved.Ref, "error", rmErr)
		}
	}
	if err != nil {
		return IngestResult{}, fmt.Errorf("register content: %w", err)
	}
	s.log.Info("ingested content", "content_blob_id", blob.ID, "size", blob.FileSizeBytes, "reused", reused)
	return IngestResult{Blob: blob, Reused: reused, Filename: saved.Filename}, nil
}

// Create validates and persists a new document, or reuses the live document
// that already holds the same content (see CreateInput).
func (s *Service) Create(ctx context.Context, in CreateInput) (CreateResult, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.DocumentTypeCode = strings.TrimSpace(in.DocumentTypeCode)
	if in.Title == "" {
		return CreateResult{}, fmt.Errorf("%w: title is required", core.ErrInvalidInput)
	}
	if utf8.RuneCountInString(in.Title) > MaxTitleLength {
		return CreateResult{}, fmt.Errorf("%w: title exceeds %d characters", core.ErrInvalidInput, MaxTitleLength)
	}
	if utf8.RuneCountInString(in.Description) > MaxDescriptionLength {
		return CreateResult{}, fmt.Errorf("%w: description exceeds %d characters", core.ErrInvalidInput, MaxDescriptionLength)
	}
	if in.DocumentTypeCode == "" {
		return CreateResult{}, fmt.Errorf("%w: document_type_code is required", core.ErrInvalidInput)
	}
	// Complete the governance/identity input consistently with the document.
	in.Governance.Kind = core.SubjectKindDocument
	in.Governance.DisplayLabel = in.Title
	in.Governance.CanonicalURL = in.ExternalURL
	in.Governance.OperatorID = in.OperatorID
	if in.Governance.OwnerUserID == "" {
		in.Governance.OwnerUserID = in.OperatorID
	}
	if in.PageCount < 0 {
		return CreateResult{}, fmt.Errorf("%w: page_count must not be negative", core.ErrInvalidInput)
	}
	res, err := s.repo.Create(ctx, in)
	if err != nil {
		return CreateResult{}, fmt.Errorf("create document: %w", err)
	}
	s.log.Info("created document", "document_id", res.Document.ID, "type", in.DocumentTypeCode, "reused", res.Reused)
	return res, nil
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

// AddVersion appends a new current version to a mutable document (rejected when
// the document is locked or deleted). The content may repeat an earlier version's.
func (s *Service) AddVersion(ctx context.Context, documentID uuid.UUID, in VersionInput) (*Document, *Version, *core.AuditEvent, error) {
	if documentID == uuid.Nil {
		return nil, nil, nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	if in.PageCount < 0 {
		return nil, nil, nil, fmt.Errorf("%w: page_count must not be negative", core.ErrInvalidInput)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	doc, version, ev, err := s.repo.AddVersion(ctx, documentID, in)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("add document version: %w", err)
	}
	s.log.Info("added document version", "document_id", documentID, "version_no", version.VersionNo)
	return doc, version, ev, nil
}

// ListVersions returns the versions of a document, newest first.
func (s *Service) ListVersions(ctx context.Context, documentID uuid.UUID) ([]*Version, error) {
	if documentID == uuid.Nil {
		return nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	return s.repo.ListVersions(ctx, documentID)
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
func (s *Service) Finalize(ctx context.Context, id uuid.UUID, operatorID, reason string, alsoLock bool) (*Document, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	doc, ev, err := s.repo.Finalize(ctx, id, operatorID, reason, alsoLock)
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
func (s *Service) SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	ev, err := s.repo.SoftDelete(ctx, id, operatorID, reason)
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
