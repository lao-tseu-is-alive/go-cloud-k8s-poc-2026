/** Lower-cases a text and removes its accents, so "École" and "ecole" compare equal. */
export function foldText (text: string): string {
  return text.normalize('NFD').replaceAll(/\p{M}/gu, '').toLocaleLowerCase('fr-CH')
}

/**
 * The filter of a searchable picker (v-autocomplete custom-filter, GLD-057): matches the
 * query anywhere in the item title, ignoring case and accents. It returns the match index
 * so Vuetify highlights it, true when folding changed the length (no highlight), or -1.
 */
export function foldedFilter (value: unknown, query: string): number | boolean {
  if (typeof value !== 'string') {
    return -1
  }
  const folded = foldText(value)
  const index = folded.indexOf(foldText(query))
  if (index === -1 || folded.length === value.length) {
    return index
  }
  return true
}
