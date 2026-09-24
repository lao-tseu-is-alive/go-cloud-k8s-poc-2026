<script setup lang="ts">
  import type { ActorAddress } from '@/api/types'
  import { useI18n } from 'vue-i18n'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { addressLines, swissMapUrl } from '@/utils/address'

  // Read-only display of an actor's current addresses, principal first.
  defineProps<{ addresses?: ActorAddress[] }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
</script>

<template>
  <div>
    <p v-if="!addresses || addresses.length === 0" class="text-medium-emphasis">
      {{ t('messages.common.noData') }}
    </p>

    <v-row v-else dense>
      <v-col v-for="address in addresses" :key="address.id" cols="12" sm="6">
        <v-sheet border class="pa-3 h-100" rounded>
          <div class="d-flex align-center ga-2 mb-1">
            <v-chip label size="small">{{ enumLabel('AddressType', address.addressType) }}</v-chip>

            <v-icon
              v-if="address.isPrincipal"
              color="primary"
              icon="mdi-star"
              size="small"
              :title="t('fields.address.principal')"
            />

            <span v-if="address.label" class="text-caption text-medium-emphasis">{{ address.label }}</span>
          </div>

          <address class="address-block">
            <div v-for="(line, i) in addressLines(address)" :key="i">{{ line }}</div>
          </address>

          <a
            v-if="swissMapUrl(address)"
            class="text-caption"
            :href="swissMapUrl(address)"
            rel="noopener noreferrer"
            target="_blank"
          >{{ t('actions.actor.showOnMap') }}</a>
        </v-sheet>
      </v-col>
    </v-row>
  </div>
</template>

<style scoped>
  .address-block {
    font-style: normal;
  }
</style>
