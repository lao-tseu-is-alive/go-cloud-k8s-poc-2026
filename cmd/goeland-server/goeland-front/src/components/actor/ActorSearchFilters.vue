<script setup lang="ts">
  import type { ActorKind, SearchActorsParams } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ACTOR_KINDS } from '@/components/actor/actorForm'
  import OrganizationCategorySelect from '@/components/actor/OrganizationCategorySelect.vue'
  import { useI18nEnum } from '@/composables/useI18nEnum'

  const model = defineModel<SearchActorsParams>({ required: true })
  const emit = defineEmits<{ search: [], reset: [] }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()

  const kindItems = computed(() =>
    ACTOR_KINDS.map((k: ActorKind) => ({ value: k, title: enumLabel('ActorKind', k) })),
  )
</script>

<template>
  <v-card class="mb-4">
    <v-card-text>
      <v-row dense>
        <v-col cols="12" md="4">
          <v-text-field
            v-model="model.query"
            clearable
            density="compact"
            hide-details
            :label="t('fields.search.query')"
            prepend-inner-icon="mdi-magnify"
            @keyup.enter="emit('search')"
          />
        </v-col>

        <v-col cols="12" md="3" sm="6">
          <v-select
            v-model="model.actorKind"
            clearable
            density="compact"
            hide-details
            item-title="title"
            item-value="value"
            :items="kindItems"
            :label="t('fields.actor.actor_kind')"
          />
        </v-col>

        <v-col cols="12" md="3" sm="6">
          <OrganizationCategorySelect v-model="model.organizationCategoryCode" clearable />
        </v-col>

        <v-col class="d-flex align-center flex-wrap ga-2" cols="12" md="2">
          <v-checkbox v-model="model.onlyActive" density="compact" hide-details :label="t('fields.actor.only_active')" />
          <v-checkbox v-model="model.includeDeleted" density="compact" hide-details :label="t('fields.document.include_deleted')" />
        </v-col>
      </v-row>

      <div class="d-flex justify-end ga-2 mt-2">
        <v-btn variant="text" @click="emit('reset')">{{ t('actions.common.reset') }}</v-btn>
        <v-btn color="primary" variant="tonal" @click="emit('search')">{{ t('actions.common.search') }}</v-btn>
      </div>
    </v-card-text>
  </v-card>
</template>
