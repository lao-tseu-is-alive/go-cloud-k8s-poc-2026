package legacyimport

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// sourceActor is a legacy actor with its person or organization specialization.
type sourceActor struct {
	// ID is the source column id.
	ID int64 `db:"id"`
	// Person is the source column person.
	Person bool `db:"person"`
	// Name is the source column name.
	Name string `db:"name"`
	// NameForSearch is the source column name_for_search.
	NameForSearch string `db:"name_for_search"`
	// Active is the source column active.
	Active bool `db:"active"`
	// PublicationCode is the source column publication_code.
	PublicationCode int32 `db:"publication_code"`
	// CreatedAt is the source column created_at.
	CreatedAt *time.Time `db:"created_at"`
	// CreatorID is the source column creator_id.
	CreatorID *int64 `db:"creator_id"`
	// TitleID is the source column title_id.
	TitleID *int64 `db:"title_id"`
	// LastName is the source column last_name.
	LastName string `db:"last_name"`
	// FirstName is the source column first_name.
	FirstName string `db:"first_name"`
	// LegalName is the source column legal_name.
	LegalName string `db:"legal_name"`
	// Category is the source column category.
	Category string `db:"category"`
	// OrgComplement is the source column org_complement.
	OrgComplement string `db:"org_complement"`
	// RegisterID is the source column register_id.
	RegisterID *int64 `db:"register_id"`
}

var actorColumns = []string{
	"id", "actor_kind", "display_name", "name_for_search", "is_active", "publication_code", "legal_name",
	"organization_category_id", "org_complement", "is_ch_register", "ch_register_ref", "created_at", "created_by",
	"salutation", "last_name", "first_name",
}

// importActors writes the actors: a person keeps its salutation, last and
// first name only (GLD-039) and the register link as a flag and an opaque
// reference; an organization its legal name, category (matched by label) and
// complement.
func (imp *Importer) importActors(ctx context.Context, c *StageCounts) error {
	actors, err := querySource[sourceActor](ctx, imp, actorsSQL)
	if err != nil {
		return err
	}
	c.Read = len(actors)
	categories, err := imp.codeIDs(ctx, `SELECT label, id FROM organization_category`)
	if err != nil {
		return err
	}
	subjects := make([]subject, 0, len(actors))
	rows := make([][]any, 0, len(actors))
	for _, a := range actors {
		s := newSubject(core.SubjectKindActor, "acteur", a.ID, actorDisplayName(a))
		s.createdAt, s.createdBy = a.CreatedAt, imp.operator(a.CreatorID)
		subjects = append(subjects, s)
		rows = append(rows, imp.actorRow(s, a, categories, c))
	}
	if err := imp.writeSubjects(ctx, subjects); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "actor", actorColumns, rows); err != nil {
		return err
	}
	c.Written = len(rows)
	return nil
}

// actorDisplayName is the legacy name, or one built from the parts.
func actorDisplayName(a *sourceActor) string {
	for _, name := range []string{a.Name, strings.TrimSpace(a.FirstName + " " + a.LastName), a.LegalName} {
		if name != "" {
			return name
		}
	}
	return fmt.Sprintf("Acteur %d", a.ID)
}

// actorRow is the actor row of a legacy actor.
func (imp *Importer) actorRow(s subject, a *sourceActor, categories map[string]uuid.UUID, c *StageCounts) []any {
	created := firstTime(a.CreatedAt, &imp.now)
	if a.Person {
		salutation := int16(0)
		if a.TitleID != nil && (*a.TitleID == 1 || *a.TitleID == 2) { // Mme, M. (same numbers in the POC)
			salutation = int16(*a.TitleID)
		}
		last := a.LastName
		if last == "" {
			last = s.label
			c.adjust("person without a last name, display name used")
		}
		ref := ""
		if a.RegisterID != nil {
			ref = strconv.FormatInt(*a.RegisterID, 10)
		}
		return []any{s.id, int16(1), s.label, a.NameForSearch, a.Active, a.PublicationCode, "", nil, "", a.RegisterID != nil, ref,
			created, s.createdBy, salutation, last, a.FirstName}
	}
	legal := a.LegalName
	if legal == "" {
		legal = s.label
		c.adjust("organization without a legal name, display name used")
	}
	var category *uuid.UUID
	if id, ok := categories[a.Category]; ok {
		category = &id
	} else if a.Category != "" {
		c.adjust("unknown organization category, left empty")
	}
	return []any{s.id, int16(2), s.label, a.NameForSearch, a.Active, a.PublicationCode, legal, category, a.OrgComplement, false, "",
		created, s.createdBy, int16(0), "", ""}
}

// contactTypes maps the legacy complement types to the POC contact types; the
// others become OTHER with the legacy label, except the financial ones.
var contactTypes = map[int64]actor.ContactType{
	1:  actor.ContactTypePhone,
	2:  actor.ContactTypePhonePrivate,
	3:  actor.ContactTypePhonePro,
	4:  actor.ContactTypeFax,
	5:  actor.ContactTypeEmail,
	6:  actor.ContactTypePostalBox,
	8:  actor.ContactTypeWebsite,
	9:  actor.ContactTypeVATNumber,
	21: actor.ContactTypeIDEFederal,
	22: actor.ContactTypeCommercialRegister,
	24: actor.ContactTypeABACUSDebtor,
}

