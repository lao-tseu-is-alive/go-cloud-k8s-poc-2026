package access

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Repository persists grants and groups; PostgresRepository implements it.
type Repository interface {
	MyAccess(ctx context.Context, userID string, subjectID uuid.UUID) (core.Access, error)
	ListGrants(ctx context.Context, operatorID string, subjectID uuid.UUID, includeRevoked bool) ([]*Grant, error)
	SetGrant(ctx context.Context, in SetGrantInput) (*Grant, *core.AuditEvent, error)
	RevokeGrant(ctx context.Context, grantID uuid.UUID, operatorID, reason string) (*Grant, *core.AuditEvent, error)
	ListGroups(ctx context.Context, query string, includeArchived bool) ([]*Group, error)
	GetGroup(ctx context.Context, operatorID string, id uuid.UUID) (*Group, []*Member, error)
	CreateGroup(ctx context.Context, in GroupInput) (*Group, *core.AuditEvent, error)
	UpdateGroup(ctx context.Context, id uuid.UUID, in GroupInput) (*Group, *core.AuditEvent, error)
	ArchiveGroup(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Group, *core.AuditEvent, error)
	AddMember(ctx context.Context, in MemberInput) (*Member, *core.AuditEvent, error)
	RemoveMember(ctx context.Context, in MemberInput) (*core.AuditEvent, error)
}

// Service validates and normalizes access operations.
type Service struct {
	repo Repository
	log  *slog.Logger
}

// NewService builds the access service. A nil logger falls back to slog.Default.
func NewService(repo Repository, log *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, errors.New("access service: repository is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, log: log}, nil
}

// MyAccess returns the caller's effective level on a subject.
func (s *Service) MyAccess(ctx context.Context, userID string, subjectID uuid.UUID) (core.Access, error) {
	return s.repo.MyAccess(ctx, userID, subjectID)
}

// ListGrants returns a subject's grants (READ on the subject).
func (s *Service) ListGrants(ctx context.Context, operatorID string, subjectID uuid.UUID, includeRevoked bool) ([]*Grant, error) {
	return s.repo.ListGrants(ctx, operatorID, subjectID, includeRevoked)
}

// SetGrant gives or changes a grant (FULL_CONTROL on the subject).
func (s *Service) SetGrant(ctx context.Context, in SetGrantInput) (*Grant, *core.AuditEvent, error) {
	in.GranteeID, in.Reason = strings.TrimSpace(in.GranteeID), strings.TrimSpace(in.Reason)
	switch {
	case in.GranteeKind != GranteeUser && in.GranteeKind != GranteeGroup && in.GranteeKind != GranteeOrgUnit:
		return nil, nil, fmt.Errorf("%w: unknown grantee kind %q", core.ErrInvalidInput, in.GranteeKind)
	case in.GranteeID == "":
		return nil, nil, fmt.Errorf("%w: grantee id is required", core.ErrInvalidInput)
	case !in.Level.Valid():
		return nil, nil, fmt.Errorf("%w: level must be READ to FULL_CONTROL", core.ErrInvalidInput)
	case in.Reason == "":
		return nil, nil, fmt.Errorf("%w: a reason is required", core.ErrInvalidInput)
	}
	return s.repo.SetGrant(ctx, in)
}

// RevokeGrant revokes a current grant (FULL_CONTROL on its subject).
func (s *Service) RevokeGrant(ctx context.Context, grantID uuid.UUID, operatorID, reason string) (*Grant, *core.AuditEvent, error) {
	if reason = strings.TrimSpace(reason); reason == "" {
		return nil, nil, fmt.Errorf("%w: a reason is required", core.ErrInvalidInput)
	}
	return s.repo.RevokeGrant(ctx, grantID, operatorID, reason)
}

// ListGroups returns the groups matching query.
func (s *Service) ListGroups(ctx context.Context, query string, includeArchived bool) ([]*Group, error) {
	return s.repo.ListGroups(ctx, strings.TrimSpace(query), includeArchived)
}

// GetGroup returns a group and its members (READ on the group).
func (s *Service) GetGroup(ctx context.Context, operatorID string, id uuid.UUID) (*Group, []*Member, error) {
	return s.repo.GetGroup(ctx, operatorID, id)
}

// CreateGroup creates a group; its creator gets FULL_CONTROL.
func (s *Service) CreateGroup(ctx context.Context, in GroupInput) (*Group, *core.AuditEvent, error) {
	in, err := in.normalize()
	if err != nil {
		return nil, nil, err
	}
	return s.repo.CreateGroup(ctx, in)
}

// UpdateGroup renames or redescribes a live group (MANAGE on it).
func (s *Service) UpdateGroup(ctx context.Context, id uuid.UUID, in GroupInput) (*Group, *core.AuditEvent, error) {
	in, err := in.normalize()
	if err != nil {
		return nil, nil, err
	}
	return s.repo.UpdateGroup(ctx, id, in)
}

// ArchiveGroup archives a group (FULL_CONTROL on it).
func (s *Service) ArchiveGroup(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Group, *core.AuditEvent, error) {
	if reason = strings.TrimSpace(reason); reason == "" {
		return nil, nil, fmt.Errorf("%w: a reason is required", core.ErrInvalidInput)
	}
	return s.repo.ArchiveGroup(ctx, id, operatorID, reason)
}

// AddMember adds a user to a group (MANAGE on it).
func (s *Service) AddMember(ctx context.Context, in MemberInput) (*Member, *core.AuditEvent, error) {
	if in.UserID = strings.TrimSpace(in.UserID); in.UserID == "" {
		return nil, nil, fmt.Errorf("%w: user id is required", core.ErrInvalidInput)
	}
	return s.repo.AddMember(ctx, in)
}

// RemoveMember removes a member (MANAGE on the group).
func (s *Service) RemoveMember(ctx context.Context, in MemberInput) (*core.AuditEvent, error) {
	in.UserID, in.Reason = strings.TrimSpace(in.UserID), strings.TrimSpace(in.Reason)
	return s.repo.RemoveMember(ctx, in)
}

// normalize trims a group input and requires a name.
func (in GroupInput) normalize() (GroupInput, error) {
	in.Name, in.Description, in.Reason = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description), strings.TrimSpace(in.Reason)
	if in.Name == "" {
		return in, fmt.Errorf("%w: a group name is required", core.ErrInvalidInput)
	}
	return in, nil
}
