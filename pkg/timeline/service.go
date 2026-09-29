package timeline

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

const (
	// MaxTitleLength is the maximum number of code points in an entry title.
	MaxTitleLength = 500
	// MaxBodyLength is the maximum number of code points in an entry body.
	MaxBodyLength = 20000
	// MaxDocumentsPerRequest bounds the documents cited by one create request.
	MaxDocumentsPerRequest = 50
	// futureTolerance absorbs clock skew between a client and the server when
	// checking that a business date is not in the future.
	futureTolerance = time.Minute
)

// Service contains the transport-independent timeline business logic.
type Service struct {
	repo Repository
	log  *slog.Logger
	// now is the clock used to reject future business dates (tests override it).
	now func() time.Time
}

// NewService constructs a Service backed by the timeline repository. A nil
// logger falls back to slog.Default.
func NewService(repo Repository, log *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, log: log, now: time.Now}, nil
}

// Create validates and adds a draft entry to an open case.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Entry, *core.AuditEvent, error) {
	if in.CaseID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	content, err := s.normalizeContent(entryContent{Type: in.Type, Title: in.Title, Body: in.Body, Visibility: in.Visibility, OccurredAt: in.OccurredAt})
	if err != nil {
		return nil, nil, err
	}
	in.Type, in.Title, in.Body, in.Visibility = content.Type, content.Title, content.Body, content.Visibility
	if in.CorrectsEntryID != nil && *in.CorrectsEntryID == uuid.Nil {
		in.CorrectsEntryID = nil
	}
	if in.DocumentIDs, err = uniqueDocumentIDs(in.DocumentIDs); err != nil {
		return nil, nil, err
	}
	e, ev, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("create timeline entry: %w", err)
	}
	s.log.Info("added timeline entry", "case_id", in.CaseID, "entry_id", e.ID, "type", e.Type.String())
	return e, ev, nil
}

// Get loads an entry by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Entry, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: timeline entry id is required", core.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// List returns a page of a case timeline, most recent business date first.
func (s *Service) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	if filter.CaseID == uuid.Nil {
		return ListResult{}, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	limit, err := core.NormalizePageSize(filter.Limit)
	if err != nil {
		return ListResult{}, err
	}
	filter.Limit = limit
	filter.Offset = max(filter.Offset, 0)
	for _, t := range filter.Types {
		if !t.Valid() {
			return ListResult{}, fmt.Errorf("%w: unknown entry type %d", core.ErrInvalidInput, t)
		}
	}
	return s.repo.List(ctx, filter)
}

// Update replaces the content of a draft.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Entry, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: timeline entry id is required", core.ErrInvalidInput)
	}
	content, err := s.normalizeContent(entryContent{Type: in.Type, Title: in.Title, Body: in.Body, Visibility: in.Visibility, OccurredAt: in.OccurredAt})
	if err != nil {
		return nil, nil, err
	}
	in.Type, in.Title, in.Body, in.Visibility = content.Type, content.Title, content.Body, content.Visibility
	in.Reason = strings.TrimSpace(in.Reason)
	e, ev, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return nil, nil, fmt.Errorf("update timeline entry: %w", err)
	}
	s.log.Info("updated timeline entry", "entry_id", id)
	return e, ev, nil
}

// Validate endorses a draft: it becomes immutable and the current version of
// each cited document is pinned. Until GLD-017 the author may validate their
// own entry.
func (s *Service) Validate(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	return s.transition(id, "validate", func() (*Entry, *core.AuditEvent, error) {
		return s.repo.Validate(ctx, id, operatorID, strings.TrimSpace(reason))
	})
}

// Lock freezes a draft as is, without endorsing it; cited document versions
// are pinned as on validation.
func (s *Service) Lock(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	return s.transition(id, "lock", func() (*Entry, *core.AuditEvent, error) {
		return s.repo.Lock(ctx, id, operatorID, strings.TrimSpace(reason))
	})
}

// Withdraw sets a draft aside; a reason is required.
func (s *Service) Withdraw(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, nil, fmt.Errorf("%w: a reason is required to withdraw a timeline entry", core.ErrInvalidInput)
	}
	return s.transition(id, "withdraw", func() (*Entry, *core.AuditEvent, error) {
		return s.repo.Withdraw(ctx, id, operatorID, reason)
	})
}

