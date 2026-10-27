package core

import (
	"strings"
	"testing"
)

func TestPrefixTSQuery(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"  -- ":            "",
		"ave rhod":         "'ave':* & 'rhod':*",
		"l'église":         "'l' & 'église':*",
		"2026-000123":      "'2026':* & '000123':*",
		"a' | b:* & !c":    "'a' & 'b' & 'c'",
		"Zürich (CH)":      "'Zürich':* & 'CH':*",
		"x\\y'z":           "'x' & 'y' & 'z'",
		"un deux trois 4 ": "'un':* & 'deux':* & 'trois':* & '4'",
	}
	for in, want := range cases {
		if got := PrefixTSQuery(in); got != want {
			t.Errorf("PrefixTSQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueryWordsBounded(t *testing.T) {
	words := QueryWords(strings.Repeat("mot ", 20))
	if len(words) != maxQueryWords {
		t.Fatalf("got %d words, want %d", len(words), maxQueryWords)
	}
}