// financialTypes are bank references (CCP, IBAN): not needed by the POC, not imported.
var financialTypes = map[int64]bool{10: true, 19: true}

var actorContactColumns = []string{"actor_id", "contact_type", "value", "label"}

// importActorContacts writes the typed complements, normalized and checked by
// the same rules as the API (actor.NormalizeContactValue).
func (imp *Importer) importActorContacts(ctx context.Context, c *StageCounts) error {
	n, err := imp.copyFromSource(ctx, "actor_contact", actorContactColumns, actorContactsSQL, func(rows pgx.Rows) ([]any, error) {
		var actorID, typeID int64
		var typeLabel, value string
		if err := rows.Scan(&actorID, &typeID, &typeLabel, &value); err != nil {
			return nil, err
		}
		c.Read++
		return imp.contactRow(ID(string(core.SubjectKindActor), actorID), typeID, typeLabel, value, c), nil
	})
	c.Written = n
	return err
}

// contactRow maps one complement, or returns nil (counted) to leave it out.
func (imp *Importer) contactRow(actorID uuid.UUID, typeID int64, typeLabel, value string, c *StageCounts) []any {
	if !imp.known(actorID, core.SubjectKindActor) {
		c.skip("actor not imported")
		return nil
	}
	if financialTypes[typeID] {
		c.skip("financial reference (not imported)")
		return nil
	}
	contactType, label := contactTypes[typeID], ""
	if contactType == actor.ContactTypeUnspecified {
		contactType, label = actor.ContactTypeOther, typeLabel
	}
	normalized, err := actor.NormalizeContactValue(contactType, value)
	if err != nil || strings.TrimSpace(normalized) == "" {
		c.skip(fmt.Sprintf("invalid %s", contactType))
		return nil
	}
	return []any{actorID, int16(contactType), normalized, label}
}

// sourceAddress is the legacy correspondence address of an actor.
type sourceAddress struct {
	// ActorID is the source column actor_id.
	ActorID int64 `db:"actor_id"`
	// Street is the source column street.
	Street string `db:"street"`
	// HouseNumber is the source column house_number.
	HouseNumber string `db:"house_number"`
	// PostalBox is the source column postal_box.
	PostalBox string `db:"postal_box"`
	// PostalCode is the source column postal_code.
	PostalCode string `db:"postal_code"`
	// Locality is the source column locality.
	Locality string `db:"locality"`
	// Country is the source column country.
	Country string `db:"country"`
}

var (
	addressColumns      = []string{"id", "street", "house_number", "address_line2", "postal_code", "locality", "country_code", "created_by"}
	actorAddressColumns = []string{"actor_id", "address_id", "address_type", "is_principal", "created_by"}
	swissPostalCode     = regexp.MustCompile(`^[1-9][0-9]{3}$`)
)

// importActorAddresses writes the correspondence address of each actor as its
// principal address, when street, postal code and locality are known; the
// country is matched by its French name (blank means Switzerland).
func (imp *Importer) importActorAddresses(ctx context.Context, c *StageCounts) error {
	addresses, err := querySource[sourceAddress](ctx, imp, actorAddressesSQL)
	if err != nil {
		return err
	}
	c.Read = len(addresses)
	countries, err := imp.countries(ctx)
	if err != nil {
		return err
	}
	var rows, links [][]any
	for _, a := range addresses {
		row := imp.addressRow(a, countries, c)
		if row == nil {
			continue
		}
		rows = append(rows, row)
		links = append(links, []any{ID(string(core.SubjectKindActor), a.ActorID), row[0], int16(actor.AddressTypeCorrespondence), true, OperatorID})
	}
	if err := imp.copyRows(ctx, "address", addressColumns, rows); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "actor_address", actorAddressColumns, links); err != nil {
		return err
	}
	c.Written = len(rows)
	return nil
}

// addressRow maps one address, or returns nil (counted) to leave it out.
func (imp *Importer) addressRow(a *sourceAddress, countries map[string]string, c *StageCounts) []any {
	if !imp.known(ID(string(core.SubjectKindActor), a.ActorID), core.SubjectKindActor) {
		c.skip("actor not imported")
		return nil
	}
	if a.Street == "" || a.PostalCode == "" || a.Locality == "" {
		c.skip("incomplete (street, postal code or locality missing)")
		return nil
	}
	country := "CH"
	if a.Country != "" {
		code, ok := countries[a.Country]
		if !ok {
			c.skip("unknown country")
			return nil
		}
		country = code
	}
	if country == "CH" && !swissPostalCode.MatchString(a.PostalCode) {
		c.skip("invalid Swiss postal code")
		return nil
	}
	line2 := ""
	if a.PostalBox != "" {
		line2 = "Case postale " + a.PostalBox
	}
	return []any{ID(keyAddress, a.ActorID), a.Street, a.HouseNumber, line2, a.PostalCode, a.Locality, country, OperatorID}
}

// countries reads the legacy country names with their ISO code.
func (imp *Importer) countries(ctx context.Context) (map[string]string, error) {
	rows, err := imp.source.Query(ctx, countriesSQL)
	if err != nil {
		return nil, fmt.Errorf("read countries: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, code string
		if err := rows.Scan(&name, &code); err != nil {
			return nil, err
		}
		out[name] = code
	}
	return out, rows.Err()
}
