package task

import (
	"time"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TypeToProto converts a task type to its proto representation; nil stays nil.
func TypeToProto(t *TaskType) *goelandv1.TaskType {
	if t == nil {
		return nil
	}
	return &goelandv1.TaskType{Id: t.ID.String(), Code: t.Code, Label: t.Label, Description: t.Description, IsActive: t.IsActive}
}

// DomainToProto converts a task to its proto representation at now (which
// decides whether it is overdue); nil stays nil.
func DomainToProto(t *Task, now time.Time) *goelandv1.Task {
	if t == nil {
		return nil
	}
	assignments := make([]*goelandv1.TaskAssignment, 0, len(t.Assignments))
	for _, a := range t.Assignments {
		assignments = append(assignments, AssignmentToProto(a))
	}
	return &goelandv1.Task{
		Id:                 t.ID.String(),
		CaseId:             t.CaseID.String(),
		CaseLabel:          t.CaseLabel,
		TaskType:           TypeToProto(t.Type),
		Title:              t.Title,
		Description:        t.Description,
		Status:             goelandv1.TaskStatus(t.Status),
		Origin:             goelandv1.TaskOrigin(t.Origin),
		OriginRef:          t.OriginRef,
		AssigneeUserId:     stringValue(t.AssigneeUserID),
		AssigneeOrgUnitId:  core.UUIDPtrString(t.AssigneeOrgUnitID),
		AssigneeLabel:      t.AssigneeLabel,
		DueAt:              core.TimestampPtrOrNil(t.DueAt),
		Overdue:            t.Overdue(now),
		CreatedAt:          core.TimestampOrNil(t.CreatedAt),
		CreatedBy:          t.CreatedBy,
		UpdatedAt:          core.TimestampOrNil(t.UpdatedAt),
		UpdatedBy:          t.UpdatedBy,
		StartedAt:          core.TimestampPtrOrNil(t.StartedAt),
		StartedBy:          t.StartedBy,
		CompletedAt:        core.TimestampPtrOrNil(t.CompletedAt),
		CompletedBy:        t.CompletedBy,
		CompletionNote:     t.CompletionNote,
		CancelledAt:        core.TimestampPtrOrNil(t.CancelledAt),
		CancelledBy:        t.CancelledBy,
		CancellationReason: t.CancellationReason,
		Assignments:        assignments,
	}
}

// DomainsToProto maps a slice of tasks at now.
func DomainsToProto(tasks []*Task, now time.Time) []*goelandv1.Task {
	out := make([]*goelandv1.Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, DomainToProto(t, now))
	}
	return out
}

// AssignmentToProto converts one history row.
func AssignmentToProto(a *Assignment) *goelandv1.TaskAssignment {
	return &goelandv1.TaskAssignment{
		Id:                a.ID.String(),
		AssigneeUserId:    stringValue(a.AssigneeUserID),
		AssigneeOrgUnitId: core.UUIDPtrString(a.AssigneeOrgUnitID),
		AssigneeLabel:     a.AssigneeLabel,
		AssignedAt:        core.TimestampOrNil(a.AssignedAt),
		AssignedBy:        a.AssignedBy,
		Reason:            a.Reason,
		EndedAt:           core.TimestampPtrOrNil(a.EndedAt),
	}
}

// stringValue renders an optional string; nil becomes "".
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
