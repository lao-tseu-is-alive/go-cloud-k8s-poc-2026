<script setup lang="ts">
  import type { SubjectRef } from '@/api/types'
  import { useI18n } from 'vue-i18n'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { formatDateTime } from '@/utils/formatters'

  defineProps<{ subject?: SubjectRef }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
</script>

<template>
  <v-card v-if="subject" variant="tonal">
    <v-card-item>
      <template #prepend>
        <v-icon icon="mdi-file-document-outline" size="large" />
      </template>

      <v-card-title>{{ subject.displayLabel }}</v-card-title>

      <v-card-subtitle>
        <v-chip class="mr-2" label size="small">{{ enumLabel('SubjectKind', subject.kind) }}</v-chip>
        <span class="text-caption">{{ subject.id }}</span>
      </v-card-subtitle>
    </v-card-item>

    <v-card-text v-if="subject.canonicalUrl || subject.createdAt" class="text-caption">
      <div v-if="subject.canonicalUrl">
        {{ t('fields.subject.canonical_url') }}:
        <a :href="subject.canonicalUrl" rel="noopener" target="_blank">{{ subject.canonicalUrl }}</a>
      </div>

      <div v-if="subject.createdAt">
        {{ t('fields.subject.created_at') }}: {{ formatDateTime(subject.createdAt) }}
      </div>
    </v-card-text>
  </v-card>
</template>
