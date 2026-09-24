package casefile

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ConnectServer exposes Service through the generated CaseService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	goelandv1connect.UnimplementedCaseServiceHandler
}

// NewConnectServer builds a CaseService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("case service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log}, nil
}

// CreateCase opens a new case.
func (s *ConnectServer) CreateCase(ctx context.Context, req *connect.Request[goelandv1.CreateCaseRequest]) (*connect.Response[goelandv1.CreateCaseResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	in := CreateInput{
		CaseTypeCode: msg.CaseTypeCode,
		Title:        msg.Title,
		Description:  msg.Description,
		Metadata:     core.StructToMap(msg.Metadata),
		BusinessRef:  core.BusinessRefRequestFromProto(msg.BusinessRef),
		OperatorID:   core.OperatorID(user),
	}
	if gov := msg.InitialGovernance; gov != nil {
		in.Governance.OwnerUserID = gov.OwnerUserId
		in.Governance.OwnerOrgID = gov.OwnerOrgId
		in.Governance.ConfidentialityLevel = gov.ConfidentialityLevel
		in.Governance.RetentionUntil = gov.RetentionUntil
		in.Governance.SortFinal = gov.SortFinal
		in.Governance.Metadata = gov.Metadata
	}
	c, ev, err := s.service.Create(ctx, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateCaseResponse{Case: DomainToProto(c), CreatedEvent: core.DomainAuditEventToProto(ev)}), nil
}

// GetCase retrieves a case with optional relationships (both directions) and audit.
func (s *ConnectServer) GetCase(ctx context.Context, req *connect.Request[goelandv1.GetCaseRequest]) (*connect.Response[goelandv1.GetCaseResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	c, err := s.service.Get(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.GetCaseResponse{Case: DomainToProto(c)}
	if req.Msg.IncludeRelationships {
		rels, err := s.service.Relationships(ctx, id)
		if err != nil {
			return nil, s.mapError(err)
		}
		resp.Relationships = core.DomainRelationshipsToProto(rels)
	}
	if req.Msg.IncludeAudit {
		audit, err := s.service.RecentAudit(ctx, id)
		if err != nil {
			return nil, s.mapError(err)
		}
		resp.RecentAudit = core.DomainAuditEventsToProto(audit)
	}
	return connect.NewResponse(resp), nil
}

// UpdateCase replaces the editable metadata of a case.
func (s *ConnectServer) UpdateCase(ctx context.Context, req *connect.Request[goelandv1.UpdateCaseRequest]) (*connect.Response[goelandv1.UpdateCaseResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	c, ev, err := s.service.Update(ctx, id, UpdateInput{
		Title:       req.Msg.Title,
		Description: req.Msg.Description,
		Metadata:    core.StructToMap(req.Msg.Metadata),
		OperatorID:  core.OperatorID(user),
		Reason:      req.Msg.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateCaseResponse{Case: DomainToProto(c), UpdateEvent: core.DomainAuditEventToProto(ev)}), nil
}

// TransitionCase moves a case to another status.
func (s *ConnectServer) TransitionCase(ctx context.Context, req *connect.Request[goelandv1.TransitionCaseRequest]) (*connect.Response[goelandv1.TransitionCaseResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	c, ev, err := s.service.Transition(ctx, id, TransitionInput{
		Target:     Status(req.Msg.TargetStatus),
		OperatorID: core.OperatorID(user),
		Reason:     req.Msg.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.TransitionCaseResponse{Case: DomainToProto(c), TransitionEvent: core.DomainAuditEventToProto(ev)}), nil
}

// SearchCases runs the filtered case search.
func (s *ConnectServer) SearchCases(ctx context.Context, req *connect.Request[goelandv1.SearchCasesRequest]) (*connect.Response[goelandv1.SearchCasesResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := s.service.Search(ctx, SearchFilter{
		Query:          req.Msg.Query,
		CaseTypeCode:   req.Msg.CaseTypeCode,
		Status:         Status(req.Msg.Status),
		IncludeDeleted: req.Msg.IncludeDeleted,
		Limit:          int(req.Msg.PageSize),
		Offset:         offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.SearchCasesResponse{
		Cases:         DomainsToProto(result.Cases),
		NextPageToken: core.NextPageToken(offset, len(result.Cases), result.TotalSize),
		TotalSize:     result.TotalSize,
	}), nil
}

// DeleteCase logically deletes a case.
func (s *ConnectServer) DeleteCase(ctx context.Context, req *connect.Request[goelandv1.DeleteCaseRequest]) (*connect.Response[goelandv1.DeleteCaseResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	ev, err := s.service.SoftDelete(ctx, id, core.OperatorID(user), req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.DeleteCaseResponse{DeletedCaseId: id.String(), DeleteEvent: core.DomainAuditEventToProto(ev)}), nil
}

// ListCaseTypes returns the case type catalogue.
func (s *ConnectServer) ListCaseTypes(ctx context.Context, req *connect.Request[goelandv1.ListCaseTypesRequest]) (*connect.Response[goelandv1.ListCaseTypesResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	types, err := s.service.ListTypes(ctx, req.Msg.OnlyActive)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.CaseType, 0, len(types))
	for _, t := range types {
		out = append(out, TypeToProto(t))
	}
	return connect.NewResponse(&goelandv1.ListCaseTypesResponse{CaseTypes: out}), nil
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "case", err)
}

// CreateCaseType adds a case type (administrators only).
func (s *ConnectServer) CreateCaseType(ctx context.Context, req *connect.Request[goelandv1.CreateCaseTypeRequest]) (*connect.Response[goelandv1.CreateCaseTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.CreateCaseType(ctx, CaseTypeInput{Code: m.Code, Label: m.Label, Description: m.Description, BusinessRefNamespace: m.BusinessRefNamespace, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateCaseTypeResponse{CaseType: TypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// UpdateCaseType changes a case type (administrators only).
func (s *ConnectServer) UpdateCaseType(ctx context.Context, req *connect.Request[goelandv1.UpdateCaseTypeRequest]) (*connect.Response[goelandv1.UpdateCaseTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.UpdateCaseType(ctx, m.Code, CaseTypeUpdate{Label: m.Label, Description: m.Description, BusinessRefNamespace: m.BusinessRefNamespace, IsActive: m.IsActive, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateCaseTypeResponse{CaseType: TypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}
