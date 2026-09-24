<script setup lang="ts">
  import type { SubjectRelationship } from '@/api/types'
  import { useI18n } from 'vue-i18n'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'

  // Presentational: the parent owns loading + the link/unlink API calls (ending is
  // handled by the table's own dialog, which emits `ended`).
  defineProps<{ relationships?: SubjectRelationship[], canManage?: boolean }>()
  const emit = defineEmits<{ 'add-link': [], 'unlink': [rel: SubjectRelationship], 'ended': [rel: SubjectRelationship] }>()
  const { t } = useI18n()
</script>

<template>
  <div>
    <div class="d-flex justify-end mb-2">
      <v-btn
        v-if="canManage"
        prepend-icon="mdi-link-variant-plus"
        size="small"
        variant="tonal"
        @click="emit('add-link')"
      >
        {{ t('actions.document.link') }}
      </v-btn>
    </div>

    <RelationshipTable
      :can-unlink="canManage"
      :relationships="relationships"
      @ended="emit('ended', $event)"
      @unlink="emit('unlink', $event)"
    />
  </div>
</template>
