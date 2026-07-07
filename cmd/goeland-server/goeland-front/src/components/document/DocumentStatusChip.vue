<script setup lang="ts">
  import type { DocumentStatus } from '@/api/types'
  import { computed } from 'vue'
  import { useI18nEnum } from '@/composables/useI18nEnum'

  const props = defineProps<{ status?: DocumentStatus }>()
  const { enumLabel } = useI18nEnum()

  const color = computed(() => {
    switch (props.status) {
      case 'DOCUMENT_STATUS_FINAL': { return 'success'
      }
      case 'DOCUMENT_STATUS_DRAFT': { return 'info'
      }
      case 'DOCUMENT_STATUS_SUPERSEDED': { return 'warning'
      }
      case 'DOCUMENT_STATUS_ARCHIVED': { return 'secondary'
      }
      default: { return 'default'
      }
    }
  })
</script>

<template>
  <v-chip :color="color" label size="small">
    {{ enumLabel('DocumentStatus', status) }}
  </v-chip>
</template>
