<script setup lang="ts">
  import type { CatalogueConfig, CatalogueEntry, CatalogueField } from './referenceCatalogues'
  import type { ListSort } from '@/utils/listSort'
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { createReferenceEntry, updateReferenceEntry } from '@/api/referenceClient'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useUiStore } from '@/stores/ui'
  import { sortRows } from '@/utils/listSort'
  import { maxLength, required } from '@/utils/validation'
  import { CONFIDENTIAL_LEVEL, CONFIDENTIALITY_LEVELS, REFERENCE_CODE, SUBJECT_KINDS } from './referenceCatalogues'

  // Generic editor of one reference catalogue: list (active and inactive),
  // create, edit and (de)activate. Codes are immutable; entries are never deleted.
  // The row-actions slot adds catalogue-specific actions (case type default grants).
  const props = defineProps<{ config: CatalogueConfig }>()
  const emit = defineEmits<{ changed: [] }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const entries = ref<CatalogueEntry[]>([])
  const loading = ref(false)
  const dialogOpen = ref(false)
  const editing = ref<CatalogueEntry | null>(null)
  const draft = ref<Record<string, unknown>>({})
  const reason = ref('')
  const saving = ref(false)
  const form = ref()

  const columns = computed(() => props.config.fields.filter(f => f.column))
  // The whole catalogue is loaded: sorted here by any column (GLD-056).
  const sort = ref<ListSort>()
  const sortedEntries = computed(() => sortRows(entries.value, sort.value, sortValue))
  const kindItems = computed(() => SUBJECT_KINDS.map(k => ({ value: k, title: enumLabel('SubjectKind', k) })))
  const confidentialityItems = computed(() => CONFIDENTIALITY_LEVELS.map(level => ({ value: level, title: confidentialityLabel(level) })))

  function confidentialityLabel (level: number): string {
    return level >= CONFIDENTIAL_LEVEL ? t('fields.reference.confidentialLevel', { level }) : String(level)
  }

  async function load () {
    loading.value = true
    try {
      entries.value = await props.config.list() // already ordered by the server
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }
  onMounted(load)
  defineExpose({ reload: load })

  function openCreate () {
    editing.value = null
    draft.value = { code: '', isActive: true, isDirected: true, defaultConfidentialityLevel: 0 }
    reason.value = ''
    dialogOpen.value = true
  }

  function openEdit (entry: CatalogueEntry) {
    editing.value = entry
    draft.value = { ...entry, isActive: isActive(entry) }
    reason.value = ''
    dialogOpen.value = true
  }

  function editable (field: CatalogueField): boolean {
    return !editing.value || !field.createOnly
  }

  function rules (field: CatalogueField) {
    const list = field.required ? [required(t)] : []
    return field.max ? [...list, maxLength(t, field.max)] : list
  }

  const codeRules = [required(t), (v: unknown) => REFERENCE_CODE.test(typeof v === 'string' ? v : '') || t('validation.referenceCode')]

  // Create sends every field; update sends the mutable ones and the activation.
  function payload (): Record<string, unknown> {
    const body: Record<string, unknown> = { reason: reason.value.trim() || undefined }
    for (const field of props.config.fields) {
      if (editable(field)) body[field.key] = draft.value[field.key]
    }
    if (editing.value) {
      body.isActive = draft.value.isActive
    } else {
      body.code = draft.value.code
    }
    return body
  }

  async function save () {
    const validation = await form.value?.validate()
    if (validation && !validation.valid) return
    saving.value = true
    try {
      await (editing.value
        ? updateReferenceEntry(props.config.catalogue, editing.value.code, payload())
        : createReferenceEntry(props.config.catalogue, payload()))
      ui.notify(t('messages.reference.saved'), 'success')
      dialogOpen.value = false
      await load()
      emit('changed')
    } catch (error) {
      report(error)
    } finally {
      saving.value = false
    }
  }

  // proto3 JSON omits false booleans: an entry is active only when isActive is true.
  function isActive (entry: CatalogueEntry): boolean {
    return entry.isActive === true
  }

  function cell (entry: CatalogueEntry, field: CatalogueField): string {
    const value = entry[field.key]
    if (field.kind === 'subjectKind') return enumLabel('SubjectKind', typeof value === 'string' ? value : undefined)
    if (field.kind === 'confidentiality') return confidentialityLabel(typeof value === 'number' ? value : 0)
    return typeof value === 'string' && value !== '' ? value : '—'
  }

  // The sort key of a column: what the cell shows (an empty cell last), active entries first.
  function sortValue (entry: CatalogueEntry, key: string): string | number | undefined {
    if (key === 'code') return entry.code
    if (key === 'isActive') return isActive(entry) ? 0 : 1
    const field = columns.value.find(f => f.key === key)
    const shown = field ? cell(entry, field) : undefined
    return shown === '—' ? undefined : shown
  }
</script>

<template>
  <div>
    <div class="d-flex align-center mb-2">
      <p class="text-caption text-medium-emphasis mb-0">{{ t(`messages.reference.hint.${config.catalogue}`) }}</p>
      <v-spacer />

      <v-btn
        color="primary"
        prepend-icon="mdi-plus"
        size="small"
        variant="flat"
        @click="openCreate"
      >
        {{ t('actions.reference.create') }}
      </v-btn>
    </div>

    <v-progress-linear v-if="loading" indeterminate />

    <v-table density="compact">
      <thead>
        <tr>
          <SortableHeader v-model:sort="sort" field="code" :label="t('fields.reference.code')" />

          <SortableHeader
            v-for="field in columns"
            :key="field.key"
            v-model:sort="sort"
            :field="field.key"
            :label="t(`fields.reference.${field.key}`)"
          />

          <SortableHeader v-model:sort="sort" field="isActive" :label="t('fields.reference.isActive')" />
          <th scope="col" />
        </tr>
      </thead>

      <tbody>
        <tr v-for="entry in sortedEntries" :key="entry.code" :class="{ 'text-disabled': !isActive(entry) }">
          <td><code>{{ entry.code }}</code></td>
          <td v-for="field in columns" :key="field.key">{{ cell(entry, field) }}</td>

          <td>
            <v-chip :color="isActive(entry) ? 'success' : 'grey'" label size="x-small">
              {{ isActive(entry) ? t('fields.reference.active') : t('fields.reference.inactive') }}
            </v-chip>
          </td>

          <td class="text-right text-no-wrap">
            <slot :entry="entry" name="row-actions" />

            <v-btn
              :aria-label="t('actions.reference.edit')"
              icon="mdi-pencil"
              size="small"
              :title="t('actions.reference.edit')"
              variant="text"
              @click="openEdit(entry)"
            />
          </td>
        </tr>
      </tbody>
    </v-table>

    <v-dialog v-model="dialogOpen" max-width="620">
      <v-card>
        <v-card-title>{{ editing ? t('actions.reference.edit') : t('actions.reference.create') }}</v-card-title>

        <v-card-text>
          <v-form ref="form" @submit.prevent="save">
            <v-text-field
              v-model="draft.code"
              :disabled="!!editing"
              :hint="t('messages.reference.codeHint')"
              :label="t('fields.reference.code')"
              persistent-hint
              :rules="editing ? [] : codeRules"
            />

            <template v-for="field in config.fields" :key="field.key">
              <v-select
                v-if="field.kind === 'subjectKind'"
                v-model="draft[field.key]"
                class="mt-2"
                :disabled="!editable(field)"
                item-title="title"
                item-value="value"
                :items="kindItems"
                :label="t(`fields.reference.${field.key}`)"
                :rules="rules(field)"
              />

              <v-select
                v-else-if="field.kind === 'confidentiality'"
                v-model="draft[field.key]"
                class="mt-2"
                item-title="title"
                item-value="value"
                :items="confidentialityItems"
                :label="t(`fields.reference.${field.key}`)"
              />

              <v-checkbox
                v-else-if="field.kind === 'boolean'"
                v-model="draft[field.key]"
                density="compact"
                :disabled="!editable(field)"
                hide-details
                :label="t(`fields.reference.${field.key}`)"
              />

              <v-textarea
                v-else-if="field.kind === 'textarea'"
                v-model="draft[field.key]"
                auto-grow
                class="mt-2"
                :label="t(`fields.reference.${field.key}`)"
                rows="2"
                :rules="rules(field)"
              />

              <v-text-field
                v-else
                v-model="draft[field.key]"
                class="mt-2"
                :label="t(`fields.reference.${field.key}`)"
                :rules="rules(field)"
              />
            </template>

            <v-switch
              v-if="editing"
              v-model="draft.isActive"
              color="success"
              :hint="t('messages.reference.inactiveHint')"
              :label="t('fields.reference.isActive')"
              persistent-hint
            />

            <v-text-field v-model="reason" class="mt-2" :label="t('fields.reference.reason')" :rules="[maxLength(t, 2000)]" />
          </v-form>
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialogOpen = false">{{ t('actions.common.cancel') }}</v-btn>
          <v-btn color="primary" :loading="saving" variant="flat" @click="save">{{ t('actions.common.save') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>
