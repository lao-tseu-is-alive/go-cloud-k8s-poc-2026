<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { createActor } from '@/api/actorClient'
  import { buildCreateRequest, emptyActorForm } from '@/components/actor/actorForm'
  import ActorMainForm from '@/components/actor/ActorMainForm.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'

  const { t } = useI18n()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const form = ref()
  const model = ref(emptyActorForm())
  const saving = ref(false)

  async function submit () {
    const validation = await form.value?.validate()
    if (validation && !validation.valid) return

    saving.value = true
    try {
      const actor = await createActor(buildCreateRequest(model.value))
      ui.notify(t('messages.actor.createSuccess'), 'success')
      const id = actor.subjectRef?.id
      router.push(id ? `/actors/${id}` : '/actors')
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
      <h1 class="text-h5">{{ t('pages.actors.create.title') }}</h1>
    </div>

    <v-row justify="center">
      <v-col cols="12" lg="7" md="8">
        <v-form ref="form" @submit.prevent="submit">
          <v-card>
            <v-card-title class="text-subtitle-1">{{ t('sections.actor.identity') }}</v-card-title>

            <v-card-text>
              <ActorMainForm v-model="model" />
            </v-card-text>

            <v-card-actions>
              <v-spacer />
              <v-btn variant="text" @click="router.back()">{{ t('actions.common.cancel') }}</v-btn>

              <v-btn color="primary" :loading="saving" type="submit" variant="flat">
                {{ t('actions.actor.create') }}
              </v-btn>
            </v-card-actions>
          </v-card>
        </v-form>
      </v-col>
    </v-row>
  </v-container>
</template>
