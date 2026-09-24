<script setup lang="ts">
  import { storeToRefs } from 'pinia'
  import { useI18n } from 'vue-i18n'
  import { useAuthStore } from '@/stores/auth'
  import DevTokenForm from './DevTokenForm.vue'

  const { t } = useI18n()
  const auth = useAuthStore()
  const { isAuthenticated, mode, displayName } = storeToRefs(auth)
</script>

<template>
  <div>
    <!-- Connected: show identity + sign out -->
    <v-menu v-if="isAuthenticated" location="bottom end">
      <template #activator="{ props }">
        <v-btn v-bind="props" prepend-icon="mdi-account-circle" variant="text">
          {{ displayName }}
        </v-btn>
      </template>

      <v-list>
        <v-list-item :subtitle="mode.toUpperCase()" :title="displayName" />
        <v-divider />
        <v-list-item prepend-icon="mdi-logout" :title="t('auth.signOut')" @click="auth.signOut()" />
      </v-list>
    </v-menu>

    <!-- dev mode, not connected: token entry -->
    <v-menu v-else-if="mode === 'dev'" :close-on-content-click="false" location="bottom end">
      <template #activator="{ props }">
        <v-btn v-bind="props" color="warning" prepend-icon="mdi-key" variant="tonal">
          {{ t('auth.signIn') }}
        </v-btn>
      </template>

      <v-card min-width="320">
        <v-card-text>
          <DevTokenForm />
        </v-card-text>
      </v-card>
    </v-menu>

    <!-- jwt mode, not connected: redirect to SSO. Outlined without a color so it
         inherits the app bar's on-primary text (color="primary" was invisible on it). -->
    <v-btn
      v-else
      prepend-icon="mdi-login"
      variant="outlined"
      @click="auth.signIn()"
    >
      {{ t('auth.signIn') }}
    </v-btn>
  </div>
</template>
