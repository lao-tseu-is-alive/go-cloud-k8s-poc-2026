package caseexport

import (
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// statusToProto maps a job state to its proto enum.
var statusToProto = map[Status]goelandv1.CaseExportStatus{
	StatusQueued:  goelandv1.CaseExportStatus_CASE_EXPORT_STATUS_QUEUED,
	StatusRunning: goelandv1.CaseExportStatus_CASE_EXPORT_STATUS_RUNNING,
	StatusDone:    goelandv1.CaseExportStatus_CASE_EXPORT_STATUS_DONE,
	StatusFailed:  goelandv1.CaseExportStatus_CASE_EXPORT_STATUS_FAILED,
	StatusExpired: goelandv1.CaseExportStatus_CASE_EXPORT_STATUS_EXPIRED,
}

// ToProto maps an export job; the storage reference stays internal.
func ToProto(exp *Export) *goelandv1.CaseExport {
	if exp == nil {
		return nil
	}
	return &goelandv1.CaseExport{
		Id:          exp.ID.String(),
		CaseId:      exp.CaseID.String(),
		Status:      statusToProto[exp.Status],
		RequestedBy: exp.RequestedBy,
		Reason:      exp.Reason,
		RequestedAt: core.TimestampOrNil(exp.RequestedAt),
		StartedAt:   core.TimestampPtrOrNil(exp.StartedAt),
		FinishedAt:  core.TimestampPtrOrNil(exp.FinishedAt),
		ExpiresAt:   core.TimestampPtrOrNil(exp.ExpiresAt),
		SizeBytes:   exp.SizeBytes,
		Sha256:      exp.SHA256,
		Error:       exp.Error,
	}
}
