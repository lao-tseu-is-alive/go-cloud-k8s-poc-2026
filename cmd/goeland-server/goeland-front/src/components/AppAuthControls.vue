<script setup lang="ts">
  import { storeToRefs } from 'pinia'
  import { useI18n } from 'vue-i18n'
  import { buildHint, useAppInfo } from '@/composables/useAppInfo'
  import { useAuthStore } from '@/stores/auth'
  import DevTokenForm from './DevTokenForm.vue'

  // compact shows the user's icon only, for a narrow app bar.
  defineProps<{ compact?: boolean }>()
  const { t } = useI18n()
  const auth = useAuthStore()
  const { isAuthenticated, mode, displayName, me, isAdmin, scopes } = storeToRefs(auth)
  const appInfo = useAppInfo()
</script>

<template>
  <div>
    <!-- Connected: show identity + sign out -->
    <v-menu v-if="isAuthenticated" location="bottom end">
      <template #activator="{ props }">
        <v-btn
          v-if="compact"
          v-bind="props"
          :aria-label="displayName || t('auth.connected')"
          :icon="isAdmin ? 'mdi-shield-account' : 'mdi-account-circle'"
          :title="displayName || t('auth.connected')"
          variant="text"
        />

        <v-btn v-else v-bind="props" :prepend-icon="isAdmin ? 'mdi-shield-account' : 'mdi-account-circle'" variant="text">
          {{ displayName || t('auth.connected') }}
        </v-btn>
      </template>

      <v-list min-width="280">
        <v-list-item :subtitle="me?.email" :title="displayName || t('auth.connected')">
          <template #append>
            <v-chip v-if="isAdmin" color="warning" label size="small">{{ t('auth.admin') }}</v-chip>
          </template>
        </v-list-item>

        <v-list-item class="text-caption text-medium-emphasis">
          {{ t('auth.modeLabel', { mode: mode.toUpperCase() }) }}
          <span v-if="me?.id"> · #{{ me.id }}</span>
          <div v-if="scopes.length > 0">{{ t('auth.scopes') }} : {{ scopes.join(', ') }}</div>
        </v-list-item>

        <v-divider />
        <v-list-item prepend-icon="mdi-checkbox-marked-circle-outline" :title="t('nav.myTasks')" to="/tasks/mine" />
        <v-list-item prepend-icon="mdi-account-group-outline" :title="t('nav.groups')" to="/groups" />
        <v-list-item v-if="isAdmin" prepend-icon="mdi-cog-outline" :title="t('nav.admin')" to="/admin" />
        <v-divider />

        <v-list-item
          v-if="appInfo.version"
          class="text-caption"
          prepend-icon="mdi-information-outline"
          :subtitle="buildHint(appInfo, t)"
          :title="t('app.about', { version: appInfo.version })"
        />

        <v-list-item base-color="error" prepend-icon="mdi-logout" :title="t('auth.signOut')" @click="auth.signOut()" />
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
