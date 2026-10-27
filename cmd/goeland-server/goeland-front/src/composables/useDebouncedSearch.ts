import { onBeforeUnmount, ref, type Ref } from 'vue'
import { useApiErrors } from '@/composables/useApiErrors'

/** The delay between the last keystroke and the search, in milliseconds. */
export const SEARCH_DEBOUNCE_MS = 250

/**
 * A search run while the user types: schedule() waits SEARCH_DEBOUNCE_MS after
 * the last call, cancels the request still in flight (only the latest answer is
 * kept) and reports errors other than the cancellation.
 */
export function useDebouncedSearch<T> (
  searcher: (query: string, signal: AbortSignal) => Promise<T>,
  initial: T,
): { results: Ref<T>, loading: Ref<boolean>, run: (query: string) => Promise<void>, schedule: (query: string) => void, cancel: () => void } {
  const { report } = useApiErrors()
  const results = ref(initial) as Ref<T>
  const loading = ref(false)
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined

  async function run (query: string) {
    clearTimeout(timer)
    controller?.abort()
    const current = new AbortController()
    controller = current
    loading.value = true
    try {
      results.value = await searcher(query.trim(), current.signal)
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError')) {
        report(error)
      }
    } finally {
      if (controller === current) {
        loading.value = false
      }
    }
  }

  function schedule (query: string) {
    clearTimeout(timer)
    timer = setTimeout(() => void run(query), SEARCH_DEBOUNCE_MS)
  }

  function cancel () {
    clearTimeout(timer)
    controller?.abort()
    loading.value = false
  }

  onBeforeUnmount(cancel)
  return { results, loading, run, schedule, cancel }
}
