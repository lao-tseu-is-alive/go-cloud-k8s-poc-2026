import type { SubjectRelationship } from '@/api/types'
/**
 * The relationships of a subject, loaded page by page in both directions
 * (GLD-053): a unit or an employee may take part in tens of thousands of
 * cases, so a detail page shows the latest ones and loads more on demand.
 */
import type { Ref } from 'vue'
import { computed, ref } from 'vue'
import { listRelationshipsPage } from '@/api/coreClient'

const PAGE_SIZE = 50

interface Direction {
  outgoing: boolean
  nextPageToken?: string
  total: number
  capped: boolean
  rows: SubjectRelationship[]
}

export function usePagedRelationships (subjectId: Ref<string>) {
  const directions = ref<Direction[]>([])
  const loading = ref(false)

  const relationships = computed(() => directions.value.flatMap(d => d.rows))
  const hasMore = computed(() => directions.value.some(d => !!d.nextPageToken))
  const total = computed(() => directions.value.reduce((sum, d) => sum + d.total, 0))
  const capped = computed(() => directions.value.some(d => d.capped))

  async function fetchPage (d: Direction) {
    const res = await listRelationshipsPage(subjectId.value, { outgoing: d.outgoing, pageSize: PAGE_SIZE, pageToken: d.nextPageToken })
    d.rows = [...d.rows, ...(res.relationships ?? [])]
    d.nextPageToken = res.nextPageToken || undefined
    d.total = res.totalSize ?? d.rows.length
    d.capped = !!res.totalSizeCapped
  }

  async function run (pick: (d: Direction) => boolean) {
    loading.value = true
    try {
      await Promise.all(directions.value.filter(d => pick(d)).map(d => fetchPage(d)))
    } finally {
      loading.value = false
    }
  }

  /** Reloads the first page of both directions (outgoing first). */
  async function reload () {
    directions.value = [
      { outgoing: true, total: 0, capped: false, rows: [] },
      { outgoing: false, total: 0, capped: false, rows: [] },
    ]
    await run(() => true)
  }

  /** Loads the next page of the directions that have one. */
  async function loadMore () {
    await run(d => !!d.nextPageToken)
  }

  return { relationships, loading, hasMore, total, capped, reload, loadMore }
}
