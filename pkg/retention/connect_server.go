package retention

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

// ConnectServer exposes Service through the generated RetentionService contract.
type ConnectServer struct {
	service *Service
	authz   *core.Authorizer
	log     *slog.Logger
	goelandv1connect.UnimplementedRetentionServiceHandler
}

// NewConnectServer builds a RetentionService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, authz *core.Authorizer, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("retention service is required")
	}
	if authz == nil {
		return nil, errors.New("retention connect server: an authorizer is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, authz: authz, log: log}, nil
}

// ListRetentionPolicies returns the retention schedule.
func (s *ConnectServer) ListRetentionPolicies(ctx context.Context, req *connect.Request[goelandv1.ListRetentionPoliciesRequest]) (*connect.Response[goelandv1.ListRetentionPoliciesResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	policies, err := s.service.ListPolicies(ctx, req.Msg.OnlyActive)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.RetentionPolicy, 0, len(policies))
	for _, p := range policies {
		out = append(out, PolicyToProto(p))
	}
	return connect.NewResponse(&goelandv1.ListRetentionPoliciesResponse{RetentionPolicies: out}), nil
}

// CreateRetentionPolicy adds a policy (administrators only).
func (s *ConnectServer) CreateRetentionPolicy(ctx context.Context, req *connect.Request[goelandv1.CreateRetentionPolicyRequest]) (*connect.Response[goelandv1.CreateRetentionPolicyResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.CreatePolicy(ctx, PolicyInput{
		Code: m.Code, Label: m.Label, Description: m.Description, LegalBasis: m.LegalBasis,
		DurationYears: m.DurationYears, FinalDisposition: DispositionFromProto(m.FinalDisposition),
		OperatorID: core.OperatorID(user), Reason: m.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateRetentionPolicyResponse{RetentionPolicy: PolicyToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// UpdateRetentionPolicy changes a policy (administrators only).
func (s *ConnectServer) UpdateRetentionPolicy(ctx context.Context, req *connect.Request[goelandv1.UpdateRetentionPolicyRequest]) (*connect.Response[goelandv1.UpdateRetentionPolicyResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in := PolicyUpdate{
		Label: m.Label, Description: m.Description, LegalBasis: m.LegalBasis, DurationYears: m.DurationYears,
		IsActive: m.IsActive, OperatorID: core.OperatorID(user), Reason: m.Reason,
	}
	if m.FinalDisposition != nil {
		d := DispositionFromProto(*m.FinalDisposition)
		in.FinalDisposition = &d
	}
	entry, change, err := s.service.UpdatePolicy(ctx, m.Code, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateRetentionPolicyResponse{RetentionPolicy: PolicyToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// GetCaseRetention reads the retention of a case: READ on the case, or the
// records manager scope (who follows every case, confidential ones included).
func (s *ConnectServer) GetCaseRetention(ctx context.Context, req *connect.Request[goelandv1.GetCaseRetentionRequest]) (*connect.Response[goelandv1.GetCaseRetentionResponse], error) {
	caseID, err := core.ParseUUID(req.Msg.CaseId)
	if err != nil {
		return nil, err
	}
	user, err := core.RequireCaller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	if !user.HasScope(core.ScopeRecords) {
		if err := s.authz.Require(ctx, user, caseID, core.LevelRead); err != nil {
			return nil, err
		}
	}
	res, err := s.service.CaseRetention(ctx, caseID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(CaseRetentionToProto(res)), nil
}

// PlaceRetentionHold places a legal hold on a case (records manager).
func (s *ConnectServer) PlaceRetentionHold(ctx context.Context, req *connect.Request[goelandv1.PlaceRetentionHoldRequest]) (*connect.Response[goelandv1.PlaceRetentionHoldResponse], error) {
	in, err := holdInput(ctx, req.Msg.CaseId, req.Msg.Reason)
	if err != nil {
		return nil, err
	}
	hold, ev, err := s.service.PlaceHold(ctx, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.PlaceRetentionHoldResponse{Hold: HoldToProto(hold), AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}

// ReleaseRetentionHold releases the legal hold of a case (records manager).
func (s *ConnectServer) ReleaseRetentionHold(ctx context.Context, req *connect.Request[goelandv1.ReleaseRetentionHoldRequest]) (*connect.Response[goelandv1.ReleaseRetentionHoldResponse], error) {
	in, err := holdInput(ctx, req.Msg.CaseId, req.Msg.Reason)
	if err != nil {
		return nil, err
	}
	hold, ev, err := s.service.ReleaseHold(ctx, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ReleaseRetentionHoldResponse{Hold: HoldToProto(hold), AuditEvent: core.DomainAuditEventToProto(ev)}), nil
}

// holdInput checks the records manager scope and builds a hold change.
func holdInput(ctx context.Context, rawCaseID, reason string) (HoldInput, error) {
	caseID, err := core.ParseUUID(rawCaseID)
	if err != nil {
		return HoldInput{}, err
	}
	user, err := core.RequireCaller(ctx, core.ScopeRecords)
	if err != nil {
		return HoldInput{}, err
	}
	return HoldInput{CaseID: caseID, Reason: reason, OperatorID: core.OperatorID(user)}, nil
}

// ListDueRetentions pages the due retentions (records manager or administrator);
// the label of a case the caller may not read is left out.
func (s *ConnectServer) ListDueRetentions(ctx context.Context, req *connect.Request[goelandv1.ListDueRetentionsRequest]) (*connect.Response[goelandv1.ListDueRetentionsResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRecords); err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	res, err := s.service.ListDue(ctx, DueFilter{
		DueOn: req.Msg.DueOn, Disposition: DispositionFromProto(req.Msg.FinalDisposition), IncludeHeld: req.Msg.IncludeHeld,
		Limit: int(req.Msg.PageSize), Offset: offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	ids := make([]uuid.UUID, 0, len(res.Retentions))
	for _, r := range res.Retentions {
		ids = append(ids, r.SubjectID)
	}
	hidden, err := s.authz.Unreadable(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*goelandv1.DueRetention, 0, len(res.Retentions))
	for _, r := range res.Retentions {
		due := &goelandv1.DueRetention{
			Retention: RetentionToProto(&r.Retention), CaseLabel: r.CaseLabel, BusinessRef: r.BusinessRef,
			CaseTypeCode: r.CaseTypeCode, OnHold: r.OnHold,
		}
		if hidden[r.SubjectID] {
			due.CaseLabel = ""
		}
		out = append(out, due)
	}
	return connect.NewResponse(&goelandv1.ListDueRetentionsResponse{
		Retentions: out, NextPageToken: core.NextPageToken(offset, len(out), res.TotalSize), TotalSize: res.TotalSize,
	}), nil
}

// ApplyCaseTypeRetention gives a case type's policy to its closed cases
// without retention (administrators only).
func (s *ConnectServer) ApplyCaseTypeRetention(ctx context.Context, req *connect.Request[goelandv1.ApplyCaseTypeRetentionRequest]) (*connect.Response[goelandv1.ApplyCaseTypeRetentionResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	res, err := s.service.ApplyCaseType(ctx, ApplyInput{CaseTypeCode: req.Msg.CaseTypeCode, Reason: req.Msg.Reason, OperatorID: core.OperatorID(user)})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ApplyCaseTypeRetentionResponse{Applied: res.Applied, Remaining: res.Remaining}), nil
}

func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "retention", err)
}
