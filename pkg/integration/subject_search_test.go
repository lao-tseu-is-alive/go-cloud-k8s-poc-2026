package integration

import (
	"strings"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// searchSubjects runs a global search and returns the groups by kind.
func searchSubjects(t *testing.T, env *testEnv, filter core.SubjectSearchFilter) []*core.SubjectSearchGroup {
	t.Helper()
	groups, err := env.coreSvc.SearchSubjects(env.ctx, filter)
	if err != nil {
		t.Fatalf("search subjects %q: %v", filter.Query, err)
	}
	return groups
}

// TestGlobalSearch covers GLD-064: the global search finds subjects of every
// kind by the start of each word of their label, accent-insensitive, and by
// exact business reference first; it counts and returns only what the viewer
// may read, leaves deleted subjects out and follows a renamed label.
func TestGlobalSearch(t *testing.T) {
	env := newTestEnv(t)
	token := uniqueToken()
	prefix := token[:len(token)-4]
	operator := viewerOf(t, env, testOperator)

	public, _, err := env.caseSvc.Create(env.ctx, caseInput("Église "+token+" publique", 0))
	if err != nil {
		t.Fatalf("create public case: %v", err)
	}
	if _, _, err := env.caseSvc.Create(env.ctx, caseInput("Église "+token+" secrète", core.ConfidentialLevel)); err != nil {
		t.Fatalf("create confidential case: %v", err)
	}
	deleted, _, err := env.caseSvc.Create(env.ctx, caseInput("Église "+token+" supprimée", 0))
	if err != nil {
		t.Fatalf("create case to delete: %v", err)
	}
	if _, err := env.caseSvc.SoftDelete(env.ctx, deleted.ID, testOperator, "test"); err != nil {
		t.Fatalf("delete case: %v", err)
	}
	if _, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{
		ActorKind: actor.KindOrganization, DisplayName: "Bureau " + token, LegalName: "Bureau SA", OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("create actor: %v", err)
	}

	groups := searchSubjects(t, env, core.SubjectSearchFilter{Viewer: operator, Query: "EGL " + prefix})
	if len(groups) != len(core.DefaultSearchKinds) || groups[0].Kind != core.SubjectKindCase || groups[1].Kind != core.SubjectKindActor {
		t.Fatalf("one group per default kind, cases then actors: %+v", groups)
	}
	if g := groups[0]; g.Total != 2 || len(g.Subjects) != 2 || g.TotalCapped {
		t.Fatalf("the creator finds its public and confidential cases, not the deleted one: %d %d", g.Total, len(g.Subjects))
	}
	if g := groups[1]; g.Total != 0 {
		t.Fatalf("every word must match: the actor has no word starting with egl, got %d", g.Total)
	}
	if g := searchSubjects(t, env, core.SubjectSearchFilter{Viewer: operator, Query: prefix})[1]; g.Total != 1 || !strings.Contains(g.Subjects[0].DisplayLabel, token) {
		t.Fatalf("the actor is found by the start of a word: %+v", g)
	}

	if res := searchCases(t, env, "egl "+prefix, operator); res.TotalSize != 2 {
		t.Fatalf("the case search matches from the start of every word too: %d", res.TotalSize)
	}

	other := viewerOf(t, env, newUser(t, env, "Olga Recherche").UserID)
	if g := searchSubjects(t, env, core.SubjectSearchFilter{Viewer: other, Query: prefix, Kinds: []core.SubjectKind{core.SubjectKindCase}}); len(g) != 1 || g[0].Total != 1 || g[0].Subjects[0].ID != public.ID {
		t.Fatalf("another user finds and counts only the public case: %+v", g)
	}

	if g := searchSubjects(t, env, core.SubjectSearchFilter{Viewer: operator, Query: prefix, Kinds: []core.SubjectKind{core.SubjectKindCase}, LimitPerKind: 1})[0]; g.Total != 2 || len(g.Subjects) != 1 {
		t.Fatalf("the limit cuts the subjects, not the count: %d %d", g.Total, len(g.Subjects))
	}

	ref, _, err := createCase(t, env, "Dossier sans mot commun", core.BusinessRefRequest{Value: "GS-" + token})
	if err != nil {
		t.Fatalf("create case with reference: %v", err)
	}
	if g := searchSubjects(t, env, core.SubjectSearchFilter{Viewer: operator, Query: "GS-" + token, Kinds: []core.SubjectKind{core.SubjectKindCase}})[0]; g.Total != 1 || g.Subjects[0].ID != ref.ID {
		t.Fatalf("the exact business reference finds its subject: %+v", g)
	}

	renamed := uniqueToken()
	if _, _, err := env.caseSvc.Update(env.ctx, public.ID, casefile.UpdateInput{Title: "Temple " + renamed, OperatorID: testOperator}); err != nil {
		t.Fatalf("rename case: %v", err)
	}
	if g := searchSubjects(t, env, core.SubjectSearchFilter{Viewer: operator, Query: "temple " + renamed})[0]; g.Total != 1 || g.Subjects[0].ID != public.ID {
		t.Fatalf("a renamed subject is found by its new label: %+v", g)
	}

	for _, g := range searchSubjects(t, env, core.SubjectSearchFilter{Viewer: operator, Query: " -- "}) {
		if g.Total != 0 || len(g.Subjects) != 0 {
			t.Fatalf("a query without a word finds nothing: %+v", g)
		}
	}
}
