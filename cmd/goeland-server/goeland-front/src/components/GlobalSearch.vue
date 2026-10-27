<script setup lang="ts">
  import type { SubjectSearchGroup } from '@/api/types'
  import type { SearchSuggestion } from '@/utils/globalSearch'
  import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchSubjects } from '@/api/coreClient'
  import { useDebouncedSearch } from '@/composables/useDebouncedSearch'
  import { asSuggestion, buildSearchItems, resultsRoute, searchable } from '@/utils/globalSearch'

  // The global search (GLD-064): one field in the app bar over cases, actors,
  // documents and things, suggestions grouped by kind with their counts; "/"
  // focuses it from anywhere, Enter opens every result. On a small screen it
  // takes the whole width of the bar's second row (block).
  const props = defineProps<{ block?: boolean }>()
  const emit = defineEmits<{ navigated: [] }>()
  const { t } = useI18n()
  const router = useRouter()

  const search = ref('')
  const field = ref<{ focus: () => void, blur: () => void }>()

  const { results: groups, loading, schedule, cancel } = useDebouncedSearch<SubjectSearchGroup[]>(
    (query, signal) => searchSubjects({ query }, signal), [])

  const items = computed(() => searchable(search.value) ? buildSearchItems(t, search.value, groups.value) : [])
  const noDataText = computed(() => {
    if (!searchable(search.value)) return t('globalSearch.minChars')
    return loading.value ? t('globalSearch.searching') : t('globalSearch.noResult', { query: search.value.trim() })
  })

  watch(search, term => {
    if (searchable(term ?? '')) {
      schedule(term)
    } else {
      cancel()
      groups.value = []
    }
  })

  // A pick navigates and empties the field; the model itself stays empty.
  async function open (picked: SearchSuggestion | null) {
    if (!picked) return
    cancel()
    await router.push(picked.to)
    search.value = ''
    groups.value = []
    await nextTick()
    field.value?.blur()
    emit('navigated')
  }

  // Enter before the suggestions arrive still opens every result.
  function onEnter () {
    if (items.value.length === 0 && searchable(search.value)) {
      void open({ value: 'all', title: '', icon: '', to: resultsRoute(search.value) })
    }
  }

  // "/" focuses the field, unless the user is typing somewhere else.
  function onSlash (event: KeyboardEvent) {
    if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey) return
    const target = event.target as HTMLElement | null
    if (target && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))) return
    event.preventDefault()
    field.value?.focus()
  }
  onMounted(() => window.addEventListener('keydown', onSlash))
  onBeforeUnmount(() => window.removeEventListener('keydown', onSlash))
</script>

<template>
  <v-autocomplete
    ref="field"
    v-model:search="search"
    :aria-label="t('globalSearch.label')"
    auto-select-first
    bg-color="surface"
    class="global-search"
    :class="{ 'global-search--block': props.block }"
    density="compact"
    flat
    hide-details
    item-value="value"
    :items="items"
    :loading="loading"
    menu-icon=""
    :menu-props="{ maxHeight: 'min(75vh, 640px)', minWidth: props.block ? undefined : 420 }"
    :model-value="null"
    :no-data-text="noDataText"
    no-filter
    :placeholder="t('globalSearch.placeholder')"
    prepend-inner-icon="mdi-magnify"
    return-object
    rounded
    variant="solo"
    @keydown.enter="onEnter"
    @update:model-value="open"
  >
    <template #append-inner>
      <kbd v-if="!search" class="global-search__key" :title="t('globalSearch.shortcut')">/</kbd>
    </template>

    <template #subheader="{ props: header }">
      <v-list-subheader class="font-weight-bold">{{ header.title }}</v-list-subheader>
    </template>

    <template #item="{ props: itemProps, item }">
      <v-list-item
        v-bind="itemProps"
        :class="{ 'text-primary': asSuggestion(item).more }"
        :prepend-icon="asSuggestion(item).icon"
        :subtitle="asSuggestion(item).subtitle"
      />
    </template>
  </v-autocomplete>
</template>

<style scoped>
  /* The app bar is crowded: a fixed width keeps the query readable. */
  .global-search {
    width: 280px;
  }

  .global-search--block {
    width: 100%;
  }

  .global-search__key {
    border: 1px solid rgba(var(--v-theme-on-surface), 0.3);
    border-radius: 4px;
    font-size: 0.75rem;
    line-height: 1;
    opacity: 0.7;
    padding: 2px 6px;
  }
</style>
