<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useAuthStore } from '@/stores/auth'

  // dev mode: paste the server's static token (held in memory only).
  const { t } = useI18n()
  const auth = useAuthStore()
  const devToken = ref('')
  const rejected = ref(false)
  const busy = ref(false)

  async function apply () {
    busy.value = true
    rejected.value = !(await auth.applyDevToken(devToken.value))
    busy.value = false
  }
</script>

<template>
  <div>
    <v-text-field
      v-model="devToken"
      autofocus
      :error-messages="rejected ? t('auth.devTokenRejected') : undefined"
      :hint="t('auth.devTokenHint')"
      :label="t('auth.devTokenLabel')"
      persistent-hint
      type="password"
      @keyup.enter="apply"
    />

    <div class="d-flex justify-end mt-2">
      <v-btn
        color="primary"
        :disabled="!devToken.trim()"
        :loading="busy"
        variant="flat"
        @click="apply"
      >
        {{ t('auth.apply') }}
      </v-btn>
    </div>
  </div>
</template>
