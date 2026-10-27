package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
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

// TestListFilters covers GLD-074: the case, document and actor searches
// filter on the creator and the creation day (Europe/Zurich), cases on their
// opening day and documents on their official date.
func TestListFilters(t *testing.T) {
	env := newTestEnv(t)
	token := uniqueToken()
	operator := viewerOf(t, env, testOperator)
	other := newUser(t, env, "Rita Filtre")
	zurich, err := time.LoadLocation(core.BusinessRefPeriodTimeZone)
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(zurich).Format(time.DateOnly)
	yesterday := time.Now().In(zurich).AddDate(0, 0, -1).Format(time.DateOnly)

	mine, _, err := env.caseSvc.Create(env.ctx, caseInput("Filtre "+token+" mien", 0))
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	theirs := caseInput("Filtre "+token+" sien", 0)
	theirs.OperatorID = other.UserID
	if _, _, err := env.caseSvc.Create(env.ctx, theirs); err != nil {
		t.Fatalf("create case of another user: %v", err)
	}

	cases := func(f casefile.SearchFilter) casefile.SearchResult {
		t.Helper()
		f.Query, f.Viewer = token, operator
		res, err := env.caseSvc.Search(env.ctx, f)
		if err != nil {
			t.Fatalf("search cases: %v", err)
		}
		return res
	}
	if res := cases(casefile.SearchFilter{Record: core.RecordFilter{CreatedBy: testOperator}}); res.TotalSize != 1 || res.Cases[0].ID != mine.ID {
		t.Fatalf("the creator filter keeps the operator's case only: %d", res.TotalSize)
	}
	created, _ := core.ParseDateRange(today, today)
	if res := cases(casefile.SearchFilter{Record: core.RecordFilter{Created: created}}); res.TotalSize != 2 {
		t.Fatalf("both cases were created today: %d", res.TotalSize)
	}
	before, _ := core.ParseDateRange("", yesterday)
	if res := cases(casefile.SearchFilter{Record: core.RecordFilter{Created: before}}); res.TotalSize != 0 {
		t.Fatalf("no case was created before today: %d", res.TotalSize)
	}
	if res := cases(casefile.SearchFilter{Opened: created}); res.TotalSize != 2 {
		t.Fatalf("both cases were opened today: %d", res.TotalSize)
	}
	if res := cases(casefile.SearchFilter{Opened: before}); res.TotalSize != 0 {
		t.Fatalf("no case was opened before today: %d", res.TotalSize)
	}

	official := time.Date(2001, 5, 4, 0, 0, 0, 0, time.UTC)
	blob := ingest(t, env, "filtre "+token).Blob
	if _, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Plan " + token, OfficialDate: &official, ContentBlobID: &blob.ID, OperatorID: testOperator,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	documents := func(r core.DateRange) int32 {
		t.Helper()
		res, err := env.docSvc.Search(env.ctx, document.SearchFilter{Query: token, Viewer: operator, Official: r})
		if err != nil {
			t.Fatalf("search documents: %v", err)
		}
		return res.TotalSize
	}
	in2001, _ := core.ParseDateRange("2001-01-01", "2001-05-04")
	after, _ := core.ParseDateRange("2001-05-05", "")
	if documents(in2001) != 1 || documents(after) != 0 {
		t.Fatalf("the official date filter keeps its bounds: %d %d", documents(in2001), documents(after))
	}

	if _, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{
		ActorKind: actor.KindOrganization, DisplayName: "Atelier " + token, LegalName: "Atelier SA", OperatorID: other.UserID,
	}); err != nil {
		t.Fatalf("create actor: %v", err)
	}
	for creator, want := range map[string]int32{other.UserID: 1, testOperator: 0} {
		res, err := env.actorSvc.Search(env.ctx, actor.SearchFilter{Query: token, Viewer: operator, Record: core.RecordFilter{CreatedBy: creator}})
		if err != nil || res.TotalSize != want {
			t.Fatalf("actors created by %s: %d, want %d (%v)", creator, res.TotalSize, want, err)
		}
	}
}
