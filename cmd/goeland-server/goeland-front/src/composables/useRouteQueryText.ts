import { type Ref, watch } from 'vue'
import { useRoute } from 'vue-router'

/**
 * Seeds a list page's text filter from ?q= (the global search opens a list
 * filtered by its query, GLD-064) and searches again when another ?q= arrives
 * while the page is open. Call it before the page's first load.
 */
export function useRouteQueryText (filters: Ref<{ query?: string }>, search: () => void): void {
  const route = useRoute()
  const read = (): string | undefined => typeof route.query.q === 'string' ? route.query.q : undefined
  const first = read()
  if (first !== undefined) {
    filters.value.query = first
  }
  watch(() => route.query.q, () => {
    const q = read()
    if (q === undefined) {
      return
    }
    filters.value.query = q
    search()
  })
}
