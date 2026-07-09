package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

const testOperator = "integration-operator"

// TestMigrationsIdempotentAndSeeded asserts the embedded schema applies cleanly a
// second time and that the reference data (document types) is present afterwards.
func TestMigrationsIdempotentAndSeeded(t *testing.T) {
	env := newTestEnv(t) // newTestEnv already migrated once.

	// Re-applying must be a no-op, not an error.
	if err := coremodule.Migrate(env.ctx, env.pool); err != nil {
		t.Fatalf("migrations are not idempotent: %v", err)
	}

	types, err := env.docSvc.ListTypes(env.ctx, true)
	if err != nil {
		t.Fatalf("list document types: %v", err)
	}
	if len(types) < 7 {
		t.Fatalf("expected the 7 seeded document types, got %d", len(types))
	}
	if !containsTypeCode(types, "INCOMING_LETTER") {
		t.Fatalf("seeded document type INCOMING_LETTER is missing")
	}
}

// TestDocumentLifecycle walks a document through its whole life against a real
// database, asserting the transaction/lifecycle invariants at each step.
func TestDocumentLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx

	token := uniqueToken()
	created, createEvent, _, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "INCOMING_LETTER",
		Title:            "Integration lifecycle " + token,
		Description:      "created by TestDocumentLifecycle",
		OperatorID:       testOperator,
		Governance:       core.CreateSubjectInput{ConfidentialityLevel: 1},
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	if created.Status != document.StatusDraft {
		t.Fatalf("new document should be DRAFT, got status %d", created.Status)
	}
	if createEvent == nil || createEvent.EventType != "DOCUMENT_CREATED" {
		t.Fatalf("expected a DOCUMENT_CREATED audit event, got %+v", createEvent)
	}
	docID := created.ID

	t.Run("search finds the new draft", func(t *testing.T) {
		res, err := env.docSvc.Search(ctx, document.SearchFilter{Query: token})
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if !containsDocID(res.Documents, docID) {
			t.Fatalf("search for %q did not return the created document", token)
		}
	})

	t.Run("metadata update while draft", func(t *testing.T) {
		updated, ev, err := env.docSvc.UpdateMetadata(ctx, docID, document.UpdateInput{
			Title:      "Integration lifecycle updated " + token,
			OperatorID: testOperator,
			Reason:     "integration update",
		})
		if err != nil {
			t.Fatalf("update metadata: %v", err)
		}
		if ev == nil || ev.EventType != "DOCUMENT_METADATA_UPDATED" {
			t.Fatalf("expected DOCUMENT_METADATA_UPDATED event, got %+v", ev)
		}
		// The canonical subject label must track the new title (QW5 invariant).
		if updated.Subject == nil || updated.Subject.DisplayLabel != updated.Title {
			t.Fatalf("subject label was not kept in sync with the title: %+v", updated.Subject)
		}
	})

	t.Run("link to an actor subject", func(t *testing.T) {
		actor, _, _, err := env.coreSvc.CreateSubjectRef(ctx, core.CreateSubjectInput{
			Kind:         core.SubjectKindActor,
			DisplayLabel: "Author actor " + token,
			OperatorID:   testOperator,
		})
		if err != nil {
			t.Fatalf("create actor subject: %v", err)
		}
		rel, _, err := env.docSvc.Link(ctx, core.LinkInput{
			SourceSubjectID:      docID,
			TargetSubjectID:      actor.ID,
			RelationshipTypeCode: "DOCUMENT_AUTHORED_BY_ACTOR",
			OperatorID:           testOperator,
		})
		if err != nil {
			t.Fatalf("link document to actor: %v", err)
		}
		if rel == nil {
			t.Fatal("expected a relationship to be returned")
		}
		rels, err := env.docSvc.Relationships(ctx, docID)
		if err != nil {
			t.Fatalf("list relationships: %v", err)
		}
		if len(rels) == 0 {
			t.Fatal("document should have at least one outgoing relationship after linking")
		}
	})

	t.Run("finalize and lock", func(t *testing.T) {
		final, ev, err := env.docSvc.Finalize(ctx, docID, testOperator, "integration finalize", true)
		if err != nil {
			t.Fatalf("finalize: %v", err)
		}
		if !final.IsFinal {
			t.Fatal("finalized document should report is_final=true")
		}
		if final.RecordMetadata == nil || !final.RecordMetadata.IsLocked {
			t.Fatalf("finalize with lock should leave the governance record locked: %+v", final.RecordMetadata)
		}
		if ev == nil || ev.EventType != "DOCUMENT_FINALIZED" {
			t.Fatalf("expected DOCUMENT_FINALIZED event, got %+v", ev)
		}
	})

	t.Run("locked record rejects mutation", func(t *testing.T) {
		_, _, err := env.docSvc.UpdateMetadata(ctx, docID, document.UpdateInput{
			Title:      "should be rejected " + token,
			OperatorID: testOperator,
		})
		if !errors.Is(err, core.ErrLocked) {
			t.Fatalf("updating a locked document should fail with ErrLocked, got %v", err)
		}
	})

	t.Run("soft delete then reject further mutation", func(t *testing.T) {
		ev, err := env.docSvc.SoftDelete(ctx, docID, testOperator, "integration delete")
		if err != nil {
			t.Fatalf("soft delete: %v", err)
		}
		if ev == nil || ev.EventType != "DOCUMENT_DELETED" {
			t.Fatalf("expected DOCUMENT_DELETED event, got %+v", ev)
		}
		if _, _, err := env.docSvc.Finalize(ctx, docID, testOperator, "again", false); !errors.Is(err, core.ErrDeleted) {
			t.Fatalf("finalizing a deleted document should fail with ErrDeleted, got %v", err)
		}
		// A soft-deleted document must disappear from the default search.
		res, err := env.docSvc.Search(ctx, document.SearchFilter{Query: token})
		if err != nil {
			t.Fatalf("search after delete: %v", err)
		}
		if containsDocID(res.Documents, docID) {
			t.Fatal("soft-deleted document should not appear in the default search")
		}
	})

	t.Run("audit trail records every step", func(t *testing.T) {
		events, err := env.docSvc.RecentAudit(ctx, docID)
		if err != nil {
			t.Fatalf("recent audit: %v", err)
		}
		seen := map[string]bool{}
		for _, ev := range events {
			seen[ev.EventType] = true
		}
		for _, want := range []string{
			"DOCUMENT_CREATED",
			"DOCUMENT_METADATA_UPDATED",
			"DOCUMENT_FINALIZED",
			"DOCUMENT_DELETED",
		} {
			if !seen[want] {
				t.Errorf("audit trail is missing a %s event (got %v)", want, keys(seen))
			}
		}
	})
}

func containsTypeCode(types []*document.DocumentType, code string) bool {
	for _, dt := range types {
		if dt.Code == code {
			return true
		}
	}
	return false
}

func containsDocID(docs []*document.Document, id uuid.UUID) bool {
	for _, d := range docs {
		if d.ID == id {
			return true
		}
	}
	return false
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
