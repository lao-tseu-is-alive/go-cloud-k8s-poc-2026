<script setup lang="ts">
  import type { SecurityGroup } from '@/api/types'
  import type { ListSort } from '@/utils/listSort'
  import { computed, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { createGroup, listGroups } from '@/api/accessClient'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { sortRows } from '@/utils/listSort'
  import { maxLength, required } from '@/utils/validation'

  // Security groups (GLD-048): named sets of internal users given grants like
  // an org unit, for cross-unit audiences. Anyone may create one and fully
  // controls it; archived groups grant nothing.
  const { t } = useI18n()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const groups = ref<SecurityGroup[]>([])
  // Every group is loaded at once (a few hundred at most): sorted here.
  const sort = ref<ListSort>()
  const sortedGroups = computed(() => sortRows(groups.value, sort.value, (g, field) => {
    if (field === 'members') return g.memberCount ?? 0
    return field === 'description' ? g.description : g.name
  }))
  const query = ref('')
  const includeArchived = ref(false)
  const loading = ref(false)
  const creating = ref(false)
  const name = ref('')
  const description = ref('')
  const busy = ref(false)

  async function load () {
    loading.value = true
    try {
      groups.value = await listGroups(query.value, includeArchived.value)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function open (g: SecurityGroup) {
    if (g.subjectRef?.id) void router.push(`/groups/${g.subjectRef.id}`)
  }

  async function doCreate () {
    if (!name.value.trim()) return
    busy.value = true
    try {
      const res = await createGroup(name.value.trim(), description.value.trim())
      ui.notify(t('groups.messages.created'), 'success')
      creating.value = false
      if (res.group) open(res.group)
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  watch(includeArchived, load)
  onMounted(load)
</script>

<template>
  <v-container>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h4">{{ t('groups.title') }}</h1>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-plus" variant="flat" @click="creating = true; name = ''; description = ''">{{ t('groups.actions.create') }}</v-btn>
    </div>

    <p class="text-body-2 text-medium-emphasis mb-4">{{ t('groups.intro') }}</p>

    <v-card class="mb-4">
      <v-card-text class="d-flex flex-wrap align-center ga-4">
        <v-text-field
          v-model="query"
          clearable
          hide-details
          :label="t('actions.common.search')"
          max-width="420"
          prepend-inner-icon="mdi-magnify"
          @keydown.enter="load"
        />

        <v-switch v-model="includeArchived" color="primary" hide-details :label="t('groups.includeArchived')" />
        <v-btn variant="tonal" @click="load">{{ t('actions.common.search') }}</v-btn>
      </v-card-text>
    </v-card>

    <v-card>
      <v-progress-linear v-if="loading" color="primary" indeterminate />
      <p v-if="!loading && groups.length === 0" class="pa-4 text-medium-emphasis">{{ t('messages.common.noData') }}</p>

      <v-table v-else>
        <thead>
          <tr>
            <SortableHeader v-model:sort="sort" field="name" :label="t('groups.fields.name')" />
            <SortableHeader v-model:sort="sort" field="description" :label="t('groups.fields.description')" />
            <SortableHeader v-model:sort="sort" field="members" :label="t('groups.fields.members')" />
          </tr>
        </thead>

        <tbody>
          <tr
            v-for="g in sortedGroups"
            :key="g.subjectRef?.id"
            class="clickable"
            tabindex="0"
            @click="open(g)"
            @keydown.enter="open(g)"
          >
            <td>
              {{ g.name }}
              <v-chip v-if="g.archivedAt" class="ml-2" size="x-small" variant="outlined">{{ t('groups.archived') }}</v-chip>
            </td>

            <td class="text-medium-emphasis">{{ g.description || '—' }}</td>
            <td>{{ g.memberCount ?? 0 }}</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>

    <v-dialog v-model="creating" max-width="520">
      <v-card :title="t('groups.actions.create')">
        <v-card-text>
          <v-text-field v-model="name" :label="t('groups.fields.name')" :rules="[required(t), maxLength(t, 200)]" />

          <v-textarea
            v-model="description"
            auto-grow
            :label="t('groups.fields.description')"
            rows="2"
            :rules="[maxLength(t, 2000)]"
          />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="creating = false">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="primary"
            :disabled="!name.trim()"
            :loading="busy"
            variant="flat"
            @click="doCreate"
          >{{ t('groups.actions.create') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>

<style scoped>
  .clickable {
    cursor: pointer;
  }
</style>
