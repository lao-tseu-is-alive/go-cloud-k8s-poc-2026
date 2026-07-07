<script setup lang="ts">
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'

  // Confirmation dialog for the critical Finalize action. Emits `submit` with the
  // reason + governance-lock choice; the parent performs the API call.
  const props = defineProps<{ busy?: boolean }>()
  const emit = defineEmits<{ submit: [payload: { reason: string, alsoLock: boolean }] }>()
  const open = defineModel<boolean>({ required: true })

  const { t } = useI18n()
  const reason = ref('')
  const alsoLock = ref(false)

  watch(open, isOpen => {
    if (isOpen) {
      reason.value = ''
      alsoLock.value = false
    }
  })
</script>

<template>
  <v-dialog v-model="open" max-width="520">
    <v-card>
      <v-card-title>{{ t('finalize.title') }}</v-card-title>

      <v-card-text>
        <v-alert class="mb-3" density="compact" type="warning" variant="tonal">
          {{ t('messages.document.finalizeConfirm') }}
        </v-alert>

        <v-text-field v-model="reason" :label="t('finalize.reason')" />
        <v-switch v-model="alsoLock" color="primary" :label="t('finalize.alsoLock')" />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="open = false">{{ t('actions.common.cancel') }}</v-btn>

        <v-btn
          color="warning"
          :loading="props.busy"
          variant="flat"
          @click="emit('submit', { reason: reason.trim(), alsoLock })"
        >
          {{ t('actions.document.finalize') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
