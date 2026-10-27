<script setup lang="ts">
  import { ref, useSlots } from 'vue'
  import { useI18n } from 'vue-i18n'

  // The filter panel of a list (GLD-074): the main filters in one row, the
  // others (periods, creator, options) behind « Plus de filtres », open from
  // the start when one of them is set; Search and Reset at the end.
  const props = defineProps<{
    /** How many filters of the « more » slot hold a value. */
    moreCount?: number
  }>()
  const emit = defineEmits<{ search: [], reset: [] }>()
  const slots = useSlots()
  const { t } = useI18n()

  const open = ref((props.moreCount ?? 0) > 0)
</script>

<template>
  <v-card class="mb-4">
    <v-card-text>
      <v-row align="center" dense>
        <slot />
      </v-row>

      <v-expand-transition>
        <v-row
          v-if="slots.more"
          v-show="open"
          align="center"
          class="mt-1"
          dense
        >
          <slot name="more" />
        </v-row>
      </v-expand-transition>

      <div class="d-flex align-center flex-wrap ga-2 mt-2">
        <v-btn
          v-if="slots.more"
          :append-icon="open ? 'mdi-chevron-up' : 'mdi-chevron-down'"
          prepend-icon="mdi-tune-variant"
          size="small"
          variant="text"
          @click="open = !open"
        >
          {{ t('filters.more') }}
          <v-badge
            v-if="moreCount"
            class="ms-3"
            color="primary"
            :content="moreCount"
            inline
          />
        </v-btn>

        <v-spacer />
        <v-btn variant="text" @click="emit('reset')">{{ t('actions.common.reset') }}</v-btn>
        <v-btn color="primary" variant="tonal" @click="emit('search')">{{ t('actions.common.search') }}</v-btn>
      </div>
    </v-card-text>
  </v-card>
</template>
