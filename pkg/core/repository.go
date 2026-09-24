package core

import (
	"context"

	"github.com/google/uuid"
)

// Repository persists transversal core entities: subjects, governance metadata,
// typed relationships and the append-only audit log.
type Repository interface {
	CreateSubject(ctx context.Context, in CreateSubjectInput) (*SubjectRef, *RecordMetadata, *AuditEvent, error)
	GetSubject(ctx context.Context, id uuid.UUID) (*SubjectRef, error)
	AssignBusinessRef(ctx context.Context, subjectID uuid.UUID, req BusinessRefRequest, operatorID, reason string) (*SubjectRef, *AuditEvent, error)
	LookupSubjects(ctx context.Context, filter LookupFilter, limit int) ([]*SubjectRef, error)
	GetRecordMetadata(ctx context.Context, subjectID uuid.UUID) (*RecordMetadata, error)
	LinkSubjects(ctx context.Context, in LinkInput) (*SubjectRelationship, *AuditEvent, error)
	UnlinkSubjects(ctx context.Context, relationshipID uuid.UUID, operatorID, reason string) (*SubjectRelationship, *AuditEvent, error)
	EndRelationship(ctx context.Context, in EndInput) (*SubjectRelationship, *AuditEvent, error)
	ListRelationships(ctx context.Context, filter RelationshipFilter) (RelationshipResult, error)
	ListRelationshipTypes(ctx context.Context, onlyActive bool, sourceKind, targetKind SubjectKind) ([]*RelationshipType, error)
	AppendAuditEvent(ctx context.Context, ev AuditEvent) (*AuditEvent, error)
	RecordUser(ctx context.Context, profile UserProfile) (*AppUser, error)
	GetUsers(ctx context.Context, userIDs []string) ([]*AppUser, error)
	ListAuditEvents(ctx context.Context, filter AuditFilter) (AuditResult, error)
}
