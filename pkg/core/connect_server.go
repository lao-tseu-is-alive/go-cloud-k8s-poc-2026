package core

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
)

// ConnectServer exposes Service through the generated CoreService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	goelandv1connect.UnimplementedCoreServiceHandler
}

// NewConnectServer builds a CoreService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("core service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log}, nil
}

// CreateSubjectRef creates a subject identity + governance record.
func (s *ConnectServer) CreateSubjectRef(ctx context.Context, req *connect.Request[goelandv1.CreateSubjectRefRequest]) (*connect.Response[goelandv1.CreateSubjectRefResponse], error) {
	user, err := RequireCaller(ctx, ScopeWrite)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	in := CreateSubjectInput{
		Kind:         SubjectKindFromProto(msg.Kind),
		DisplayLabel: msg.DisplayLabel,
		CanonicalURL: msg.CanonicalUrl,
		OperatorID:   OperatorID(user),
	}
	if gov := msg.InitialMetadata; gov != nil {
		in.OwnerUserID = gov.OwnerUserId
		in.OwnerOrgID = gov.OwnerOrgId
		in.ConfidentialityLevel = gov.ConfidentialityLevel
		in.RetentionUntil = gov.RetentionUntil
		in.SortFinal = gov.SortFinal
		in.Metadata = gov.Metadata
	}
	if in.OwnerUserID == "" {
		in.OwnerUserID = in.OperatorID
	}
	ref, md, ev, err := s.service.CreateSubjectRef(ctx, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateSubjectRefResponse{
		SubjectRef:   DomainSubjectRefToProto(ref),
		Metadata:     DomainRecordMetadataToProto(md),
		CreatedEvent: DomainAuditEventToProto(ev),
	}), nil
}

// GetSubjectRef retrieves a subject with optional governance + audit info.
func (s *ConnectServer) GetSubjectRef(ctx context.Context, req *connect.Request[goelandv1.GetSubjectRefRequest]) (*connect.Response[goelandv1.GetSubjectRefResponse], error) {
	if _, err := RequireCaller(ctx, ScopeRead); err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	ref, md, audit, err := s.service.GetSubjectRef(ctx, id, req.Msg.IncludeMetadata, req.Msg.IncludeAuditSummary)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.GetSubjectRefResponse{
		SubjectRef:  DomainSubjectRefToProto(ref),
		Metadata:    DomainRecordMetadataToProto(md),
		RecentAudit: DomainAuditEventsToProto(audit),
	}), nil
}

// LinkSubjects creates a typed, validated relationship.
func (s *ConnectServer) LinkSubjects(ctx context.Context, req *connect.Request[goelandv1.LinkSubjectsRequest]) (*connect.Response[goelandv1.LinkSubjectsResponse], error) {
	user, err := RequireCaller(ctx, ScopeWrite)
	if err != nil {
		return nil, err
	}
	sourceID, err := parseUUID(req.Msg.SourceSubjectId)
	if err != nil {
		return nil, err
	}
	targetID, err := parseUUID(req.Msg.TargetSubjectId)
	if err != nil {
		return nil, err
	}
	rel, ev, err := s.service.LinkSubjects(ctx, LinkInput{
		SourceSubjectID:      sourceID,
		TargetSubjectID:      targetID,
		RelationshipTypeCode: req.Msg.RelationshipTypeCode,
		RoleDetail:           req.Msg.RoleDetail,
		OperatorID:           OperatorID(user),
		ValidFrom:            TimePtrFromProto(req.Msg.ValidFrom),
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.LinkSubjectsResponse{
		Relationship: DomainRelationshipToProto(rel),
		AuditEvent:   DomainAuditEventToProto(ev),
	}), nil
}

// UnlinkSubjects soft-deletes an existing relationship.
func (s *ConnectServer) UnlinkSubjects(ctx context.Context, req *connect.Request[goelandv1.UnlinkSubjectsRequest]) (*connect.Response[goelandv1.UnlinkSubjectsResponse], error) {
	user, err := RequireCaller(ctx, ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.RelationshipId)
	if err != nil {
		return nil, err
	}
	_, ev, err := s.service.UnlinkSubjects(ctx, id, OperatorID(user), req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UnlinkSubjectsResponse{
		RelationshipId: id.String(),
		AuditEvent:     DomainAuditEventToProto(ev),
	}), nil
}

// ListRelationships lists outgoing or incoming relationships for a subject.
func (s *ConnectServer) ListRelationships(ctx context.Context, req *connect.Request[goelandv1.ListRelationshipsRequest]) (*connect.Response[goelandv1.ListRelationshipsResponse], error) {
	if _, err := RequireCaller(ctx, ScopeRead); err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.SubjectId)
	if err != nil {
		return nil, err
	}
	offset, err := ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := s.service.ListRelationships(ctx, RelationshipFilter{
		SubjectID:            id,
		Outgoing:             req.Msg.Outgoing,
		RelationshipTypeCode: req.Msg.RelationshipTypeCode,
		Limit:                int(req.Msg.PageSize),
		Offset:               offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ListRelationshipsResponse{
		Relationships: DomainRelationshipsToProto(result.Relationships),
		NextPageToken: NextPageToken(offset, len(result.Relationships), result.TotalSize),
		TotalSize:     result.TotalSize,
	}), nil
}

// ListRelationshipTypes lists the catalogue of allowed relationship types.
func (s *ConnectServer) ListRelationshipTypes(ctx context.Context, req *connect.Request[goelandv1.ListRelationshipTypesRequest]) (*connect.Response[goelandv1.ListRelationshipTypesResponse], error) {
	if _, err := RequireCaller(ctx, ScopeRead); err != nil {
		return nil, err
	}
	types, err := s.service.ListRelationshipTypes(ctx, req.Msg.OnlyActive,
		SubjectKindFromProto(req.Msg.SourceKind), SubjectKindFromProto(req.Msg.TargetKind))
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.RelationshipType, 0, len(types))
	for _, t := range types {
		out = append(out, DomainRelationshipTypeToProto(t))
	}
	return connect.NewResponse(&goelandv1.ListRelationshipTypesResponse{RelationshipTypes: out}), nil
}

// ListAuditEvents returns the probative history for a subject.
func (s *ConnectServer) ListAuditEvents(ctx context.Context, req *connect.Request[goelandv1.ListAuditEventsRequest]) (*connect.Response[goelandv1.ListAuditEventsResponse], error) {
	if _, err := RequireCaller(ctx, ScopeRead); err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.SubjectId)
	if err != nil {
		return nil, err
	}
	offset, err := ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := s.service.ListAuditEvents(ctx, AuditFilter{
		SubjectID: id,
		EventType: req.Msg.EventType,
		From:      TimePtrFromProto(req.Msg.From),
		To:        TimePtrFromProto(req.Msg.To),
		Limit:     int(req.Msg.PageSize),
		Offset:    offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ListAuditEventsResponse{
		Events:        DomainAuditEventsToProto(result.Events),
		NextPageToken: NextPageToken(offset, len(result.Events), result.TotalSize),
		TotalSize:     result.TotalSize,
	}), nil
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	if mapped := MapError(err); mapped != nil {
		return mapped
	}
	s.log.Error("core request failed", "error", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// parseUUID parses a UUID request field, returning a Connect InvalidArgument error on failure.
func parseUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	return id, nil
}
