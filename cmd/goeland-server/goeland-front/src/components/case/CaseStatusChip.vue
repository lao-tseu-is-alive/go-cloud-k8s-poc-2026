<script setup lang="ts">
  import type { CaseStatus } from '@/api/types'
  import { caseStatusColor } from '@/components/case/caseForm'
  import { useI18nEnum } from '@/composables/useI18nEnum'

  defineProps<{ status?: CaseStatus }>()
  const { enumLabel } = useI18nEnum()
</script>

<template>
  <v-chip :color="caseStatusColor(status)" label size="small">{{ enumLabel('CaseStatus', status) }}</v-chip>
</template>
