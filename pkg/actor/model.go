package actor

import (
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Kind mirrors the actor.actor_kind column and the ActorKind proto enum.
type Kind int16

const (
	KindUnspecified  Kind = 0
	KindPerson       Kind = 1
	KindOrganization Kind = 2
)

// Valid reports whether k is a persisted actor kind.
func (k Kind) Valid() bool { return k == KindPerson || k == KindOrganization }

// ContactType mirrors the actor_contact.contact_type column and the ContactType
// proto enum. Values are kept in sync with proto/goeland/v1/actor.proto.
type ContactType int16

const (
	ContactTypeUnspecified        ContactType = 0
	ContactTypePhone              ContactType = 1
	ContactTypePhonePrivate       ContactType = 2
	ContactTypePhonePro           ContactType = 3
	ContactTypeMobile             ContactType = 4
	ContactTypeFax                ContactType = 5
	ContactTypeEmail              ContactType = 6
	ContactTypeWebsite            ContactType = 7
	ContactTypePostalBox          ContactType = 8
	ContactTypeIDEFederal         ContactType = 20
	ContactTypeVATNumber          ContactType = 21
	ContactTypeABACUSDebtor       ContactType = 22
	ContactTypeCommercialRegister ContactType = 23
	ContactTypeOther              ContactType = 99
)

// validContactTypes is the set of contact types accepted on the wire.
var validContactTypes = map[ContactType]struct{}{
	ContactTypePhone: {}, ContactTypePhonePrivate: {}, ContactTypePhonePro: {},
	ContactTypeMobile: {}, ContactTypeFax: {}, ContactTypeEmail: {},
	ContactTypeWebsite: {}, ContactTypePostalBox: {}, ContactTypeIDEFederal: {},
	ContactTypeVATNumber: {}, ContactTypeABACUSDebtor: {},
	ContactTypeCommercialRegister: {}, ContactTypeOther: {},
}

// Valid reports whether t is a known, persistable contact type (excludes the zero value).
func (t ContactType) Valid() bool {
	_, ok := validContactTypes[t]
	return ok
}

// OrganizationCategory is a controlled classification of organizations
// (production DicoActMoralCategory).
type OrganizationCategory struct {
	ID       uuid.UUID `db:"id"`
	Code     string    `db:"code"`
	Label    string    `db:"label"`
	IsActive bool      `db:"is_active"`
}

// Contact is one typed contact channel or business identifier of an actor
// (production ActeurComplement row).
type Contact struct {
	ID          uuid.UUID   `db:"id"`
	ActorID     uuid.UUID   `db:"actor_id"`
	ContactType ContactType `db:"contact_type"`
	Value       string      `db:"value"`
	IsPrimary   bool        `db:"is_primary"`
	Label       string      `db:"label"`
	CreatedAt   time.Time   `db:"created_at"`
}

// Actor is the actor-specific projection (1:1 with an ACTOR subject_ref).
// Nullable columns use pointers so pgx can scan SQL NULLs.
type Actor struct {
	ID              uuid.UUID  `db:"id"`
	ActorKind       Kind       `db:"actor_kind"`
	DisplayName     string     `db:"display_name"`
	NameForSearch   string     `db:"name_for_search"`
	IsActive        bool       `db:"is_active"`
	PublicationCode int32      `db:"publication_code"`
	LegalName       string     `db:"legal_name"`
	CategoryID      *uuid.UUID `db:"organization_category_id"`
	OrgComplement   string     `db:"org_complement"`
	IsCHRegister    bool       `db:"is_ch_register"`
	CHRegisterRef   string     `db:"ch_register_ref"`
	CreatedAt       time.Time  `db:"created_at"`
	CreatedBy       string     `db:"created_by"`
	UpdatedAt       time.Time  `db:"updated_at"`

	// Hydrated associations (nil on write paths).
	Subject        *core.SubjectRef      `db:"-"`
	RecordMetadata *core.RecordMetadata  `db:"-"`
	Category       *OrganizationCategory `db:"-"`
	Contacts       []*Contact            `db:"-"`
}

// ContactInput is a single contact carried on create/update.
type ContactInput struct {
	ContactType ContactType
	Value       string
	IsPrimary   bool
	Label       string
}

// CreateInput holds the client-controlled fields for a new actor.
type CreateInput struct {
	ActorKind       Kind
	DisplayName     string
	PublicationCode int32
	// ORGANIZATION specialization.
	LegalName    string
	CategoryCode string
	OrgComplement string
	// PERSON specialization (no PII).
	IsCHRegister  bool
	CHRegisterRef string

	Contacts   []ContactInput
	OperatorID string
	Governance core.CreateSubjectInput // owner/confidentiality carried from the request
}

// UpdateInput holds the mutable fields of an actor. Pointer fields are only
// applied when non-nil so partial updates leave untouched columns intact.
type UpdateInput struct {
	DisplayName     *string
	IsActive        *bool
	PublicationCode *int32
	// ORGANIZATION specialization.
	LegalName     *string
	CategoryCode  *string
	OrgComplement *string
	// PERSON specialization.
	IsCHRegister  *bool
	CHRegisterRef *string

	ReplaceContacts bool
	Contacts        []ContactInput

	OperatorID string
	Reason     string
}

// SearchFilter controls actor search.
type SearchFilter struct {
	Query              string
	ActorKind          Kind // KindUnspecified = any
	OrganizationCatCode string
	OnlyActive         bool
	IncludeDeleted     bool
	Limit              int
	Offset             int
}

// SearchResult holds a page of actors and the total count before pagination.
type SearchResult struct {
	Actors    []*Actor
	TotalSize int32
}
