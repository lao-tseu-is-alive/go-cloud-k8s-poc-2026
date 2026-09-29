package task

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
	// MaxTitleLength is the maximum number of code points in a task title.
	MaxTitleLength = 500
	// MaxDescriptionLength is the maximum number of code points in a description.
	MaxDescriptionLength = 4000
	// MaxNoteLength is the maximum number of code points in a note or reason.
	MaxNoteLength = 4000
)

// Service contains the transport-independent task business logic.
type Service struct {
	repo Repository
	log  *slog.Logger
}

// NewService constructs a Service backed by the task repository. A nil logger
// falls back to slog.Default.
func NewService(repo Repository, log *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, log: log}, nil
}

// Create validates and adds a manual task to an open case.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Task, *core.AuditEvent, error) {
	if in.CaseID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	var err error
	if in.Content, err = normalizeContent(in.Content); err != nil {
		return nil, nil, err
	}
	in.Assignee = NormalizeAssignee(in.Assignee)
	t, ev, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("create task: %w", err)
	}
	s.log.Info("created task", "case_id", in.CaseID, "task_id", t.ID)
	return t, ev, nil
}

// Get loads a task with its assignment history.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Task, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: task id is required", core.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// ListCase returns a page of the tasks of a case, pending ones first.
func (s *Service) ListCase(ctx context.Context, filter CaseFilter) (ListResult, error) {
	if filter.CaseID == uuid.Nil {
		return ListResult{}, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	var err error
	if filter.Limit, filter.Offset, err = page(filter.Limit, filter.Offset); err != nil {
		return ListResult{}, err
	}
	if err := checkStatuses(filter.Statuses); err != nil {
		return ListResult{}, err
	}
	return s.repo.ListCase(ctx, filter)
}

// ListMine returns a page of the caller's tasks, earliest deadline first;
// without statuses, the pending ones.
func (s *Service) ListMine(ctx context.Context, filter MineFilter) (ListResult, error) {
	if strings.TrimSpace(filter.UserID) == "" {
		return ListResult{}, core.ErrUnauthenticated
	}
	var err error
	if filter.Limit, filter.Offset, err = page(filter.Limit, filter.Offset); err != nil {
		return ListResult{}, err
	}
	if err := checkStatuses(filter.Statuses); err != nil {
		return ListResult{}, err
	}
	if len(filter.Statuses) == 0 {
		filter.Statuses = []Status{StatusOpen, StatusInProgress}
	}
	return s.repo.ListMine(ctx, filter)
}

// Update validates and replaces the content of a pending task.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Task, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: task id is required", core.ErrInvalidInput)
	}
	var err error
	if in.Content, err = normalizeContent(in.Content); err != nil {
		return nil, nil, err
	}
	in.Reason = strings.TrimSpace(in.Reason)
	t, ev, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return nil, nil, fmt.Errorf("update task: %w", err)
	}
	return t, ev, nil
}

// Assign (re)assigns a pending task to one user or unit, or unassigns it
// (zero Assignee); the change is kept in the history.
func (s *Service) Assign(ctx context.Context, id uuid.UUID, to Assignee, operatorID, reason string) (*Task, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: task id is required", core.ErrInvalidInput)
	}
	t, ev, err := s.repo.Assign(ctx, id, NormalizeAssignee(to), operatorID, strings.TrimSpace(reason))
	if err != nil {
		return nil, nil, fmt.Errorf("assign task: %w", err)
	}
	s.log.Info("assigned task", "task_id", id)
	return t, ev, nil
}

// ChangeStatus applies a lifecycle move. Cancelling and reopening need a
// reason; completing accepts an optional note.
func (s *Service) ChangeStatus(ctx context.Context, id uuid.UUID, move Move, operatorID, note string) (*Task, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: task id is required", core.ErrInvalidInput)
	}
	if _, known := moves[move]; !known {
		return nil, nil, fmt.Errorf("%w: unknown task move %d", core.ErrInvalidInput, move)
	}
	note = strings.TrimSpace(note)
	switch {
	case (move == MoveCancel || move == MoveReopen) && note == "":
		return nil, nil, fmt.Errorf("%w: a reason is required", core.ErrInvalidInput)
	case utf8.RuneCountInString(note) > MaxNoteLength:
		return nil, nil, fmt.Errorf("%w: note exceeds %d characters", core.ErrInvalidInput, MaxNoteLength)
	}
	t, ev, err := s.repo.ChangeStatus(ctx, id, move, operatorID, note)
	if err != nil {
		return nil, nil, fmt.Errorf("change task status: %w", err)
	}
	s.log.Info("changed task status", "task_id", id, "status", t.Status.String())
	return t, ev, nil
}

// ListTypes returns the task type catalogue.
func (s *Service) ListTypes(ctx context.Context, onlyActive bool) ([]*TaskType, error) {
	return s.repo.ListTypes(ctx, onlyActive)
}

// NormalizeAssignee trims the user id and treats blank or nil ids as absent.
func NormalizeAssignee(a Assignee) Assignee {
	if a.UserID != nil {
		if id := strings.TrimSpace(*a.UserID); id != "" {
			a.UserID = &id
		} else {
			a.UserID = nil
		}
	}
	if a.OrgUnitID != nil && *a.OrgUnitID == uuid.Nil {
		a.OrgUnitID = nil
	}
	return a
}

// normalizeContent trims and checks the editable content of a task.
func normalizeContent(c Content) (Content, error) {
	c.TypeCode = strings.TrimSpace(c.TypeCode)
	c.Title = strings.TrimSpace(c.Title)
	c.Description = strings.TrimSpace(c.Description)
	switch {
	case c.TypeCode == "":
		return c, fmt.Errorf("%w: task_type_code is required", core.ErrInvalidInput)
	case c.Title == "":
		return c, fmt.Errorf("%w: title is required", core.ErrInvalidInput)
	case utf8.RuneCountInString(c.Title) > MaxTitleLength:
		return c, fmt.Errorf("%w: title exceeds %d characters", core.ErrInvalidInput, MaxTitleLength)
	case utf8.RuneCountInString(c.Description) > MaxDescriptionLength:
		return c, fmt.Errorf("%w: description exceeds %d characters", core.ErrInvalidInput, MaxDescriptionLength)
	}
	return c, nil
}

// page normalizes a page size and offset.
func page(limit, offset int) (int, int, error) {
	limit, err := core.NormalizePageSize(limit)
	if err != nil {
		return 0, 0, err
	}
	return limit, max(offset, 0), nil
}

// checkStatuses rejects unknown status filters.
func checkStatuses(statuses []Status) error {
	for _, st := range statuses {
		if !st.Valid() {
			return fmt.Errorf("%w: unknown task status %d", core.ErrInvalidInput, st)
		}
	}
	return nil
}