// transition checks the id, runs a status change and logs it.
func (s *Service) transition(id uuid.UUID, verb string, run func() (*Entry, *core.AuditEvent, error)) (*Entry, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: timeline entry id is required", core.ErrInvalidInput)
	}
	e, ev, err := run()
	if err != nil {
		return nil, nil, fmt.Errorf("%s timeline entry: %w", verb, err)
	}
	s.log.Info("changed timeline entry status", "entry_id", id, "status", e.Status.String())
	return e, ev, nil
}

// LinkDocument cites a document in a draft (and links it to the case when needed).
func (s *Service) LinkDocument(ctx context.Context, entryID, documentID uuid.UUID, operatorID string) (*Entry, *core.AuditEvent, error) {
	if entryID == uuid.Nil || documentID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: timeline entry id and document id are required", core.ErrInvalidInput)
	}
	e, ev, err := s.repo.LinkDocument(ctx, entryID, documentID, operatorID)
	if err != nil {
		return nil, nil, fmt.Errorf("link timeline document: %w", err)
	}
	return e, ev, nil
}

// UnlinkDocument removes a cited document from a draft.
func (s *Service) UnlinkDocument(ctx context.Context, entryID, documentID uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	if entryID == uuid.Nil || documentID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: timeline entry id and document id are required", core.ErrInvalidInput)
	}
	e, ev, err := s.repo.UnlinkDocument(ctx, entryID, documentID, operatorID, strings.TrimSpace(reason))
	if err != nil {
		return nil, nil, fmt.Errorf("unlink timeline document: %w", err)
	}
	return e, ev, nil
}

// entryContent is the operator-editable content shared by create and update.
type entryContent struct {
	// Type must be operator-creatable.
	Type EntryType
	// Title is trimmed, at most MaxTitleLength code points.
	Title string
	// Body is trimmed, required, at most MaxBodyLength code points.
	Body string
	// Visibility defaults to VisibilityCaseParticipants.
	Visibility Visibility
	// OccurredAt, when set, may not lie in the future.
	OccurredAt *time.Time
}

// normalizeContent trims and checks the editable content of an entry.
func (s *Service) normalizeContent(c entryContent) (entryContent, error) {
	switch {
	case c.Type == TypeSystem:
		return c, fmt.Errorf("%w: SYSTEM entries are written by the server only", core.ErrInvalidInput)
	case c.Type == TypeAIProposal:
		return c, fmt.Errorf("%w: AI_PROPOSAL entries are reserved for the AI component", core.ErrInvalidInput)
	case !c.Type.OperatorCreatable():
		return c, fmt.Errorf("%w: unknown entry type %d", core.ErrInvalidInput, c.Type)
	}
	c.Title = strings.TrimSpace(c.Title)
	c.Body = strings.TrimSpace(c.Body)
	switch {
	case c.Body == "":
		return c, fmt.Errorf("%w: body is required", core.ErrInvalidInput)
	case utf8.RuneCountInString(c.Body) > MaxBodyLength:
		return c, fmt.Errorf("%w: body exceeds %d characters", core.ErrInvalidInput, MaxBodyLength)
	case utf8.RuneCountInString(c.Title) > MaxTitleLength:
		return c, fmt.Errorf("%w: title exceeds %d characters", core.ErrInvalidInput, MaxTitleLength)
	}
	if c.Visibility == VisibilityUnspecified {
		c.Visibility = VisibilityCaseParticipants
	}
	if !c.Visibility.Valid() {
		return c, fmt.Errorf("%w: unknown visibility %d", core.ErrInvalidInput, c.Visibility)
	}
	if c.OccurredAt != nil && c.OccurredAt.After(s.now().Add(futureTolerance)) {
		return c, fmt.Errorf("%w: occurred_at may not lie in the future", core.ErrInvalidInput)
	}
	return c, nil
}

// uniqueDocumentIDs drops duplicates and rejects nil ids or too many documents.
func uniqueDocumentIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) > MaxDocumentsPerRequest {
		return nil, fmt.Errorf("%w: at most %d documents per entry request", core.ErrInvalidInput, MaxDocumentsPerRequest)
	}
	return out, nil
}
