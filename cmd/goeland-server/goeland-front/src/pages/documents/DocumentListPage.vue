<script setup lang="ts">
  import type { GoDocument, SearchDocumentsParams } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchDocuments } from '@/api/documentClient'
  import DocumentSearchFilters from '@/components/document/DocumentSearchFilters.vue'
  import DocumentStatusChip from '@/components/document/DocumentStatusChip.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { formatDate, formatDateTime } from '@/utils/formatters'

  const { t } = useI18n()
  const router = useRouter()
  const { report } = useApiErrors()

  const PAGE_SIZE = 25
  const filters = ref<SearchDocumentsParams>({ query: '', documentTypeCode: undefined, onlyFinal: false, onlyRecords: false, includeDeleted: false })
  const documents = ref<GoDocument[]>([])
  const nextPageToken = ref('')
  const totalSize = ref(0)
  const loading = ref(false)

  async function load (reset: boolean) {
    loading.value = true
    try {
      const res = await searchDocuments({
        ...filters.value,
        pageSize: PAGE_SIZE,
        pageToken: reset ? undefined : nextPageToken.value || undefined,
      })
      const page = res.documents ?? []
      documents.value = reset ? page : [...documents.value, ...page]
      nextPageToken.value = res.nextPageToken ?? ''
      totalSize.value = res.totalSize ?? documents.value.length
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function onSearch () {
    nextPageToken.value = ''
    void load(true)
  }

  function onReset () {
    filters.value = { query: '', documentTypeCode: undefined, onlyFinal: false, onlyRecords: false, includeDeleted: false }
    onSearch()
  }

  function openDocument (doc: GoDocument) {
    const id = doc.subjectRef?.id
    if (id) router.push(`/documents/${id}`)
  }

  onMounted(() => load(true))
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center justify-space-between mb-4 flex-wrap ga-2">
      <h1 class="text-h5">{{ t('pages.documents.list.title') }}</h1>

      <v-btn color="primary" prepend-icon="mdi-plus" to="/documents/new" variant="flat">
        {{ t('nav.createDocument') }}
      </v-btn>
    </div>

    <DocumentSearchFilters v-model="filters" @reset="onReset" @search="onSearch" />

    <v-card>
      <v-table hover>
        <thead>
          <tr>
            <th>{{ t('fields.document.title') }}</th>
            <th>{{ t('fields.document.document_type') }}</th>
            <th>{{ t('fields.document.status') }}</th>
            <th>{{ t('fields.document.official_date') }}</th>
            <th>{{ t('fields.document.is_final') }}</th>
            <th>{{ t('fields.common.created_at') }}</th>
          </tr>
        </thead>

        <tbody>
          <tr
            v-for="doc in documents"
            :key="doc.subjectRef?.id"
            :class="{ 'text-disabled': doc.recordMetadata?.deletedAt }"
            style="cursor: pointer"
            @click="openDocument(doc)"
          >
            <td>
              {{ doc.title }}
              <v-icon v-if="doc.recordMetadata?.isLocked" icon="mdi-lock" size="x-small" />
            </td>

            <td>{{ doc.documentType?.label ?? doc.documentType?.code ?? '—' }}</td>
            <td><DocumentStatusChip :status="doc.status" /></td>
            <td>{{ formatDate(doc.officialDate) }}</td>

            <td>
              <v-icon
                :color="doc.isFinal ? 'success' : undefined"
                :icon="doc.isFinal ? 'mdi-check-circle' : 'mdi-circle-outline'"
                size="small"
              />
            </td>

            <td class="text-caption">{{ formatDateTime(doc.createdAt) }}</td>
          </tr>

          <tr v-if="!loading && documents.length === 0">
            <td class="text-medium-emphasis text-center py-6" colspan="6">{{ t('messages.common.noData') }}</td>
          </tr>
        </tbody>
      </v-table>

      <v-card-actions>
        <span class="text-caption text-medium-emphasis">{{ documents.length }} / {{ totalSize }}</span>
        <v-spacer />

        <v-btn
          v-if="nextPageToken"
          :loading="loading"
          variant="text"
          @click="load(false)"
        >
          {{ t('actions.common.refresh') }} +
        </v-btn>
      </v-card-actions>

      <v-progress-linear v-if="loading" color="primary" indeterminate />
    </v-card>
  </v-container>
</template>
