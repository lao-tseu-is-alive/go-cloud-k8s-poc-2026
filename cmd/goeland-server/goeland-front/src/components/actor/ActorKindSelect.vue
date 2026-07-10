<script setup lang="ts">
  import type { ActorKind } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ACTOR_KINDS } from '@/components/actor/actorForm'
  import { useI18nEnum } from '@/composables/useI18nEnum'

  const props = defineProps<{
    label?: string
    disabled?: boolean
    rules?: Array<(v: unknown) => true | string>
  }>()
  const model = defineModel<ActorKind>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()

  const items = computed(() => ACTOR_KINDS.map(k => ({ value: k, title: enumLabel('ActorKind', k) })))
</script>

<template>
  <v-select
    v-model="model"
    :disabled="props.disabled"
    item-title="title"
    item-value="value"
    :items="items"
    :label="props.label ?? t('fields.actor.actor_kind')"
    :rules="props.rules"
  />
</template>
