<script setup lang="ts">
  import type { ThingFormModel } from './thingForm'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { geometryRule, SAMPLE_POLYGON } from '@/utils/geometry'
  import { maxLength, required } from '@/utils/validation'
  import GeometryPreview from './GeometryPreview.vue'
  import { BUILDING_STATUSES } from './thingForm'

  // Thing fields shared by the create page and the detail edit panel: name,
  // texts, the detail block of the type's specialization and the LV95 geometry
  // (GeoJSON) with a live preview.
  const model = defineModel<ThingFormModel>({ required: true })
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()

  const isParcel = computed(() => model.value.specialization === 'THING_SPECIALIZATION_PARCEL')
  const isBuilding = computed(() => model.value.specialization === 'THING_SPECIALIZATION_BUILDING')
  const statusItems = computed(() => BUILDING_STATUSES.map(s => ({ value: s, title: enumLabel('BuildingStatus', s) })))
  // The server derives the name of a parcel (or of a building with an EGID).
  const nameRules = computed(() => (isParcel.value || (isBuilding.value && model.value.egid) ? [maxLength(t, 300)] : [required(t), maxLength(t, 300)]))
  const egridRule = (v: unknown) => (typeof v !== 'string' || v.trim() === '' || /^CH[0-9A-Z]{12}$/i.test(v.trim()) || t('validation.egrid'))
  const communeRule = (v: unknown) => (Number(v) >= 1 && Number(v) <= 9999) || t('validation.communeOfs')
</script>

<template>
  <div>
    <v-row v-if="isParcel" dense>
      <v-col cols="12" sm="3">
        <v-text-field
          v-model.number="model.communeOfs"
          :hint="t('fields.thing.commune_ofs_hint')"
          :label="t('fields.thing.commune_ofs')"
          persistent-hint
          :rules="[communeRule]"
          type="number"
        />
      </v-col>

      <v-col cols="12" sm="3">
        <v-text-field v-model="model.parcelNumber" :label="t('fields.thing.parcel_number')" :rules="[required(t), maxLength(t, 20)]" />
      </v-col>

      <v-col cols="12" sm="4">
        <v-text-field v-model="model.egrid" :label="t('fields.thing.egrid')" placeholder="CH123456789012" :rules="[egridRule]" />
      </v-col>

      <v-col cols="12" sm="2">
        <v-text-field v-model.number="model.surfaceM2" :label="t('fields.thing.surface_m2')" min="0" type="number" />
      </v-col>
    </v-row>

    <v-row v-if="isBuilding" dense>
      <v-col cols="12" sm="3">
        <v-text-field v-model.number="model.egid" :label="t('fields.thing.egid')" min="1" type="number" />
      </v-col>

      <v-col cols="12" sm="3">
        <v-text-field v-model="model.ecaNumber" :label="t('fields.thing.eca_number')" :rules="[maxLength(t, 20)]" />
      </v-col>

      <v-col cols="12" sm="2">
        <v-text-field
          v-model.number="model.constructionYear"
          :label="t('fields.thing.construction_year')"
          max="2100"
          min="1000"
          type="number"
        />
      </v-col>

      <v-col cols="12" sm="4">
        <v-select
          v-model="model.buildingStatus"
          item-title="title"
          item-value="value"
          :items="statusItems"
          :label="t('fields.thing.building_status')"
        />
      </v-col>
    </v-row>

    <v-text-field
      v-model="model.name"
      :hint="isParcel || isBuilding ? t('fields.thing.name_hint') : ''"
      :label="t('fields.thing.name')"
      persistent-hint
      :rules="nameRules"
    />

    <v-textarea
      v-model="model.description"
      auto-grow
      class="mt-2"
      :label="t('fields.thing.description')"
      rows="2"
      :rules="[maxLength(t, 4000)]"
    />

    <v-text-field v-model="model.externalRef" :label="t('fields.thing.external_ref')" :rules="[maxLength(t, 200)]" />

    <v-divider class="my-3" />

    <div class="d-flex align-center mb-1">
      <span class="text-subtitle-2">{{ t('fields.thing.geometry') }}</span>
      <v-spacer />
      <v-btn size="small" variant="text" @click="model.geometry = SAMPLE_POLYGON">{{ t('actions.thing.sampleGeometry') }}</v-btn>
    </div>

    <v-row dense>
      <v-col cols="12" md="8">
        <v-textarea
          v-model="model.geometry"
          class="geojson"
          :hint="t('fields.thing.geometry_hint')"
          persistent-hint
          rows="5"
          :rules="[geometryRule(t, model.specialization)]"
        />
      </v-col>

      <v-col class="d-flex justify-center" cols="12" md="4">
        <GeometryPreview :geojson="model.geometry" :size="160" />
      </v-col>
    </v-row>
  </div>
</template>

<style scoped>
  .geojson :deep(textarea) {
    font-family: monospace;
    font-size: 0.8rem;
  }
</style>
