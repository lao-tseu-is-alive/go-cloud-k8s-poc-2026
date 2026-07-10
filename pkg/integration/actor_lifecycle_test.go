package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TestActorCategoriesSeeded asserts the organization category reference data
// (production DicoActMoralCategory) is present after migration.
func TestActorCategoriesSeeded(t *testing.T) {
	env := newTestEnv(t)

	cats, err := env.actorSvc.ListCategories(env.ctx, true)
	if err != nil {
		t.Fatalf("list organization categories: %v", err)
	}
	if len(cats) < 33 {
		t.Fatalf("expected the 33 seeded organization categories, got %d", len(cats))
	}
	if !containsCategoryCode(cats, "BUREAU_ARCHITECTE") {
		t.Fatalf("seeded organization category BUREAU_ARCHITECTE is missing")
	}
}

// TestOrganizationActorLifecycle walks an ORGANIZATION actor through its whole
// life against a real database, asserting the lifecycle + specialization invariants.
func TestOrganizationActorLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	token := uniqueToken()

	created, createEvent, err := env.actorSvc.Create(ctx, actor.CreateInput{
		ActorKind:    actor.KindOrganization,
		DisplayName:  "Org " + token,
		LegalName:    "Bureau " + token + " SA",
		CategoryCode: "BUREAU_ARCHITECTE",
		Contacts: []actor.ContactInput{
			{ContactType: actor.ContactTypeEmail, Value: token + "@example.test", IsPrimary: true},
			{ContactType: actor.ContactTypeIDEFederal, Value: "CHE-123.456.789"},
		},
		OperatorID: testOperator,
		Governance: core.CreateSubjectInput{ConfidentialityLevel: 1},
	})
	if err != nil {
		t.Fatalf("create organization actor: %v", err)
	}
	if createEvent == nil || createEvent.EventType != "ACTOR_CREATED" {
		t.Fatalf("expected an ACTOR_CREATED audit event, got %+v", createEvent)
	}
	if created.ActorKind != actor.KindOrganization {
		t.Fatalf("expected ORGANIZATION kind, got %d", created.ActorKind)
	}
	if created.Category == nil || created.Category.Code != "BUREAU_ARCHITECTE" {
		t.Fatalf("expected hydrated category BUREAU_ARCHITECTE, got %+v", created.Category)
	}
	if len(created.Contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(created.Contacts))
	}
	actorID := created.ID

	t.Run("search finds the new actor", func(t *testing.T) {
		res, err := env.actorSvc.Search(ctx, actor.SearchFilter{Query: token})
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if !containsActorID(res.Actors, actorID) {
			t.Fatalf("search for %q did not return the created actor", token)
		}
	})

	t.Run("update syncs subject label and replaces contacts", func(t *testing.T) {
		newName := "Org updated " + token
		updated, ev, err := env.actorSvc.Update(ctx, actorID, actor.UpdateInput{
			DisplayName:     &newName,
			ReplaceContacts: true,
			Contacts: []actor.ContactInput{
				{ContactType: actor.ContactTypePhone, Value: "+41 21 000 00 00"},
			},
			OperatorID: testOperator,
			Reason:     "integration update",
		})
		if err != nil {
			t.Fatalf("update actor: %v", err)
		}
		if ev == nil || ev.EventType != "ACTOR_UPDATED" {
			t.Fatalf("expected ACTOR_UPDATED event, got %+v", ev)
		}
		if updated.Subject == nil || updated.Subject.DisplayLabel != newName {
			t.Fatalf("subject label was not kept in sync with the display name: %+v", updated.Subject)
		}
		if len(updated.Contacts) != 1 || updated.Contacts[0].ContactType != actor.ContactTypePhone {
			t.Fatalf("contacts were not replaced wholesale: %+v", updated.Contacts)
		}
	})

	t.Run("actor is a relationship target from a case", func(t *testing.T) {
		caseSubj, _, _, err := env.coreSvc.CreateSubjectRef(ctx, core.CreateSubjectInput{
			Kind:         core.SubjectKindCase,
			DisplayLabel: "Case " + token,
			OperatorID:   testOperator,
		})
		if err != nil {
			t.Fatalf("create case subject: %v", err)
		}
		if _, _, err := env.coreSvc.LinkSubjects(ctx, core.LinkInput{
			SourceSubjectID:      caseSubj.ID,
			TargetSubjectID:      actorID,
			RelationshipTypeCode: "CASE_HAS_ACTOR_REQUESTER",
			OperatorID:           testOperator,
		}); err != nil {
			t.Fatalf("link case to actor: %v", err)
		}
		rels, err := env.actorSvc.Relationships(ctx, actorID)
		if err != nil {
			t.Fatalf("list actor relationships: %v", err)
		}
		if len(rels) == 0 {
			t.Fatal("actor should have at least one incoming relationship after linking")
		}
	})

	t.Run("soft delete then reject further mutation", func(t *testing.T) {
		ev, err := env.actorSvc.SoftDelete(ctx, actorID, testOperator, "integration delete")
		if err != nil {
			t.Fatalf("soft delete: %v", err)
		}
		if ev == nil || ev.EventType != "ACTOR_DELETED" {
			t.Fatalf("expected ACTOR_DELETED event, got %+v", ev)
		}
		newName := "should be rejected " + token
		if _, _, err := env.actorSvc.Update(ctx, actorID, actor.UpdateInput{
			DisplayName: &newName, OperatorID: testOperator,
		}); !errors.Is(err, core.ErrDeleted) {
			t.Fatalf("updating a deleted actor should fail with ErrDeleted, got %v", err)
		}
		res, err := env.actorSvc.Search(ctx, actor.SearchFilter{Query: token})
		if err != nil {
			t.Fatalf("search after delete: %v", err)
		}
		if containsActorID(res.Actors, actorID) {
			t.Fatal("soft-deleted actor should not appear in the default search")
		}
	})

	t.Run("audit trail records every step", func(t *testing.T) {
		events, err := env.actorSvc.RecentAudit(ctx, actorID)
		if err != nil {
			t.Fatalf("recent audit: %v", err)
		}
		seen := map[string]bool{}
		for _, ev := range events {
			seen[ev.EventType] = true
		}
		for _, want := range []string{"ACTOR_CREATED", "ACTOR_UPDATED", "ACTOR_DELETED"} {
			if !seen[want] {
				t.Errorf("audit trail is missing a %s event (got %v)", want, keys(seen))
			}
		}
	})
}

