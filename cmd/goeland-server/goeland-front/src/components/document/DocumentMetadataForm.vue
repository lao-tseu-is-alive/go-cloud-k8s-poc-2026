<script setup lang="ts">
  import type { DocumentFormModel } from './documentForm'
  import { useI18n } from 'vue-i18n'
  import { maxLength, required } from '@/utils/validation'

  // The mutable metadata shared by create + edit. document_type is handled
  // separately (immutable after creation), and OUTPUT_ONLY fields are never here.
  const props = defineProps<{ readonly?: boolean, showRecordFlag?: boolean }>()
  const model = defineModel<DocumentFormModel>({ required: true })
  const { t } = useI18n()
</script>

<template>
  <v-row>
    <v-col cols="12">
      <v-text-field
        v-model="model.title"
        counter="500"
        :label="t('fields.document.title')"
        :readonly="props.readonly"
        :rules="[required(t), maxLength(t, 500)]"
      />
    </v-col>

    <v-col cols="12">
      <v-textarea
        v-model="model.description"
        auto-grow
        counter="4000"
        :label="t('fields.document.description')"
        :readonly="props.readonly"
        rows="2"
        :rules="[maxLength(t, 4000)]"
      />
    </v-col>

    <v-col cols="12" md="6">
      <v-text-field
        v-model="model.officialDate"
        :label="t('fields.document.official_date')"
        placeholder="YYYY-MM-DD"
        :readonly="props.readonly"
        type="date"
      />
    </v-col>

    <v-col cols="12" md="6">
      <v-text-field
        v-model="model.language"
        counter="10"
        :hint="'ISO 639 — fr, de, it, en'"
        :label="t('fields.document.language')"
        :readonly="props.readonly"
        :rules="[maxLength(t, 10)]"
      />
    </v-col>

    <v-col v-if="showRecordFlag" cols="12">
      <v-switch
        v-model="model.isRecord"
        color="primary"
        :disabled="props.readonly"
        :label="t('fields.document.is_record')"
      />
    </v-col>
  </v-row>
</template>
