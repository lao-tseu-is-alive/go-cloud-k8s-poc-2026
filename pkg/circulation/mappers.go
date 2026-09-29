package circulation

import (
	"time"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// DomainToProto converts a circulation (with its recipients) at now, which
// decides whether it is overdue; nil stays nil.
func DomainToProto(c *Circulation, now time.Time) *goelandv1.Circulation {
	if c == nil {
		return nil
	}
	recipients := make([]*goelandv1.CirculationRecipient, 0, len(c.Recipients))
	for _, rec := range c.Recipients {
		recipients = append(recipients, RecipientToProto(rec, c))
	}
	return &goelandv1.Circulation{
		Id:                 c.ID.String(),
		CaseId:             c.CaseID.String(),
		Title:              c.Title,
		Message:            c.Message,
		DueAt:              core.TimestampPtrOrNil(c.DueAt),
		Overdue:            c.Overdue(now),
		Status:             goelandv1.CirculationStatus(c.Status),
		CurrentStep:        c.CurrentStep,
		StepCount:          c.StepCount,
		CreatedAt:          core.TimestampOrNil(c.CreatedAt),
		CreatedBy:          c.CreatedBy,
		CompletedAt:        core.TimestampPtrOrNil(c.CompletedAt),
		CancelledAt:        core.TimestampPtrOrNil(c.CancelledAt),
		CancelledBy:        c.CancelledBy,
		CancellationReason: c.CancellationReason,
		Recipients:         recipients,
	}
}

// DomainsToProto maps a slice of circulations at now.
func DomainsToProto(list []*Circulation, now time.Time) []*goelandv1.Circulation {
	out := make([]*goelandv1.Circulation, 0, len(list))
	for _, c := range list {
		out = append(out, DomainToProto(c, now))
	}
	return out
}

// RecipientToProto converts one recipient of circulation c.
func RecipientToProto(rec *Recipient, c *Circulation) *goelandv1.CirculationRecipient {
	out := &goelandv1.CirculationRecipient{
		Id:                rec.ID.String(),
		Step:              rec.Step,
		AssigneeOrgUnitId: core.UUIDPtrString(rec.AssigneeOrgUnitID),
		AssigneeLabel:     rec.AssigneeLabel,
		TaskId:            core.UUIDPtrString(rec.TaskID),
		Awaiting:          rec.Awaiting(c),
		ResponseText:      rec.ResponseText,
		RespondedAt:       core.TimestampPtrOrNil(rec.RespondedAt),
		RespondedBy:       rec.RespondedBy,
		ResponseEntryId:   core.UUIDPtrString(rec.ResponseEntryID),
	}
	if rec.AssigneeUserID != nil {
		out.AssigneeUserId = *rec.AssigneeUserID
	}
	if rec.TaskStatus != nil {
		out.TaskStatus = goelandv1.TaskStatus(*rec.TaskStatus)
	}
	if rec.Response != nil {
		out.Response = goelandv1.CirculationResponse(*rec.Response)
	}
	return out
}
