<script setup lang="ts">
  import type { ThingType } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listThingTypes } from '@/api/thingClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { foldedFilter } from '@/utils/textSearch'

  // A searchable picker (GLD-057): the catalogue comes ordered by label and typing filters it.
  // Bound value is the thing type *code*; the selected type is also emitted so
  // the form can follow its specialization (parcel, building, generic).
  const props = defineProps<{ label?: string, clearable?: boolean, rules?: Array<(v: unknown) => true | string> }>()
  const model = defineModel<string | undefined>()
  const emit = defineEmits<{ selected: [type: ThingType | undefined] }>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const types = ref<ThingType[]>([])
  const loading = ref(false)

  onMounted(async () => {
    loading.value = true
    try {
      types.value = await listThingTypes(true)
      emit('selected', types.value.find(tt => tt.code === model.value))
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  })
</script>

<template>
  <v-autocomplete
    v-model="model"
    auto-select-first
    :clearable="props.clearable"
    :custom-filter="foldedFilter"
    item-title="label"
    item-value="code"
    :items="types"
    :label="props.label ?? t('fields.thing.thing_type')"
    :loading="loading"
    :rules="props.rules"
    @update:model-value="code => emit('selected', types.find(tt => tt.code === code))"
  />
</template>
