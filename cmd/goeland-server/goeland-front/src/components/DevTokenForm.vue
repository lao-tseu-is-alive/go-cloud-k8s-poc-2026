<script setup lang="ts">
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useAuthStore } from '@/stores/auth'

  // dev mode: paste the server's static token (held in memory only).
  const { t } = useI18n()
  const auth = useAuthStore()
  const devToken = ref('')
</script>

<template>
  <div>
    <v-text-field
      v-model="devToken"
      autofocus
      :hint="t('auth.devTokenHint')"
      :label="t('auth.devTokenLabel')"
      persistent-hint
      type="password"
      @keyup.enter="auth.applyDevToken(devToken)"
    />

    <div class="d-flex justify-end mt-2">
      <v-btn color="primary" :disabled="!devToken.trim()" variant="flat" @click="auth.applyDevToken(devToken)">
        {{ t('auth.apply') }}
      </v-btn>
    </div>
  </div>
</template>
