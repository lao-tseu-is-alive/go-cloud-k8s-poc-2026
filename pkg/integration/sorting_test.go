package integration

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
)

// TestListSorting covers order_by (GLD-055): a sorted search returns its rows
// in that order in both directions, and an unknown field is refused.
func TestListSorting(t *testing.T) {
	env := newTestEnv(t)
	token := uniqueToken()
	for _, title := range []string{"Tri Charlie " + token, "Tri alpha " + token, "Tri Bravo " + token} {
		openCase(t, env, title)
	}
	titles := func(orderBy string) []string {
		res, err := env.caseSvc.Search(env.ctx, casefile.SearchFilter{Query: token, OrderBy: orderBy, Viewer: operatorViewer})
		if err != nil {
			t.Fatalf("search ordered by %q: %v", orderBy, err)
		}
		out := make([]string, len(res.Cases))
		for i, c := range res.Cases {
			out[i] = strings.TrimSuffix(c.Title, " "+token)
		}
		return out
	}
	if got := titles("title"); got[0] != "Tri alpha" || got[1] != "Tri Bravo" || got[2] != "Tri Charlie" {
		t.Fatalf("ascending by title (case-insensitive collation): %v", got)
	}
	if got := titles("title desc"); got[0] != "Tri Charlie" || got[2] != "Tri alpha" {
		t.Fatalf("descending by title: %v", got)
	}
	if _, err := env.caseSvc.Search(env.ctx, casefile.SearchFilter{Query: token, OrderBy: "owner"}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("an unknown sort field: want ErrInvalidInput, got %v", err)
	}
	if _, err := env.taskSvc.ListMine(env.ctx, task.MineFilter{UserID: testOperator, OrderBy: "due_at desc"}); err != nil {
		t.Fatalf("my tasks sorted by deadline: %v", err)
	}
}

// TestRelationshipSorting covers the sorted relationship table of a detail
// page (GLD-056): both directions in one list, ordered by the other end's
// label or the type, and an unknown field refused.
func TestRelationshipSorting(t *testing.T) {
	env := newTestEnv(t)
	token := uniqueToken()
	c := openCase(t, env, "Relations "+token)
	for _, name := range []string{"Zinc " + token, "acier " + token} {
		a, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{ActorKind: actor.KindOrganization, DisplayName: name, LegalName: name, OperatorID: testOperator})
		if err != nil {
			t.Fatalf("create actor: %v", err)
		}
		link(t, env, c.ID, core.LinkInput{TargetSubjectID: a.ID, RelationshipTypeCode: "CASE_HAS_ACTOR_REQUESTER"})
	}
	related := openCase(t, env, "Bureau "+token)
	link(t, env, related.ID, core.LinkInput{TargetSubjectID: c.ID, RelationshipTypeCode: "CASE_RELATED_TO_CASE"})

	labels := func(orderBy string, other func(*core.SubjectRelationship) *core.SubjectRef) []string {
		res, err := env.coreSvc.ListRelationships(env.ctx, core.RelationshipFilter{SubjectID: c.ID, BothDirections: true, OrderBy: orderBy, Viewer: operatorViewer})
		if err != nil {
			t.Fatalf("relationships ordered by %q: %v", orderBy, err)
		}
		if res.TotalSize != 3 {
			t.Fatalf("both directions: want 3 edges, got %d", res.TotalSize)
		}
		out := make([]string, len(res.Relationships))
		for i, rel := range res.Relationships {
			out[i] = strings.TrimSuffix(other(rel).DisplayLabel, " "+token)
		}
		return out
	}
	target := func(rel *core.SubjectRelationship) *core.SubjectRef { return rel.Target }
	if got := labels("target", target); got[0] != "acier" || got[1] != "Relations" || got[2] != "Zinc" {
		t.Fatalf("ascending by target (the incoming edge targets the case, case-insensitive): %v", got)
	}
	if got := labels("target desc", target); got[0] != "Zinc" || got[2] != "acier" {
		t.Fatalf("descending by target: %v", got)
	}
	source := func(rel *core.SubjectRelationship) *core.SubjectRef { return rel.Source }
	if got := labels("source", source); got[0] != "Bureau" || got[1] != "Relations" {
		t.Fatalf("ascending by source: %v", got)
	}
	if got := labels("type", source); got[2] != "Bureau" {
		t.Fatalf("by type label, \"Affaire liée à affaire\" comes last: %v", got)
	}
	if _, err := env.coreSvc.ListRelationships(env.ctx, core.RelationshipFilter{SubjectID: c.ID, OrderBy: "deleted_at"}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("an unknown sort field: want ErrInvalidInput, got %v", err)
	}
}

// TestCatalogueOrderedByLabel checks that a type picker gets its catalogue by label, not by
// code (GLD-057): the imported LEG_<id> codes said nothing to the user.
func TestCatalogueOrderedByLabel(t *testing.T) {
	env := newTestEnv(t)
	token := uniqueToken()
	first, last := referenceCode("IT_Z"), referenceCode("IT_A")
	for code, label := range map[string]string{first: "Aa " + token, last: "Zz " + token} {
		if _, _, err := env.caseSvc.CreateCaseType(env.ctx, casefile.CaseTypeInput{Code: code, Label: label, BusinessRefNamespace: "ITC", OperatorID: testOperator}); err != nil {
			t.Fatalf("create case type %s: %v", code, err)
		}
	}
	types, err := env.caseSvc.ListTypes(env.ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]string, 0, len(types))
	for _, ct := range types {
		codes = append(codes, ct.Code)
	}
	if slices.Index(codes, first) > slices.Index(codes, last) {
		t.Fatalf("case types must come by label: %q after %q", first, last)
	}
}
