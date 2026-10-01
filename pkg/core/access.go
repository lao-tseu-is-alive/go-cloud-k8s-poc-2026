package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Authorization primitives (GLD-048, model GLD-017). Every mutation and single
// read checks the caller's effective level on the subject inside its
// transaction (EnsureAccessTx), like the lifecycle guards.

// Level is an access level on a subject; levels are ordered.
type Level int16

// Access levels, ordered: each includes the ones below it.
const (
	// LevelNone gives no access.
	LevelNone Level = 0
	// LevelRead reads the subject.
	LevelRead Level = 1
	// LevelContribute adds timeline entries and attaches documents.
	LevelContribute Level = 2
	// LevelManage edits the subject, its lifecycle, relations, tasks and circulations.
	LevelManage Level = 3
	// LevelFullControl also manages grants and deletes the subject.
	LevelFullControl Level = 4
)

// ConfidentialLevel is the confidentiality level from which a subject needs an
// explicit grant: no baseline and no application role reaches it.
const ConfidentialLevel = 2

// Valid reports whether l is a grantable level (READ to FULL_CONTROL).
func (l Level) Valid() bool { return l >= LevelRead && l <= LevelFullControl }

// String names the level as the API does.
func (l Level) String() string {
	switch l {
	case LevelRead:
		return "READ"
	case LevelContribute:
		return "CONTRIBUTE"
	case LevelManage:
		return "MANAGE"
	case LevelFullControl:
		return "FULL_CONTROL"
	default:
		return "NONE"
	}
}

// AccessSource tells which rule produced an effective level.
type AccessSource string

// Sources of an effective level, from the most specific.
const (
	// SourcePersonal is a grant to the user.
	SourcePersonal AccessSource = "PERSONAL"
	// SourceGroup is the highest grant to one of the user's groups.
	SourceGroup AccessSource = "GROUP"
	// SourceOrgUnit is the grant to the nearest of the user's units or their ancestors.
	SourceOrgUnit AccessSource = "ORG_UNIT"
	// SourceRole is an application role (ADMIN, or a role covering the kind).
	SourceRole AccessSource = "ROLE"
	// SourceBaseline is the default: READ unless the subject is confidential.
	SourceBaseline AccessSource = "BASELINE"
)

// Access is a user's effective level on a subject and where it comes from.
type Access struct {
	// SubjectID is the subject.
	SubjectID uuid.UUID
	// Kind is the subject's kind.
	Kind SubjectKind
	// Confidential reports confidentiality_level >= ConfidentialLevel.
	Confidential bool
	// Level is the effective level.
	Level Level
	// Source is the rule that produced Level.
	Source AccessSource
}

// accessRow is what effectiveAccessSQL computes: each layer's level (nil when
// the layer gives nothing) and the subject's kind and confidentiality.
type accessRow struct {
	// Kind is the subject's kind.
	Kind SubjectKind `db:"kind"`
	// Confidentiality is the subject's confidentiality_level.
	Confidentiality int32 `db:"confidentiality"`
	// Personal is the user's own grant.
	Personal *int16 `db:"personal"`
	// GroupLevel is the highest grant of the user's live groups.
	GroupLevel *int16 `db:"group_level"`
	// UnitLevel is the grant of the nearest of the user's units or their ancestors.
	UnitLevel *int16 `db:"unit_level"`
	// RoleLevel is the level of the user's application roles on the kind.
	RoleLevel *int16 `db:"role_level"`
}

// resolve applies the precedence: personal, groups, nearest unit, then (not on
// a confidential subject) role and baseline READ.
func (r accessRow) resolve(subjectID uuid.UUID) Access {
	a := Access{SubjectID: subjectID, Kind: r.Kind, Confidential: r.Confidentiality >= ConfidentialLevel}
	switch {
	case r.Personal != nil:
		a.Level, a.Source = Level(*r.Personal), SourcePersonal
	case r.GroupLevel != nil:
		a.Level, a.Source = Level(*r.GroupLevel), SourceGroup
	case r.UnitLevel != nil:
		a.Level, a.Source = Level(*r.UnitLevel), SourceOrgUnit
	case a.Confidential:
		a.Level, a.Source = LevelNone, SourceBaseline
	case r.RoleLevel != nil:
		a.Level, a.Source = Level(*r.RoleLevel), SourceRole
	default:
		a.Level, a.Source = LevelRead, SourceBaseline
	}
	return a
}

// EffectiveAccessTx computes userID's effective level on subjectID using q;
// an unknown subject is ErrNotFound.
func EffectiveAccessTx(ctx context.Context, q Querier, userID string, subjectID uuid.UUID) (Access, error) {
	rows, err := q.Query(ctx, effectiveAccessSQL, pgx.NamedArgs{"user_id": userID, "subject_id": subjectID})
	if err != nil {
		return Access{}, fmt.Errorf("effective access: %w", err)
	}
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[accessRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return Access{}, fmt.Errorf("%w: subject %s", ErrNotFound, subjectID)
	}
	if err != nil {
		return Access{}, fmt.Errorf("effective access: %w", err)
	}
	return row.resolve(subjectID), nil
}

