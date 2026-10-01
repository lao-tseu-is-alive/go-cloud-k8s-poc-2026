package access

import (
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

var sourceToProto = map[core.AccessSource]goelandv1.AccessSource{
	core.SourcePersonal: goelandv1.AccessSource_ACCESS_SOURCE_PERSONAL,
	core.SourceGroup:    goelandv1.AccessSource_ACCESS_SOURCE_GROUP,
	core.SourceOrgUnit:  goelandv1.AccessSource_ACCESS_SOURCE_ORG_UNIT,
	core.SourceRole:     goelandv1.AccessSource_ACCESS_SOURCE_ROLE,
	core.SourceBaseline: goelandv1.AccessSource_ACCESS_SOURCE_BASELINE,
}

// AccessToProto converts an effective access.
func AccessToProto(a core.Access) *goelandv1.Access {
	return &goelandv1.Access{
		SubjectId: a.SubjectID.String(), Kind: core.SubjectKindToProto(a.Kind), Confidential: a.Confidential,
		Level: core.LevelToProto(a.Level), Source: sourceToProto[a.Source],
	}
}

// GrantToProto converts a grant.
func GrantToProto(g *Grant) *goelandv1.AccessGrant {
	if g == nil {
		return nil
	}
	out := &goelandv1.AccessGrant{
		Id: g.ID.String(), SubjectId: g.SubjectID.String(), GranteeKind: core.GranteeKindToProto(g.GranteeKind),
		GranteeId: g.GranteeID(), GranteeLabel: g.GranteeLabel, Level: core.LevelToProto(g.Level),
		GrantedAt: core.TimestampOrNil(g.GrantedAt), GrantedBy: g.GrantedBy, GrantReason: g.GrantReason,
		RevokedAt: core.TimestampPtrOrNil(g.RevokedAt), RevokeReason: g.RevokeReason,
	}
	if g.RevokedBy != nil {
		out.RevokedBy = *g.RevokedBy
	}
	return out
}

// GroupToProto converts a group.
func GroupToProto(g *Group) *goelandv1.SecurityGroup {
	if g == nil {
		return nil
	}
	return &goelandv1.SecurityGroup{
		SubjectRef: core.DomainSubjectRefToProto(g.Subject), Name: g.Name, Description: g.Description,
		ArchivedAt: core.TimestampPtrOrNil(g.ArchivedAt), MemberCount: g.MemberCount,
	}
}

// MemberToProto converts a membership.
func MemberToProto(m *Member) *goelandv1.GroupMember {
	if m == nil {
		return nil
	}
	return &goelandv1.GroupMember{
		User:           &goelandv1.User{Id: m.UserID, SubjectId: m.UserSubjectID.String(), DisplayName: m.DisplayName, Email: m.Email},
		RelationshipId: m.RelationshipID.String(), Since: core.TimestampOrNil(m.Since), AddedBy: m.AddedBy,
	}
}
