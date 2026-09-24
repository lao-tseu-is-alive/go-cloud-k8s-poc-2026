<script setup lang="ts">
  import type { ActorContact } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { CONTACT_TYPES } from '@/components/actor/actorForm'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { contactKind, contactPlaceholder, contactValueRule } from '@/utils/contactRules'
  import { maxLength, required } from '@/utils/validation'

  // Two-way bound list of typed complements (phone, e-mail, IDE, ...), edited in
  // place. Each value is checked against its type; blank rows are dropped by the
  // form's cleanContacts() before submit, and the server normalizes what it stores.
  const model = defineModel<ActorContact[]>({ default: () => [] })

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()

  const typeItems = computed(() =>
    CONTACT_TYPES.map(c => ({ value: c, title: enumLabel('ContactType', c) })),
  )

  function addContact () {
    model.value = [...model.value, { contactType: 'CONTACT_TYPE_PHONE', value: '', isPrimary: false, label: '' }]
  }

  function removeContact (index: number) {
    model.value = model.value.filter((_, i) => i !== index)
  }

  // OTHER needs a label saying what the value is.
  function labelRules (contact: ActorContact) {
    return contactKind(contact.contactType) === 'other' && contact.value.trim() !== ''
      ? [required(t), maxLength(t, 100)]
      : [maxLength(t, 100)]
  }
</script>

<template>
  <div>
    <div class="d-flex align-center justify-space-between mb-1">
      <span class="text-subtitle-2">{{ t('sections.actor.contacts') }}</span>

      <v-btn prepend-icon="mdi-plus" size="small" variant="text" @click="addContact">
        {{ t('actions.actor.addContact') }}
      </v-btn>
    </div>

    <p class="text-caption text-medium-emphasis mb-2">{{ t('messages.actor.contactsHint') }}</p>

    <p v-if="model.length === 0" class="text-medium-emphasis text-caption">
      {{ t('messages.common.noData') }}
    </p>

    <v-row v-for="(contact, i) in model" :key="i" dense>
      <v-col cols="12" sm="3">
        <v-select
          v-model="contact.contactType"
          density="compact"
          item-title="title"
          item-value="value"
          :items="typeItems"
          :label="t('fields.actor.contact_type')"
        />
      </v-col>

      <v-col cols="12" sm="4">
        <v-text-field
          v-model="contact.value"
          density="compact"
          :label="t('fields.actor.contact_value')"
          :placeholder="contactPlaceholder(contact.contactType)"
          :rules="[contactValueRule(t, contact.contactType), maxLength(t, 400)]"
        />
      </v-col>

      <v-col cols="12" sm="3">
        <v-text-field
          v-model="contact.label"
          density="compact"
          :label="t('fields.actor.contact_label')"
          :rules="labelRules(contact)"
        />
      </v-col>

      <v-col class="d-flex align-center" cols="8" sm="1">
        <v-checkbox
          v-model="contact.isPrimary"
          density="compact"
          hide-details
          :title="t('fields.actor.contact_primary')"
        />
      </v-col>

      <v-col class="text-right" cols="4" sm="1">
        <v-btn
          :aria-label="t('actions.actor.removeContact')"
          color="error"
          icon="mdi-delete"
          size="small"
          :title="t('actions.actor.removeContact')"
          variant="text"
          @click="removeContact(i)"
        />
      </v-col>
    </v-row>
  </div>
</template>