// EnsureAccessTx requires userID to hold at least need on subjectID; a lower
// level is ErrPermissionDenied.
func EnsureAccessTx(ctx context.Context, q Querier, userID string, subjectID uuid.UUID, need Level) error {
	access, err := EffectiveAccessTx(ctx, q, userID, subjectID)
	if err != nil {
		return err
	}
	if access.Level < need {
		return fmt.Errorf("%w: this operation needs %s on the %s, the caller has %s", ErrPermissionDenied,
			need, access.Kind, access.Level)
	}
	return nil
}

// insertCreatorGrantsTx gives a new subject's creator FULL_CONTROL and its
// owning unit MANAGE (called with the subject's governance).
func insertCreatorGrantsTx(ctx context.Context, q Querier, in CreateSubjectInput, subjectID uuid.UUID) error {
	if in.OperatorID != "" {
		if _, err := q.Exec(ctx, insertUserGrantSQL, pgx.NamedArgs{
			"subject_id": subjectID, "user_id": in.OperatorID, "level": int16(LevelFullControl),
			"granted_by": in.OperatorID, "reason": "creator of the subject",
		}); err != nil {
			return fmt.Errorf("insert creator grant: %w", err)
		}
	}
	if in.OwnerOrgID != nil {
		if _, err := q.Exec(ctx, insertSubjectGrantSQL, pgx.NamedArgs{
			"subject_id": subjectID, "grantee_kind": string(SubjectKindOrgUnit), "grantee_subject_id": *in.OwnerOrgID,
			"level": int16(LevelManage), "granted_by": in.OperatorID, "reason": "owning unit of the subject",
		}); err != nil {
			return fmt.Errorf("insert owning unit grant: %w", err)
		}
	}
	return nil
}

// Relationship types whose grant rules differ from the default.
const (
	// relCaseHasDocument attaches a document to a case: CONTRIBUTE on the case suffices.
	relCaseHasDocument = "CASE_HAS_DOCUMENT"
)

// membershipTypes are the relationships that make a user part of a grantee;
// they are managed from the group or unit (MANAGE on the target), never from
// the user's own subject.
var membershipTypes = map[string]bool{"USER_MEMBER_OF_ORG_UNIT": true, "USER_MEMBER_OF_GROUP": true}

// ensureLinkAccessTx requires the levels a new relationship needs: MANAGE on the
// source (CONTRIBUTE to attach a document to a case) and READ on the target,
// or MANAGE on the group or unit for a membership.
func ensureLinkAccessTx(ctx context.Context, q Querier, operatorID string, source, target uuid.UUID, typeCode string) error {
	if membershipTypes[typeCode] {
		return EnsureAccessTx(ctx, q, operatorID, target, LevelManage)
	}
	need := LevelManage
	if typeCode == relCaseHasDocument {
		need = LevelContribute
	}
	if err := EnsureAccessTx(ctx, q, operatorID, source, need); err != nil {
		return err
	}
	return EnsureAccessTx(ctx, q, operatorID, target, LevelRead)
}

// ensureRelationshipChangeAccessTx requires the level to end or unlink an
// existing relationship: MANAGE on its source, or on its group or unit for a
// membership.
func ensureRelationshipChangeAccessTx(ctx context.Context, q Querier, operatorID string, rel *SubjectRelationship) error {
	var code string
	if err := q.QueryRow(ctx, relationshipTypeCodeSQL, pgx.NamedArgs{"id": rel.RelationshipTypeID}).Scan(&code); err != nil {
		return fmt.Errorf("relationship type: %w", err)
	}
	if membershipTypes[code] {
		return EnsureAccessTx(ctx, q, operatorID, rel.TargetSubjectID, LevelManage)
	}
	return EnsureAccessTx(ctx, q, operatorID, rel.SourceSubjectID, LevelManage)
}

// CopyGrantsTx gives to the current grants of from to subject to, for the
// grantees that have none there yet (a document deposited from a case).
func CopyGrantsTx(ctx context.Context, q Querier, from, to uuid.UUID, operatorID string) error {
	if _, err := q.Exec(ctx, copyGrantsSQL, pgx.NamedArgs{"from": from, "to": to, "operator_id": operatorID}); err != nil {
		return fmt.Errorf("copy grants: %w", err)
	}
	return nil
}

// IsAssigneeTx reports whether userID is the assigned user, or a direct member
// of the assigned org unit (a task or a circulation recipient): the assignee
// may act on its own work without a level on the case.
func IsAssigneeTx(ctx context.Context, q Querier, userID string, assigneeUserID *string, assigneeOrgUnitID *uuid.UUID) (bool, error) {
	if assigneeUserID != nil {
		return *assigneeUserID == userID, nil
	}
	if assigneeOrgUnitID == nil {
		return false, nil
	}
	var member bool
	if err := q.QueryRow(ctx, isUnitMemberSQL, pgx.NamedArgs{"user_id": userID, "unit_id": *assigneeOrgUnitID}).Scan(&member); err != nil {
		return false, fmt.Errorf("unit membership: %w", err)
	}
	return member, nil
}

// EnsureAssigneeOrAccessTx lets the assignee through, and otherwise requires
// need on the case.
func EnsureAssigneeOrAccessTx(ctx context.Context, q Querier, userID string, caseID uuid.UUID, assigneeUserID *string, assigneeOrgUnitID *uuid.UUID, need Level) error {
	assignee, err := IsAssigneeTx(ctx, q, userID, assigneeUserID, assigneeOrgUnitID)
	if err != nil || assignee {
		return err
	}
	return EnsureAccessTx(ctx, q, userID, caseID, need)
}
