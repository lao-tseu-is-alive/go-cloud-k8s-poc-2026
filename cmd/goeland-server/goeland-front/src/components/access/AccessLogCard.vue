<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import AccessAuditPanel from '@/components/access/AccessAuditPanel.vue'

  // The access log of one subject (GLD-089), in the governance part of its
  // page: who consulted, downloaded or exported it and who was refused, shown
  // to anyone who may read the subject, as in the legacy Goéland.
  defineProps<{ subjectId: string }>()
  const { t } = useI18n()
</script>

<template>
  <v-card class="mb-4" tag="section">
    <v-card-title class="text-subtitle-1">{{ t('access.audit.subjectTitle') }}</v-card-title>
    <v-card-text><AccessAuditPanel :page-size="10" :subject-id="subjectId" /></v-card-text>
  </v-card>
</template>
