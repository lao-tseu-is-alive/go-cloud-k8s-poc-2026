<script setup lang="ts">
  import type { CreateDocumentRequest, UploadResult } from '@/api/types'
  import type { DocumentFormModel } from '@/components/document/documentForm'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { createDocument } from '@/api/documentClient'
  import DocumentMetadataForm from '@/components/document/DocumentMetadataForm.vue'
  import DocumentTypeSelect from '@/components/document/DocumentTypeSelect.vue'
  import DocumentUploadField from '@/components/document/DocumentUploadField.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { required } from '@/utils/validation'

  const { t } = useI18n()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const form = ref()
  const documentTypeCode = ref<string | undefined>(undefined)
  const upload = ref<UploadResult | null>(null)
  const saving = ref(false)
  const model = ref<DocumentFormModel>({ title: '', description: '', officialDate: '', language: '', isRecord: false })

  function onUploaded (result: UploadResult) {
    upload.value = result
    // Prefill the title from the filename if empty, as a convenience.
    if (!model.value.title && result.filename) {
      model.value.title = result.filename.replace(/\.[^.]+$/, '')
    }
  }

  async function submit () {
    const validation = await form.value?.validate()
    if (validation && !validation.valid) return
    if (!documentTypeCode.value) return

    const req: CreateDocumentRequest = {
      documentTypeCode: documentTypeCode.value,
      title: model.value.title.trim(),
      description: model.value.description.trim() || undefined,
      officialDate: model.value.officialDate || undefined,
      language: model.value.language.trim() || undefined,
      isRecord: model.value.isRecord,
    }
    if (upload.value) {
      req.contentBlobId = upload.value.contentBlobId
    }

    saving.value = true
    try {
      const { document: doc, reused } = await createDocument(req)
      // Identical content already had a live document: it was reused (spec v2 §20).
      ui.notify(t(reused ? 'messages.document.reused' : 'messages.document.createSuccess'), reused ? 'info' : 'success')
      const id = doc.subjectRef?.id
      router.push(id ? `/documents/${id}` : '/documents')
    } catch (error) {
      report(error)
    } finally {
      saving.value = false
    }
  }
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center ga-2 mb-4">
      <v-btn icon="mdi-arrow-left" variant="text" @click="router.back()" />
      <h1 class="text-h5">{{ t('pages.documents.create.title') }}</h1>
    </div>

    <v-row justify="center">
      <v-col cols="12" lg="7" md="8">
        <v-form ref="form" @submit.prevent="submit">
          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.storage') }}</v-card-title>

            <v-card-text>
              <DocumentUploadField @uploaded="onUploaded" />
            </v-card-text>
          </v-card>

          <v-card>
            <v-card-title class="text-subtitle-1">{{ t('sections.document.summary') }}</v-card-title>

            <v-card-text>
              <DocumentTypeSelect
                v-model="documentTypeCode"
                :rules="[required(t)]"
              />

              <DocumentMetadataForm v-model="model" show-record-flag />
            </v-card-text>

            <v-card-actions>
              <v-spacer />
              <v-btn variant="text" @click="router.back()">{{ t('actions.common.cancel') }}</v-btn>

              <v-btn color="primary" :loading="saving" type="submit" variant="flat">
                {{ t('actions.document.create') }}
              </v-btn>
            </v-card-actions>
          </v-card>
        </v-form>
      </v-col>
    </v-row>
  </v-container>
</template>
