package integration

import (
	"errors"
	"strings"
	"testing"

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
