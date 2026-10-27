<script lang="ts" setup>
  import { storeToRefs } from 'pinia'
  import { computed, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useDisplay } from 'vuetify'
  import AppAuthControls from '@/components/AppAuthControls.vue'
  import GlobalSearch from '@/components/GlobalSearch.vue'
  import SignInPanel from '@/components/SignInPanel.vue'
  import { buildHint, useAppInfo } from '@/composables/useAppInfo'
  import { SUPPORTED_LOCALES } from '@/plugins/i18n'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'
  import { setDateLocale } from '@/utils/formatters'
  import { navItemsFor } from '@/utils/navigation'

  const { t, locale } = useI18n()
  const auth = useAuthStore()
  const ui = useUiStore()
  const { isAuthenticated, isAdmin, ready } = storeToRefs(auth)
  const { snackbar } = storeToRefs(ui)
  const appInfo = useAppInfo()
  const versionHint = computed(() => buildHint(appInfo.value, t))
  const { lgAndUp, xlAndUp } = useDisplay()
  const navItems = computed(() => navItemsFor(isAdmin.value))
  // The drawer and the search row exist below the lg breakpoint only; widening closes them.
  const drawer = ref(false)
  const searchOpen = ref(false)
  watch(lgAndUp, wide => {
    if (wide) {
      drawer.value = false
      searchOpen.value = false
    }
  })

  // Dates on screen follow the interface language.
  watch(locale, l => setDateLocale(l), { immediate: true })

  onMounted(() => auth.bootstrap())
</script>

<template>
  <v-app>
    <!--
      The bar adapts to the screen (Vuetify 4 breakpoints): on an extra-large screen (xl, about
      1545 px and more) every module keeps its label; on a large one (lg, about 1145 px) the main
      modules keep theirs and the others show their icon with a tooltip; below, the modules move
      to a drawer behind the menu button and the search opens on a second row.
    -->
    <v-app-bar color="primary" density="comfortable" flat>
      <template v-if="isAuthenticated && !lgAndUp" #prepend>
        <v-app-bar-nav-icon :aria-label="t('nav.menu')" @click="drawer = !drawer" />
      </template>

      <v-app-bar-title class="app-title">
        <span class="font-weight-bold">{{ t('app.title') }}</span>
        <span class="ml-1 d-none d-sm-inline">{{ t('app.edition') }}</span>

        <v-chip
          v-if="appInfo.version && xlAndUp"
          class="ml-2 app-version"
          label
          size="x-small"
          :title="versionHint"
          variant="tonal"
        >v{{ appInfo.version }}</v-chip>

        <span class="text-caption ml-2 d-none d-xxl-inline">— {{ t('app.subtitle') }}</span>
      </v-app-bar-title>

      <template v-if="isAuthenticated && lgAndUp">
        <template v-for="item in navItems" :key="item.to">
          <v-btn v-if="xlAndUp || item.primary" :prepend-icon="item.icon" :to="item.to" variant="text">{{ t(item.label) }}</v-btn>

          <v-btn
            v-else
            :aria-label="t(item.label)"
            :icon="item.icon"
            :title="t(item.label)"
            :to="item.to"
            variant="text"
          />
        </template>

        <!-- d-flex on the field itself would break the v-input grid layout. -->
        <div class="mx-2"><GlobalSearch /></div>
      </template>

      <v-btn
        v-if="isAuthenticated && !lgAndUp"
        :aria-label="t(searchOpen ? 'globalSearch.close' : 'globalSearch.label')"
        :aria-pressed="searchOpen"
        :icon="searchOpen ? 'mdi-close' : 'mdi-magnify'"
        variant="text"
        @click="searchOpen = !searchOpen"
      />

      <v-menu location="bottom end">
        <template #activator="{ props }">
          <v-btn v-bind="props" :aria-label="t('nav.language')" icon="mdi-translate" variant="text" />
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

      <AppAuthControls class="ml-1" :compact="!xlAndUp" />

      <template v-if="isAuthenticated && !lgAndUp && searchOpen" #extension>
        <div class="px-3 w-100"><GlobalSearch autofocus block @navigated="searchOpen = false" /></div>
      </template>
    </v-app-bar>

    <v-navigation-drawer v-if="isAuthenticated && !lgAndUp" v-model="drawer" temporary>
      <v-list nav>
        <v-list-item
          v-for="item in navItems"
          :key="item.to"
          :prepend-icon="item.icon"
          :title="t(item.label)"
          :to="item.to"
          @click="drawer = false"
        />
      </v-list>
    </v-navigation-drawer>

    <v-main>
      <!-- Gate the app behind authentication -->
      <template v-if="ready">
        <router-view v-if="isAuthenticated" />

        <v-container v-else class="d-flex justify-center align-center" style="min-height: 60vh">
          <SignInPanel />
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

<style scoped>
  .app-version {
    font-family: ui-monospace, monospace;
  }

  /* The title never shrinks to an ellipsis: the bar drops labels first. */
  .app-title {
    flex: 0 0 auto;
    min-width: max-content;
    margin-inline-end: auto;
  }
</style>
