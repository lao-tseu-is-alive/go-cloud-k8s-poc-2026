<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import DateRangeFields from './DateRangeFields.vue'
  import UserPicker from './UserPicker.vue'

  // The filters every list shares (GLD-074): the creation period of the
  // record and the internal user who created it.
  const model = defineModel<{ createdFrom?: string, createdTo?: string, createdBy?: string }>({ required: true })
  const emit = defineEmits<{ search: [] }>()
  const { t } = useI18n()
</script>

<template>
  <DateRangeFields
    v-model:from="model.createdFrom"
    v-model:to="model.createdTo"
    :label="t('filters.created')"
    @search="emit('search')"
  />

  <v-col cols="12" md="4">
    <UserPicker v-model="model.createdBy" density="compact" hide-details :label="t('filters.createdBy')" />
  </v-col>
</template>
