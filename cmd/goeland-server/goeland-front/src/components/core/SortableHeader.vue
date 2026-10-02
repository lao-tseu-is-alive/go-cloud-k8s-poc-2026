<script setup lang="ts">
  import type { ListSort } from '@/utils/listSort'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { nextSort } from '@/utils/listSort'

  // A column header that sorts its list (GLD-055): a click sorts ascending,
  // the next one descending; aria-sort tells assistive technologies.
  const props = defineProps<{ label: string, field: string }>()
  const sort = defineModel<ListSort | undefined>('sort')
  const { t } = useI18n()

  const active = computed(() => sort.value?.field === props.field)
  const ariaSort = computed(() => {
    if (!active.value) return 'none'
    return sort.value?.desc ? 'descending' : 'ascending'
  })
  const icon = computed(() => {
    if (!active.value) return 'mdi-swap-vertical'
    return sort.value?.desc ? 'mdi-arrow-down' : 'mdi-arrow-up'
  })
</script>

<template>
  <th :aria-sort="ariaSort" scope="col">
    <button
      class="sortable-header"
      :class="{ 'sortable-header--active': active }"
      :title="t('messages.common.sortBy', { column: label })"
      type="button"
      @click="sort = nextSort(sort, field)"
    >
      {{ label }}
      <v-icon :icon="icon" size="x-small" />
    </button>
  </th>
</template>

<style scoped>
  .sortable-header {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font: inherit;
    color: inherit;
    background: none;
    border: 0;
    padding: 0;
    cursor: pointer;
  }

  .sortable-header .v-icon {
    opacity: 0.35;
  }

  .sortable-header--active .v-icon,
  .sortable-header:hover .v-icon,
  .sortable-header:focus-visible .v-icon {
    opacity: 1;
  }
</style>
