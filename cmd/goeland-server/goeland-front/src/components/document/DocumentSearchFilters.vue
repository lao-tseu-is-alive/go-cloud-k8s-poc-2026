<script setup lang="ts">
  import type { SearchDocumentsParams } from '@/api/types'
  import { useI18n } from 'vue-i18n'
  import DocumentTypeSelect from './DocumentTypeSelect.vue'

  const model = defineModel<SearchDocumentsParams>({ required: true })
  const emit = defineEmits<{ search: [], reset: [] }>()
  const { t } = useI18n()
</script>

<template>
  <v-card class="mb-4" variant="tonal">
    <v-card-text>
      <v-row align="center" dense>
        <v-col cols="12" md="4">
          <v-text-field
            v-model="model.query"
            clearable
            density="comfortable"
            hide-details
            :label="t('fields.search.query')"
            prepend-inner-icon="mdi-magnify"
            @keyup.enter="emit('search')"
          />
        </v-col>

        <v-col cols="12" md="3">
          <DocumentTypeSelect
            v-model="model.documentTypeCode"
            clearable
          />
        </v-col>

        <v-col cols="6" md="2">
          <v-switch
            v-model="model.onlyFinal"
            color="primary"
            density="compact"
            hide-details
            :label="t('fields.document.only_final')"
          />
        </v-col>

        <v-col cols="6" md="1">
          <v-switch
            v-model="model.onlyRecords"
            color="primary"
            density="compact"
            hide-details
            :label="t('fields.document.only_records')"
          />
        </v-col>

        <v-col class="d-flex ga-2 justify-end" cols="12" md="2">
          <v-btn color="primary" variant="flat" @click="emit('search')">
            {{ t('actions.common.search') }}
          </v-btn>

          <v-btn variant="text" @click="emit('reset')">
            {{ t('actions.common.reset') }}
          </v-btn>
        </v-col>

        <v-col cols="12">
          <v-switch
            v-model="model.includeDeleted"
            color="secondary"
            density="compact"
            hide-details
            :label="t('fields.document.include_deleted')"
          />
        </v-col>
      </v-row>
    </v-card-text>
  </v-card>
</template>
