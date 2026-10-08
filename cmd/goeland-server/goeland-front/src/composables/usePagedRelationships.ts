import type { SubjectRelationship } from '@/api/types'
import type { ListSort } from '@/utils/listSort'
/**
 * The relationships of a subject in both directions, loaded page by page
 * (GLD-053): a unit or an employee may take part in tens of thousands of
 * cases, so a detail page shows the latest ones and loads more on demand.
 * The table sorts on the server (GLD-056): a new sort reloads the first page.
 */
import type { Ref } from 'vue'
import { ref, watch } from 'vue'
import { listRelationshipsPage } from '@/api/coreClient'
import { useApiErrors } from '@/composables/useApiErrors'
import { toOrderBy } from '@/utils/listSort'

const PAGE_SIZE = 50

export function usePagedRelationships (subjectId: Ref<string>) {
  const { report } = useApiErrors()
  const relationships = ref<SubjectRelationship[]>([])
  const nextPageToken = ref<string>()
  const hasMore = ref(false)
  const total = ref(0)
  const capped = ref(false)
  const loading = ref(false)
  const sort = ref<ListSort>()
  // Each reload supersedes the pages still in flight (a quick second click on a header).
  let generation = 0

  async function fetchPage (pageToken: string | undefined, gen: number) {
    loading.value = true
    try {
      const res = await listRelationshipsPage(subjectId.value, {
        bothDirections: true, orderBy: toOrderBy(sort.value), pageSize: PAGE_SIZE, pageToken,
      })
      if (gen !== generation) {
        return
      }
      relationships.value = [...(pageToken ? relationships.value : []), ...(res.relationships ?? [])]
      nextPageToken.value = res.nextPageToken || undefined
      hasMore.value = !!nextPageToken.value
      total.value = res.totalSize ?? relationships.value.length
      capped.value = !!res.totalSizeCapped
    } finally {
      if (gen === generation) {
        loading.value = false
      }
    }
  }

  /** Reloads the first page in the current sort. */
  async function reload () {
    generation++
    await fetchPage(undefined, generation)
  }

  /** Loads the next page, if any. */
  async function loadMore () {
    if (nextPageToken.value) {
      await fetchPage(nextPageToken.value, generation)
    }
  }

  watch(sort, () => {
    reload().catch(error => report(error))
  })

  return { relationships, loading, hasMore, total, capped, sort, reload, loadMore }
}
