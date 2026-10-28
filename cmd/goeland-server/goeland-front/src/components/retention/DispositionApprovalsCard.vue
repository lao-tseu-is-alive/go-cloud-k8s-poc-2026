<script setup lang="ts">
  import type { DispositionItem, FinalDisposition } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { decideDispositionItem, listMyDispositionApprovals } from '@/api/retentionClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useUiStore } from '@/stores/ui'
  import { formatDate } from '@/utils/formatters'
  import { dispositionColor } from '@/utils/retention'
  import { maxLength, required } from '@/utils/validation'

  // The cases a records manager proposed for transfer or elimination that wait
  // for the caller's decision, as manager of their owning unit or of the case
  // (GLD-032). Shown only when there are some.
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const items = ref<DispositionItem[]>([])
  const dispositions = ref<Record<string, FinalDisposition>>({})
  const deciding = ref<{ item: DispositionItem, approve: boolean } | null>(null)
  const reason = ref('')
  const busy = ref(false)
  const form = ref()

  async function load () {
    try {
      const res = await listMyDispositionApprovals()
      items.value = res.items ?? []
      dispositions.value = res.dispositions ?? {}
    } catch (error) {
      report(error)
    }
  }
  onMounted(load)

  function open (item: DispositionItem, approve: boolean) {
    deciding.value = { item, approve }
    reason.value = ''
  }

  async function submit () {
    const validation = await form.value?.validate()
    if (!deciding.value || (validation && !validation.valid)) return
    busy.value = true
    try {
      await decideDispositionItem(deciding.value.item.id, deciding.value.approve, reason.value.trim())
      ui.notify(t('retention.approvals.done'), 'success')
      deciding.value = null
      await load()
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }
</script>

<template>
  <v-card v-if="items.length > 0" class="mb-4">
    <v-card-title class="text-subtitle-1">{{ t('retention.approvals.title', { n: items.length }) }}</v-card-title>

    <v-card-text>
      <p class="text-caption text-medium-emphasis mb-2">{{ t('retention.approvals.hint') }}</p>

      <v-table density="compact">
        <thead>
          <tr>
            <th scope="col">{{ t('retention.page.case') }}</th>
            <th scope="col">{{ t('retention.page.disposition') }}</th>
            <th scope="col">{{ t('retention.page.until') }}</th>
            <th scope="col">{{ t('retention.bordereau.approver') }}</th>
            <th scope="col" />
          </tr>
        </thead>

        <tbody>
          <tr v-for="item in items" :key="item.id">
            <td>
              <router-link :to="`/cases/${item.caseId}`">
                <span v-if="item.businessRef" class="text-medium-emphasis mr-1">{{ item.businessRef }}</span>
                {{ item.caseLabel || t('retention.page.hidden') }}
              </router-link>
            </td>

            <td>
              <v-chip :color="dispositionColor(dispositions[item.id])" label size="x-small">{{ enumLabel('FinalDisposition', dispositions[item.id]) }}</v-chip>
            </td>

            <td>{{ formatDate(item.untilOn) }}</td>
            <td>{{ item.approverLabel || t('retention.page.hidden') }}</td>

            <td class="text-right text-no-wrap">
              <v-btn color="success" size="small" variant="text" @click="open(item, true)">{{ t('retention.approvals.approve') }}</v-btn>
              <v-btn color="error" size="small" variant="text" @click="open(item, false)">{{ t('retention.approvals.reject') }}</v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>

      <v-form v-if="deciding" ref="form" class="mt-3" @submit.prevent="submit">
        <v-textarea
          v-model="reason"
          auto-grow
          counter="2000"
          :label="deciding.approve ? t('retention.approvals.approveReason') : t('retention.approvals.rejectReason')"
          rows="2"
          :rules="[required(t), maxLength(t, 2000)]"
        />

        <div class="d-flex ga-2">
          <v-btn :color="deciding.approve ? 'success' : 'error'" :loading="busy" type="submit">
            {{ deciding.approve ? t('retention.approvals.approve') : t('retention.approvals.reject') }}
          </v-btn>

          <v-btn variant="text" @click="deciding = null">{{ t('actions.common.cancel') }}</v-btn>
        </div>
      </v-form>
    </v-card-text>
  </v-card>
</template>
