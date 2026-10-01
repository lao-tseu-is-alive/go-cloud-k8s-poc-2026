package access

import (
	"time"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// GranteeKind is who a grant is given to; it mirrors access_grant.grantee_kind.
type GranteeKind string

// Grantee kinds.
const (
	// GranteeUser is an internal user (operator id).
	GranteeUser GranteeKind = "USER"
	// GranteeGroup is a security group (its subject id).
	GranteeGroup GranteeKind = "GROUP"
	// GranteeOrgUnit is an org unit and its sub-units (its subject id).
	GranteeOrgUnit GranteeKind = "ORG_UNIT"
)

// Audit event types written on the subject of a grant, or on a group.
const (
	// EventAccessGranted records a new grant.
	EventAccessGranted = "ACCESS_GRANTED"
	// EventAccessChanged records a grant whose level was changed (the old row is kept).
	EventAccessChanged = "ACCESS_CHANGED"
	// EventAccessRevoked records a revoked grant.
	EventAccessRevoked = "ACCESS_REVOKED"
	// EventGroupCreated records a new security group.
	EventGroupCreated = "GROUP_CREATED"
	// EventGroupUpdated records a renamed or redescribed group.
	EventGroupUpdated = "GROUP_UPDATED"
	// EventGroupArchived records an archived group.
	EventGroupArchived = "GROUP_ARCHIVED"
)

// Grant is one grant on a subject; a revoked one is kept as history.
type Grant struct {
	// ID identifies the grant.
	ID uuid.UUID `db:"id"`
	// SubjectID is the subject the grant is on.
	SubjectID uuid.UUID `db:"subject_id"`
	// GranteeKind is who the grant is given to.
	GranteeKind GranteeKind `db:"grantee_kind"`
	// GranteeUserID is the user of a USER grant.
	GranteeUserID *string `db:"grantee_user_id"`
	// GranteeSubjectID is the group or unit of a GROUP or ORG_UNIT grant.
	GranteeSubjectID *uuid.UUID `db:"grantee_subject_id"`
	// GranteeLabel is the grantee's display name.
	GranteeLabel string `db:"grantee_label"`
	// Level is the granted level.
	Level core.Level `db:"level"`
	// GrantedAt is when the grant was made.
	GrantedAt time.Time `db:"granted_at"`
	// GrantedBy is the operator who made it.
	GrantedBy string `db:"granted_by"`
	// GrantReason explains it.
	GrantReason string `db:"grant_reason"`
	// RevokedAt is when it was revoked or replaced; nil while current.
	RevokedAt *time.Time `db:"revoked_at"`
	// RevokedBy is who revoked it; nil while current.
	RevokedBy *string `db:"revoked_by"`
	// RevokeReason explains the revocation.
	RevokeReason string `db:"revoke_reason"`
}

// GranteeID is the user id or the group or unit subject id of the grantee.
func (g *Grant) GranteeID() string {
	if g.GranteeUserID != nil {
		return *g.GranteeUserID
	}
	if g.GranteeSubjectID != nil {
		return g.GranteeSubjectID.String()
	}
	return ""
}

// SetGrantInput gives or changes a grant.
type SetGrantInput struct {
	// SubjectID is the subject.
	SubjectID uuid.UUID
	// GranteeKind is who the grant is given to.
	GranteeKind GranteeKind
	// GranteeID is the user id, or the group or unit subject id.
	GranteeID string
	// Level is the level to give (READ to FULL_CONTROL).
	Level core.Level
	// Reason explains the grant; it is audited.
	Reason string
	// OperatorID is who makes it (server-derived).
	OperatorID string
}

// Group is a security group: a named set of internal users (a GROUP subject).
type Group struct {
	// ID is the group's subject id.
	ID uuid.UUID `db:"id"`
	// Name is unique among live groups.
	Name string `db:"name"`
	// Description tells what the group is for.
	Description string `db:"description"`
	// ArchivedAt is when the group was archived; nil while live.
	ArchivedAt *time.Time `db:"archived_at"`
	// ArchivedBy is who archived it.
	ArchivedBy string `db:"archived_by"`
	// MemberCount is the number of current members.
	MemberCount int32 `db:"member_count"`
	// Subject is the group's subject_ref (hydrated).
	Subject *core.SubjectRef `db:"-"`
}

// Member is a current member of a group.
type Member struct {
	// UserID is the member's operator id.
	UserID string `db:"user_id"`
	// UserSubjectID is the member's USER subject.
	UserSubjectID uuid.UUID `db:"user_subject_id"`
	// DisplayName is the member's name.
	DisplayName string `db:"display_name"`
	// Email is the member's e-mail address.
	Email string `db:"email"`
	// RelationshipID is the USER_MEMBER_OF_GROUP relationship.
	RelationshipID uuid.UUID `db:"relationship_id"`
	// Since is when the user joined.
	Since time.Time `db:"since"`
	// AddedBy is who added the user.
	AddedBy string `db:"added_by"`
}

// GroupInput creates or updates a group.
type GroupInput struct {
	// Name is the group's name.
	Name string
	// Description tells what the group is for.
	Description string
	// Reason explains an update.
	Reason string
	// OperatorID is who makes the change (server-derived).
	OperatorID string
}

// MemberInput adds or removes a member.
type MemberInput struct {
	// GroupID is the group.
	GroupID uuid.UUID
	// UserID is the member's operator id.
	UserID string
	// Reason explains a removal.
	Reason string
	// OperatorID is who makes the change (server-derived).
	OperatorID string
}
