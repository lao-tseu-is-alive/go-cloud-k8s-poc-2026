<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import { formatTotal } from '@/utils/formatters'

  // The foot of a paged list: rows shown out of the (capped) total, the next
  // page on demand, and a progress bar while a page loads.
  defineProps<{ count: number, total: number, capped: boolean, more: boolean, loading: boolean }>()
  const emit = defineEmits<{ more: [] }>()
  const { t } = useI18n()
</script>

<template>
  <v-card-actions>
    <span class="text-caption text-medium-emphasis">{{ count }} / {{ formatTotal(total, capped) }}</span>
    <v-spacer />
    <v-btn v-if="more" :loading="loading" variant="text" @click="emit('more')">{{ t('actions.common.loadMore') }}</v-btn>
  </v-card-actions>

  <v-progress-linear v-if="loading" color="primary" indeterminate />
</template>
