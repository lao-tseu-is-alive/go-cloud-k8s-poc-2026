import type { ListSort } from '@/utils/listSort'
import type { ListFilters } from '@/utils/routeFilters'
import type { Ref } from 'vue'
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useApiErrors } from '@/composables/useApiErrors'
import { toOrderBy } from '@/utils/listSort'
import { filtersFromQuery, filtersToQuery, otherParams, sameFilters } from '@/utils/routeFilters'

/** The page parameters every paged search takes besides its filters. */
export interface PageParams {
  orderBy?: string
  pageSize?: number
  pageToken?: string
}

/** One page of a search, whatever the name of its items in the response. */
export interface SearchPage<T> {
  items: T[]
  nextPageToken?: string
  totalSize?: number
  totalSizeCapped?: boolean
}

const PAGE_SIZE = 25

/**
 * The state of a paged list (case, document, actor, thing): its filters kept
 * in the URL (utils/routeFilters, GLD-074), the server sort (GLD-055), the
 * pages loaded so far and the capped total (GLD-053). A newer search
 * supersedes the pages still in flight. A URL change made elsewhere (the
 * global search opening the list with another ?q=, the browser's back button)
 * searches again.
 */
export function usePagedSearch<P extends ListFilters, T> (
  fetchPage: (params: P & PageParams) => Promise<SearchPage<T>>,
  empty: () => P,
) {
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()

  const filters = ref(filtersFromQuery(route.query, empty())) as Ref<P>
  const sort = ref<ListSort>()
  const items = ref<T[]>([]) as Ref<T[]>
  const nextPageToken = ref('')
  const totalSize = ref(0)
  const totalCapped = ref(false)
  const loading = ref(false)
  let generation = 0

  async function load (reset: boolean) {
    const gen = reset ? ++generation : generation
    loading.value = true
    try {
      const res = await fetchPage({
        ...filters.value,
        orderBy: toOrderBy(sort.value),
        pageSize: PAGE_SIZE,
        pageToken: reset ? undefined : nextPageToken.value || undefined,
      })
      if (gen !== generation) {
        return
      }
      items.value = reset ? res.items : [...items.value, ...res.items]
      nextPageToken.value = res.nextPageToken ?? ''
      totalSize.value = res.totalSize ?? items.value.length
      totalCapped.value = !!res.totalSizeCapped
    } catch (error) {
      if (gen === generation) {
        report(error)
      }
    } finally {
      if (gen === generation) {
        loading.value = false
      }
    }
  }

  /** Searches from the first page and records the filters in the URL, as a new
   * history entry: the back button returns to the previous filters. */
  function search () {
    void router.push({ query: { ...otherParams(route.query, empty()), ...filtersToQuery(filters.value, empty()) } })
    void load(true)
  }

  /** Clears every filter and searches again. */
  function reset () {
    filters.value = empty()
    search()
  }

  watch(sort, () => void load(true))
  // Leaving the page changes the route too: only this page's URL counts.
  const path = route.path
  watch(() => route.query, query => {
    if (route.path !== path) {
      return
    }
    const wanted = filtersFromQuery(query, empty())
    if (!sameFilters(wanted, filters.value, empty())) {
      filters.value = wanted
      void load(true)
    }
  })
  onMounted(() => void load(true))

  return { filters, sort, items, nextPageToken, totalSize, totalCapped, loading, load, search, reset }
}
