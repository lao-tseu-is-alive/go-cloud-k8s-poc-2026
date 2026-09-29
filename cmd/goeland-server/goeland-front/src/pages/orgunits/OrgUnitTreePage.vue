<script setup lang="ts">
  import type { OrgUnitInput, OrgUnitNode } from '@/api/types'
  import { storeToRefs } from 'pinia'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { createOrgUnit, listOrgUnits } from '@/api/orgUnitClient'
  import { buildTree, emptyOrgUnitForm } from '@/components/orgunit/orgUnitForm'
  import OrgUnitFormDialog from '@/components/orgunit/OrgUnitFormDialog.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'

  // The organization as a tree (about a thousand units, loaded at once): search
  // filters it, a unit opens its page, administrators add units.
  const { t } = useI18n()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const { isAdmin } = storeToRefs(useAuthStore())

  const nodes = ref<OrgUnitNode[]>([])
  const loading = ref(false)
  const includeDissolved = ref(false)
  const search = ref('')
  const opened = ref<string[]>([])
  const createOpen = ref(false)
  const createBusy = ref(false)

  const tree = computed(() => buildTree(nodes.value))
  const liveCount = computed(() => nodes.value.filter(n => !n.dissolved).length)

  async function load () {
    loading.value = true
    try {
      nodes.value = await listOrgUnits(includeDissolved.value)
      // Open the first two levels so the structure reads at a glance.
      opened.value = tree.value.flatMap(root => [root.id, ...(root.children ?? []).map(c => c.id)])
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  async function create (input: OrgUnitInput) {
    createBusy.value = true
    try {
      const unit = await createOrgUnit(input)
      ui.notify(t('orgUnits.messages.created'), 'success')
      createOpen.value = false
      await router.push(`/org-units/${encodeURIComponent(unit.subjectRef?.id ?? '')}`)
    } catch (error) {
      report(error)
    } finally {
      createBusy.value = false
    }
  }

  watch(includeDissolved, load, { immediate: true })
</script>

<template>
  <v-container fluid>
    <div class="d-flex flex-wrap align-center ga-2 mb-4">
      <h1 class="text-h5">{{ t('orgUnits.title') }}</h1>
      <v-chip size="small">{{ t('orgUnits.count', { n: liveCount }) }}</v-chip>
      <v-spacer />

      <v-btn
        v-if="isAdmin"
        color="primary"
        prepend-icon="mdi-plus"
        variant="flat"
        @click="createOpen = true"
      >
        {{ t('orgUnits.actions.create') }}
      </v-btn>
    </div>

    <v-card>
      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-4 mb-2">
          <v-text-field
            v-model="search"
            clearable
            density="compact"
            hide-details
            :label="t('orgUnits.search')"
            prepend-inner-icon="mdi-magnify"
            style="max-width: 420px"
          />

          <v-switch
            v-model="includeDissolved"
            color="primary"
            density="compact"
            hide-details
            :label="t('orgUnits.showDissolved')"
          />
        </div>

        <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />

        <p v-if="!loading && nodes.length === 0" class="text-medium-emphasis">{{ t('orgUnits.empty') }}</p>

        <v-treeview
          v-else
          v-model:opened="opened"
          density="compact"
          item-title="title"
          item-value="id"
          :items="tree"
          :search="search"
        >
          <template #prepend="{ item }">
            <v-icon :color="item.node.dissolved ? 'grey' : 'primary'" icon="mdi-sitemap-outline" size="small" />
          </template>

          <template #title="{ item }">
            <router-link class="unit-link" :class="{ dissolved: item.node.dissolved }" :to="`/org-units/${encodeURIComponent(item.id)}`">
              {{ item.title }}
            </router-link>

            <v-chip v-if="item.node.dissolved" class="ml-2" size="x-small">{{ t('orgUnits.dissolved') }}</v-chip>
          </template>
        </v-treeview>
      </v-card-text>
    </v-card>

    <OrgUnitFormDialog
      v-model="createOpen"
      :busy="createBusy"
      :initial="emptyOrgUnitForm()"
      mode="create"
      @submit="create"
    />
  </v-container>
</template>

<style scoped>
  .unit-link {
    color: inherit;
    text-decoration: none;
  }

  .unit-link:hover,
  .unit-link:focus-visible {
    text-decoration: underline;
  }

  .unit-link.dissolved {
    color: rgb(var(--v-theme-on-surface), 0.5);
    text-decoration: line-through;
  }
</style>
