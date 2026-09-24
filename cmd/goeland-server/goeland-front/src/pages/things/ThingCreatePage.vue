<script setup lang="ts">
  import type { ThingType } from '@/api/types'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { createThing } from '@/api/thingClient'
  import { buildCreateThingRequest, emptyThingForm } from '@/components/thing/thingForm'
  import ThingMainForm from '@/components/thing/ThingMainForm.vue'
  import ThingTypeSelect from '@/components/thing/ThingTypeSelect.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { required } from '@/utils/validation'

  const { t } = useI18n()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const form = ref()
  const model = ref(emptyThingForm())
  const saving = ref(false)

  function onType (type: ThingType | undefined) {
    model.value.specialization = type?.specialization ?? 'THING_SPECIALIZATION_UNSPECIFIED'
  }

  async function submit () {
    const validation = await form.value?.validate()
    if (validation && !validation.valid) return
    saving.value = true
    try {
      const thing = await createThing(buildCreateThingRequest(model.value))
      ui.notify(t('messages.thing.createSuccess'), 'success')
      const id = thing.subjectRef?.id
      router.push(id ? `/things/${id}` : '/things')
    } catch (error) {
      report(error)
    } finally {
      saving.value = false
    }
  }
</script>

<template>
  <v-container>
    <div class="d-flex align-center mb-4">
      <v-btn icon="mdi-arrow-left" variant="text" @click="router.back()" />
      <h1 class="text-h5 ml-2">{{ t('pages.things.create.title') }}</h1>
    </div>

    <v-card class="mx-auto" max-width="900">
      <v-card-text>
        <v-form ref="form" @submit.prevent="submit">
          <ThingTypeSelect v-model="model.thingTypeCode" :rules="[required(t)]" @selected="onType" />
          <ThingMainForm v-if="model.thingTypeCode" v-model="model" />

          <div class="d-flex justify-end ga-2 mt-4">
            <v-btn variant="text" @click="router.back()">{{ t('actions.common.cancel') }}</v-btn>
            <v-btn color="primary" :loading="saving" type="submit" variant="flat">{{ t('actions.thing.create') }}</v-btn>
          </div>
        </v-form>
      </v-card-text>
    </v-card>
  </v-container>
</template>
