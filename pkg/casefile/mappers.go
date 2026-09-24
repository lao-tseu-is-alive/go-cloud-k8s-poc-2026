package casefile

import (
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TypeToProto converts a case type to its proto representation; nil stays nil.
func TypeToProto(t *CaseType) *goelandv1.CaseType {
	if t == nil {
		return nil
	}
	return &goelandv1.CaseType{
		Id:                   t.ID.String(),
		Code:                 t.Code,
		Label:                t.Label,
		Description:          t.Description,
		BusinessRefNamespace: t.BusinessRefNamespace,
		IsActive:             t.IsActive,
	}
}

// DomainToProto converts a case (with hydrated associations) to its proto representation.
func DomainToProto(c *Case) *goelandv1.Case {
	if c == nil {
		return nil
	}
	return &goelandv1.Case{
		SubjectRef:     core.DomainSubjectRefToProto(c.Subject),
		CaseType:       TypeToProto(c.Type),
		Title:          c.Title,
		Description:    c.Description,
		Status:         goelandv1.CaseStatus(c.Status),
		OpenedAt:       core.TimestampOrNil(c.OpenedAt),
		ClosedAt:       core.TimestampPtrOrNil(c.ClosedAt),
		ClosedBy:       c.ClosedBy,
		ClosureReason:  c.ClosureReason,
		Metadata:       core.StructFromMap(c.Metadata),
		CreatedAt:      core.TimestampOrNil(c.CreatedAt),
		CreatedBy:      c.CreatedBy,
		UpdatedAt:      core.TimestampOrNil(c.UpdatedAt),
		RecordMetadata: core.DomainRecordMetadataToProto(c.RecordMetadata),
	}
}

// DomainsToProto maps a slice of cases.
func DomainsToProto(cases []*Case) []*goelandv1.Case {
	out := make([]*goelandv1.Case, 0, len(cases))
	for _, c := range cases {
		out = append(out, DomainToProto(c))
	}
	return out
}
