package access

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ConnectServer exposes Service through the generated AccessService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	goelandv1connect.UnimplementedAccessServiceHandler
}

// NewConnectServer builds an AccessService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("access service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log}, nil
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "access", err)
}

// caller returns the authenticated operator id for scope.
func caller(ctx context.Context, scope string) (string, error) {
	user, err := core.RequireCaller(ctx, scope)
	if err != nil {
		return "", err
	}
	return core.OperatorID(user), nil
}

// GetMyAccess returns the caller's effective level on a subject.
func (s *ConnectServer) GetMyAccess(ctx context.Context, req *connect.Request[goelandv1.GetMyAccessRequest]) (*connect.Response[goelandv1.GetMyAccessResponse], error) {
	operator, err := caller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.SubjectId)
	if err != nil {
		return nil, err
	}
	a, err := s.service.MyAccess(ctx, operator, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.GetMyAccessResponse{Access: AccessToProto(a)}), nil
}

// ListGrants lists a subject's grants.
func (s *ConnectServer) ListGrants(ctx context.Context, req *connect.Request[goelandv1.ListGrantsRequest]) (*connect.Response[goelandv1.ListGrantsResponse], error) {
	operator, err := caller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.SubjectId)
	if err != nil {
		return nil, err
	}
	grants, err := s.service.ListGrants(ctx, operator, id, req.Msg.IncludeRevoked)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.AccessGrant, len(grants))
	for i, g := range grants {
		out[i] = GrantToProto(g)
	}
	return connect.NewResponse(&goelandv1.ListGrantsResponse{Grants: out}), nil
}

// SetGrant gives or changes a grant.
func (s *ConnectServer) SetGrant(ctx context.Context, req *connect.Request[goelandv1.SetGrantRequest]) (*connect.Response[goelandv1.SetGrantResponse], error) {
	operator, err := caller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.SubjectId)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	g, ev, err := s.service.SetGrant(ctx, SetGrantInput{
		SubjectID: id, GranteeKind: GranteeKindFromProto(m.GranteeKind), GranteeID: m.GranteeId,
		Level: LevelFromProto(m.Level), Reason: m.Reason, OperatorID: operator,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.SetGrantResponse{Grant: GrantToProto(g), AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}

// RevokeGrant revokes a grant.
func (s *ConnectServer) RevokeGrant(ctx context.Context, req *connect.Request[goelandv1.RevokeGrantRequest]) (*connect.Response[goelandv1.RevokeGrantResponse], error) {
	operator, err := caller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.GrantId)
	if err != nil {
		return nil, err
	}
	g, ev, err := s.service.RevokeGrant(ctx, id, operator, req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.RevokeGrantResponse{Grant: GrantToProto(g), AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}

// ListGroups lists the security groups.
func (s *ConnectServer) ListGroups(ctx context.Context, req *connect.Request[goelandv1.ListGroupsRequest]) (*connect.Response[goelandv1.ListGroupsResponse], error) {
	operator, err := caller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	groups, err := s.service.ListGroups(ctx, operator, req.Msg.Query, req.Msg.IncludeArchived)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.SecurityGroup, len(groups))
	for i, g := range groups {
		out[i] = GroupToProto(g)
	}
	return connect.NewResponse(&goelandv1.ListGroupsResponse{Groups: out}), nil
}

// GetGroup reads a group and its members.
func (s *ConnectServer) GetGroup(ctx context.Context, req *connect.Request[goelandv1.GetGroupRequest]) (*connect.Response[goelandv1.GetGroupResponse], error) {
	operator, err := caller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	group, members, err := s.service.GetGroup(ctx, operator, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.GroupMember, len(members))
	for i, m := range members {
		out[i] = MemberToProto(m)
	}
	return connect.NewResponse(&goelandv1.GetGroupResponse{Group: GroupToProto(group), Members: out}), nil
}

// CreateGroup creates a security group.
func (s *ConnectServer) CreateGroup(ctx context.Context, req *connect.Request[goelandv1.CreateGroupRequest]) (*connect.Response[goelandv1.CreateGroupResponse], error) {
	operator, err := caller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	g, ev, err := s.service.CreateGroup(ctx, GroupInput{Name: req.Msg.Name, Description: req.Msg.Description, OperatorID: operator})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateGroupResponse{Group: GroupToProto(g), AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}

// UpdateGroup renames or redescribes a group.
func (s *ConnectServer) UpdateGroup(ctx context.Context, req *connect.Request[goelandv1.UpdateGroupRequest]) (*connect.Response[goelandv1.UpdateGroupResponse], error) {
	operator, err := caller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	g, ev, err := s.service.UpdateGroup(ctx, id, GroupInput{Name: m.Name, Description: m.Description, Reason: m.Reason, OperatorID: operator})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateGroupResponse{Group: GroupToProto(g), AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}

// ArchiveGroup archives a group.
func (s *ConnectServer) ArchiveGroup(ctx context.Context, req *connect.Request[goelandv1.ArchiveGroupRequest]) (*connect.Response[goelandv1.ArchiveGroupResponse], error) {
	operator, err := caller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	g, ev, err := s.service.ArchiveGroup(ctx, id, operator, req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ArchiveGroupResponse{Group: GroupToProto(g), AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}

// AddGroupMember adds a user to a group.
func (s *ConnectServer) AddGroupMember(ctx context.Context, req *connect.Request[goelandv1.AddGroupMemberRequest]) (*connect.Response[goelandv1.AddGroupMemberResponse], error) {
	operator, err := caller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.GroupId)
	if err != nil {
		return nil, err
	}
	m, ev, err := s.service.AddMember(ctx, MemberInput{GroupID: id, UserID: req.Msg.UserId, OperatorID: operator})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.AddGroupMemberResponse{Member: MemberToProto(m), AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}

// RemoveGroupMember removes a member from a group.
func (s *ConnectServer) RemoveGroupMember(ctx context.Context, req *connect.Request[goelandv1.RemoveGroupMemberRequest]) (*connect.Response[goelandv1.RemoveGroupMemberResponse], error) {
	operator, err := caller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.GroupId)
	if err != nil {
		return nil, err
	}
	ev, err := s.service.RemoveMember(ctx, MemberInput{GroupID: id, UserID: req.Msg.UserId, Reason: req.Msg.Reason, OperatorID: operator})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.RemoveGroupMemberResponse{AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}
