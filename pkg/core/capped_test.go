package core

import (
	"strings"
	"testing"
)

func TestCapTotal(t *testing.T) {
	if total, capped := CapTotal(MaxCountedTotal); total != MaxCountedTotal || capped {
		t.Fatalf("the cap itself is exact: %d %v", total, capped)
	}
	if total, capped := CapTotal(CountLimit); total != MaxCountedTotal || !capped {
		t.Fatalf("one more match is reported as a lower bound: %d %v", total, capped)
	}
	if next := NextPageToken(MaxCountedTotal-20, 20, MaxCountedTotal); next != "" {
		t.Fatal("paging stops at the cap")
	}
}

func TestCappedPageSQL(t *testing.T) {
	sql := CappedPageSQL("SELECT c.id AS id, c.created_at AS sort_key FROM case_file c", "c.id", "case_file", "c", SortOrder{Desc: true})
	for _, want := range []string{"LIMIT @count_limit", "LIMIT @limit OFFSET @offset", "ORDER BY sort_key DESC, id DESC", "AS total_count"} {
		if !strings.Contains(sql, want) {
			t.Errorf("missing %q in %s", want, sql)
		}
	}
}

func TestCapWindowTotal(t *testing.T) {
	if total, capped := CapWindowTotal(42, false, 0, 20, 20); total != 42 || capped {
		t.Fatalf("a window covering the table gives the exact total: %d %v", total, capped)
	}
	if total, capped := CapWindowTotal(0, true, 40, 20, 20); total != 61 || !capped {
		t.Fatalf("a full page beyond the window keeps paging: %d %v", total, capped)
	}
	if total, capped := CapWindowTotal(0, true, 40, 7, 20); total != 47 || !capped {
		t.Fatalf("a short page is the last one: %d %v", total, capped)
	}
}

func TestParseOrderBy(t *testing.T) {
	allowed := []string{"created_at", "title"}
	def := Sort{Field: "created_at", Desc: true}
	for in, want := range map[string]Sort{
		"":           def,
		"title":      {Field: "title"},
		"Title DESC": {Field: "title", Desc: true},
		"title asc":  {Field: "title"},
	} {
		if got, err := ParseOrderBy(in, allowed, def); err != nil || got != want {
			t.Errorf("ParseOrderBy(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"owner", "title sideways", "title desc now"} {
		if _, err := ParseOrderBy(bad, allowed, def); err == nil {
			t.Errorf("ParseOrderBy(%q) must fail", bad)
		}
	}
}
