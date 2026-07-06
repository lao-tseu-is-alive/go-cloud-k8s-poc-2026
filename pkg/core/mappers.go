package core

import (
	"time"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"google.golang.org/protobuf/types/known/structpb"
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
}

// kindFromProto is the inverse of kindToProto.
var kindFromProto = map[goelandv1.SubjectKind]SubjectKind{
	goelandv1.SubjectKind_SUBJECT_KIND_CASE:     SubjectKindCase,
	goelandv1.SubjectKind_SUBJECT_KIND_DOCUMENT: SubjectKindDocument,
	goelandv1.SubjectKind_SUBJECT_KIND_THING:    SubjectKindThing,
	goelandv1.SubjectKind_SUBJECT_KIND_ACTOR:    SubjectKindActor,
	goelandv1.SubjectKind_SUBJECT_KIND_USER:     SubjectKindUser,
	goelandv1.SubjectKind_SUBJECT_KIND_ORG_UNIT: SubjectKindOrgUnit,
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

// structFromMap converts a map to a proto Struct, returning nil on empty/invalid input.
func structFromMap(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
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
		OwnerOrgId:           md.OwnerOrgID,
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
		BeforeState:   structFromMap(ev.BeforeState),
		AfterState:    structFromMap(ev.AfterState),
		Reason:        ev.Reason,
		CorrelationId: correlation,
		RequestId:     ev.RequestID,
		Metadata:      structFromMap(ev.Metadata),
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
		Id:           rt.ID.String(),
		Code:         rt.Code,
		Label:        rt.Label,
		SourceKind:   SubjectKindToProto(rt.SourceKind),
		TargetKind:   SubjectKindToProto(rt.TargetKind),
		IsDirected:   rt.IsDirected,
		InverseLabel: rt.InverseLabel,
		Description:  rt.Description,
		IsActive:     rt.IsActive,
	}
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
	}
}

// DomainRelationshipsToProto maps a slice of relationships.
func DomainRelationshipsToProto(rels []*SubjectRelationship) []*goelandv1.SubjectRelationship {
	out := make([]*goelandv1.SubjectRelationship, 0, len(rels))
	for _, rel := range rels {
		out = append(out, DomainRelationshipToProto(rel))
	}
	return out
}
