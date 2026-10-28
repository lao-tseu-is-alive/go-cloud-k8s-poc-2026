<script setup lang="ts">
  import type { CaseType, DueRetention, FinalDisposition } from '@/api/types'
  import { storeToRefs } from 'pinia'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { listCaseTypes } from '@/api/caseClient'
  import { applyCaseTypeRetention, FINAL_DISPOSITIONS, listDueRetentions } from '@/api/retentionClient'
  import BordereauCreateDialog from '@/components/retention/BordereauCreateDialog.vue'
  import BordereauList from '@/components/retention/BordereauList.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'
  import { formatDate } from '@/utils/formatters'
  import { dispositionColor, zurichToday } from '@/utils/retention'
  import { maxLength, required } from '@/utils/validation'

  // The retention deadlines (GLD-032): the closed cases whose retention ends on
  // or before a day, oldest first, for the records manager. Administrators also
  // give a case type's policy to its already closed cases. Due cases are
  // selected into a disposition bordereau (second tab).
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const { isAdmin, isRecordsManager } = storeToRefs(useAuthStore())
  const PAGE_SIZE = 50
  const router = useRouter()
  const tab = ref('due')
  // The selected due cases (not on hold), put in a new bordereau.
  const selected = ref<string[]>([])
  const creating = ref(false)

  const dueOn = ref(zurichToday())
  const disposition = ref<FinalDisposition | null>(null)
  const includeHeld = ref(false)
  const rows = ref<DueRetention[]>([])
  const total = ref(0)
  const nextToken = ref('')
  const loading = ref(false)

  const dispositionItems = computed(() => FINAL_DISPOSITIONS.map(d => ({ value: d, title: enumLabel('FinalDisposition', d) })))

  async function load (append = false) {
    if (!isRecordsManager.value) return
    loading.value = true
    try {
      const res = await listDueRetentions({
        dueOn: dueOn.value || undefined,
        finalDisposition: disposition.value ?? undefined,
        includeHeld: includeHeld.value || undefined,
        pageSize: PAGE_SIZE,
        pageToken: append ? nextToken.value : undefined,
      })
      rows.value = append ? [...rows.value, ...(res.retentions ?? [])] : (res.retentions ?? [])
      if (!append) selected.value = []
      total.value = res.totalSize ?? 0
      nextToken.value = res.nextPageToken ?? ''
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }
  watch([dueOn, disposition, includeHeld, isRecordsManager], () => load(), { immediate: true })

  // Application of a case type's policy to its closed cases (administrators).
  const caseTypes = ref<CaseType[]>([])
  const applyType = ref<string | null>(null)
  const applyReason = ref('')
  const applying = ref(false)
  const applyForm = ref()
  const typesWithPolicy = computed(() => caseTypes.value.filter(ct => ct.defaultRetentionPolicyCode))

  async function loadTypes () {
    if (!isAdmin.value) return
    try {
      caseTypes.value = await listCaseTypes(false)
    } catch (error) {
      report(error)
    }
  }
  watch(isAdmin, loadTypes, { immediate: true })

  async function apply () {
    const validation = await applyForm.value?.validate()
    if (!applyType.value || (validation && !validation.valid)) return
    applying.value = true
    try {
      const res = await applyCaseTypeRetention(applyType.value, applyReason.value.trim())
      ui.notify(t('retention.page.applied', { applied: res.applied ?? 0, remaining: res.remaining ?? 0 }), 'success')
      await load()
    } catch (error) {
      report(error)
    } finally {
      applying.value = false
    }
  }
</script>

<template>
  <v-container fluid>
    <div class="d-flex flex-wrap align-center ga-2 mb-4">
      <h1 class="text-h5">{{ t('retention.page.title') }}</h1>
      <v-chip v-if="isRecordsManager" size="small">{{ total }}</v-chip>
    </div>

    <v-alert v-if="!isRecordsManager" type="warning" variant="tonal">{{ t('retention.page.forbidden') }}</v-alert>

    <template v-else>
      <v-tabs v-model="tab" class="mb-2" color="primary">
        <v-tab value="due">{{ t('retention.page.dueTab') }}</v-tab>
        <v-tab value="bordereaux">{{ t('retention.bordereau.tab') }}</v-tab>
      </v-tabs>

      <v-card v-if="tab === 'bordereaux'" class="mb-4">
        <v-card-text><BordereauList /></v-card-text>
      </v-card>

      <v-card v-else class="mb-4">
        <v-card-text>
          <div class="d-flex flex-wrap align-center ga-2 mb-2">
            <p class="text-caption text-medium-emphasis mb-0 flex-grow-1">{{ t('retention.page.hint') }}</p>

            <v-btn
              color="primary"
              :disabled="selected.length === 0"
              prepend-icon="mdi-file-sign"
              size="small"
              variant="flat"
              @click="creating = true"
            >{{ t('retention.bordereau.createN', { n: selected.length }) }}</v-btn>
          </div>

          <v-row dense>
            <v-col cols="12" md="3" sm="6">
              <v-text-field
                v-model="dueOn"
                density="compact"
                hide-details
                :label="t('retention.page.dueOn')"
                type="date"
              />
            </v-col>

            <v-col cols="12" md="3" sm="6">
              <v-select
                v-model="disposition"
                clearable
                density="compact"
                hide-details
                item-title="title"
                item-value="value"
                :items="dispositionItems"
                :label="t('retention.page.disposition')"
              />
            </v-col>

            <v-col cols="12" md="4">
              <v-switch
                v-model="includeHeld"
                color="primary"
                density="compact"
                hide-details
                :label="t('retention.page.includeHeld')"
              />
            </v-col>
          </v-row>

          <v-progress-linear v-if="loading" class="my-2" color="primary" indeterminate />
          <p v-if="!loading && rows.length === 0" class="text-medium-emphasis mt-2">{{ t('retention.page.empty') }}</p>

          <v-table v-else density="compact">
            <thead>
              <tr>
                <th scope="col"><span class="d-sr-only">{{ t('retention.bordereau.select') }}</span></th>
                <th scope="col">{{ t('retention.page.case') }}</th>
                <th scope="col">{{ t('retention.page.type') }}</th>
                <th scope="col">{{ t('retention.page.until') }}</th>
                <th scope="col">{{ t('retention.page.disposition') }}</th>
                <th scope="col" />
              </tr>
            </thead>

            <tbody>
              <tr v-for="row in rows" :key="row.retention.id">
                <td>
                  <v-checkbox-btn
                    v-model="selected"
                    :aria-label="t('retention.bordereau.select')"
                    density="compact"
                    :disabled="row.onHold"
                    :value="row.retention.subjectId"
                  />
                </td>

                <td>
                  <router-link :to="`/cases/${row.retention.subjectId}`">
                    <span v-if="row.businessRef" class="text-medium-emphasis mr-1">{{ row.businessRef }}</span>
                    {{ row.caseLabel || t('retention.page.hidden') }}
                  </router-link>
                </td>

                <td class="text-caption"><code>{{ row.caseTypeCode }}</code></td>
                <td>{{ formatDate(row.retention.untilOn) }}</td>

                <td>
                  <v-chip :color="dispositionColor(row.retention.finalDisposition)" label size="x-small">
                    {{ enumLabel('FinalDisposition', row.retention.finalDisposition) }}
                  </v-chip>
                </td>

                <td>
                  <v-chip
                    v-if="row.onHold"
                    color="warning"
                    label
                    prepend-icon="mdi-lock-outline"
                    size="x-small"
                  >{{ t('retention.onHold') }}</v-chip>
                </td>
              </tr>
            </tbody>
          </v-table>

          <div v-if="nextToken" class="d-flex justify-center mt-2">
            <v-btn :loading="loading" variant="text" @click="load(true)">{{ t('actions.common.loadMore') }} ({{ rows.length }} / {{ total }})</v-btn>
          </div>
        </v-card-text>
      </v-card>

      <BordereauCreateDialog v-model="creating" :case-ids="selected" @created="router.push(`/retention/bordereaux/${$event}`)" />

      <v-card v-if="isAdmin && tab === 'due'">
        <v-card-title class="text-subtitle-1">{{ t('retention.page.apply') }}</v-card-title>

        <v-card-text>
          <p class="text-caption text-medium-emphasis mb-2">{{ t('retention.page.applyHint') }}</p>

          <v-form ref="applyForm" @submit.prevent="apply">
            <v-row dense>
              <v-col cols="12" md="4">
                <v-select
                  v-model="applyType"
                  density="compact"
                  item-title="label"
                  item-value="code"
                  :items="typesWithPolicy"
                  :label="t('retention.page.applyType')"
                  :rules="[required(t)]"
                />
              </v-col>

              <v-col cols="12" md="5">
                <v-text-field
                  v-model="applyReason"
                  density="compact"
                  :label="t('retention.page.applyReason')"
                  :rules="[required(t), maxLength(t, 2000)]"
                />
              </v-col>

              <v-col cols="12" md="3">
                <v-btn color="primary" :loading="applying" type="submit">{{ t('retention.page.applySubmit') }}</v-btn>
              </v-col>
            </v-row>
          </v-form>
        </v-card-text>
      </v-card>
    </template>
  </v-container>
</template>
