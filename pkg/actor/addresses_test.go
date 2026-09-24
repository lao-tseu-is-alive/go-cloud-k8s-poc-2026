package actor

import (
	"errors"
	"strings"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

func validAddress() AddressInput {
	return AddressInput{AddressType: AddressTypeHeadOffice, Street: " Place de la Palud ", HouseNumber: "2", PostalCode: "1003", Locality: "Lausanne"}
}

func TestNormalizeAddressesDefaults(t *testing.T) {
	branch := validAddress()
	branch.AddressType, branch.PostalCode, branch.Locality = AddressTypeBranch, "1260", "Nyon"
	got, err := normalizeAddresses([]AddressInput{validAddress(), branch})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Street != "Place de la Palud" || got[0].CountryCode != "CH" {
		t.Fatalf("not trimmed or country not defaulted: %+v", got[0])
	}
	if !got[0].IsPrincipal || got[1].IsPrincipal {
		t.Fatalf("the first address must become principal when none is: %+v", got)
	}
	foreign := validAddress()
	foreign.CountryCode, foreign.PostalCode = "fr", "75001"
	if got, err := normalizeAddresses([]AddressInput{foreign}); err != nil || got[0].CountryCode != "FR" {
		t.Fatalf("foreign postal codes are free and the country upper-cased: %+v, %v", got, err)
	}
}

func TestNormalizeAddressesRejects(t *testing.T) {
	mutate := func(f func(*AddressInput)) []AddressInput {
		a := validAddress()
		f(&a)
		return []AddressInput{a}
	}
	twoPrincipals := []AddressInput{validAddress(), validAddress()}
	twoPrincipals[0].IsPrincipal, twoPrincipals[1].IsPrincipal = true, true
	tooMany := make([]AddressInput, MaxAddresses+1)
	for i := range tooMany {
		tooMany[i] = validAddress()
	}
	cases := map[string][]AddressInput{
		"unspecified type":    mutate(func(a *AddressInput) { a.AddressType = AddressTypeUnspecified }),
		"missing street":      mutate(func(a *AddressInput) { a.Street = " " }),
		"missing locality":    mutate(func(a *AddressInput) { a.Locality = "" }),
		"swiss postal code":   mutate(func(a *AddressInput) { a.PostalCode = "10003" }),
		"swiss postal zero":   mutate(func(a *AddressInput) { a.PostalCode = "0999" }),
		"bad country":         mutate(func(a *AddressInput) { a.CountryCode = "CHE" }),
		"other without label": mutate(func(a *AddressInput) { a.AddressType = AddressTypeOther }),
		"long street":         mutate(func(a *AddressInput) { a.Street = strings.Repeat("x", 201) }),
		"two principals":      twoPrincipals,
		"too many":            tooMany,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeAddresses(in); !errors.Is(err, core.ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}
