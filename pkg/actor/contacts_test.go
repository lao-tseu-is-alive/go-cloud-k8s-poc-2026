package actor

import (
	"errors"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

func TestNormalizeContactValueAccepts(t *testing.T) {
	cases := []struct {
		name  string
		typ   ContactType
		in    string
		wants string
	}{
		{"swiss national phone", ContactTypePhone, "021 315 22 22", "+41213152222"},
		{"international phone", ContactTypeMobile, "+41 (79) 123-45-67", "+41791234567"},
		{"00 prefix", ContactTypeFax, "0033 1 23 45 67 89", "+33123456789"},
		{"email domain lower-cased", ContactTypeEmail, "Info@Lausanne.CH", "Info@lausanne.ch"},
		{"website scheme added", ContactTypeWebsite, "www.Lausanne.ch/parcs", "https://www.lausanne.ch/parcs"},
		{"postal box short form", ContactTypePostalBox, "CP 5354", "Case postale 5354"},
		{"postal box number only", ContactTypePostalBox, "1002", "Case postale 1002"},
		{"IDE any spacing", ContactTypeIDEFederal, "che 123 456 788", "CHE-123.456.788"},
		{"IDE canonical", ContactTypeIDEFederal, "CHE-123.456.788", "CHE-123.456.788"},
		{"VAT suffix", ContactTypeVATNumber, "CHE-123.456.788 mwst", "CHE-123.456.788 MWST"},
		{"ABACUS digits", ContactTypeABACUSDebtor, "12 345", "12345"},
		{"register id", ContactTypeCommercialRegister, "CH55010123456", "CH-550.1.012.345-6"},
		{"register url", ContactTypeCommercialRegister, "https://www.zefix.ch/fr/search/entity/list/firm/123", "https://www.zefix.ch/fr/search/entity/list/firm/123"},
		{"other stays free", ContactTypeOther, " whatever ", "whatever"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeContactValue(tc.typ, tc.in)
			if err != nil || got != tc.wants {
				t.Fatalf("NormalizeContactValue(%v, %q) = %q, %v; want %q", tc.typ, tc.in, got, err, tc.wants)
			}
		})
	}
}

func TestNormalizeContactValueRejects(t *testing.T) {
	cases := []struct {
		name string
		typ  ContactType
		in   string
	}{
		{"letters as phone", ContactTypePhone, "xxxxxxxxxxxxxxxxxxxxxxxx"},
		{"too short phone", ContactTypePhone, "12345"},
		{"national without prefix", ContactTypePhone, "213152222"},
		{"email without domain dot", ContactTypeEmail, "a@localhost"},
		{"email with display name", ContactTypeEmail, "Ada <ada@example.ch>"},
		{"email garbage", ContactTypeEmail, "xxxxxxxx"},
		{"website ftp", ContactTypeWebsite, "ftp://example.ch"},
		{"website no dot", ContactTypeWebsite, "intranet"},
		{"postal box text", ContactTypePostalBox, "somewhere"},
		{"IDE bad check digit", ContactTypeIDEFederal, "CHE-123.456.789"},
		{"IDE too short", ContactTypeIDEFederal, "CHE-123.456"},
		{"VAT without suffix", ContactTypeVATNumber, "CHE-123.456.788"},
		{"VAT bad IDE", ContactTypeVATNumber, "CHE-123.456.789 TVA"},
		{"ABACUS letters", ContactTypeABACUSDebtor, "D-123"},
		{"register garbage", ContactTypeCommercialRegister, "xxxx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NormalizeContactValue(tc.typ, tc.in); !errors.Is(err, core.ErrInvalidInput) {
				t.Fatalf("NormalizeContactValue(%v, %q): want ErrInvalidInput, got %v", tc.typ, tc.in, err)
			}
		})
	}
}

func TestNormalizeContactsRequiresLabelForOther(t *testing.T) {
	if _, err := normalizeContacts([]ContactInput{{ContactType: ContactTypeOther, Value: "x"}}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("OTHER without label: want ErrInvalidInput, got %v", err)
	}
	got, err := normalizeContacts([]ContactInput{{ContactType: ContactTypePhone, Value: "021 315 22 22"}})
	if err != nil || got[0].Value != "+41213152222" {
		t.Fatalf("phone not normalized: %+v, %v", got, err)
	}
}
