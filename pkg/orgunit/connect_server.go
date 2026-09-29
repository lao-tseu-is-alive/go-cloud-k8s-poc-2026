package orgunit

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ConnectServer exposes Service through the generated OrgUnitService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	goelandv1connect.UnimplementedOrgUnitServiceHandler
}

// NewConnectServer builds an OrgUnitService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("org unit service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log}, nil
}

// ListOrgUnits returns the whole tree as a flat list.
func (s *ConnectServer) ListOrgUnits(ctx context.Context, req *connect.Request[goelandv1.ListOrgUnitsRequest]) (*connect.Response[goelandv1.ListOrgUnitsResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	nodes, err := s.service.List(ctx, req.Msg.IncludeDissolved)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ListOrgUnitsResponse{Units: NodesToProto(nodes)}), nil
}

// SearchOrgUnits runs the filtered unit search.
func (s *ConnectServer) SearchOrgUnits(ctx context.Context, req *connect.Request[goelandv1.SearchOrgUnitsRequest]) (*connect.Response[goelandv1.SearchOrgUnitsResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := s.service.Search(ctx, SearchFilter{
		Query:            req.Msg.Query,
		IncludeDissolved: req.Msg.IncludeDissolved,
		Limit:            int(req.Msg.PageSize),
		Offset:           offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.SearchOrgUnitsResponse{
		Units:         DomainsToProto(result.Units),
		NextPageToken: core.NextPageToken(offset, len(result.Units), result.TotalSize),
		TotalSize:     result.TotalSize,
	}), nil
}

// GetOrgUnit retrieves a unit with its place in the tree and optional relationships and audit.
func (s *ConnectServer) GetOrgUnit(ctx context.Context, req *connect.Request[goelandv1.GetOrgUnitRequest]) (*connect.Response[goelandv1.GetOrgUnitResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	detail, err := s.service.Get(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.GetOrgUnitResponse{
		OrgUnit:   DomainToProto(detail.Unit),
		Ancestors: NodesToProto(detail.Ancestors),
		Children:  NodesToProto(detail.Children),
	}
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

// CreateOrgUnit adds a unit (administrators only).
func (s *ConnectServer) CreateOrgUnit(ctx context.Context, req *connect.Request[goelandv1.CreateOrgUnitRequest]) (*connect.Response[goelandv1.CreateOrgUnitResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	parentID, err := core.OptionalUUID(m.ParentId)
	if err != nil {
		return nil, err
	}
	u, ev, err := s.service.Create(ctx, CreateInput{
		ExternalRef: m.ExternalRef,
		Input: Input{
			TypeCode: m.OrgUnitTypeCode, Abbreviation: m.Abbreviation, Label: m.Label, Description: m.Description, Email: m.Email,
			ParentID: parentID, OperatorID: core.OperatorID(user), Reason: m.Reason,
		},
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateOrgUnitResponse{OrgUnit: DomainToProto(u), CreatedEvent: core.DomainAuditEventToProto(ev)}), nil
}

// UpdateOrgUnit replaces the editable fields of a live unit (administrators only).
func (s *ConnectServer) UpdateOrgUnit(ctx context.Context, req *connect.Request[goelandv1.UpdateOrgUnitRequest]) (*connect.Response[goelandv1.UpdateOrgUnitResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	id, err := core.ParseUUID(m.Id)
	if err != nil {
		return nil, err
	}
	parentID, err := core.OptionalUUID(m.ParentId)
	if err != nil {
		return nil, err
	}
	u, ev, err := s.service.Update(ctx, id, Input{
		TypeCode: m.OrgUnitTypeCode, Abbreviation: m.Abbreviation, Label: m.Label, Description: m.Description, Email: m.Email,
		ParentID: parentID, OperatorID: core.OperatorID(user), Reason: m.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateOrgUnitResponse{OrgUnit: DomainToProto(u), UpdateEvent: core.DomainAuditEventToProto(ev)}), nil
}

// DissolveOrgUnit dissolves a unit (administrators only).
func (s *ConnectServer) DissolveOrgUnit(ctx context.Context, req *connect.Request[goelandv1.DissolveOrgUnitRequest]) (*connect.Response[goelandv1.DissolveOrgUnitResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	u, ev, err := s.service.Dissolve(ctx, id, core.OperatorID(user), req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.DissolveOrgUnitResponse{OrgUnit: DomainToProto(u), DissolveEvent: core.DomainAuditEventToProto(ev)}), nil
}

// ListOrgUnitTypes returns the unit type catalogue.
func (s *ConnectServer) ListOrgUnitTypes(ctx context.Context, req *connect.Request[goelandv1.ListOrgUnitTypesRequest]) (*connect.Response[goelandv1.ListOrgUnitTypesResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	types, err := s.service.ListTypes(ctx, req.Msg.OnlyActive)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.OrgUnitType, 0, len(types))
	for _, t := range types {
		out = append(out, TypeToProto(t))
	}
	return connect.NewResponse(&goelandv1.ListOrgUnitTypesResponse{OrgUnitTypes: out}), nil
}

// CreateOrgUnitType adds a unit type (administrators only).
func (s *ConnectServer) CreateOrgUnitType(ctx context.Context, req *connect.Request[goelandv1.CreateOrgUnitTypeRequest]) (*connect.Response[goelandv1.CreateOrgUnitTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.CreateType(ctx, TypeInput{Code: m.Code, Label: m.Label, Description: m.Description, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateOrgUnitTypeResponse{OrgUnitType: TypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// UpdateOrgUnitType changes a unit type (administrators only).
func (s *ConnectServer) UpdateOrgUnitType(ctx context.Context, req *connect.Request[goelandv1.UpdateOrgUnitTypeRequest]) (*connect.Response[goelandv1.UpdateOrgUnitTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.UpdateType(ctx, m.Code, TypeUpdate{Label: m.Label, Description: m.Description, IsActive: m.IsActive, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateOrgUnitTypeResponse{OrgUnitType: TypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "org unit", err)
}