// TestPersonActorSpecialization asserts a PERSON actor carries only its
// register-link specialization and no organization fields.
func TestPersonActorSpecialization(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	token := uniqueToken()

	created, _, err := env.actorSvc.Create(ctx, actor.CreateInput{
		ActorKind:     actor.KindPerson,
		DisplayName:   "Person " + token,
		IsCHRegister:  true,
		CHRegisterRef: "REG-" + token,
		OperatorID:    testOperator,
	})
	if err != nil {
		t.Fatalf("create person actor: %v", err)
	}
	if created.ActorKind != actor.KindPerson {
		t.Fatalf("expected PERSON kind, got %d", created.ActorKind)
	}
	if !created.IsCHRegister || created.CHRegisterRef != "REG-"+token {
		t.Fatalf("person register specialization not persisted: %+v", created)
	}
	if created.LegalName != "" || created.CategoryID != nil {
		t.Fatalf("person actor must not carry organization fields: %+v", created)
	}

	t.Run("organization legal_name is required", func(t *testing.T) {
		if _, _, err := env.actorSvc.Create(ctx, actor.CreateInput{
			ActorKind:   actor.KindOrganization,
			DisplayName: "NoLegalName " + token,
			OperatorID:  testOperator,
		}); !errors.Is(err, core.ErrInvalidInput) {
			t.Fatalf("creating an organization without legal_name should fail with ErrInvalidInput, got %v", err)
		}
	})
}

func containsCategoryCode(cats []*actor.OrganizationCategory, code string) bool {
	for _, c := range cats {
		if c.Code == code {
			return true
		}
	}
	return false
}

func containsActorID(actors []*actor.Actor, id uuid.UUID) bool {
	for _, a := range actors {
		if a.ID == id {
			return true
		}
	}
	return false
}
