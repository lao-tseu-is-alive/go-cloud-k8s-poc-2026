package actor

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Kind mirrors the actor.actor_kind column and the ActorKind proto enum.
type Kind int16

const (
	// KindUnspecified is the proto zero value; never persisted, and "any kind"
	// in SearchFilter.
	KindUnspecified Kind = 0
	// KindPerson is a physical person; it carries no personal data, only an
	// optional opaque population-register link.
	KindPerson Kind = 1
	// KindOrganization is a legal entity (company, association, authority...).
	KindOrganization Kind = 2
)

// Valid reports whether k is a persisted actor kind.
func (k Kind) Valid() bool { return k == KindPerson || k == KindOrganization }

// ContactType mirrors the actor_contact.contact_type column and the ContactType
// proto enum. Values are kept in sync with proto/goeland/v1/actor.proto.
type ContactType int16

// Values 1-19 are contact channels; 20-29 are business identifiers looked up
// by value.
const (
	// ContactTypeUnspecified is the proto zero value; rejected on input.
	ContactTypeUnspecified ContactType = 0
	// ContactTypePhone is a general phone number.
	ContactTypePhone ContactType = 1
	// ContactTypePhonePrivate is a private phone number.
	ContactTypePhonePrivate ContactType = 2
	// ContactTypePhonePro is a professional phone number.
	ContactTypePhonePro ContactType = 3
	// ContactTypeMobile is a mobile phone number.
	ContactTypeMobile ContactType = 4
	// ContactTypeFax is a fax number.
	ContactTypeFax ContactType = 5
	// ContactTypeEmail is an e-mail address.
	ContactTypeEmail ContactType = 6
	// ContactTypeWebsite is a website URL.
	ContactTypeWebsite ContactType = 7
	// ContactTypePostalBox is a postal box (case postale).
	ContactTypePostalBox ContactType = 8
	// ContactTypeIDEFederal is the Swiss federal business identifier (IDE/UID).
	ContactTypeIDEFederal ContactType = 20
	// ContactTypeVATNumber is a VAT (TVA) number.
	ContactTypeVATNumber ContactType = 21
	// ContactTypeABACUSDebtor is the debtor number in the ABACUS accounting system.
	ContactTypeABACUSDebtor ContactType = 22
	// ContactTypeCommercialRegister is a commercial register (registre du
	// commerce) reference.
	ContactTypeCommercialRegister ContactType = 23
	// ContactTypeOther is any other channel; Label should describe it.
	ContactTypeOther ContactType = 99
)

// Salutation is how a person is addressed; it mirrors the Salutation proto enum
// and the actor.salutation column.
type Salutation int16

const (
	// SalutationUnspecified means not specified (address by name only).
	SalutationUnspecified Salutation = 0
	// SalutationMadame is "Madame".
	SalutationMadame Salutation = 1
	// SalutationMonsieur is "Monsieur".
	SalutationMonsieur Salutation = 2
	// SalutationNeutral is a neutral form of address (names, no title).
	SalutationNeutral Salutation = 3
)

// Valid reports whether s is a known salutation (the zero value included).
func (s Salutation) Valid() bool {
	return s >= SalutationUnspecified && s <= SalutationNeutral
}

// MaxPersonNameLength is the maximum number of code points of a first or last name.
const MaxPersonNameLength = 100

// contactTypeNames lists the persistable contact types (the zero value is
// excluded) with the name used in messages.
var contactTypeNames = map[ContactType]string{
	ContactTypePhone: "phone", ContactTypePhonePrivate: "private phone", ContactTypePhonePro: "professional phone",
	ContactTypeMobile: "mobile", ContactTypeFax: "fax", ContactTypeEmail: "e-mail",
	ContactTypeWebsite: "website", ContactTypePostalBox: "postal box", ContactTypeIDEFederal: "IDE",
	ContactTypeVATNumber: "VAT number", ContactTypeABACUSDebtor: "ABACUS debtor",
	ContactTypeCommercialRegister: "commercial register", ContactTypeOther: "other",
}

// Valid reports whether t is a known, persistable contact type (excludes the zero value).
func (t ContactType) Valid() bool {
	_, ok := contactTypeNames[t]
	return ok
}

// String returns the contact type name used in messages.
func (t ContactType) String() string {
	if name, ok := contactTypeNames[t]; ok {
		return name
	}
	return fmt.Sprintf("contact type %d", int16(t))
}

