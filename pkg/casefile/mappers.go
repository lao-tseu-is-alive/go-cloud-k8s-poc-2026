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
		Id:                          t.ID.String(),
		Code:                        t.Code,
		Label:                       t.Label,
		Description:                 t.Description,
		BusinessRefNamespace:        t.BusinessRefNamespace,
		IsActive:                    t.IsActive,
		DefaultConfidentialityLevel: t.DefaultConfidentialityLevel,
		DefaultGrants:               DefaultGrantsToProto(t.DefaultGrants),
	}
}

// DefaultGrantsToProto converts a default grant template.
func DefaultGrantsToProto(grants []core.DefaultGrant) []*goelandv1.CaseTypeDefaultGrant {
	out := make([]*goelandv1.CaseTypeDefaultGrant, len(grants))
	for i, g := range grants {
		out[i] = &goelandv1.CaseTypeDefaultGrant{
			GranteeKind: core.GranteeKindToProto(g.GranteeKind), GranteeId: g.GranteeID(),
			GranteeLabel: g.GranteeLabel, Level: core.LevelToProto(g.Level),
		}
	}
	return out
}

// DefaultGrantsFromProto converts and checks the shape of a template.
func DefaultGrantsFromProto(grants []*goelandv1.CaseTypeDefaultGrant) ([]core.DefaultGrant, error) {
	out := make([]core.DefaultGrant, 0, len(grants))
	for _, g := range grants {
		line, err := core.NewDefaultGrant(core.GranteeKindFromProto(g.GranteeKind), g.GranteeId, core.LevelFromProto(g.Level))
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, nil
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
