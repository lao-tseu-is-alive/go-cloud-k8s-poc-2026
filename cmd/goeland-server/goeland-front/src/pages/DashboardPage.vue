<script setup lang="ts">
  import { computed, onMounted, reactive } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { searchActors } from '@/api/actorClient'
  import { searchCases } from '@/api/caseClient'
  import { searchDocuments } from '@/api/documentClient'
  import { listMyTasks } from '@/api/taskClient'
  import { searchThings } from '@/api/thingClient'
  import ActorSearchList from '@/components/actor/ActorSearchList.vue'
  import CaseSearchList from '@/components/case/CaseSearchList.vue'
  import DocumentSearchList from '@/components/document/DocumentSearchList.vue'
  import MyTasksPanel from '@/components/task/MyTasksPanel.vue'
  import ThingSearchList from '@/components/thing/ThingSearchList.vue'
  import { formatTotal } from '@/utils/formatters'
  import { kindIcon } from '@/utils/subjects'

  // « Mon tableau de bord » (GLD-088), the home page: the caller's open tasks,
  // then one tab per kind listing what the caller changed or consulted, latest
  // first, each with the kind's own filters. The tab and the filters live in
  // the URL; another tab starts without filters.
  const TABS = ['tasks', 'cases', 'documents', 'things', 'actors'] as const
  type Tab = typeof TABS[number]
  const isTab = (v: unknown): v is Tab => TABS.includes(v as Tab)
  const ICONS: Record<Tab, string> = {
    tasks: 'mdi-checkbox-marked-circle-outline',
    cases: kindIcon('SUBJECT_KIND_CASE'),
    documents: kindIcon('SUBJECT_KIND_DOCUMENT'),
    things: kindIcon('SUBJECT_KIND_THING'),
    actors: kindIcon('SUBJECT_KIND_ACTOR'),
  }

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()

  const tab = computed<Tab>({
    get: () => isTab(route.query.tab) ? route.query.tab : 'tasks',
    set: next => void router.replace({ query: { tab: next } }),
  })

  // The count of every tab, read once (a single row per kind); a list's own
  // total, which follows its filters, replaces it once loaded.
  const totals = reactive<Partial<Record<Tab, { size: number, capped: boolean }>>>({})
  const OPEN_TASKS = ['TASK_STATUS_OPEN', 'TASK_STATUS_IN_PROGRESS'] as const
  const COUNTS: Record<Tab, () => Promise<{ totalSize?: number, totalSizeCapped?: boolean }>> = {
    tasks: () => listMyTasks({ statuses: [...OPEN_TASKS], includeUnits: true, pageSize: 1 }),
    cases: () => searchCases({ myActivity: true, pageSize: 1 }),
    documents: () => searchDocuments({ myActivity: true, pageSize: 1 }),
    things: () => searchThings({ myActivity: true, pageSize: 1 }),
    actors: () => searchActors({ myActivity: true, pageSize: 1 }),
  }

  async function count (kind: Tab) {
    try {
      const res = await COUNTS[kind]()
      totals[kind] ??= { size: res.totalSize ?? 0, capped: !!res.totalSizeCapped }
    } catch {
      // the tab stays without a count; its list reports the error when opened
    }
  }

  function onTotal (kind: Tab, size: number, capped: boolean) {
    totals[kind] = { size, capped }
  }

  function label (kind: Tab): string {
    const total = totals[kind]
    return total ? formatTotal(total.size, total.capped) : '…'
  }

  onMounted(() => {
    for (const kind of TABS) void count(kind)
  })
</script>

<template>
  <v-container fluid>
    <h1 class="text-h5 mb-4">{{ t('dashboard.title') }}</h1>

    <v-tabs v-model="tab" class="mb-4" color="primary" show-arrows>
      <v-tab v-for="kind in TABS" :key="kind" :prepend-icon="ICONS[kind]" :value="kind">
        {{ t(`dashboard.tabs.${kind}`) }}
        <v-chip class="ms-2" label size="x-small">{{ label(kind) }}</v-chip>
      </v-tab>
    </v-tabs>

    <p v-if="tab !== 'tasks'" class="text-body-2 text-medium-emphasis mb-3">{{ t('dashboard.hint') }}</p>

    <MyTasksPanel v-if="tab === 'tasks'" />
    <CaseSearchList v-else-if="tab === 'cases'" my-activity @total="(size, capped) => onTotal('cases', size, capped)" />
    <DocumentSearchList v-else-if="tab === 'documents'" my-activity @total="(size, capped) => onTotal('documents', size, capped)" />
    <ThingSearchList v-else-if="tab === 'things'" my-activity @total="(size, capped) => onTotal('things', size, capped)" />
    <ActorSearchList v-else my-activity @total="(size, capped) => onTotal('actors', size, capped)" />
  </v-container>
</template>
