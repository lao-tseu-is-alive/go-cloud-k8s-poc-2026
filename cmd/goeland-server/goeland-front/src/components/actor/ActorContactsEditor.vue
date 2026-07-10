<script setup lang="ts">
  import type { ActorContact } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { CONTACT_TYPES } from '@/components/actor/actorForm'
  import { useI18nEnum } from '@/composables/useI18nEnum'

  // Two-way bound list of contacts, edited in place. Blank rows are dropped by
  // the form's cleanContacts() before submit.
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
</script>

<template>
  <div>
    <div class="d-flex align-center justify-space-between mb-2">
      <span class="text-subtitle-2">{{ t('sections.actor.contacts') }}</span>

      <v-btn prepend-icon="mdi-plus" size="small" variant="text" @click="addContact">
        {{ t('actions.actor.addContact') }}
      </v-btn>
    </div>

    <p v-if="model.length === 0" class="text-medium-emphasis text-caption">
      {{ t('messages.common.noData') }}
    </p>

    <v-row v-for="(contact, i) in model" :key="i" class="align-center" dense>
      <v-col cols="12" sm="4">
        <v-select
          v-model="contact.contactType"
          density="compact"
          hide-details
          item-title="title"
          item-value="value"
          :items="typeItems"
          :label="t('fields.actor.contact_type')"
        />
      </v-col>

      <v-col cols="12" sm="5">
        <v-text-field
          v-model="contact.value"
          density="compact"
          hide-details
          :label="t('fields.actor.contact_value')"
        />
      </v-col>

      <v-col class="d-flex align-center" cols="8" sm="2">
        <v-checkbox
          v-model="contact.isPrimary"
          density="compact"
          hide-details
          :label="t('fields.actor.contact_primary')"
        />
      </v-col>

      <v-col class="text-right" cols="4" sm="1">
        <v-btn
          color="error"
          icon="mdi-delete"
          size="small"
          variant="text"
          @click="removeContact(i)"
        />
      </v-col>
    </v-row>
  </div>
</template>