// OrganizationCategory is a controlled classification of organizations
// (production DicoActMoralCategory).
// Categories are seeded reference data.
type OrganizationCategory struct {
	// ID is the catalogue row identity.
	ID uuid.UUID `db:"id"`
	// Code is the unique, non-blank stable key used by APIs.
	Code string `db:"code"`
	// Label is the human label.
	Label string `db:"label"`
	// IsActive reports whether the category is offered for new organizations.
	IsActive bool `db:"is_active"`
}

// Contact is one typed contact channel or business identifier of an actor
// (production ActeurComplement row).
type Contact struct {
	// ID is the contact row identity.
	ID uuid.UUID `db:"id"`
	// ActorID is the owning actor.
	ActorID uuid.UUID `db:"actor_id"`
	// ContactType is a Valid, non-zero contact type.
	ContactType ContactType `db:"contact_type"`
	// Value is the trimmed, non-blank channel or identifier, at most
	// MaxContactValueLength code points.
	Value string `db:"value"`
	// IsPrimary marks the preferred contact; not enforced as unique.
	IsPrimary bool `db:"is_primary"`
	// Label qualifies the contact in free text; empty when not needed.
	Label string `db:"label"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
}

// Actor is the actor-specific projection (1:1 with an ACTOR subject_ref).
// Nullable columns use pointers so pgx can scan SQL NULLs.
//
// Roles are never columns: an actor is attached to cases and documents only
// through typed core relationships (CASE_HAS_ACTOR_*, DOCUMENT_*_ACTOR).
// Database CHECK constraints keep the organization-only and person-only
// fields empty for the other kind. The generated search_vector column is
// deliberately absent: the application never writes or scans it.
type Actor struct {
	// ID is the ACTOR subject_ref ID; a composite foreign key pins the kind.
	ID uuid.UUID `db:"id"`
	// ActorKind is PERSON or ORGANIZATION, fixed at creation.
	ActorKind Kind `db:"actor_kind"`
	// DisplayName is the trimmed, non-blank name, at most MaxDisplayNameLength
	// code points; it is mirrored into the subject's display label.
	DisplayName string `db:"display_name"`
	// NameForSearch is the lower-cased DisplayName, recomputed on every rename.
	NameForSearch string `db:"name_for_search"`
	// IsActive is the business activation flag, distinct from soft deletion.
	IsActive bool `db:"is_active"`
	// PublicationCode is the opaque, non-negative legacy publication code.
	PublicationCode int32 `db:"publication_code"`
	// LegalName is the organization's legal name (raison sociale), required
	// for an organization and empty for a person.
	LegalName string `db:"legal_name"`
	// CategoryID references the OrganizationCategory; nil for a person or an
	// uncategorized organization.
	CategoryID *uuid.UUID `db:"organization_category_id"`
	// OrgComplement is an organization name complement; empty for a person.
	OrgComplement string `db:"org_complement"`
	// IsCHRegister reports whether the person is known to the population
	// register; always false for an organization.
	IsCHRegister bool `db:"is_ch_register"`
	// CHRegisterRef is an opaque key into the population register, never civil
	// data; empty for an organization.
	CHRegisterRef string `db:"ch_register_ref"`
	// Salutation is a person's form of address; unspecified for an organization.
	Salutation Salutation `db:"salutation"`
	// LastName is a person's family name; empty for an organization (and for
	// persons recorded before GLD-039).
	LastName string `db:"last_name"`
	// FirstName is a person's given name or names; empty when unknown or for an
	// organization.
	FirstName string `db:"first_name"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who created the actor.
	CreatedBy string `db:"created_by"`
	// UpdatedAt is maintained by a database trigger on every update.
	UpdatedAt time.Time `db:"updated_at"`

	// Subject is the hydrated identity on read paths; nil on writes.
	Subject *core.SubjectRef `db:"-"`
	// RecordMetadata is the hydrated governance record on read paths; nil on writes.
	RecordMetadata *core.RecordMetadata `db:"-"`
	// Category is the hydrated organization category; nil when CategoryID is nil.
	Category *OrganizationCategory `db:"-"`
	// Contacts are the hydrated contacts ordered by type then creation time.
	Contacts []*Contact `db:"-"`
	// Addresses are the current addresses, principal first; hydrated by the
	// repository.
	Addresses []*Address `db:"-"`
}

