package actor

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// AddressType is the role of an address for one actor; it mirrors the
// AddressType proto enum and the actor_address.address_type column.
type AddressType int16

const (
	// AddressTypeUnspecified is the proto zero value; rejected on input.
	AddressTypeUnspecified AddressType = 0
	// AddressTypeHeadOffice is an organization's registered office (siège).
	AddressTypeHeadOffice AddressType = 1
	// AddressTypeBranch is a branch or establishment of the same actor.
	AddressTypeBranch AddressType = 2
	// AddressTypeCorrespondence is where mail is sent.
	AddressTypeCorrespondence AddressType = 3
	// AddressTypeBilling is where invoices are sent.
	AddressTypeBilling AddressType = 4
	// AddressTypeResidence is a person's home (domicile).
	AddressTypeResidence AddressType = 5
	// AddressTypeOther is any other role; the label must describe it.
	AddressTypeOther AddressType = 6
)

// Valid reports whether t is a persistable address type (excludes the zero value).
func (t AddressType) Valid() bool {
	return t >= AddressTypeHeadOffice && t <= AddressTypeOther
}

const (
	// MaxAddresses bounds the addresses of one actor.
	MaxAddresses = 20
	// defaultCountryCode is used when an address has no country.
	defaultCountryCode = "CH"
)

var (
	countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)
	swissPostalCode    = regexp.MustCompile(`^[1-9]\d{3}$`)
)

// Address is one current address of an actor: the actor_address link (its id,
// role, principal flag and label) joined with the address row.
type Address struct {
	// ID is the actor_address link identity.
	ID uuid.UUID `db:"id"`
	// ActorID is the linked actor.
	ActorID uuid.UUID `db:"actor_id"`
	// AddressID is the linked address row.
	AddressID uuid.UUID `db:"address_id"`
	// AddressType is the role of the address for this actor.
	AddressType AddressType `db:"address_type"`
	// IsPrincipal marks the actor's principal address (at most one current).
	IsPrincipal bool `db:"is_principal"`
	// Label qualifies the link in free text; required for AddressTypeOther.
	Label string `db:"label"`
	// CreatedAt is when the address was linked to the actor.
	CreatedAt time.Time `db:"created_at"`
	// Street is the street name.
	Street string `db:"street"`
	// HouseNumber is the number with its suffix; empty when none.
	HouseNumber string `db:"house_number"`
	// AddressLine2 is a complement such as c/o, building or floor.
	AddressLine2 string `db:"address_line2"`
	// PostalCode is the postal code (four digits in Switzerland).
	PostalCode string `db:"postal_code"`
	// Locality is the locality name.
	Locality string `db:"locality"`
	// CountryCode is the ISO 3166-1 alpha-2 country.
	CountryCode string `db:"country_code"`
}

// AddressInput is a user-supplied address of an actor.
type AddressInput struct {
	// AddressType is the required role of the address.
	AddressType AddressType
	// IsPrincipal marks the principal address; the first one is used when none is.
	IsPrincipal bool
	// Street is the required street name (at most 200 code points).
	Street string
	// HouseNumber is the optional number (at most 20 code points).
	HouseNumber string
	// AddressLine2 is an optional complement (at most 200 code points).
	AddressLine2 string
	// PostalCode is the required postal code (at most 20; four digits in CH).
	PostalCode string
	// Locality is the required locality (at most 100 code points).
	Locality string
	// CountryCode is the ISO 3166-1 alpha-2 country; empty means CH.
	CountryCode string
	// Label is an optional note (at most 100); required for AddressTypeOther.
	Label string
}

// normalizeAddresses trims and validates addresses and settles the principal
// one: more than one is rejected, and the first becomes principal when none is.
func normalizeAddresses(in []AddressInput) ([]AddressInput, error) {
	if len(in) > MaxAddresses {
		return nil, fmt.Errorf("%w: at most %d addresses", core.ErrInvalidInput, MaxAddresses)
	}
	out := make([]AddressInput, 0, len(in))
	principals := 0
	for i, a := range in {
		normalized, err := normalizeAddress(a)
		if err != nil {
			return nil, fmt.Errorf("%w (address %d)", err, i+1)
		}
		if normalized.IsPrincipal {
			principals++
		}
		out = append(out, normalized)
	}
	switch {
	case principals > 1:
		return nil, fmt.Errorf("%w: only one address can be principal", core.ErrInvalidInput)
	case principals == 0 && len(out) > 0:
		out[0].IsPrincipal = true
	}
	return out, nil
}

// normalizeAddress trims one address, defaults the country to CH and checks
// the required fields, lengths and the Swiss postal code format.
func normalizeAddress(a AddressInput) (AddressInput, error) {
	a.Street = strings.TrimSpace(a.Street)
	a.HouseNumber = strings.TrimSpace(a.HouseNumber)
	a.AddressLine2 = strings.TrimSpace(a.AddressLine2)
	a.PostalCode = strings.TrimSpace(a.PostalCode)
	a.Locality = strings.TrimSpace(a.Locality)
	a.Label = strings.TrimSpace(a.Label)
	a.CountryCode = strings.ToUpper(strings.TrimSpace(a.CountryCode))
	if a.CountryCode == "" {
		a.CountryCode = defaultCountryCode
	}
	switch {
	case !a.AddressType.Valid():
		return a, fmt.Errorf("%w: invalid address_type", core.ErrInvalidInput)
	case a.Street == "" || a.PostalCode == "" || a.Locality == "":
		return a, fmt.Errorf("%w: street, postal_code and locality are required", core.ErrInvalidInput)
	case !countryCodePattern.MatchString(a.CountryCode):
		return a, fmt.Errorf("%w: country_code must be an ISO 3166-1 alpha-2 code", core.ErrInvalidInput)
	case a.CountryCode == defaultCountryCode && !swissPostalCode.MatchString(a.PostalCode):
		return a, fmt.Errorf("%w: a Swiss postal code has four digits (1000-9999)", core.ErrInvalidInput)
	case a.AddressType == AddressTypeOther && a.Label == "":
		return a, fmt.Errorf("%w: an address of type OTHER needs a label describing it", core.ErrInvalidInput)
	}
	return a, checkAddressLengths(a)
}

// checkAddressLengths enforces the per-field limits (code points).
func checkAddressLengths(a AddressInput) error {
	limits := []struct {
		name  string
		value string
		max   int
	}{
		{"street", a.Street, 200}, {"house_number", a.HouseNumber, 20}, {"address_line2", a.AddressLine2, 200},
		{"postal_code", a.PostalCode, 20}, {"locality", a.Locality, 100}, {"label", a.Label, 100},
	}
	for _, l := range limits {
		if utf8.RuneCountInString(l.value) > l.max {
			return fmt.Errorf("%w: %s exceeds %d characters", core.ErrInvalidInput, l.name, l.max)
		}
	}
	return nil
}
