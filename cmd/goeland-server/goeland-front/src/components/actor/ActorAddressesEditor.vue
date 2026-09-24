<script setup lang="ts">
  import type { ActorAddress, ActorKind } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ADDRESS_TYPES, emptyAddress } from '@/components/actor/actorForm'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { countryCodeRule, postalCodeRule } from '@/utils/address'
  import { maxLength, required } from '@/utils/validation'

  // Two-way bound list of postal addresses, edited in place. Exactly one can be
  // principal (the server makes the first one principal when none is); fully
  // blank rows are dropped before submit, and the server validates the rest.
  const props = defineProps<{ actorKind: ActorKind }>()
  const model = defineModel<ActorAddress[]>({ default: () => [] })

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()

  const typeItems = computed(() =>
    ADDRESS_TYPES.map(a => ({ value: a, title: enumLabel('AddressType', a) })),
  )

  function addAddress () {
    const next = emptyAddress(props.actorKind)
    next.isPrincipal = model.value.length === 0
    model.value = [...model.value, next]
  }

  function removeAddress (index: number) {
    model.value = model.value.filter((_, i) => i !== index)
  }

  function setPrincipal (index: number) {
    model.value = model.value.map((a, i) => ({ ...a, isPrincipal: i === index }))
  }

  function isBlank (a: ActorAddress): boolean {
    return [a.street, a.postalCode, a.locality].every(v => !v?.trim())
  }

  // Required fields apply once a row has started to be filled.
  function requiredRule (a: ActorAddress) {
    return isBlank(a) ? [] : [required(t)]
  }
</script>

<template>
  <div>
    <div class="d-flex align-center justify-space-between mb-1">
      <span class="text-subtitle-2">{{ t('sections.actor.addresses') }}</span>

      <v-btn prepend-icon="mdi-map-marker-plus" size="small" variant="text" @click="addAddress">
        {{ t('actions.actor.addAddress') }}
      </v-btn>
    </div>

    <p class="text-caption text-medium-emphasis mb-2">{{ t('messages.actor.addressesHint') }}</p>

    <p v-if="model.length === 0" class="text-medium-emphasis text-caption">
      {{ t('messages.common.noData') }}
    </p>

    <v-sheet
      v-for="(address, i) in model"
      :key="i"
      border
      class="pa-3 mb-3"
      rounded
    >
      <v-row dense>
        <v-col cols="12" sm="5">
          <v-select
            v-model="address.addressType"
            density="compact"
            item-title="title"
            item-value="value"
            :items="typeItems"
            :label="t('fields.address.type')"
          />
        </v-col>

        <v-col cols="12" sm="5">
          <v-text-field
            v-model="address.label"
            density="compact"
            :label="t('fields.address.label')"
            :rules="address.addressType === 'ADDRESS_TYPE_OTHER' && !isBlank(address) ? [required(t), maxLength(t, 100)] : [maxLength(t, 100)]"
          />
        </v-col>

        <v-col class="d-flex align-center justify-end" cols="12" sm="2">
          <v-btn
            :aria-label="t('fields.address.principal')"
            :color="address.isPrincipal ? 'primary' : undefined"
            :icon="address.isPrincipal ? 'mdi-star' : 'mdi-star-outline'"
            size="small"
            :title="t('fields.address.principal')"
            variant="text"
            @click="setPrincipal(i)"
          />

          <v-btn
            :aria-label="t('actions.actor.removeAddress')"
            color="error"
            icon="mdi-delete"
            size="small"
            :title="t('actions.actor.removeAddress')"
            variant="text"
            @click="removeAddress(i)"
          />
        </v-col>

        <v-col cols="9" sm="8">
          <v-text-field
            v-model="address.street"
            density="compact"
            :label="t('fields.address.street')"
            :rules="[...requiredRule(address), maxLength(t, 200)]"
          />
        </v-col>

        <v-col cols="3" sm="4">
          <v-text-field
            v-model="address.houseNumber"
            density="compact"
            :label="t('fields.address.house_number')"
            :rules="[maxLength(t, 20)]"
          />
        </v-col>

        <v-col cols="12">
          <v-text-field
            v-model="address.addressLine2"
            density="compact"
            :label="t('fields.address.address_line2')"
            :rules="[maxLength(t, 200)]"
          />
        </v-col>

        <v-col cols="4" sm="3">
          <v-text-field
            v-model="address.postalCode"
            density="compact"
            :label="t('fields.address.postal_code')"
            :rules="[...requiredRule(address), postalCodeRule(t, address.countryCode), maxLength(t, 20)]"
          />
        </v-col>

        <v-col cols="8" sm="7">
          <v-text-field
            v-model="address.locality"
            density="compact"
            :label="t('fields.address.locality')"
            :rules="[...requiredRule(address), maxLength(t, 100)]"
          />
        </v-col>

        <v-col cols="12" sm="2">
          <v-text-field
            v-model="address.countryCode"
            density="compact"
            :label="t('fields.address.country_code')"
            maxlength="2"
            :rules="[countryCodeRule(t)]"
          />
        </v-col>
      </v-row>
    </v-sheet>
  </div>
</template>
