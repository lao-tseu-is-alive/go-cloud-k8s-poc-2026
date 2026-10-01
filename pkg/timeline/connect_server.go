package timeline

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

// ConnectServer exposes Service through the generated TimelineService contract.
type ConnectServer struct {
	service *Service
	authz   *core.Authorizer
	log     *slog.Logger
	goelandv1connect.UnimplementedTimelineServiceHandler
}

// NewConnectServer builds a TimelineService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, authz *core.Authorizer, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("timeline service is required")
	}
	if authz == nil {
		return nil, errors.New("timeline connect server: an authorizer is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, authz: authz, log: log}, nil
}

// ListTimelineEntries returns a page of a case timeline.
func (s *ConnectServer) ListTimelineEntries(ctx context.Context, req *connect.Request[goelandv1.ListTimelineEntriesRequest]) (*connect.Response[goelandv1.ListTimelineEntriesResponse], error) {
	caseID, err := core.ParseUUID(req.Msg.CaseId)
	if err != nil {
		return nil, err
	}
	user, err := s.authz.Caller(ctx, core.ScopeRead, caseID, core.LevelRead)
	if err != nil {
		return nil, err
	}
	access, err := s.authz.Access(ctx, user, caseID)
	if err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	types := make([]EntryType, 0, len(req.Msg.EntryTypes))
	for _, t := range req.Msg.EntryTypes {
		types = append(types, EntryType(t))
	}
	result, err := s.service.List(ctx, ListFilter{
		CaseID:           caseID,
		Types:            types,
		IncludeWithdrawn: req.Msg.IncludeWithdrawn,
		ViewerID:         core.OperatorID(user),
		MaxVisibility:    MaxVisibility(access.Level),
		Limit:            int(req.Msg.PageSize),
		Offset:           offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ListTimelineEntriesResponse{
		Entries:       DomainsToProto(result.Entries),
		NextPageToken: core.NextPageToken(offset, len(result.Entries), result.TotalSize),
		TotalSize:     result.TotalSize,
		DraftCount:    result.DraftCount,
	}), nil
}

// CreateTimelineEntry adds a draft entry to a case.
func (s *ConnectServer) CreateTimelineEntry(ctx context.Context, req *connect.Request[goelandv1.CreateTimelineEntryRequest]) (*connect.Response[goelandv1.CreateTimelineEntryResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	caseID, err := core.ParseUUID(msg.CaseId)
	if err != nil {
		return nil, err
	}
	corrects, err := core.OptionalUUID(msg.CorrectsEntryId)
	if err != nil {
		return nil, err
	}
	documentIDs := make([]uuid.UUID, 0, len(msg.DocumentIds))
	for _, raw := range msg.DocumentIds {
		id, err := core.ParseUUID(raw)
		if err != nil {
			return nil, err
		}
		documentIDs = append(documentIDs, id)
	}
	e, ev, err := s.service.Create(ctx, CreateInput{
		CaseID:          caseID,
		Type:            EntryType(msg.EntryType),
		Title:           msg.Title,
		Body:            msg.Body,
		Visibility:      Visibility(msg.Visibility),
		OccurredAt:      core.TimePtrFromProto(msg.OccurredAt),
		CorrectsEntryID: corrects,
		DocumentIDs:     documentIDs,
		OperatorID:      core.OperatorID(user),
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateTimelineEntryResponse{Entry: DomainToProto(e), CreatedEvent: core.DomainAuditEventToProto(ev)}), nil
}

// GetTimelineEntry retrieves one entry with its documents.
func (s *ConnectServer) GetTimelineEntry(ctx context.Context, req *connect.Request[goelandv1.GetTimelineEntryRequest]) (*connect.Response[goelandv1.GetTimelineEntryResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	e, err := s.service.Get(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	need := e.Visibility.Needs()
	if e.CreatedBy == core.OperatorID(user) {
		need = core.LevelRead
	}
	if err := s.authz.Require(ctx, user, e.CaseID, need); err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.GetTimelineEntryResponse{Entry: DomainToProto(e)}), nil
}

// UpdateTimelineEntry replaces the content of a draft.
func (s *ConnectServer) UpdateTimelineEntry(ctx context.Context, req *connect.Request[goelandv1.UpdateTimelineEntryRequest]) (*connect.Response[goelandv1.UpdateTimelineEntryResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := core.ParseUUID(msg.Id)
	if err != nil {
		return nil, err
	}
	e, ev, err := s.service.Update(ctx, id, UpdateInput{
		Type:       EntryType(msg.EntryType),
		Title:      msg.Title,
		Body:       msg.Body,
		Visibility: Visibility(msg.Visibility),
		OccurredAt: core.TimePtrFromProto(msg.OccurredAt),
		OperatorID: core.OperatorID(user),
		Reason:     msg.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateTimelineEntryResponse{Entry: DomainToProto(e), UpdateEvent: core.DomainAuditEventToProto(ev)}), nil
}

// ValidateTimelineEntry endorses a draft.
func (s *ConnectServer) ValidateTimelineEntry(ctx context.Context, req *connect.Request[goelandv1.ValidateTimelineEntryRequest]) (*connect.Response[goelandv1.ValidateTimelineEntryResponse], error) {
	e, ev, err := s.changeStatus(ctx, req.Msg.Id, req.Msg.Reason, s.service.Validate)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.ValidateTimelineEntryResponse{Entry: DomainToProto(e), ValidateEvent: core.DomainAuditEventToProto(ev)}), nil
}

// LockTimelineEntry freezes a draft as is.
func (s *ConnectServer) LockTimelineEntry(ctx context.Context, req *connect.Request[goelandv1.LockTimelineEntryRequest]) (*connect.Response[goelandv1.LockTimelineEntryResponse], error) {
	e, ev, err := s.changeStatus(ctx, req.Msg.Id, req.Msg.Reason, s.service.Lock)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.LockTimelineEntryResponse{Entry: DomainToProto(e), LockEvent: core.DomainAuditEventToProto(ev)}), nil
}

// WithdrawTimelineEntry sets a draft aside.
func (s *ConnectServer) WithdrawTimelineEntry(ctx context.Context, req *connect.Request[goelandv1.WithdrawTimelineEntryRequest]) (*connect.Response[goelandv1.WithdrawTimelineEntryResponse], error) {
	e, ev, err := s.changeStatus(ctx, req.Msg.Id, req.Msg.Reason, s.service.Withdraw)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.WithdrawTimelineEntryResponse{Entry: DomainToProto(e), WithdrawEvent: core.DomainAuditEventToProto(ev)}), nil
}

// statusChange is the signature shared by Service.Validate, Lock and Withdraw.
type statusChange func(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error)

// changeStatus authorizes a write, parses the entry id and runs a status change.
func (s *ConnectServer) changeStatus(ctx context.Context, rawID, reason string, change statusChange) (*Entry, *core.AuditEvent, error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, nil, err
	}
	id, err := core.ParseUUID(rawID)
	if err != nil {
		return nil, nil, err
	}
	e, ev, err := change(ctx, id, core.OperatorID(user), reason)
	if err != nil {
		return nil, nil, s.mapError(err)
	}
	return e, ev, nil
}

// LinkTimelineDocument cites a document in a draft.
func (s *ConnectServer) LinkTimelineDocument(ctx context.Context, req *connect.Request[goelandv1.LinkTimelineDocumentRequest]) (*connect.Response[goelandv1.LinkTimelineDocumentResponse], error) {
	operatorID, entryID, documentID, err := s.linkArgs(ctx, req.Msg.EntryId, req.Msg.DocumentId)
	if err != nil {
		return nil, err
	}
	e, ev, err := s.service.LinkDocument(ctx, entryID, documentID, operatorID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.LinkTimelineDocumentResponse{Entry: DomainToProto(e), LinkEvent: core.DomainAuditEventToProto(ev)}), nil
}

// UnlinkTimelineDocument removes a document from a draft.
func (s *ConnectServer) UnlinkTimelineDocument(ctx context.Context, req *connect.Request[goelandv1.UnlinkTimelineDocumentRequest]) (*connect.Response[goelandv1.UnlinkTimelineDocumentResponse], error) {
	operatorID, entryID, documentID, err := s.linkArgs(ctx, req.Msg.EntryId, req.Msg.DocumentId)
	if err != nil {
		return nil, err
	}
	e, ev, err := s.service.UnlinkDocument(ctx, entryID, documentID, operatorID, req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UnlinkTimelineDocumentResponse{Entry: DomainToProto(e), UnlinkEvent: core.DomainAuditEventToProto(ev)}), nil
}

// linkArgs authorizes a write and returns the operator with the parsed entry
// and document ids.
func (s *ConnectServer) linkArgs(ctx context.Context, rawEntryID, rawDocumentID string) (string, uuid.UUID, uuid.UUID, error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return "", uuid.Nil, uuid.Nil, err
	}
	entryID, err := core.ParseUUID(rawEntryID)
	if err != nil {
		return "", uuid.Nil, uuid.Nil, err
	}
	documentID, err := core.ParseUUID(rawDocumentID)
	return core.OperatorID(user), entryID, documentID, err
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "timeline", err)
}
