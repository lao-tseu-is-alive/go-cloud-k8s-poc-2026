package integration

import (
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/access"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// viewerOf resolves a user's principals as the adapters do.
func viewerOf(t *testing.T, env *testEnv, userID string) core.Viewer {
	t.Helper()
	v, err := core.ViewerTx(env.ctx, env.pool, userID)
	if err != nil {
		t.Fatalf("viewer %s: %v", userID, err)
	}
	return v
}

// searchCases returns the cases matching query that viewer may read.
func searchCases(t *testing.T, env *testEnv, query string, viewer core.Viewer) casefile.SearchResult {
	t.Helper()
	res, err := env.caseSvc.Search(env.ctx, casefile.SearchFilter{Query: query, Viewer: viewer})
	if err != nil {
		t.Fatalf("search cases: %v", err)
	}
	return res
}

// TestListsHideUnreadable covers GLD-049: a confidential case is absent from
// another user's search (count included) and from the relationships of a
// public subject linked to it, until a grant — here through the user's unit —
// makes it readable.
func TestListsHideUnreadable(t *testing.T) {
	env := newTestEnv(t)
	token := uniqueToken()
	c, _, err := env.caseSvc.Create(env.ctx, caseInput("Secret "+token, core.ConfidentialLevel))
	if err != nil {
		t.Fatalf("create confidential case: %v", err)
	}
	a := newActor(t, env)
	if _, _, err := env.coreSvc.LinkSubjects(env.ctx, core.LinkInput{SourceSubjectID: c.ID, TargetSubjectID: a.ID, RelationshipTypeCode: "CASE_HAS_ACTOR_REQUESTER", OperatorID: testOperator}); err != nil {
		t.Fatalf("link actor: %v", err)
	}
	x := newUser(t, env, "Xavier Filtre")
	unit := newUnit(t, env, "SERVICE", "Service filtre "+token, nil)
	member(t, env, x, unit.ID, "USER_MEMBER_OF_ORG_UNIT")

	if res := searchCases(t, env, token, viewerOf(t, env, testOperator)); len(res.Cases) != 1 || res.TotalSize != 1 {
		t.Fatalf("the creator finds its confidential case: %d (total %d)", len(res.Cases), res.TotalSize)
	}
	if res := searchCases(t, env, token, viewerOf(t, env, x.UserID)); len(res.Cases) != 0 || res.TotalSize != 0 {
		t.Fatalf("another user must not find it, nor count it: %d (total %d)", len(res.Cases), res.TotalSize)
	}
	if rels, err := env.actorSvc.Relationships(env.ctx, a.ID, viewerOf(t, env, x.UserID)); err != nil || len(rels) != 0 {
		t.Fatalf("the public actor must not reveal the confidential case: %d (%v)", len(rels), err)
	}

	grant(t, env, c.ID, access.GranteeOrgUnit, unit.ID.String(), core.LevelRead)
	if res := searchCases(t, env, token, viewerOf(t, env, x.UserID)); len(res.Cases) != 1 {
		t.Fatalf("a grant to the user's unit makes it searchable: %d", len(res.Cases))
	}
	if rels, err := env.actorSvc.Relationships(env.ctx, a.ID, viewerOf(t, env, x.UserID)); err != nil || len(rels) != 1 {
		t.Fatalf("the readable case is listed from the actor: %d (%v)", len(rels), err)
	}
}

// TestDocumentContentNeedsRead covers the governed download and automatic
// reuse (GLD-049): the content of a confidential document is refused to a
// non-reader, and uploading the same bytes gives that user a new document
// instead of reusing (and revealing) the confidential one.
func TestDocumentContentNeedsRead(t *testing.T) {
	env := newTestEnv(t)
	x := newUser(t, env, "Xavier Contenu")
	blob := ingest(t, env, "secret "+uniqueToken()).Blob
	secret, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Plan secret " + uniqueToken(), ContentBlobID: &blob.ID, OperatorID: testOperator,
		Governance: core.CreateSubjectInput{ConfidentialityLevel: core.ConfidentialLevel},
	})
	if err != nil {
		t.Fatalf("create confidential document: %v", err)
	}
	id := secret.Document.ID

	if got, err := env.docSvc.Content(env.ctx, testOperator, id, nil); err != nil || got.ID != blob.ID {
		t.Fatalf("the creator downloads the current version: %+v (%v)", got, err)
	}
	_, err = env.docSvc.Content(env.ctx, x.UserID, id, nil)
	denied(t, "downloading a confidential document without READ", err)
	other := uuid.New()
	if _, err := env.docSvc.Content(env.ctx, testOperator, id, &other); err == nil {
		t.Fatal("a version of another document must not be served")
	}

	again, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Même plan " + uniqueToken(), ContentBlobID: &blob.ID, OperatorID: x.UserID,
	})
	if err != nil {
		t.Fatalf("create with the same content: %v", err)
	}
	if again.Reused || again.Document.ID == id {
		t.Fatal("the same bytes must not reuse a document the user cannot read")
	}
	reused, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Encore " + uniqueToken(), ContentBlobID: &blob.ID, OperatorID: testOperator,
	})
	if err != nil || !reused.Reused || reused.Document.ID != id {
		t.Fatalf("the creator still reuses its own document: %+v (%v)", reused, err)
	}
}

