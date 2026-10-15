<script setup lang="ts">
  import type { ActorContact } from '@/api/types'
  import { useI18n } from 'vue-i18n'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { contactHref, formatContactValue, phoneDirectoryHref } from '@/utils/contactRules'

  // Read-only display of an actor's typed contacts.
  defineProps<{ contacts?: ActorContact[] }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
</script>

<template>
  <div>
    <p v-if="!contacts || contacts.length === 0" class="text-medium-emphasis">
      {{ t('messages.common.noData') }}
    </p>

    <v-table v-else density="compact">
      <tbody>
        <tr v-for="(contact, i) in contacts" :key="i">
          <td class="text-medium-emphasis" style="width: 40%">
            {{ enumLabel('ContactType', contact.contactType) }}
            <v-icon v-if="contact.isPrimary" color="primary" icon="mdi-star" size="x-small" />
          </td>

          <td>
            <a
              v-if="contactHref(contact.contactType, contact.value)"
              :href="contactHref(contact.contactType, contact.value)"
              rel="noopener noreferrer"
              target="_blank"
            >{{ formatContactValue(contact.contactType, contact.value) }}</a>

            <span v-else>{{ formatContactValue(contact.contactType, contact.value) }}</span>

            <a
              v-if="phoneDirectoryHref(contact.value) && contactHref(contact.contactType, contact.value)?.startsWith('tel:')"
              class="ml-2 text-caption font-weight-bold"
              :href="phoneDirectoryHref(contact.value)"
              rel="noopener noreferrer"
              target="_blank"
              :title="t('actorPage.header.phoneDirectory')"
            >CH</a>

            <span v-if="contact.label" class="text-caption text-medium-emphasis"> — {{ contact.label }}</span>
          </td>
        </tr>
      </tbody>
    </v-table>
  </div>
</template>
