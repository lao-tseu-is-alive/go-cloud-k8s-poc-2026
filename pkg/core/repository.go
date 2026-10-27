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
	SearchSubjects(ctx context.Context, filter SubjectSearchFilter) ([]SubjectSearchHit, error)
	GetRecordMetadata(ctx context.Context, subjectID uuid.UUID) (*RecordMetadata, error)
	LinkSubjects(ctx context.Context, in LinkInput) (*SubjectRelationship, *AuditEvent, error)
	UnlinkSubjects(ctx context.Context, relationshipID uuid.UUID, operatorID, reason string) (*SubjectRelationship, *AuditEvent, error)
	EndRelationship(ctx context.Context, in EndInput) (*SubjectRelationship, *AuditEvent, error)
	ListRelationships(ctx context.Context, filter RelationshipFilter) (RelationshipResult, error)
	SummarizeRelationships(ctx context.Context, subjectID uuid.UUID, viewer Viewer) ([]*RelationshipGroup, error)
	ListRelationshipTypes(ctx context.Context, onlyActive bool, sourceKind, targetKind SubjectKind) ([]*RelationshipType, error)
	AppendAuditEvent(ctx context.Context, ev AuditEvent) (*AuditEvent, error)
	CreateRelationshipType(ctx context.Context, in RelationshipTypeInput) (*RelationshipType, *ReferenceChange, error)
	UpdateRelationshipType(ctx context.Context, code string, in RelationshipTypeUpdate) (*RelationshipType, *ReferenceChange, error)
	ListReferenceChanges(ctx context.Context, filter ReferenceFilter) (ReferenceResult, error)
	// OutboxStatus describes the transactional outbox (GLD-028).
	OutboxStatus(ctx context.Context) (OutboxStatus, error)
	// ReplayOutbox puts failed outbox events back in the queue: those of ids, or all.
	ReplayOutbox(ctx context.Context, ids []uuid.UUID, all bool) (int32, error)
	ListAccessAudit(ctx context.Context, filter AccessAuditFilter) (AccessAuditResult, error)
	RecordUser(ctx context.Context, profile UserProfile) (*AppUser, error)
	GetUsers(ctx context.Context, userIDs []string) ([]*AppUser, error)
	SearchUsers(ctx context.Context, query string, limit int) ([]*AppUser, error)
	ListAppRoles(ctx context.Context) ([]*AppRole, error)
	ListRoleHolders(ctx context.Context, roleCode string) ([]*AppUser, error)
	ListUserRoles(ctx context.Context, userID string, includeRevoked bool) ([]*UserRole, error)
	ActiveRoles(ctx context.Context, userID string) ([]string, error)
	GrantUserRole(ctx context.Context, in RoleChangeInput) (*UserRole, *AuditEvent, error)
	RevokeUserRole(ctx context.Context, in RoleChangeInput) (*UserRole, *AuditEvent, error)
	ListAuditEvents(ctx context.Context, filter AuditFilter) (AuditResult, error)
}
