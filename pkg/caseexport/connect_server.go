package caseexport

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ConnectServer exposes Service through the generated CaseExportService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	goelandv1connect.UnimplementedCaseExportServiceHandler
}

// NewConnectServer builds a CaseExportService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("case export service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log}, nil
}

// RequestCaseExport queues an export; the service checks MANAGE on the case
// and records the request in the sensitive read audit.
func (s *ConnectServer) RequestCaseExport(ctx context.Context, req *connect.Request[goelandv1.RequestCaseExportRequest]) (*connect.Response[goelandv1.RequestCaseExportResponse], error) {
	caseID, err := core.ParseUUID(req.Msg.CaseId)
	if err != nil {
		return nil, err
	}
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	exp, err := s.service.RequestExport(ctx, RequestInput{CaseID: caseID, Reason: req.Msg.Reason, OperatorID: core.OperatorID(user)})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.RequestCaseExportResponse{Export: ToProto(exp)}), nil
}

// GetCaseExport returns an export to its requester or an administrator.
func (s *ConnectServer) GetCaseExport(ctx context.Context, req *connect.Request[goelandv1.GetCaseExportRequest]) (*connect.Response[goelandv1.GetCaseExportResponse], error) {
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	user, err := core.RequireCaller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	exp, err := s.service.Get(ctx, id, core.OperatorID(user), user.HasScope(core.ScopeAdmin))
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.GetCaseExportResponse{Export: ToProto(exp)}), nil
}

// ListCaseExports returns the caller's exports of a case (an administrator's: all).
func (s *ConnectServer) ListCaseExports(ctx context.Context, req *connect.Request[goelandv1.ListCaseExportsRequest]) (*connect.Response[goelandv1.ListCaseExportsResponse], error) {
	caseID, err := core.ParseUUID(req.Msg.CaseId)
	if err != nil {
		return nil, err
	}
	user, err := core.RequireCaller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	list, err := s.service.List(ctx, caseID, core.OperatorID(user), user.HasScope(core.ScopeAdmin))
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.ListCaseExportsResponse{Exports: make([]*goelandv1.CaseExport, len(list))}
	for i, exp := range list {
		resp.Exports[i] = ToProto(exp)
	}
	return connect.NewResponse(resp), nil
}

// mapError translates domain errors into Connect errors.
func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "case export", err)
}
