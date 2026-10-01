package task

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ConnectServer exposes Service through the generated TaskService contract.
type ConnectServer struct {
	service *Service
	authz   *core.Authorizer
	log     *slog.Logger
	// now is the clock deciding whether a task is overdue.
	now func() time.Time
	goelandv1connect.UnimplementedTaskServiceHandler
}

// NewConnectServer builds a TaskService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, authz *core.Authorizer, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("task service is required")
	}
	if authz == nil {
		return nil, errors.New("task connect server: an authorizer is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, authz: authz, log: log, now: time.Now}, nil
}

// ListCaseTasks returns a page of the tasks of a case.
func (s *ConnectServer) ListCaseTasks(ctx context.Context, req *connect.Request[goelandv1.ListCaseTasksRequest]) (*connect.Response[goelandv1.ListCaseTasksResponse], error) {
	caseID, err := core.ParseUUID(req.Msg.CaseId)
	if err != nil {
		return nil, err
	}
	if _, err := s.authz.Caller(ctx, core.ScopeRead, caseID, core.LevelRead); err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	res, err := s.service.ListCase(ctx, CaseFilter{
		CaseID: caseID, Statuses: statusesFromProto(req.Msg.Statuses), Limit: int(req.Msg.PageSize), Offset: offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ListCaseTasksResponse{
		Tasks:         DomainsToProto(res.Tasks, s.now()),
		NextPageToken: core.NextPageToken(offset, len(res.Tasks), res.TotalSize),
		TotalSize:     res.TotalSize,
		OpenCount:     res.OpenCount,
	}), nil
}

// ListMyTasks returns a page of the caller's tasks.
func (s *ConnectServer) ListMyTasks(ctx context.Context, req *connect.Request[goelandv1.ListMyTasksRequest]) (*connect.Response[goelandv1.ListMyTasksResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeRead)
	if err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	res, err := s.service.ListMine(ctx, MineFilter{
		UserID: core.OperatorID(user), IncludeUnits: req.Msg.IncludeUnits, Statuses: statusesFromProto(req.Msg.Statuses),
		Limit: int(req.Msg.PageSize), Offset: offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.ListMyTasksResponse{
		Tasks:         DomainsToProto(res.Tasks, s.now()),
		NextPageToken: core.NextPageToken(offset, len(res.Tasks), res.TotalSize),
		TotalSize:     res.TotalSize,
	}), nil
}

// GetTask retrieves a task with its assignment history.
func (s *ConnectServer) GetTask(ctx context.Context, req *connect.Request[goelandv1.GetTaskRequest]) (*connect.Response[goelandv1.GetTaskResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeRead)
	if err != nil {
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
	if err := s.authz.Require(ctx, user, t.CaseID, core.LevelRead); err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.GetTaskResponse{Task: DomainToProto(t, s.now())}), nil
}

// CreateTask adds a manual task to a case.
func (s *ConnectServer) CreateTask(ctx context.Context, req *connect.Request[goelandv1.CreateTaskRequest]) (*connect.Response[goelandv1.CreateTaskResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	caseID, err := core.ParseUUID(m.CaseId)
	if err != nil {
		return nil, err
	}
	assignee, err := assigneeFromWire(m.AssigneeUserId, m.AssigneeOrgUnitId)
	if err != nil {
		return nil, err
	}
	t, ev, err := s.service.Create(ctx, CreateInput{
		Content:    Content{TypeCode: m.TaskTypeCode, Title: m.Title, Description: m.Description, DueAt: core.TimePtrFromProto(m.DueAt)},
		CaseID:     caseID,
		Assignee:   assignee,
		OperatorID: core.OperatorID(user),
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateTaskResponse{Task: DomainToProto(t, s.now()), CreatedEvent: core.DomainAuditEventToProto(ev)}), nil
}

// UpdateTask replaces the content of a pending task.
func (s *ConnectServer) UpdateTask(ctx context.Context, req *connect.Request[goelandv1.UpdateTaskRequest]) (*connect.Response[goelandv1.UpdateTaskResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	id, err := core.ParseUUID(m.Id)
	if err != nil {
		return nil, err
	}
	t, ev, err := s.service.Update(ctx, id, UpdateInput{
		Content:    Content{TypeCode: m.TaskTypeCode, Title: m.Title, Description: m.Description, DueAt: core.TimePtrFromProto(m.DueAt)},
		OperatorID: core.OperatorID(user),
		Reason:     m.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateTaskResponse{Task: DomainToProto(t, s.now()), UpdateEvent: core.DomainAuditEventToProto(ev)}), nil
}

// AssignTask (re)assigns or unassigns a pending task.
func (s *ConnectServer) AssignTask(ctx context.Context, req *connect.Request[goelandv1.AssignTaskRequest]) (*connect.Response[goelandv1.AssignTaskResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	id, err := core.ParseUUID(m.Id)
	if err != nil {
		return nil, err
	}
	assignee, err := assigneeFromWire(m.AssigneeUserId, m.AssigneeOrgUnitId)
	if err != nil {
		return nil, err
	}
	t, ev, err := s.service.Assign(ctx, id, assignee, core.OperatorID(user), m.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.AssignTaskResponse{Task: DomainToProto(t, s.now()), AssignEvent: core.DomainAuditEventToProto(ev)}), nil
}

// StartTask moves an open task to IN_PROGRESS.
func (s *ConnectServer) StartTask(ctx context.Context, req *connect.Request[goelandv1.StartTaskRequest]) (*connect.Response[goelandv1.StartTaskResponse], error) {
	t, ev, err := s.move(ctx, req.Msg.Id, MoveStart, "")
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.StartTaskResponse{Task: t, StatusEvent: ev}), nil
}

// CompleteTask completes a pending task.
func (s *ConnectServer) CompleteTask(ctx context.Context, req *connect.Request[goelandv1.CompleteTaskRequest]) (*connect.Response[goelandv1.CompleteTaskResponse], error) {
	t, ev, err := s.move(ctx, req.Msg.Id, MoveComplete, req.Msg.Note)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.CompleteTaskResponse{Task: t, StatusEvent: ev}), nil
}

// CancelTask cancels a pending task.
func (s *ConnectServer) CancelTask(ctx context.Context, req *connect.Request[goelandv1.CancelTaskRequest]) (*connect.Response[goelandv1.CancelTaskResponse], error) {
	t, ev, err := s.move(ctx, req.Msg.Id, MoveCancel, req.Msg.Reason)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.CancelTaskResponse{Task: t, StatusEvent: ev}), nil
}

// ReopenTask reopens a done or cancelled task.
func (s *ConnectServer) ReopenTask(ctx context.Context, req *connect.Request[goelandv1.ReopenTaskRequest]) (*connect.Response[goelandv1.ReopenTaskResponse], error) {
	t, ev, err := s.move(ctx, req.Msg.Id, MoveReopen, req.Msg.Reason)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&goelandv1.ReopenTaskResponse{Task: t, StatusEvent: ev}), nil
}

// move authorizes a write, parses the task id and applies a lifecycle move.
func (s *ConnectServer) move(ctx context.Context, rawID string, move Move, note string) (*goelandv1.Task, *goelandv1.AuditEvent, error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, nil, err
	}
	id, err := core.ParseUUID(rawID)
	if err != nil {
		return nil, nil, err
	}
	t, ev, err := s.service.ChangeStatus(ctx, id, move, core.OperatorID(user), note)
	if err != nil {
		return nil, nil, s.mapError(err)
	}
	return DomainToProto(t, s.now()), core.DomainAuditEventToProto(ev), nil
}

// ListTaskTypes returns the task type catalogue.
func (s *ConnectServer) ListTaskTypes(ctx context.Context, req *connect.Request[goelandv1.ListTaskTypesRequest]) (*connect.Response[goelandv1.ListTaskTypesResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	types, err := s.service.ListTypes(ctx, req.Msg.OnlyActive)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.TaskType, 0, len(types))
	for _, t := range types {
		out = append(out, TypeToProto(t))
	}
	return connect.NewResponse(&goelandv1.ListTaskTypesResponse{TaskTypes: out}), nil
}

// CreateTaskType adds a task type (administrators only).
func (s *ConnectServer) CreateTaskType(ctx context.Context, req *connect.Request[goelandv1.CreateTaskTypeRequest]) (*connect.Response[goelandv1.CreateTaskTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.CreateType(ctx, TypeInput{Code: m.Code, Label: m.Label, Description: m.Description, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateTaskTypeResponse{TaskType: TypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// UpdateTaskType changes a task type (administrators only).
func (s *ConnectServer) UpdateTaskType(ctx context.Context, req *connect.Request[goelandv1.UpdateTaskTypeRequest]) (*connect.Response[goelandv1.UpdateTaskTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.UpdateType(ctx, m.Code, TypeUpdate{Label: m.Label, Description: m.Description, IsActive: m.IsActive, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateTaskTypeResponse{TaskType: TypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// assigneeFromWire builds an Assignee from the request fields (empty means none).
func assigneeFromWire(userID, orgUnitID string) (Assignee, error) {
	unit, err := core.OptionalUUID(orgUnitID)
	if err != nil {
		return Assignee{}, err
	}
	a := Assignee{OrgUnitID: unit}
	if userID != "" {
		a.UserID = &userID
	}
	return a, nil
}

// statusesFromProto converts status filters.
func statusesFromProto(in []goelandv1.TaskStatus) []Status {
	out := make([]Status, 0, len(in))
	for _, st := range in {
		out = append(out, Status(st))
	}
	return out
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	return core.ToConnectError(s.log, "task", err)
}
