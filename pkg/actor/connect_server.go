package actor

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ConnectServer exposes Service through the generated ActorService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	goelandv1connect.UnimplementedActorServiceHandler
}

// NewConnectServer builds an ActorService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("actor service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log}, nil
}

// CreateActor registers a new person or organization actor.
func (s *ConnectServer) CreateActor(ctx context.Context, req *connect.Request[goelandv1.CreateActorRequest]) (*connect.Response[goelandv1.CreateActorResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	in := CreateInput{
		ActorKind:       Kind(msg.ActorKind),
		DisplayName:     msg.DisplayName,
		PublicationCode: msg.PublicationCode,
		Contacts:        protoContactsToDomain(msg.Contacts),
		OperatorID:      core.OperatorID(user),
	}
	if org := msg.GetOrganization(); org != nil {
		in.LegalName = org.LegalName
		in.CategoryCode = org.CategoryCode
		in.OrgComplement = org.Complement
	}
	if p := msg.GetPerson(); p != nil {
		in.IsCHRegister = p.IsChRegister
		in.CHRegisterRef = p.ChRegisterRef
	}
	if gov := msg.InitialGovernance; gov != nil {
		in.Governance.OwnerUserID = gov.OwnerUserId
		in.Governance.OwnerOrgID = gov.OwnerOrgId
		in.Governance.ConfidentialityLevel = gov.ConfidentialityLevel
		in.Governance.RetentionUntil = gov.RetentionUntil
		in.Governance.SortFinal = gov.SortFinal
		in.Governance.Metadata = gov.Metadata
	}
	act, ev, err := s.service.Create(ctx, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateActorResponse{
		Actor:        DomainToProto(act),
		CreatedEvent: core.DomainAuditEventToProto(ev),
	}), nil
}

// GetActor retrieves an actor with optional relationships + audit.
func (s *ConnectServer) GetActor(ctx context.Context, req *connect.Request[goelandv1.GetActorRequest]) (*connect.Response[goelandv1.GetActorResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	act, err := s.service.Get(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.GetActorResponse{Actor: DomainToProto(act)}
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

// UpdateActor applies a partial update (respects locking).
func (s *ConnectServer) UpdateActor(ctx context.Context, req *connect.Request[goelandv1.UpdateActorRequest]) (*connect.Response[goelandv1.UpdateActorResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := parseUUID(msg.Id)
	if err != nil {
		return nil, err
	}
	in := UpdateInput{
		IsActive:        msg.IsActive,
		PublicationCode: msg.PublicationCode,
		ReplaceContacts: msg.ReplaceContacts,
		Contacts:        protoContactsToDomain(msg.Contacts),
		OperatorID:      core.OperatorID(user),
		Reason:          msg.Reason,
	}
	// display_name is a plain string: a non-empty value signals an intended change.
	if msg.DisplayName != "" {
		name := msg.DisplayName
		in.DisplayName = &name
	}
	if org := msg.GetOrganization(); org != nil {
		legal, cat, compl := org.LegalName, org.CategoryCode, org.Complement
		in.LegalName, in.CategoryCode, in.OrgComplement = &legal, &cat, &compl
	}
	if p := msg.GetPerson(); p != nil {
		isCH, ref := p.IsChRegister, p.ChRegisterRef
		in.IsCHRegister, in.CHRegisterRef = &isCH, &ref
	}
	act, ev, err := s.service.Update(ctx, id, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateActorResponse{
		Actor:       DomainToProto(act),
		UpdateEvent: core.DomainAuditEventToProto(ev),
	}), nil
}

// SearchActors runs an accent-insensitive + filtered search.
func (s *ConnectServer) SearchActors(ctx context.Context, req *connect.Request[goelandv1.SearchActorsRequest]) (*connect.Response[goelandv1.SearchActorsResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := s.service.Search(ctx, SearchFilter{
		Query:               req.Msg.Query,
		ActorKind:           Kind(req.Msg.ActorKind),
		OrganizationCatCode: req.Msg.OrganizationCategoryCode,
		OnlyActive:          req.Msg.OnlyActive,
		IncludeDeleted:      req.Msg.IncludeDeleted,
		Limit:               int(req.Msg.PageSize),
		Offset:              offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.SearchActorsResponse{
		Actors:        DomainsToProto(result.Actors),
		NextPageToken: core.NextPageToken(offset, len(result.Actors), result.TotalSize),
		TotalSize:     result.TotalSize,
	}), nil
}

// DeleteActor logically deletes an actor.
func (s *ConnectServer) DeleteActor(ctx context.Context, req *connect.Request[goelandv1.DeleteActorRequest]) (*connect.Response[goelandv1.DeleteActorResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	ev, err := s.service.SoftDelete(ctx, id, core.OperatorID(user), req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.DeleteActorResponse{
		DeletedActorId: id.String(),
		DeleteEvent:    core.DomainAuditEventToProto(ev),
	}), nil
}

// ListOrganizationCategories returns the organization category catalogue.
func (s *ConnectServer) ListOrganizationCategories(ctx context.Context, req *connect.Request[goelandv1.ListOrganizationCategoriesRequest]) (*connect.Response[goelandv1.ListOrganizationCategoriesResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	cats, err := s.service.ListCategories(ctx, req.Msg.OnlyActive)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.OrganizationCategory, 0, len(cats))
	for _, c := range cats {
		out = append(out, DomainCategoryToProto(c))
	}
	return connect.NewResponse(&goelandv1.ListOrganizationCategoriesResponse{Categories: out}), nil
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	if mapped := core.MapError(err); mapped != nil {
		return mapped
	}
	s.log.Error("actor request failed", "error", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// parseUUID parses a required UUID field, returning a Connect InvalidArgument error on failure.
func parseUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	return id, nil
}
