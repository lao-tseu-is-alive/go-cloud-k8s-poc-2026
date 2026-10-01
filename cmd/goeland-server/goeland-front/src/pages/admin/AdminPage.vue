<script setup lang="ts">
  import type { CaseType, ReferenceCatalogue } from '@/api/types'
  import { storeToRefs } from 'pinia'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import CaseTypeDefaultGrantsDialog from '@/components/admin/CaseTypeDefaultGrantsDialog.vue'
  import ReferenceCatalogPanel from '@/components/admin/ReferenceCatalogPanel.vue'
  import { CATALOGUES } from '@/components/admin/referenceCatalogues'
  import ReferenceChangesPanel from '@/components/admin/ReferenceChangesPanel.vue'
  import RolesAdminPanel from '@/components/admin/RolesAdminPanel.vue'
  import { useAuthStore } from '@/stores/auth'

  // Administration, for holders of the ADMIN role only: reference data (GLD-040)
  // and the application roles of the users (GLD-047). The server enforces the
  // role on every call; this page only hides what cannot work.
  const { t } = useI18n()
  const { isAdmin } = storeToRefs(useAuthStore())
  const tab = ref<ReferenceCatalogue | 'log' | 'roles'>('case_type')
  const log = ref<InstanceType<typeof ReferenceChangesPanel>>()
  const caseTypes = ref<InstanceType<typeof ReferenceCatalogPanel>[]>([])
  const defaultsOf = ref<CaseType | null>(null) // the case type whose default grants are edited

  function defaultsSaved () {
    defaultsOf.value = null
    for (const panel of caseTypes.value) void panel.reload()
    log.value?.reload()
  }
</script>

<template>
  <v-container>
    <h1 class="text-h4 mb-4">{{ t('pages.admin.title') }}</h1>

    <v-alert v-if="!isAdmin" type="warning" variant="tonal">{{ t('messages.reference.adminOnly') }}</v-alert>

    <v-card v-else>
      <v-tabs v-model="tab" show-arrows>
        <v-tab v-for="c in CATALOGUES" :key="c.catalogue" :value="c.catalogue">{{ t(`sections.reference.${c.catalogue}`) }}</v-tab>
        <v-tab prepend-icon="mdi-history" value="log">{{ t('sections.reference.log') }}</v-tab>
        <v-tab prepend-icon="mdi-shield-account-outline" value="roles">{{ t('roles.tab') }}</v-tab>
      </v-tabs>

      <v-card-text>
        <v-window v-model="tab">
          <v-window-item v-for="c in CATALOGUES" :key="c.catalogue" :value="c.catalogue">
            <ReferenceCatalogPanel v-if="c.catalogue === 'case_type'" ref="caseTypes" :config="c" @changed="log?.reload()">
              <template #row-actions="{ entry }">
                <v-btn
                  :aria-label="t('caseTypeDefaults.open')"
                  icon="mdi-shield-key-outline"
                  size="small"
                  :title="t('caseTypeDefaults.open')"
                  variant="text"
                  @click="defaultsOf = entry as unknown as CaseType"
                />
              </template>
            </ReferenceCatalogPanel>

            <ReferenceCatalogPanel v-else :config="c" @changed="log?.reload()" />
          </v-window-item>

          <v-window-item value="log">
            <ReferenceChangesPanel ref="log" />
          </v-window-item>

          <v-window-item value="roles">
            <RolesAdminPanel />
          </v-window-item>
        </v-window>
      </v-card-text>
    </v-card>

    <CaseTypeDefaultGrantsDialog :case-type="defaultsOf" @close="defaultsOf = null" @saved="defaultsSaved" />
  </v-container>
</template>
