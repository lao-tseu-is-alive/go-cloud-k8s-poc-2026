package integration

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TestActorAddressesAndBranch covers typed addresses with a principal one, a
// non-destructive replacement, and a branch linked to its head organization.
func TestActorAddressesAndBranch(t *testing.T) {
	env := newTestEnv(t)
	token := uniqueToken()
	head, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{
		ActorKind: actor.KindOrganization, DisplayName: "Migros " + token, LegalName: "Société coopérative Migros Vaud", OperatorID: testOperator,
		Addresses: []actor.AddressInput{
			{AddressType: actor.AddressTypeHeadOffice, Street: "Chemin du Chêne", HouseNumber: "5", PostalCode: "1020", Locality: "Renens"},
			{AddressType: actor.AddressTypeBilling, Street: "Case postale", PostalCode: "1001", Locality: "Lausanne", IsPrincipal: false},
		},
	})
	if err != nil {
		t.Fatalf("create organization with addresses: %v", err)
	}
	if len(head.Addresses) != 2 || !head.Addresses[0].IsPrincipal || head.Addresses[0].Locality != "Renens" || head.Addresses[0].CountryCode != "CH" {
		t.Fatalf("addresses not stored with a principal first: %+v", head.Addresses)
	}

	updated, ev, err := env.actorSvc.Update(env.ctx, head.ID, actor.UpdateInput{
		OperatorID: testOperator, ReplaceAddresses: true,
		Addresses: []actor.AddressInput{{AddressType: actor.AddressTypeHeadOffice, Street: "Rue de Genève", HouseNumber: "88", PostalCode: "1004", Locality: "Lausanne"}},
	})
	if err != nil || len(updated.Addresses) != 1 || updated.Addresses[0].Street != "Rue de Genève" {
		t.Fatalf("replace addresses: %+v (%v)", updated.Addresses, err)
	}
	// JSONB round trip: numbers come back as float64.
	if ev.AfterState["addresses_ended"] != float64(2) || ev.AfterState["addresses_added"] != float64(1) {
		t.Fatalf("address replacement not audited: %+v", ev.AfterState)
	}
	same := updated.Addresses[0]
	_, ev, err = env.actorSvc.Update(env.ctx, head.ID, actor.UpdateInput{
		OperatorID: testOperator, ReplaceAddresses: true,
		Addresses: []actor.AddressInput{{AddressType: actor.AddressTypeHeadOffice, Street: "Rue de Genève", HouseNumber: "88", PostalCode: "1004", Locality: "Lausanne"}},
	})
	if err != nil || ev.AfterState["addresses_ended"] != nil || ev.AfterState["addresses_added"] != nil {
		t.Fatalf("an identical address list changes nothing: %+v (%v)", ev.AfterState, err)
	}
	if again, err := env.actorSvc.Get(env.ctx, head.ID); err != nil || again.Addresses[0].ID != same.ID {
		t.Fatalf("an identical address keeps its link: %+v (%v)", again, err)
	}
	var ended int
	if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM actor_address WHERE actor_id = $1 AND ended_at IS NOT NULL`, head.ID).Scan(&ended); err != nil || ended != 2 {
		t.Fatalf("previous address links must be ended, not deleted: %d (%v)", ended, err)
	}

	branch, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{
		ActorKind: actor.KindOrganization, DisplayName: "Migros Nyon " + token, LegalName: "Société coopérative Migros Vaud", OperatorID: testOperator,
		Addresses: []actor.AddressInput{{AddressType: actor.AddressTypeBranch, Street: "Route de Clémenty", PostalCode: "1260", Locality: "Nyon"}},
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	if _, _, err := env.coreSvc.LinkSubjects(env.ctx, core.LinkInput{
		SourceSubjectID: branch.ID, TargetSubjectID: head.ID, RelationshipTypeCode: "ACTOR_BRANCH_OF_ACTOR", OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("link branch to head: %v", err)
	}
	for _, id := range []uuid.UUID{head.ID, branch.ID} {
		rels, err := env.actorSvc.Relationships(env.ctx, id, operatorViewer)
		if err != nil || len(rels) != 1 || rels[0].RelationshipType.Code != "ACTOR_BRANCH_OF_ACTOR" {
			t.Fatalf("the branch link must be listed from both actors: %+v (%v)", rels, err)
		}
	}
}

// TestActorContactHistory covers GLD-007: replacing contacts ends the ones that
// left the list (kept with who ended them), keeps identical ones untouched and
// audits only the counts.
func TestActorContactHistory(t *testing.T) {
	env := newTestEnv(t)
	token := uniqueToken()
	mail := actor.ContactInput{ContactType: actor.ContactTypeEmail, Value: token + "@example.test", IsPrimary: true}
	created, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{
		ActorKind: actor.KindOrganization, DisplayName: "Historique " + token, LegalName: "Historique SA", OperatorID: testOperator,
		Contacts: []actor.ContactInput{mail, {ContactType: actor.ContactTypePhone, Value: "+41 21 000 00 00"}},
	})
	if err != nil {
		t.Fatalf("create actor with contacts: %v", err)
	}
	kept := created.Contacts[0]
	if kept.ContactType != actor.ContactTypeEmail {
		kept = created.Contacts[1]
	}

	updated, ev, err := env.actorSvc.Update(env.ctx, created.ID, actor.UpdateInput{
		OperatorID: testOperator, ReplaceContacts: true,
		Contacts: []actor.ContactInput{mail, {ContactType: actor.ContactTypePhone, Value: "+41 21 111 11 11"}},
	})
	if err != nil {
		t.Fatalf("replace contacts: %v", err)
	}
	if ev.AfterState["contacts_ended"] != float64(1) || ev.AfterState["contacts_added"] != float64(1) {
		t.Fatalf("the replacement audits counts: %+v", ev.AfterState)
	}
	if len(updated.Contacts) != 2 || !slices.ContainsFunc(updated.Contacts, func(c *actor.Contact) bool { return c.ID == kept.ID }) {
		t.Fatalf("the unchanged e-mail keeps its row: %+v", updated.Contacts)
	}
	var endedValue, endedBy string
	if err := env.pool.QueryRow(env.ctx, `SELECT value, ended_by FROM actor_contact WHERE actor_id = $1 AND ended_at IS NOT NULL`, created.ID).
		Scan(&endedValue, &endedBy); err != nil || endedValue != "+41210000000" || endedBy != testOperator {
		t.Fatalf("the replaced phone is ended, not deleted: %q by %q (%v)", endedValue, endedBy, err)
	}

	_, ev, err = env.actorSvc.Update(env.ctx, created.ID, actor.UpdateInput{OperatorID: testOperator, ReplaceContacts: true, Contacts: []actor.ContactInput{}})
	if err != nil || ev.AfterState["contacts_ended"] != float64(2) {
		t.Fatalf("clearing the contacts ends them all: %+v (%v)", ev.AfterState, err)
	}
	if got, err := env.actorSvc.Get(env.ctx, created.ID); err != nil || len(got.Contacts) != 0 {
		t.Fatalf("ended contacts are not current: %+v (%v)", got, err)
	}
}

// TestActorUpdateRejectsOtherKind covers GLD-004: details of the other kind
// are an invalid argument, not a database error.
func TestActorUpdateRejectsOtherKind(t *testing.T) {
	env := newTestEnv(t)
	org := newActor(t, env)
	last, first, salutation := "Muster", "", actor.SalutationUnspecified
	if _, _, err := env.actorSvc.Update(env.ctx, org.ID, actor.UpdateInput{
		LastName: &last, FirstName: &first, Salutation: &salutation, OperatorID: testOperator,
	}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("person details on an organization: want ErrInvalidInput, got %v", err)
	}
	person, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{ActorKind: actor.KindPerson, LastName: "Kind" + uniqueToken(), OperatorID: testOperator})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	legal := "Personne SA"
	if _, _, err := env.actorSvc.Update(env.ctx, person.ID, actor.UpdateInput{LegalName: &legal, OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("organization details on a person: want ErrInvalidInput, got %v", err)
	}
}
