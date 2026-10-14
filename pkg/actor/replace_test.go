package actor

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

func TestDiffByKeyKeepsIdenticalRows(t *testing.T) {
	mail := ContactInput{ContactType: ContactTypeEmail, Value: "a@example.test", IsPrimary: true}
	phone := ContactInput{ContactType: ContactTypePhone, Value: "+41210000000"}
	fax := ContactInput{ContactType: ContactTypeFax, Value: "+41210000001"}
	current := []*Contact{
		{ID: uuid.New(), ContactType: mail.ContactType, Value: mail.Value, IsPrimary: true},
		{ID: uuid.New(), ContactType: phone.ContactType, Value: phone.Value},
		{ID: uuid.New(), ContactType: phone.ContactType, Value: phone.Value}, // a duplicate
	}
	id := func(c *Contact) uuid.UUID { return c.ID }

	ended, added := diffByKey(current, []ContactInput{phone, mail}, contactKey, id)
	if !slices.Equal(ended, []uuid.UUID{current[2].ID}) || len(added) != 0 {
		t.Fatalf("an unchanged list ends only the extra duplicate: ended %v, added %v", ended, added)
	}

	unprimary := mail
	unprimary.IsPrimary = false
	ended, added = diffByKey(current, []ContactInput{unprimary, phone, fax}, contactKey, id)
	if !slices.Equal(ended, []uuid.UUID{current[0].ID, current[2].ID}) || !slices.Equal(added, []ContactInput{unprimary, fax}) {
		t.Fatalf("a changed flag ends and re-adds the row: ended %v, added %v", ended, added)
	}

	ended, added = diffByKey(current, nil, contactKey, id)
	if len(ended) != len(current) || len(added) != 0 {
		t.Fatalf("an empty list ends every row: ended %v, added %v", ended, added)
	}
}

func TestCheckUpdateKind(t *testing.T) {
	legal, empty, last := "Migros", "", "Muster"
	ch := true
	for _, tc := range []struct {
		name string
		kind Kind
		in   UpdateInput
		ok   bool
	}{
		{"organization details on a person", KindPerson, UpdateInput{LegalName: &legal}, false},
		{"empty organization details on a person", KindPerson, UpdateInput{LegalName: &empty, CategoryCode: &empty}, true},
		{"person details on an organization", KindOrganization, UpdateInput{LastName: &last}, false},
		{"register flag on an organization", KindOrganization, UpdateInput{IsCHRegister: &ch}, false},
		{"organization details on an organization", KindOrganization, UpdateInput{LegalName: &legal}, true},
	} {
		err := checkUpdateKind(tc.kind, tc.in)
		if tc.ok != (err == nil) || (err != nil && !errors.Is(err, core.ErrInvalidInput)) {
			t.Errorf("%s: got %v", tc.name, err)
		}
	}
}
