<script setup lang="ts">
  import type { AuditEvent } from '@/api/types'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'

  // Read-only wrapper around the shared AuditTimeline for the document detail.
  defineProps<{ events?: AuditEvent[] }>()
</script>

<template>
  <AuditTimeline :events="events" />
</template>
