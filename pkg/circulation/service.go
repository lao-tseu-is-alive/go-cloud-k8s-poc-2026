package circulation

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
)

const (
	// MaxTitleLength is the maximum number of code points in a circulation title.
	MaxTitleLength = 500
	// MaxTextLength is the maximum number of code points in a message or answer.
	MaxTextLength = 4000
	// MaxRecipients bounds the recipients of one circulation.
	MaxRecipients = 50
)

// Service contains the transport-independent circulation business logic.
type Service struct {
	repo Repository
	log  *slog.Logger
}

// NewService constructs a Service backed by the circulation repository. A nil
// logger falls back to slog.Default.
func NewService(repo Repository, log *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, log: log}, nil
}

// Create validates and sends an open case to its recipients.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Circulation, *core.AuditEvent, error) {
	if in.CaseID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Message = strings.TrimSpace(in.Message)
	switch {
	case in.Title == "":
		return nil, nil, fmt.Errorf("%w: title is required", core.ErrInvalidInput)
	case utf8.RuneCountInString(in.Title) > MaxTitleLength:
		return nil, nil, fmt.Errorf("%w: title exceeds %d characters", core.ErrInvalidInput, MaxTitleLength)
	case utf8.RuneCountInString(in.Message) > MaxTextLength:
		return nil, nil, fmt.Errorf("%w: message exceeds %d characters", core.ErrInvalidInput, MaxTextLength)
	}
	recipients, err := NormalizeRecipients(in.Recipients)
	if err != nil {
		return nil, nil, err
	}
	in.Recipients = recipients
	c, ev, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("create circulation: %w", err)
	}
	s.log.Info("created circulation", "case_id", in.CaseID, "circulation_id", c.ID, "recipients", len(recipients))
	return c, ev, nil
}

// Get loads a circulation with its recipients.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Circulation, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: circulation id is required", core.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// ListCase returns the circulations of a case, newest first.
func (s *Service) ListCase(ctx context.Context, caseID uuid.UUID) ([]*Circulation, error) {
	if caseID == uuid.Nil {
		return nil, fmt.Errorf("%w: case id is required", core.ErrInvalidInput)
	}
	return s.repo.ListCase(ctx, caseID)
}

// Respond validates and records the answer of a recipient. Until GLD-017 any
// writer may record it (e.g. an answer received by mail); the operator is kept.
func (s *Service) Respond(ctx context.Context, in RespondInput) (*Circulation, *core.AuditEvent, error) {
	if in.RecipientID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: recipient id is required", core.ErrInvalidInput)
	}
	in.Text = strings.TrimSpace(in.Text)
	switch {
	case !in.Response.Valid():
		return nil, nil, fmt.Errorf("%w: unknown response %d", core.ErrInvalidInput, in.Response)
	case in.Response.NeedsText() && in.Text == "":
		return nil, nil, fmt.Errorf("%w: a %s answer needs a text", core.ErrInvalidInput, in.Response)
	case utf8.RuneCountInString(in.Text) > MaxTextLength:
		return nil, nil, fmt.Errorf("%w: text exceeds %d characters", core.ErrInvalidInput, MaxTextLength)
	}
	c, ev, err := s.repo.Respond(ctx, in)
	if err != nil {
		return nil, nil, fmt.Errorf("respond to circulation: %w", err)
	}
	s.log.Info("recorded circulation response", "circulation_id", c.ID, "response", in.Response.String())
	return c, ev, nil
}

// Cancel stops an open circulation; a reason is required.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Circulation, *core.AuditEvent, error) {
	if id == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: circulation id is required", core.ErrInvalidInput)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, nil, fmt.Errorf("%w: a reason is required to cancel a circulation", core.ErrInvalidInput)
	}
	c, ev, err := s.repo.Cancel(ctx, id, operatorID, reason)
	if err != nil {
		return nil, nil, fmt.Errorf("cancel circulation: %w", err)
	}
	return c, ev, nil
}

// NormalizeRecipients checks the recipients (1 to MaxRecipients, exactly one
// user or unit each, no one twice) and renumbers their steps from 1 in order:
// steps 0 and 1 are the first, and 1, 3, 3 becomes 1, 2, 2.
func NormalizeRecipients(in []RecipientInput) ([]RecipientInput, error) {
	if len(in) == 0 || len(in) > MaxRecipients {
		return nil, fmt.Errorf("%w: a circulation needs 1 to %d recipients", core.ErrInvalidInput, MaxRecipients)
	}
	seen := map[string]bool{}
	steps := make([]int32, 0, len(in))
	out := make([]RecipientInput, 0, len(in))
	for _, rec := range in {
		rec.Assignee = task.NormalizeAssignee(rec.Assignee)
		key, err := recipientKey(rec.Assignee)
		if err != nil {
			return nil, err
		}
		if seen[key] {
			return nil, fmt.Errorf("%w: a recipient appears twice", core.ErrInvalidInput)
		}
		seen[key] = true
		rec.Step = max(rec.Step, 1)
		if !slices.Contains(steps, rec.Step) {
			steps = append(steps, rec.Step)
		}
		out = append(out, rec)
	}
	slices.Sort(steps)
	for i := range out {
		out[i].Step = int32(slices.Index(steps, out[i].Step) + 1)
	}
	return out, nil
}

// recipientKey identifies a recipient, which must be exactly one user or unit.
func recipientKey(a task.Assignee) (string, error) {
	switch {
	case a.UserID != nil && a.OrgUnitID == nil:
		return "user:" + *a.UserID, nil
	case a.OrgUnitID != nil && a.UserID == nil:
		return "unit:" + a.OrgUnitID.String(), nil
	}
	return "", fmt.Errorf("%w: each recipient is exactly one user or one org unit", core.ErrInvalidInput)
}
