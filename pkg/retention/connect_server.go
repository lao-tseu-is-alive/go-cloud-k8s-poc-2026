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

// CreateDispositionBordereau prepares a bordereau of due cases (records manager).
func (s *ConnectServer) CreateDispositionBordereau(ctx context.Context, req *connect.Request[goelandv1.CreateDispositionBordereauRequest]) (*connect.Response[goelandv1.CreateDispositionBordereauResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeRecords)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(req.Msg.CaseIds))
	for _, raw := range req.Msg.CaseIds {
		id, err := core.ParseUUID(raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	b, err := s.service.CreateBordereau(ctx, BordereauInput{
		Disposition: DispositionFromProto(req.Msg.FinalDisposition), Reason: req.Msg.Reason, CaseIDs: ids, OperatorID: core.OperatorID(user),
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	out, err := s.bordereauToProto(ctx, b)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.CreateDispositionBordereauResponse{Bordereau: out}), nil
}

// ListDispositionBordereaux pages the bordereaux (records manager).
func (s *ConnectServer) ListDispositionBordereaux(ctx context.Context, req *connect.Request[goelandv1.ListDispositionBordereauxRequest]) (*connect.Response[goelandv1.ListDispositionBordereauxResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRecords); err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	res, err := s.service.ListBordereaux(ctx, BordereauFilter{Status: BordereauStatusFromProto(req.Msg.Status), Limit: int(req.Msg.PageSize), Offset: offset})
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.DispositionBordereau, 0, len(res.Bordereaux))
	for _, b := range res.Bordereaux {
		out = append(out, BordereauToProto(b))
	}
	return connect.NewResponse(&goelandv1.ListDispositionBordereauxResponse{
		Bordereaux: out, NextPageToken: core.NextPageToken(offset, len(out), res.TotalSize), TotalSize: res.TotalSize,
	}), nil
}

// GetDispositionBordereau reads a bordereau with its cases (records manager).
func (s *ConnectServer) GetDispositionBordereau(ctx context.Context, req *connect.Request[goelandv1.GetDispositionBordereauRequest]) (*connect.Response[goelandv1.GetDispositionBordereauResponse], error) {
	id, err := s.bordereauID(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	b, err := s.service.GetBordereau(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	out, err := s.bordereauToProto(ctx, b)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.GetDispositionBordereauResponse{Bordereau: out}), nil
}

// ListMyDispositionApprovals lists the cases waiting for the caller's decision.
func (s *ConnectServer) ListMyDispositionApprovals(ctx context.Context, _ *connect.Request[goelandv1.ListMyDispositionApprovalsRequest]) (*connect.Response[goelandv1.ListMyDispositionApprovalsResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	approvals, err := s.service.MyApprovals(ctx, core.OperatorID(user))
	if err != nil {
		return nil, s.mapError(err)
	}
	items := make([]*Item, 0, len(approvals))
	out := &goelandv1.ListMyDispositionApprovalsResponse{Dispositions: map[string]goelandv1.FinalDisposition{}}
	for _, a := range approvals {
		items = append(items, a.Item)
		out.Dispositions[a.Item.ID.String()] = DispositionToProto(a.Disposition)
	}
	if out.Items, err = s.itemsToProto(ctx, items); err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// DecideDispositionItem approves or rejects a case of a bordereau.
func (s *ConnectServer) DecideDispositionItem(ctx context.Context, req *connect.Request[goelandv1.DecideDispositionItemRequest]) (*connect.Response[goelandv1.DecideDispositionItemResponse], error) {
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	item, err := s.service.Decide(ctx, DecisionInput{ItemID: id, Approve: req.Msg.Approve, Reason: req.Msg.Reason, OperatorID: core.OperatorID(user)})
	if err != nil {
		return nil, s.mapError(err)
	}
	out, err := s.itemsToProto(ctx, []*Item{item})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.DecideDispositionItemResponse{Item: out[0]}), nil
}

// ExecuteDispositionBordereau executes a fully decided bordereau (records manager).
func (s *ConnectServer) ExecuteDispositionBordereau(ctx context.Context, req *connect.Request[goelandv1.ExecuteDispositionBordereauRequest]) (*connect.Response[goelandv1.ExecuteDispositionBordereauResponse], error) {
	id, err := s.bordereauID(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	user, _ := core.RequireCaller(ctx, core.ScopeRecords)
	b, err := s.service.Execute(ctx, id, core.OperatorID(user))
	if err != nil {
		return nil, s.mapError(err)
	}
	out, err := s.bordereauToProto(ctx, b)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.ExecuteDispositionBordereauResponse{Bordereau: out}), nil
}

// CancelDispositionBordereau abandons an open bordereau (records manager).
func (s *ConnectServer) CancelDispositionBordereau(ctx context.Context, req *connect.Request[goelandv1.CancelDispositionBordereauRequest]) (*connect.Response[goelandv1.CancelDispositionBordereauResponse], error) {
	id, err := s.bordereauID(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	user, _ := core.RequireCaller(ctx, core.ScopeRecords)
	b, err := s.service.Cancel(ctx, id, req.Msg.Reason, core.OperatorID(user))
	if err != nil {
		return nil, s.mapError(err)
	}
	out, err := s.bordereauToProto(ctx, b)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.CancelDispositionBordereauResponse{Bordereau: out}), nil
}

// bordereauID checks the records manager scope and parses a bordereau id.
func (s *ConnectServer) bordereauID(ctx context.Context, raw string) (uuid.UUID, error) {
	id, err := core.ParseUUID(raw)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := core.RequireCaller(ctx, core.ScopeRecords); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// bordereauToProto converts a bordereau, its items' unreadable labels left out.
func (s *ConnectServer) bordereauToProto(ctx context.Context, b *Bordereau) (*goelandv1.DispositionBordereau, error) {
	items, err := s.itemsToProto(ctx, b.Items)
	if err != nil {
		return nil, err
	}
	out := BordereauToProto(&Bordereau{
		ID: b.ID, FinalDisposition: b.FinalDisposition, Status: b.Status, Reason: b.Reason, PreparedBy: b.PreparedBy,
		PreparedAt: b.PreparedAt, ClosedAt: b.ClosedAt, ClosedBy: b.ClosedBy, CloseReason: b.CloseReason,
		ProofSHA256: b.ProofSHA256, ProofManifest: b.ProofManifest, ItemCount: b.ItemCount, PendingCount: b.PendingCount, ApprovedCount: b.ApprovedCount,
	})
	out.Items = items
	return out, nil
}

// itemsToProto converts items, blanking the case and approver labels the
// caller may not read (GLD-070).
func (s *ConnectServer) itemsToProto(ctx context.Context, items []*Item) ([]*goelandv1.DispositionItem, error) {
	ids := make([]uuid.UUID, 0, 2*len(items))
	for _, it := range items {
		ids = append(ids, it.CaseID, it.ApproverSubjectID)
	}
	hidden, err := s.authz.Unreadable(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*goelandv1.DispositionItem, 0, len(items))
	for _, it := range items {
		p := ItemToProto(it)
		if hidden[it.CaseID] {
			p.CaseLabel = ""
		}
		if hidden[it.ApproverSubjectID] {
			p.ApproverLabel = ""
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "retention", err)
}