// TestTimelineVisibility covers the audiences of a case timeline (GLD-049):
// READ sees the participants' entries, CONTRIBUTE the internal ones and
// MANAGE the restricted ones; an author always sees its entries, and nobody
// writes for an audience beyond its level.
func TestTimelineVisibility(t *testing.T) {
	env := newTestEnv(t)
	c := openCase(t, env, "Audiences "+uniqueToken())
	reader, contributor := newUser(t, env, "Rita Audience"), newUser(t, env, "Carl Audience")
	grant(t, env, c.ID, access.GranteeUser, contributor.UserID, core.LevelContribute)

	entry := func(operator string, v timeline.Visibility) (*timeline.Entry, error) {
		e, _, err := env.timelineSvc.Create(env.ctx, timeline.CreateInput{CaseID: c.ID, Type: timeline.TypeComment, Body: "note", Visibility: v, OperatorID: operator})
		return e, err
	}
	for _, v := range []timeline.Visibility{timeline.VisibilityCaseParticipants, timeline.VisibilityInternal, timeline.VisibilityRestricted} {
		if _, err := entry(testOperator, v); err != nil {
			t.Fatalf("entry %d: %v", v, err)
		}
	}
	_, err := entry(contributor.UserID, timeline.VisibilityRestricted)
	denied(t, "a restricted entry with CONTRIBUTE", err)
	if _, err := entry(contributor.UserID, timeline.VisibilityInternal); err != nil {
		t.Fatalf("an internal entry with CONTRIBUTE: %v", err)
	}

	visible := func(user string, level core.Level) int {
		res, err := env.timelineSvc.List(env.ctx, timeline.ListFilter{
			CaseID: c.ID, Types: []timeline.EntryType{timeline.TypeComment}, ViewerID: user, MaxVisibility: timeline.MaxVisibility(level),
		})
		if err != nil {
			t.Fatalf("list timeline: %v", err)
		}
		return len(res.Entries)
	}
	if n := visible(reader.UserID, core.LevelRead); n != 1 {
		t.Fatalf("READ sees the participants' entry only, got %d", n)
	}
	if n := visible(contributor.UserID, core.LevelContribute); n != 3 {
		t.Fatalf("CONTRIBUTE sees participants and internal entries (its own included), got %d", n)
	}
	if n := visible(testOperator, core.LevelFullControl); n != 4 {
		t.Fatalf("MANAGE sees every entry, got %d", n)
	}
}