// ContactInput is a single contact carried on create/update.
//
// The service trims Value and Label and rejects an invalid type or a blank or
// over-long value with core.ErrInvalidInput.
type ContactInput struct {
	// ContactType must be Valid.
	ContactType ContactType
	// Value is the channel or identifier; required.
	Value string
	// IsPrimary marks the preferred contact.
	IsPrimary bool
	// Label optionally qualifies the contact.
	Label string
}

// CreateInput holds the client-controlled fields for a new actor.
//
// Service.Create persists the subject, governance record, actor, contacts and
// audit event in one transaction. Fields of the other kind are cleared.
type CreateInput struct {
	// ActorKind must be KindPerson or KindOrganization.
	ActorKind Kind
	// DisplayName is required; surrounding whitespace is trimmed.
	DisplayName string
	// PublicationCode is the opaque legacy publication code.
	PublicationCode int32

	// LegalName is required for an organization (at most
	// MaxDisplayNameLength code points).
	LegalName string
	// CategoryCode optionally selects an organization category by code.
	CategoryCode string
	// OrgComplement is an optional organization name complement.
	OrgComplement string

	// IsCHRegister flags a person known to the population register.
	IsCHRegister bool
	// CHRegisterRef is the opaque register key of a person; never civil data.
	CHRegisterRef string
	// Salutation is a person's form of address.
	Salutation Salutation
	// LastName is a person's required family name (at most MaxPersonNameLength).
	LastName string
	// FirstName is a person's optional given name (at most MaxPersonNameLength).
	FirstName string

	// Contacts are the initial contacts.
	Contacts []ContactInput
	// Addresses are the initial addresses (see normalizeAddresses).
	Addresses []AddressInput
	// OperatorID is the authenticated caller, set server-side; it becomes
	// created_by and the audit actor.
	OperatorID string
	// Governance carries the requested owner and confidentiality; the service
	// overwrites its identity fields from the actor.
	Governance core.CreateSubjectInput
}

// UpdateInput holds the mutable fields of an actor. Pointer fields are only
// applied when non-nil so partial updates leave untouched columns intact.
//
// The update fails with core.ErrLocked or core.ErrDeleted when the record is
// not mutable; setting a field of the other kind is rejected by the database.
type UpdateInput struct {
	// DisplayName renames the actor and resyncs the subject label when non-nil.
	DisplayName *string
	// IsActive activates or deactivates the actor when non-nil.
	IsActive *bool
	// PublicationCode replaces the publication code when non-nil.
	PublicationCode *int32

	// LegalName replaces an organization's legal name when non-nil.
	LegalName *string
	// CategoryCode replaces an organization's category when non-nil.
	CategoryCode *string
	// OrgComplement replaces an organization's name complement when non-nil.
	OrgComplement *string

	// IsCHRegister replaces a person's register flag when non-nil.
	IsCHRegister *bool
	// CHRegisterRef replaces a person's opaque register key when non-nil.
	CHRegisterRef *string
	// Salutation replaces a person's form of address when non-nil.
	Salutation *Salutation
	// LastName replaces a person's family name when non-nil; it must not be blank.
	LastName *string
	// FirstName replaces a person's given name when non-nil.
	FirstName *string

	// ReplaceContacts, when true, replaces the whole contact list by Contacts;
	// when false, Contacts is ignored and existing contacts are kept.
	ReplaceContacts bool
	// Contacts is the new contact list, used only with ReplaceContacts.
	Contacts []ContactInput
	// ReplaceAddresses, when true, replaces the current addresses by Addresses
	// (previous links are ended, not deleted); when false, Addresses is ignored.
	ReplaceAddresses bool
	// Addresses is the new address set, used only with ReplaceAddresses.
	Addresses []AddressInput

	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// SearchFilter controls actor search.
// Results are ordered by display name.
type SearchFilter struct {
	// Query is an accent-insensitive full-text query over the display and
	// legal names; empty matches every actor.
	Query string
	// ActorKind restricts results to one kind; KindUnspecified means any.
	ActorKind Kind
	// OrganizationCatCode restricts results to one organization category;
	// empty means any, and an unknown code matches nothing.
	OrganizationCatCode string
	// OnlyActive restricts results to business-active actors.
	OnlyActive bool
	// IncludeDeleted also returns soft-deleted actors.
	IncludeDeleted bool
	// Limit is the page size, normalized to [1, core.MaxPageSize].
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// SearchResult holds a page of actors and the total count before pagination.
type SearchResult struct {
	// Actors is the requested page.
	Actors []*Actor
	// TotalSize is the number of matching actors across all pages.
	TotalSize int32
}
