<script setup lang="ts">
  import type { AuditEvent, GroupMember, SecurityGroup } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { addGroupMember, archiveGroup, getGroup, removeGroupMember, updateGroup } from '@/api/accessClient'
  import { listAuditEvents } from '@/api/coreClient'
  import AccessPanel from '@/components/access/AccessPanel.vue'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import UserLabel from '@/components/core/UserLabel.vue'
  import UserPicker from '@/components/core/UserPicker.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useMyAccess } from '@/composables/useMyAccess'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { maxLength, required } from '@/utils/validation'

  // A security group: its members (MANAGE adds and removes them), its name
  // (MANAGE), archiving (FULL_CONTROL) and the grants on the group itself.
  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  const { access, reload: reloadAccess, canManage, hasFullControl } = useMyAccess(id)
  const group = ref<SecurityGroup | null>(null)
  const members = ref<GroupMember[]>([])
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)
  const live = computed(() => !group.value?.archivedAt)

  const dialog = ref<'edit' | 'add' | 'remove' | 'archive' | null>(null)
  const name = ref('')
  const description = ref('')
  const reason = ref('')
  const userId = ref<string | undefined>()
  const removing = ref<GroupMember | null>(null)
  const busy = ref(false)

  async function load () {
    loading.value = true
    try {
      const res = await getGroup(id.value)
      group.value = res.group ?? null
      members.value = res.members ?? []
      audit.value = await listAuditEvents(id.value)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function openDialog (kind: NonNullable<typeof dialog.value>, member?: GroupMember) {
    name.value = group.value?.name ?? ''
    description.value = group.value?.description ?? ''
    reason.value = ''
    userId.value = undefined
    removing.value = member ?? null
    dialog.value = kind
  }

  async function run (action: () => Promise<unknown>, message: string) {
    busy.value = true
    try {
      await action()
      ui.notify(t(message), 'success')
      dialog.value = null
      await Promise.all([load(), reloadAccess()])
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  const doEdit = () => run(() => updateGroup(id.value, name.value.trim(), description.value.trim(), reason.value.trim()), 'groups.messages.updated')
  const doArchive = () => run(() => archiveGroup(id.value, reason.value.trim()), 'groups.messages.archived')
  function doAdd () {
    const user = userId.value
    if (user) void run(() => addGroupMember(id.value, user), 'groups.messages.memberAdded')
  }
  function doRemove () {
    const user = removing.value?.user?.id
    if (user) void run(() => removeGroupMember(id.value, user, reason.value.trim()), 'groups.messages.memberRemoved')
  }

  watch(id, load, { immediate: true })
</script>

<template>
  <v-container>
    <div class="d-flex flex-wrap align-center ga-2 mb-4">
      <v-btn icon="mdi-arrow-left" variant="text" @click="router.push('/groups')" />
      <h1 class="text-h4">{{ group?.name }}</h1>
      <v-chip v-if="group?.archivedAt" variant="outlined">{{ t('groups.archived') }}</v-chip>
    </div>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />

    <template v-if="group">
      <div v-if="live" class="d-flex ga-2 mb-4">
        <v-btn v-if="canManage" prepend-icon="mdi-pencil" variant="tonal" @click="openDialog('edit')">{{ t('groups.actions.edit') }}</v-btn>

        <v-btn
          v-if="hasFullControl"
          color="error"
          prepend-icon="mdi-archive-outline"
          variant="text"
          @click="openDialog('archive')"
        >{{ t('groups.actions.archive') }}</v-btn>
      </div>

      <v-row>
        <v-col cols="12" md="8">
          <v-card class="mb-4">
            <v-card-title class="d-flex align-center">
              {{ t('groups.fields.members') }}
              <v-spacer />

              <v-btn
                v-if="live && canManage"
                color="primary"
                prepend-icon="mdi-account-plus-outline"
                size="small"
                variant="tonal"
                @click="openDialog('add')"
              >{{ t('groups.actions.addMember') }}</v-btn>
            </v-card-title>

            <v-card-text>
              <p v-if="group.description" class="text-body-2 mb-3">{{ group.description }}</p>
              <p v-if="members.length === 0" class="text-medium-emphasis">{{ t('groups.noMembers') }}</p>

              <v-table v-else density="compact">
                <thead>
                  <tr>
                    <th scope="col">{{ t('groups.fields.user') }}</th>
                    <th scope="col">{{ t('groups.fields.since') }}</th>
                    <th scope="col" />
                  </tr>
                </thead>

                <tbody>
                  <tr v-for="m in members" :key="m.relationshipId">
                    <td>{{ m.user?.displayName || m.user?.id }}</td>
                    <td class="text-caption">{{ formatDateTime(m.since) }} · <UserLabel :id="m.addedBy" /></td>

                    <td class="text-right">
                      <v-btn
                        v-if="live && canManage"
                        color="error"
                        icon="mdi-account-remove-outline"
                        size="small"
                        :title="t('groups.actions.removeMember')"
                        variant="text"
                        @click="openDialog('remove', m)"
                      />
                    </td>
                  </tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="4">
          <v-card class="mb-4" :title="t('access.title')">
            <v-card-text><AccessPanel :access="access" :subject-id="id" @changed="reloadAccess" /></v-card-text>
          </v-card>

          <v-card :title="t('sections.document.audit')">
            <v-card-text><AuditTimeline :events="audit" /></v-card-text>
          </v-card>
        </v-col>
      </v-row>
    </template>

    <v-dialog max-width="520" :model-value="!!dialog" @update:model-value="dialog = null">
      <v-card v-if="dialog === 'edit'" :title="t('groups.actions.edit')">
        <v-card-text>
          <v-text-field v-model="name" :label="t('groups.fields.name')" :rules="[required(t), maxLength(t, 200)]" />
          <v-textarea v-model="description" auto-grow :label="t('groups.fields.description')" rows="2" />
          <v-text-field v-model="reason" :label="t('access.fields.reason')" />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = null">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="primary"
            :disabled="!name.trim()"
            :loading="busy"
            variant="flat"
            @click="doEdit"
          >{{ t('actions.common.save') }}</v-btn>
        </v-card-actions>
      </v-card>

      <v-card v-else-if="dialog === 'add'" :title="t('groups.actions.addMember')">
        <v-card-text>
          <UserPicker v-model="userId" :label="t('roles.pickUser')" />
          <p class="text-caption text-medium-emphasis">{{ t('roles.signedInOnly') }}</p>
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = null">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="primary"
            :disabled="!userId"
            :loading="busy"
            variant="flat"
            @click="doAdd"
          >{{ t('groups.actions.addMember') }}</v-btn>
        </v-card-actions>
      </v-card>

      <v-card v-else :title="dialog === 'remove' ? t('groups.actions.removeMember') : t('groups.actions.archive')">
        <v-card-text>
          <p v-if="dialog === 'archive'" class="mb-3">{{ t('groups.archiveConfirm') }}</p>
          <v-text-field v-model="reason" :label="t('access.fields.reason')" :rules="dialog === 'archive' ? [required(t)] : []" />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = null">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="error"
            :disabled="dialog === 'archive' && !reason.trim()"
            :loading="busy"
            variant="flat"
            @click="dialog === 'remove' ? doRemove() : doArchive()"
          >
            {{ t('actions.common.confirm') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>
