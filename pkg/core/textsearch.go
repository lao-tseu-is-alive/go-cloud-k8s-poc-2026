package core

import (
	"strings"
	"unicode"
)

// maxQueryWords bounds the words a text query keeps, so a pasted paragraph
// cannot build an expensive tsquery.
const maxQueryWords = 8

// QueryWords splits a free-text query into its words: runs of letters and
// decimal digits, at most maxQueryWords. Punctuation separates words, as the
// 'simple' text search parser does ("l'église" is "l" and "église",
// "2026-000123" is "2026" and "000123").
func QueryWords(query string) []string {
	words := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.Is(unicode.Nd, r)
	})
	if len(words) > maxQueryWords {
		words = words[:maxQueryWords]
	}
	return words
}

// PrefixTSQuery turns a free-text query into a to_tsquery expression matching
// every word from its start (GLD-064): "ave rhod" becomes 'ave':* & 'rhod':*,
// so typing finds "avenue de Rhodanie" before the word is complete. A
// one-letter word matches exactly, as a prefix it would match most rows. Each
// word is quoted, so no input can inject tsquery syntax. It returns "" when the
// query has no word; to_tsquery of an empty string then matches nothing.
func PrefixTSQuery(query string) string {
	words := QueryWords(query)
	terms := make([]string, len(words))
	for i, w := range words {
		terms[i] = "'" + w + "'"
		if len([]rune(w)) > 1 {
			terms[i] += ":*"
		}
	}
	return strings.Join(terms, " & ")
}

// TextMatchSQL is the predicate "vector matches the @text_query argument"
// (PrefixTSQuery of the user's query), accent-folded like every search_vector
// (immutable_unaccent, migration 0005).
func TextMatchSQL(vector string) string {
	return vector + ` @@ to_tsquery('simple', immutable_unaccent(@text_query))`
}
