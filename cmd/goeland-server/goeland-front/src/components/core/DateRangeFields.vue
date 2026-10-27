<script setup lang="ts">
  import { useI18n } from 'vue-i18n'

  // A period filter « <label> du … au … » (GLD-074): two calendar days as
  // YYYY-MM-DD, either one may stay empty (a cleared field is undefined, so it
  // leaves the URL). Enter searches.
  const from = defineModel<string | undefined>('from')
  const to = defineModel<string | undefined>('to')
  defineProps<{ label: string }>()
  const emit = defineEmits<{ search: [] }>()
  const { t } = useI18n()
</script>

<template>
  <v-col cols="6" md="2">
    <v-text-field
      clearable
      density="compact"
      hide-details
      :label="t('filters.from', { label })"
      :max="to"
      :model-value="from ?? ''"
      type="date"
      @keydown.enter="emit('search')"
      @update:model-value="from = $event || undefined"
    />
  </v-col>

  <v-col cols="6" md="2">
    <v-text-field
      clearable
      density="compact"
      hide-details
      :label="t('filters.to')"
      :min="from"
      :model-value="to ?? ''"
      type="date"
      @keydown.enter="emit('search')"
      @update:model-value="to = $event || undefined"
    />
  </v-col>
</template>
