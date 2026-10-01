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
	sql := CappedPageSQL("SELECT c.id AS id, c.created_at AS sort_key FROM case_file c", "c.id", "case_file", "c", true)
	for _, want := range []string{"LIMIT @count_limit", "LIMIT @limit OFFSET @offset", "ORDER BY sort_key DESC, id DESC", "AS total_count"} {
		if !strings.Contains(sql, want) {
			t.Errorf("missing %q in %s", want, sql)
		}
	}
}
