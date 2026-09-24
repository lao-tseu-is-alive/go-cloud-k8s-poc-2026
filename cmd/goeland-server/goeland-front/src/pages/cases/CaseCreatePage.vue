<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { createCase } from '@/api/caseClient'
  import CaseTypeSelect from '@/components/case/CaseTypeSelect.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { maxLength, required } from '@/utils/validation'

  const { t } = useI18n()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const form = ref()
  const caseTypeCode = ref<string | undefined>(undefined)
  const title = ref('')
  const description = ref('')
  const saving = ref(false)

  async function submit () {
    const validation = await form.value?.validate()
    if ((validation && !validation.valid) || !caseTypeCode.value) return
    saving.value = true
    try {
      const c = await createCase({
        caseTypeCode: caseTypeCode.value,
        title: title.value.trim(),
        description: description.value.trim() || undefined,
      })
      ui.notify(t('messages.case.createSuccess', { ref: c.subjectRef?.businessRef ?? '' }), 'success')
      const id = c.subjectRef?.id
      router.push(id ? `/cases/${id}` : '/cases')
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
      <h1 class="text-h5">{{ t('pages.cases.create.title') }}</h1>
    </div>

    <v-row justify="center">
      <v-col cols="12" lg="7" md="8">
        <v-form ref="form" @submit.prevent="submit">
          <v-card>
            <v-card-text>
              <CaseTypeSelect v-model="caseTypeCode" :rules="[required(t)]" />
              <div class="text-caption text-medium-emphasis mb-3">{{ t('pages.cases.create.businessRefHint') }}</div>
              <v-text-field v-model="title" :label="t('fields.case.title')" :rules="[required(t), maxLength(t, 500)]" />

              <v-textarea
                v-model="description"
                auto-grow
                :label="t('fields.case.description')"
                rows="3"
                :rules="[maxLength(t, 4000)]"
              />
            </v-card-text>

            <v-card-actions>
              <v-spacer />
              <v-btn variant="text" @click="router.back()">{{ t('actions.common.cancel') }}</v-btn>

              <v-btn color="primary" :loading="saving" type="submit" variant="flat">
                {{ t('actions.case.create') }}
              </v-btn>
            </v-card-actions>
          </v-card>
        </v-form>
      </v-col>
    </v-row>
  </v-container>
</template>
