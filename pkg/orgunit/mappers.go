package orgunit

import (
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TypeToProto converts an org unit type to its proto representation; nil stays nil.
func TypeToProto(t *OrgUnitType) *goelandv1.OrgUnitType {
	if t == nil {
		return nil
	}
	return &goelandv1.OrgUnitType{
		Id:          t.ID.String(),
		Code:        t.Code,
		Label:       t.Label,
		Description: t.Description,
		SortOrder:   t.SortOrder,
		IsActive:    t.IsActive,
	}
}

// DomainToProto converts a unit (with hydrated associations) to its proto representation.
func DomainToProto(u *OrgUnit) *goelandv1.OrgUnit {
	if u == nil {
		return nil
	}
	return &goelandv1.OrgUnit{
		SubjectRef:        core.DomainSubjectRefToProto(u.Subject),
		OrgUnitType:       TypeToProto(u.Type),
		Abbreviation:      u.Abbreviation,
		Label:             u.Label,
		Description:       u.Description,
		Email:             u.Email,
		ParentId:          core.UUIDPtrString(u.ParentID),
		DissolvedAt:       core.TimestampPtrOrNil(u.DissolvedAt),
		DissolvedBy:       u.DissolvedBy,
		DissolutionReason: u.DissolutionReason,
		CreatedAt:         core.TimestampOrNil(u.CreatedAt),
		CreatedBy:         u.CreatedBy,
		UpdatedAt:         core.TimestampOrNil(u.UpdatedAt),
		RecordMetadata:    core.DomainRecordMetadataToProto(u.RecordMetadata),
		ExternalRef:       u.ExternalRef,
	}
}

// DomainsToProto maps a slice of units.
func DomainsToProto(units []*OrgUnit) []*goelandv1.OrgUnit {
	out := make([]*goelandv1.OrgUnit, 0, len(units))
	for _, u := range units {
		out = append(out, DomainToProto(u))
	}
	return out
}

// NodesToProto maps tree nodes.
func NodesToProto(nodes []*Node) []*goelandv1.OrgUnitNode {
	out := make([]*goelandv1.OrgUnitNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &goelandv1.OrgUnitNode{
			Id:              n.ID.String(),
			Abbreviation:    n.Abbreviation,
			Label:           n.Label,
			OrgUnitTypeCode: n.TypeCode,
			ParentId:        core.UUIDPtrString(n.ParentID),
			Dissolved:       n.Dissolved,
		})
	}
	return out
}
