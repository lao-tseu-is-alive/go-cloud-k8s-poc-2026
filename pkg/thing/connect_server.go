package thing

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ConnectServer exposes Service through the generated ThingService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	goelandv1connect.UnimplementedThingServiceHandler
}

// NewConnectServer builds a ThingService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("thing service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log}, nil
}

// CreateThing registers a thing.
func (s *ConnectServer) CreateThing(ctx context.Context, req *connect.Request[goelandv1.CreateThingRequest]) (*connect.Response[goelandv1.CreateThingResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	in := CreateInput{
		TypeCode: msg.ThingTypeCode, Name: msg.Name, Description: msg.Description, ExternalRef: msg.ExternalRef,
		GeometryGeoJSON: msg.GeometryGeojson, Parcel: parcelFromProto(msg.GetParcel()), Building: buildingFromProto(msg.GetBuilding()),
		Metadata: core.StructToMap(msg.Metadata), OperatorID: core.OperatorID(user),
	}
	core.ApplyInitialGovernance(&in.Governance, msg.InitialGovernance)
	t, ev, err := s.service.Create(ctx, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateThingResponse{Thing: DomainToProto(t), CreatedEvent: core.DomainAuditEventToProto(ev)}), nil
}

// GetThing retrieves a thing with optional relationships (both directions) and audit.
func (s *ConnectServer) GetThing(ctx context.Context, req *connect.Request[goelandv1.GetThingRequest]) (*connect.Response[goelandv1.GetThingResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	t, err := s.service.Get(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.GetThingResponse{Thing: DomainToProto(t)}
	if req.Msg.IncludeRelationships {
		rels, err := s.service.Relationships(ctx, id)
		if err != nil {
			return nil, s.mapError(err)
		}
		resp.Relationships = core.DomainRelationshipsToProto(rels)
	}
	if req.Msg.IncludeAudit {
		events, err := s.service.RecentAudit(ctx, id)
		if err != nil {
			return nil, s.mapError(err)
		}
		resp.RecentAudit = core.DomainAuditEventsToProto(events)
	}
	return connect.NewResponse(resp), nil
}

// UpdateThing replaces the editable fields of a thing.
func (s *ConnectServer) UpdateThing(ctx context.Context, req *connect.Request[goelandv1.UpdateThingRequest]) (*connect.Response[goelandv1.UpdateThingResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	t, ev, err := s.service.Update(ctx, id, UpdateInput{
		Name: msg.Name, Description: msg.Description, ExternalRef: msg.ExternalRef, GeometryGeoJSON: msg.GeometryGeojson,
		Parcel: parcelFromProto(msg.GetParcel()), Building: buildingFromProto(msg.GetBuilding()),
		Metadata: core.StructToMap(msg.Metadata), OperatorID: core.OperatorID(user), Reason: msg.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateThingResponse{Thing: DomainToProto(t), UpdateEvent: core.DomainAuditEventToProto(ev)}), nil
}

// SearchThings runs the filtered search.
func (s *ConnectServer) SearchThings(ctx context.Context, req *connect.Request[goelandv1.SearchThingsRequest]) (*connect.Response[goelandv1.SearchThingsResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	bbox, err := ParseBBox(req.Msg.Bbox)
	if err != nil {
		return nil, s.mapError(err)
	}
	result, err := s.service.Search(ctx, SearchFilter{
		Query: req.Msg.Query, TypeCode: req.Msg.ThingTypeCode, BBox: bbox, IncludeDeleted: req.Msg.IncludeDeleted,
		Limit: int(req.Msg.PageSize), Offset: offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.Thing, len(result.Things))
	for i, t := range result.Things {
		out[i] = DomainToProto(t)
	}
	return connect.NewResponse(&goelandv1.SearchThingsResponse{
		Things: out, NextPageToken: core.NextPageToken(offset, len(out), result.TotalSize), TotalSize: result.TotalSize,
	}), nil
}

// DeleteThing logically deletes a thing.
func (s *ConnectServer) DeleteThing(ctx context.Context, req *connect.Request[goelandv1.DeleteThingRequest]) (*connect.Response[goelandv1.DeleteThingResponse], error) {
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
	return connect.NewResponse(&goelandv1.DeleteThingResponse{DeletedThingId: id.String(), DeleteEvent: core.DomainAuditEventToProto(ev)}), nil
}

// ListThingTypes returns the thing type catalogue.
func (s *ConnectServer) ListThingTypes(ctx context.Context, req *connect.Request[goelandv1.ListThingTypesRequest]) (*connect.Response[goelandv1.ListThingTypesResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	types, err := s.service.ListTypes(ctx, req.Msg.OnlyActive)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.ThingType, 0, len(types))
	for _, t := range types {
		out = append(out, TypeToProto(t))
	}
	return connect.NewResponse(&goelandv1.ListThingTypesResponse{ThingTypes: out}), nil
}

// CreateThingType adds a generic thing type (administrators only).
func (s *ConnectServer) CreateThingType(ctx context.Context, req *connect.Request[goelandv1.CreateThingTypeRequest]) (*connect.Response[goelandv1.CreateThingTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.CreateThingType(ctx, ThingTypeInput{Code: m.Code, Label: m.Label, Description: m.Description, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateThingTypeResponse{ThingType: TypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// UpdateThingType changes a thing type (administrators only).
func (s *ConnectServer) UpdateThingType(ctx context.Context, req *connect.Request[goelandv1.UpdateThingTypeRequest]) (*connect.Response[goelandv1.UpdateThingTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.UpdateThingType(ctx, m.Code, ThingTypeUpdate{Label: m.Label, Description: m.Description, IsActive: m.IsActive, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateThingTypeResponse{ThingType: TypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "thing", err)
}
