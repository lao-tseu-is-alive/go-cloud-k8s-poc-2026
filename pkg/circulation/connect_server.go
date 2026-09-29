package circulation

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
)

// ConnectServer exposes Service through the generated CirculationService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	// now is the clock deciding whether a circulation is overdue.
	now func() time.Time
	goelandv1connect.UnimplementedCirculationServiceHandler
}

// NewConnectServer builds a CirculationService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("circulation service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log, now: time.Now}, nil
}

// ListCaseCirculations returns the circulations of a case.
func (s *ConnectServer) ListCaseCirculations(ctx context.Context, req *connect.Request[goelandv1.ListCaseCirculationsRequest]) (*connect.Response[goelandv1.ListCaseCirculationsResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	caseID, err := core.ParseUUID(req.Msg.CaseId)
	if err != nil {
		return nil, err
	}
	list, err := s.service.ListCase(ctx, caseID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ListCaseCirculationsResponse{Circulations: DomainsToProto(list, s.now())}), nil
}

// GetCirculation retrieves one circulation.
func (s *ConnectServer) GetCirculation(ctx context.Context, req *connect.Request[goelandv1.GetCirculationRequest]) (*connect.Response[goelandv1.GetCirculationResponse], error) {
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
	return connect.NewResponse(&goelandv1.GetCirculationResponse{Circulation: DomainToProto(c, s.now())}), nil
}

// CreateCirculation sends a case to recipients.
func (s *ConnectServer) CreateCirculation(ctx context.Context, req *connect.Request[goelandv1.CreateCirculationRequest]) (*connect.Response[goelandv1.CreateCirculationResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	caseID, err := core.ParseUUID(m.CaseId)
	if err != nil {
		return nil, err
	}
	recipients, err := recipientsFromProto(m.Recipients)
	if err != nil {
		return nil, err
	}
	c, ev, err := s.service.Create(ctx, CreateInput{
		CaseID: caseID, Title: m.Title, Message: m.Message, DueAt: core.TimePtrFromProto(m.DueAt),
		Recipients: recipients, OperatorID: core.OperatorID(user),
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateCirculationResponse{Circulation: DomainToProto(c, s.now()), CreatedEvent: core.DomainAuditEventToProto(ev)}), nil
}

// RespondToCirculation records the answer of a recipient.
func (s *ConnectServer) RespondToCirculation(ctx context.Context, req *connect.Request[goelandv1.RespondToCirculationRequest]) (*connect.Response[goelandv1.RespondToCirculationResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.RecipientId)
	if err != nil {
		return nil, err
	}
	c, ev, err := s.service.Respond(ctx, RespondInput{
		RecipientID: id, Response: Response(req.Msg.Response), Text: req.Msg.Text, OperatorID: core.OperatorID(user),
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.RespondToCirculationResponse{Circulation: DomainToProto(c, s.now()), ResponseEvent: core.DomainAuditEventToProto(ev)}), nil
}

// CancelCirculation stops an open circulation.
func (s *ConnectServer) CancelCirculation(ctx context.Context, req *connect.Request[goelandv1.CancelCirculationRequest]) (*connect.Response[goelandv1.CancelCirculationResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	c, ev, err := s.service.Cancel(ctx, id, core.OperatorID(user), req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CancelCirculationResponse{Circulation: DomainToProto(c, s.now()), CancelEvent: core.DomainAuditEventToProto(ev)}), nil
}

// recipientsFromProto converts the recipient inputs.
func recipientsFromProto(in []*goelandv1.CirculationRecipientInput) ([]RecipientInput, error) {
	out := make([]RecipientInput, 0, len(in))
	for _, rec := range in {
		unit, err := core.OptionalUUID(rec.AssigneeOrgUnitId)
		if err != nil {
			return nil, err
		}
		a := task.Assignee{OrgUnitID: unit}
		if rec.AssigneeUserId != "" {
			userID := rec.AssigneeUserId
			a.UserID = &userID
		}
		out = append(out, RecipientInput{Step: rec.Step, Assignee: a})
	}
	return out, nil
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "circulation", err)
}
