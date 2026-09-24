<script setup lang="ts">
  import { storeToRefs } from 'pinia'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useAuthStore } from '@/stores/auth'
  import { loopbackMismatchUrl } from '@/utils/authOrigin'
  import DevTokenForm from './DevTokenForm.vue'

  // Shown instead of the app while nobody is signed in: explains how to sign in
  // for the configured mode and surfaces an unreachable auth service.
  const { t } = useI18n()
  const auth = useAuthStore()
  const { mode, error, authBaseUrl } = storeToRefs(auth)

  // 127.0.0.1 vs localhost: the auth service would refuse the redirect.
  const betterUrl = computed(() => mode.value === 'jwt'
    ? loopbackMismatchUrl(window.location.href, authBaseUrl.value)
    : undefined)
  const currentOrigin = window.location.origin
</script>

<template>
  <v-card class="pa-2" max-width="480" width="100%">
    <v-card-title class="d-flex align-center text-wrap">
      <v-icon class="mr-2" icon="mdi-lock-outline" />
      {{ t('messages.common.signInRequired') }}
    </v-card-title>

    <v-card-text>
      <v-alert
        v-if="error"
        class="mb-4"
        density="compact"
        type="warning"
        variant="tonal"
      >
        {{ t('auth.unreachable') }} <code>{{ authBaseUrl }}</code>
      </v-alert>

      <template v-if="mode === 'dev'">
        <p class="mb-4">{{ t('auth.devExplain') }}</p>
        <DevTokenForm />
      </template>

      <template v-else>
        <v-alert
          v-if="betterUrl"
          class="mb-4"
          density="compact"
          type="info"
          variant="tonal"
        >
          {{ t('auth.originMismatch') }}
          <a class="d-block mt-1" :href="betterUrl">{{ betterUrl }}</a>
        </v-alert>

        <p class="mb-2">{{ t('auth.ssoExplain') }}</p>
        <p class="text-caption text-medium-emphasis mb-4">{{ t('auth.ssoAllowlist', { origin: currentOrigin }) }}</p>

        <div class="d-flex flex-wrap ga-2 justify-end">
          <v-btn prepend-icon="mdi-refresh" variant="text" @click="auth.mintToken()">
            {{ t('auth.retry') }}
          </v-btn>

          <v-btn color="primary" prepend-icon="mdi-login" variant="flat" @click="auth.signIn()">
            {{ t('auth.signIn') }}
          </v-btn>
        </div>
      </template>
    </v-card-text>
  </v-card>
</template>
