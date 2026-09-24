package integration

import (
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
	if ev.AfterState["addresses_replaced"] != float64(1) {
		t.Fatalf("address replacement not audited: %+v", ev.AfterState)
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
		rels, err := env.actorSvc.Relationships(env.ctx, id)
		if err != nil || len(rels) != 1 || rels[0].RelationshipType.Code != "ACTOR_BRANCH_OF_ACTOR" {
			t.Fatalf("the branch link must be listed from both actors: %+v (%v)", rels, err)
		}
	}
}
