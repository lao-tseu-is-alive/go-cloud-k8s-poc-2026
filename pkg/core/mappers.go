package core

import (
	"slices"
	"time"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// kindToProto maps domain subject kinds to the proto enum.
var kindToProto = map[SubjectKind]goelandv1.SubjectKind{
	SubjectKindCase:     goelandv1.SubjectKind_SUBJECT_KIND_CASE,
	SubjectKindDocument: goelandv1.SubjectKind_SUBJECT_KIND_DOCUMENT,
	SubjectKindThing:    goelandv1.SubjectKind_SUBJECT_KIND_THING,
	SubjectKindActor:    goelandv1.SubjectKind_SUBJECT_KIND_ACTOR,
	SubjectKindUser:     goelandv1.SubjectKind_SUBJECT_KIND_USER,
	SubjectKindOrgUnit:  goelandv1.SubjectKind_SUBJECT_KIND_ORG_UNIT,
	SubjectKindGroup:    goelandv1.SubjectKind_SUBJECT_KIND_GROUP,
}

// kindFromProto is the inverse of kindToProto.
var kindFromProto = map[goelandv1.SubjectKind]SubjectKind{
	goelandv1.SubjectKind_SUBJECT_KIND_CASE:     SubjectKindCase,
	goelandv1.SubjectKind_SUBJECT_KIND_DOCUMENT: SubjectKindDocument,
	goelandv1.SubjectKind_SUBJECT_KIND_THING:    SubjectKindThing,
	goelandv1.SubjectKind_SUBJECT_KIND_ACTOR:    SubjectKindActor,
	goelandv1.SubjectKind_SUBJECT_KIND_USER:     SubjectKindUser,
	goelandv1.SubjectKind_SUBJECT_KIND_ORG_UNIT: SubjectKindOrgUnit,
	goelandv1.SubjectKind_SUBJECT_KIND_GROUP:    SubjectKindGroup,
}

// SubjectKindToProto converts a domain SubjectKind to the proto enum.
func SubjectKindToProto(k SubjectKind) goelandv1.SubjectKind {
	return kindToProto[k]
}

// SubjectKindFromProto converts a proto enum to the domain SubjectKind.
func SubjectKindFromProto(k goelandv1.SubjectKind) SubjectKind {
	return kindFromProto[k]
}

// TimestampOrNil returns a proto timestamp, or nil for the zero time.
func TimestampOrNil(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// TimestampPtrOrNil returns a proto timestamp from a *time.Time, or nil.
func TimestampPtrOrNil(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

// TimePtrFromProto converts a proto timestamp to a *time.Time (nil when unset).
func TimePtrFromProto(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

// DomainSubjectRefToProto converts a domain SubjectRef to its proto representation.
func DomainSubjectRefToProto(ref *SubjectRef) *goelandv1.SubjectRef {
	if ref == nil {
		return nil
	}
	return &goelandv1.SubjectRef{
		Id:           ref.ID.String(),
		Kind:         SubjectKindToProto(ref.Kind),
		DisplayLabel: ref.DisplayLabel,
		CanonicalUrl: ref.CanonicalURL,
		CreatedAt:    TimestampOrNil(ref.CreatedAt),

		BusinessRef:          ref.BusinessRef,
		BusinessRefNamespace: ref.BusinessRefNamespace,
	}
}

// BusinessRefRequestFromProto converts an optional proto BusinessRefRequest; nil
// yields the zero value (no reference requested).
func BusinessRefRequestFromProto(req *goelandv1.BusinessRefRequest) BusinessRefRequest {
	if req == nil {
		return BusinessRefRequest{}
	}
	return BusinessRefRequest{
		Namespace: req.GetNamespace(),
		Value:     req.GetValue(),
		Allocate:  req.GetAllocate(),
	}
}

// DomainRecordMetadataToProto converts governance metadata to its proto representation.
func DomainRecordMetadataToProto(md *RecordMetadata) *goelandv1.RecordMetadata {
	if md == nil {
		return nil
	}
	return &goelandv1.RecordMetadata{
		SubjectId:            md.SubjectID.String(),
		CreatedAt:            TimestampOrNil(md.CreatedAt),
		CreatedBy:            md.CreatedBy,
		UpdatedAt:            TimestampPtrOrNil(md.UpdatedAt),
		UpdatedBy:            md.UpdatedBy,
		DeletedAt:            TimestampPtrOrNil(md.DeletedAt),
		DeletedBy:            md.DeletedBy,
		OwnerUserId:          md.OwnerUserID,
		OwnerOrgId:           UUIDPtrString(md.OwnerOrgID),
		ConfidentialityLevel: md.ConfidentialityLevel,
		Version:              md.Version,
		IsLocked:             md.IsLocked,
		LockedAt:             TimestampPtrOrNil(md.LockedAt),
		LockedBy:             md.LockedBy,
		RetentionUntil:       md.RetentionUntil,
		SortFinal:            md.SortFinal,
		Metadata:             md.Metadata,
	}
}

// DomainAuditEventToProto converts an audit event to its proto representation.
func DomainAuditEventToProto(ev *AuditEvent) *goelandv1.AuditEvent {
	if ev == nil {
		return nil
	}
	correlation := ""
	if ev.CorrelationID != nil {
		correlation = ev.CorrelationID.String()
	}
	return &goelandv1.AuditEvent{
		Id:            ev.ID.String(),
		SubjectId:     ev.SubjectID.String(),
		EventType:     ev.EventType,
		ActorUserId:   ev.ActorUserID,
		OccurredAt:    TimestampOrNil(ev.OccurredAt),
		BeforeState:   StructFromMap(ev.BeforeState),
		AfterState:    StructFromMap(ev.AfterState),
		Reason:        ev.Reason,
		CorrelationId: correlation,
		RequestId:     ev.RequestID,
		Metadata:      StructFromMap(ev.Metadata),
	}
}

// DomainAuditEventsToProto maps a slice of audit events.
func DomainAuditEventsToProto(events []*AuditEvent) []*goelandv1.AuditEvent {
	out := make([]*goelandv1.AuditEvent, 0, len(events))
	for _, ev := range events {
		out = append(out, DomainAuditEventToProto(ev))
	}
	return out
}

// DomainRelationshipTypeToProto converts a relationship type to its proto representation.
func DomainRelationshipTypeToProto(rt *RelationshipType) *goelandv1.RelationshipType {
	if rt == nil {
		return nil
	}
	return &goelandv1.RelationshipType{
		Id:              rt.ID.String(),
		Code:            rt.Code,
		Label:           rt.Label,
		SourceKind:      SubjectKindToProto(rt.SourceKind),
		TargetKind:      SubjectKindToProto(rt.TargetKind),
		IsDirected:      rt.IsDirected,
		InverseLabel:    rt.InverseLabel,
		Description:     rt.Description,
		IsActive:        rt.IsActive,
		RoleLabel:       rt.RoleLabel,
		RoleLabelPlural: rt.RoleLabelPlural,
		RoleEnd:         RoleEndToProto(rt.RoleEnd),
		RolePreposition: rt.RolePreposition,
	}
}

// RoleEndToProto maps a role end; an empty one reads as TARGET.
func RoleEndToProto(e RoleEnd) goelandv1.RoleEnd {
	if e == RoleEndSource {
		return goelandv1.RoleEnd_ROLE_END_SOURCE
	}
	return goelandv1.RoleEnd_ROLE_END_TARGET
}

// RoleEndFromProto maps a proto role end; UNSPECIFIED is TARGET.
func RoleEndFromProto(e goelandv1.RoleEnd) RoleEnd {
	if e == goelandv1.RoleEnd_ROLE_END_SOURCE {
		return RoleEndSource
	}
	return RoleEndTarget
}

// DomainRelationshipToProto converts a relationship (with hydrated associations) to its proto representation.
func DomainRelationshipToProto(rel *SubjectRelationship) *goelandv1.SubjectRelationship {
	if rel == nil {
		return nil
	}
	return &goelandv1.SubjectRelationship{
		Id:               rel.ID.String(),
		Source:           DomainSubjectRefToProto(rel.Source),
		Target:           DomainSubjectRefToProto(rel.Target),
		RelationshipType: DomainRelationshipTypeToProto(rel.RelationshipType),
		RoleDetail:       rel.RoleDetail,
		ValidFrom:        TimestampPtrOrNil(rel.ValidFrom),
		ValidTo:          TimestampPtrOrNil(rel.ValidTo),
		CreatedAt:        TimestampOrNil(rel.CreatedAt),
		CreatedBy:        rel.CreatedBy,
		DeletedAt:        TimestampPtrOrNil(rel.DeletedAt),
		OtherDetail:      RelatedDetailToProto(rel.OtherDetail),
	}
}

// RelatedDetailToProto maps the detail of a relationship's other end.
func RelatedDetailToProto(d *RelatedDetail) *goelandv1.RelatedSubjectDetail {
	if d == nil {
		return nil
	}
	return &goelandv1.RelatedSubjectDetail{TypeCode: d.TypeCode, TypeLabel: d.TypeLabel, CaseStatus: d.CaseStatus, Date: d.Date}
}

// SubjectSearchGroupsToProto maps the groups of a global search.
func SubjectSearchGroupsToProto(groups []*SubjectSearchGroup) []*goelandv1.SubjectSearchGroup {
	out := make([]*goelandv1.SubjectSearchGroup, len(groups))
	for i, g := range groups {
		subjects := make([]*goelandv1.SubjectRef, len(g.Subjects))
		for j, ref := range g.Subjects {
			subjects[j] = DomainSubjectRefToProto(ref)
		}
		out[i] = &goelandv1.SubjectSearchGroup{
			Kind: SubjectKindToProto(g.Kind), Subjects: subjects,
			TotalSize: g.Total, TotalSizeCapped: g.TotalCapped,
		}
	}
	return out
}

// RelationshipGroupsToProto maps relationship groups.
func RelationshipGroupsToProto(groups []*RelationshipGroup) []*goelandv1.RelationshipGroup {
	out := make([]*goelandv1.RelationshipGroup, 0, len(groups))
	for _, g := range groups {
		pg := &goelandv1.RelationshipGroup{
			RelationshipType: DomainRelationshipTypeToProto(g.RelationshipType), Outgoing: g.Outgoing,
			OtherKind: SubjectKindToProto(g.OtherKind), Count: g.Count, OpenCount: g.OpenCount,
		}
		for _, f := range g.Families {
			pg.Families = append(pg.Families, &goelandv1.RelationshipFamily{Code: f.Code, Label: f.Label, Count: f.Count, OpenCount: f.OpenCount})
		}
		out = append(out, pg)
	}
	return out
}

// DomainRelationshipsToProto maps a slice of relationships.
func DomainRelationshipsToProto(rels []*SubjectRelationship) []*goelandv1.SubjectRelationship {
	out := make([]*goelandv1.SubjectRelationship, 0, len(rels))
	for _, rel := range rels {
		out = append(out, DomainRelationshipToProto(rel))
	}
	return out
}

// DomainUserToProto converts an internal user to its proto representation.
func DomainUserToProto(u *AppUser) *goelandv1.User {
	if u == nil {
		return nil
	}
	return &goelandv1.User{
		Id:          u.UserID,
		SubjectId:   u.SubjectID.String(),
		DisplayName: u.DisplayName,
		Email:       u.Email,
		IsAdmin:     u.IsAdmin,
		FirstSeenAt: TimestampOrNil(u.FirstSeenAt),
		LastSeenAt:  TimestampOrNil(u.LastSeenAt),
		Roles:       slices.Clone(u.Roles),
	}
}

// DomainUsersToProto converts a list of internal users.
func DomainUsersToProto(users []*AppUser) []*goelandv1.User {
	out := make([]*goelandv1.User, len(users))
	for i, u := range users {
		out[i] = DomainUserToProto(u)
	}
	return out
}

// DomainAppRoleToProto converts an application role to its proto representation.
func DomainAppRoleToProto(r *AppRole) *goelandv1.AppRole {
	if r == nil {
		return nil
	}
	return &goelandv1.AppRole{Code: r.Code, Label: r.Label, Description: r.Description, IsActive: r.IsActive}
}

// DomainUserRoleToProto converts a role assignment to its proto representation.
func DomainUserRoleToProto(r *UserRole) *goelandv1.UserRole {
	if r == nil {
		return nil
	}
	out := &goelandv1.UserRole{
		Id: r.ID.String(), UserId: r.UserID, RoleCode: r.RoleCode,
		GrantedAt: TimestampOrNil(r.GrantedAt), GrantedBy: r.GrantedBy, GrantReason: r.GrantReason,
		RevokedAt: TimestampPtrOrNil(r.RevokedAt), RevokeReason: r.RevokeReason,
	}
	if r.RevokedBy != nil {
		out.RevokedBy = *r.RevokedBy
	}
	return out
}

// DomainReferenceChangeToProto converts a reference change log entry to its proto representation.
func DomainReferenceChangeToProto(c *ReferenceChange) *goelandv1.ReferenceChange {
	if c == nil {
		return nil
	}
	return &goelandv1.ReferenceChange{
		Id:          c.ID.String(),
		Catalogue:   c.Catalogue,
		Code:        c.Code,
		EventType:   c.EventType,
		ActorUserId: c.ActorUserID,
		OccurredAt:  TimestampOrNil(c.OccurredAt),
		BeforeState: StructFromMap(c.BeforeState),
		AfterState:  StructFromMap(c.AfterState),
		Reason:      c.Reason,
	}
}

// OutboxEventToProto maps an outbox event (GLD-028).
func OutboxEventToProto(e *OutboxEvent) *goelandv1.OutboxEvent {
	if e == nil {
		return nil
	}
	return &goelandv1.OutboxEvent{
		Id:              e.ID.String(),
		SubjectKind:     SubjectKindToProto(e.SubjectKind),
		SubjectId:       e.SubjectID.String(),
		EventType:       e.EventType,
		AuditEventId:    e.AuditEventID.String(),
		OccurredAt:      TimestampOrNil(e.OccurredAt),
		Attempts:        e.Attempts,
		NextAttemptAt:   TimestampOrNil(e.NextAttemptAt),
		PublishedAt:     TimestampPtrOrNil(e.PublishedAt),
		FailedAt:        TimestampPtrOrNil(e.FailedAt),
		LastError:       e.LastError,
		FailedConsumers: e.FailedConsumers,
	}
}

// OutboxStatusToProto maps the outbox status with the server's retention.
func OutboxStatusToProto(s OutboxStatus, retention time.Duration) *goelandv1.GetOutboxStatusResponse {
	out := &goelandv1.GetOutboxStatusResponse{
		Pending:         s.Pending,
		Failed:          s.Failed,
		Published:       s.Published,
		OldestPendingAt: TimestampPtrOrNil(s.OldestPendingAt),
		RetentionDays:   int32(retention / (24 * time.Hour)),
		MaxAttempts:     OutboxMaxAttempts,
	}
	for _, c := range s.Consumers {
		out.Consumers = append(out.Consumers, &goelandv1.OutboxConsumerStatus{Name: c.Name, Failed: c.Failed})
	}
	for _, e := range s.RecentFailed {
		out.RecentFailed = append(out.RecentFailed, OutboxEventToProto(e))
	}
	return out
}
