<script lang="ts" setup>
  import { storeToRefs } from 'pinia'
  import { onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import AppAuthControls from '@/components/AppAuthControls.vue'
  import { SUPPORTED_LOCALES } from '@/plugins/i18n'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'

  const { t, locale } = useI18n()
  const auth = useAuthStore()
  const ui = useUiStore()
  const { isAuthenticated, ready } = storeToRefs(auth)
  const { snackbar } = storeToRefs(ui)

  onMounted(() => auth.bootstrap())
</script>

<template>
  <v-app>
    <v-app-bar color="primary" density="comfortable" flat>
      <v-app-bar-title>
        <span class="font-weight-bold">{{ t('app.title') }}</span>
        <span class="text-caption ml-2 d-none d-sm-inline">{{ t('app.subtitle') }}</span>
      </v-app-bar-title>

      <v-btn class="d-none d-sm-inline-flex" prepend-icon="mdi-file-document-multiple" to="/documents" variant="text">
        {{ t('nav.documents') }}
      </v-btn>

      <v-btn class="d-none d-sm-inline-flex" prepend-icon="mdi-account-multiple" to="/actors" variant="text">
        {{ t('nav.actors') }}
      </v-btn>

      <v-menu location="bottom end">
        <template #activator="{ props }">
          <v-btn v-bind="props" icon="mdi-translate" variant="text" />
        </template>

        <v-list>
          <v-list-item
            v-for="l in SUPPORTED_LOCALES"
            :key="l"
            :active="locale === l"
            :title="l"
            @click="locale = l"
          />
        </v-list>
      </v-menu>

      <AppAuthControls class="ml-2" />
    </v-app-bar>

    <v-main>
      <!-- Gate the app behind authentication -->
      <template v-if="ready">
        <router-view v-if="isAuthenticated" />

        <v-container v-else class="d-flex flex-column align-center justify-center" style="min-height: 60vh">
          <v-icon class="mb-4" icon="mdi-lock-outline" size="64" />
          <p class="text-medium-emphasis">{{ t('messages.common.signInRequired') }}</p>
        </v-container>
      </template>

      <v-container v-else class="d-flex justify-center" style="min-height: 60vh">
        <v-progress-circular class="mt-16" color="primary" indeterminate />
      </v-container>
    </v-main>

    <v-snackbar v-model="snackbar.show" :color="snackbar.tone" location="bottom" timeout="4000">
      {{ snackbar.message }}
    </v-snackbar>
  </v-app>
</template>
