<script setup lang="ts">
  import type { ReferenceCatalogue } from '@/api/types'
  import { storeToRefs } from 'pinia'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import ReferenceCatalogPanel from '@/components/admin/ReferenceCatalogPanel.vue'
  import { CATALOGUES } from '@/components/admin/referenceCatalogues'
  import ReferenceChangesPanel from '@/components/admin/ReferenceChangesPanel.vue'
  import { useAuthStore } from '@/stores/auth'

  // Reference data administration (GLD-040), for goeland:admin users only; the
  // server enforces the scope on every call, this page only hides what cannot work.
  const { t } = useI18n()
  const { isAdmin } = storeToRefs(useAuthStore())
  const tab = ref<ReferenceCatalogue | 'log'>('case_type')
  const log = ref<InstanceType<typeof ReferenceChangesPanel>>()
</script>

<template>
  <v-container>
    <h1 class="text-h4 mb-4">{{ t('pages.admin.title') }}</h1>

    <v-alert v-if="!isAdmin" type="warning" variant="tonal">{{ t('messages.reference.adminOnly') }}</v-alert>

    <v-card v-else>
      <v-tabs v-model="tab" show-arrows>
        <v-tab v-for="c in CATALOGUES" :key="c.catalogue" :value="c.catalogue">{{ t(`sections.reference.${c.catalogue}`) }}</v-tab>
        <v-tab prepend-icon="mdi-history" value="log">{{ t('sections.reference.log') }}</v-tab>
      </v-tabs>

      <v-card-text>
        <v-window v-model="tab">
          <v-window-item v-for="c in CATALOGUES" :key="c.catalogue" :value="c.catalogue">
            <ReferenceCatalogPanel :config="c" @changed="log?.reload()" />
          </v-window-item>

          <v-window-item value="log">
            <ReferenceChangesPanel ref="log" />
          </v-window-item>
        </v-window>
      </v-card-text>
    </v-card>
  </v-container>
</template>
