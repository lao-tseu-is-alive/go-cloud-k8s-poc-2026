package orgunit

import (
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// OrgUnitType is a controlled classification of units (seeded from the seven
// production types, administrable).
type OrgUnitType struct {
	// ID is the catalogue row identity.
	ID uuid.UUID `db:"id"`
	// Code is the unique, non-blank stable key (e.g. SERVICE).
	Code string `db:"code"`
	// Label is the human label.
	Label string `db:"label"`
	// Description documents the business meaning of the type.
	Description string `db:"description"`
	// SortOrder orders the types from the top of the hierarchy down.
	SortOrder int32 `db:"sort_order"`
	// IsActive reports whether the type is offered for new units.
	IsActive bool `db:"is_active"`
}

// OrgUnit is the org_unit row (1:1 with an ORG_UNIT subject_ref). The
// generated search_vector column is deliberately absent.
type OrgUnit struct {
	// ID is the ORG_UNIT subject_ref ID; a composite foreign key pins the kind.
	ID uuid.UUID `db:"id"`
	// TypeID references the unit's OrgUnitType.
	TypeID uuid.UUID `db:"org_unit_type_id"`
	// Abbreviation is the unit sigle, often shared with the parent (not unique).
	Abbreviation string `db:"abbreviation"`
	// Label is the unit name, unique among live siblings.
	Label string `db:"label"`
	// Description documents the unit's mission.
	Description string `db:"description"`
	// Email is the functional mailbox; empty when none.
	Email string `db:"email"`
	// ParentID is the parent unit; nil for a root.
	ParentID *uuid.UUID `db:"parent_id"`
	// ExternalRef is the id in a source system, unique when set and immutable.
	ExternalRef string `db:"external_ref"`
	// DissolvedAt is set when the unit was dissolved.
	DissolvedAt *time.Time `db:"dissolved_at"`
	// DissolvedBy is the operator who dissolved the unit.
	DissolvedBy string `db:"dissolved_by"`
	// DissolutionReason is the non-blank justification of a dissolution.
	DissolutionReason string `db:"dissolution_reason"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who created the unit.
	CreatedBy string `db:"created_by"`
	// UpdatedAt is maintained by a database trigger on every update.
	UpdatedAt time.Time `db:"updated_at"`

	// Subject is the hydrated identity on read paths.
	Subject *core.SubjectRef `db:"-"`
	// RecordMetadata is the hydrated governance record on read paths.
	RecordMetadata *core.RecordMetadata `db:"-"`
	// Type is the hydrated unit type on read paths.
	Type *OrgUnitType `db:"-"`
}

// Dissolved reports whether the unit is dissolved.
func (u *OrgUnit) Dissolved() bool { return u.DissolvedAt != nil }

// DisplayLabel is the subject label of a unit: "Label (ABBR)", or the label
// alone without abbreviation. The abbreviation tells homonyms apart
// ("Secrétariat" exists in many services).
func DisplayLabel(label, abbreviation string) string {
	if abbreviation == "" {
		return label
	}
	return label + " (" + abbreviation + ")"
}

// Node is the light form of a unit used to draw the tree and paths.
type Node struct {
	// ID is the unit identity.
	ID uuid.UUID `db:"id"`
	// Abbreviation is the unit sigle.
	Abbreviation string `db:"abbreviation"`
	// Label is the unit name.
	Label string `db:"label"`
	// TypeCode is the code of the unit type.
	TypeCode string `db:"type_code"`
	// ParentID is the parent unit; nil for a root.
	ParentID *uuid.UUID `db:"parent_id"`
	// Dissolved reports whether the unit is dissolved.
	Dissolved bool `db:"dissolved"`
}

// Detail is a unit with its place in the tree.
type Detail struct {
	// Unit is the hydrated unit.
	Unit *OrgUnit
	// Ancestors are the units above it, from the root down to its parent.
	Ancestors []*Node
	// Children are the units directly below it, dissolved ones included.
	Children []*Node
}

// Input holds the editable fields shared by create and update.
type Input struct {
	// TypeCode is the code of an active unit type (on update the current type
	// stays valid even when deactivated).
	TypeCode string
	// Abbreviation is the optional sigle; surrounding whitespace is trimmed.
	Abbreviation string
	// Label is required, unique among live siblings; whitespace is trimmed.
	Label string
	// Description is optional.
	Description string
	// Email is an optional plain address, stored lower-cased.
	Email string
	// ParentID is a live unit; nil makes a root.
	ParentID *uuid.UUID
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// CreateInput is a new unit: the editable fields plus its immutable external
// reference.
type CreateInput struct {
	Input
	// ExternalRef is the optional id in a source system, unique when set.
	ExternalRef string
}

// SearchFilter controls unit search, ordered by label.
type SearchFilter struct {
	// Viewer is who searches: only the subjects it may read are returned (GLD-049).
	Viewer core.Viewer
	// Query is matched accent-insensitively against abbreviation, label and
	// description, or exactly against the abbreviation (any case) or the
	// external reference.
	Query string
	// IncludeDissolved also returns dissolved units.
	IncludeDissolved bool
	// Limit is the page size, normalized to [1, core.MaxPageSize].
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// SearchResult holds a page of units and the total count before pagination.
type SearchResult struct {
	// Units is the requested page.
	Units []*OrgUnit
	// TotalSize is the number of matching units across all pages.
	TotalSize int32
}
