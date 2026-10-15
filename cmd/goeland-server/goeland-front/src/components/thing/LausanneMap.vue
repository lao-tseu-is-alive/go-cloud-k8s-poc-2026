<script setup lang="ts">
  import type OlLayerTile from 'ol/layer/Tile'
  import type OlSourceWMTS from 'ol/source/WMTS'
  import OlFormatGeoJSON from 'ol/format/GeoJSON'
  import OlLayerVector from 'ol/layer/Vector'
  import OlMap from 'ol/Map'
  import OlSourceVector from 'ol/source/Vector'
  import { Circle as OlCircle, Fill as OlFill, Stroke as OlStroke, Style as OlStyle } from 'ol/style'
  import OlView from 'ol/View'
  import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import GeometryPreview from '@/components/thing/GeometryPreview.vue'
  import { BASE_LAYERS, insideLausanne, lausanneBaseLayers, lausanneMapAppUrl, swissProjection } from '@/utils/lausanneMap'
  import 'ol/ol.css'

  // The map of an object (GLD-069): the City of Lausanne's base layers (plan,
  // cadastre, orthophotos) with the object drawn on top and the view fitted to
  // it. Outside the City's tiles, or when they cannot be loaded, the outline
  // preview is shown instead. A link opens the City's map application there.
  const props = withDefaults(defineProps<{ geojson?: string, height?: number }>(), { height: 460 })
  const { t } = useI18n()

  const target = ref<HTMLDivElement>()
  const map = shallowRef<OlMap>()
  const baseLayers = shallowRef<OlLayerTile<OlSourceWMTS>[]>([])
  const baseLayer = ref(BASE_LAYERS[0]!.id)
  const failed = ref(false)
  const outside = ref(false)
  const extent = ref<number[]>()

  const appUrl = computed(() => (extent.value ? lausanneMapAppUrl(extent.value, baseLayer.value) : undefined))
  const layerItems = computed(() => BASE_LAYERS.map(l => ({ value: l.id, title: t(`thingPage.map.layers.${l.labelKey}`) })))

  const style = new OlStyle({
    fill: new OlFill({ color: 'rgba(214, 40, 40, 0.18)' }),
    stroke: new OlStroke({ color: '#c62828', width: 3 }),
    image: new OlCircle({ radius: 8, fill: new OlFill({ color: '#c62828' }), stroke: new OlStroke({ color: '#ffffff', width: 2 }) }),
  })

  /** The object as a vector layer, or undefined without a readable geometry. */
  function objectLayer (): OlLayerVector<OlSourceVector> | undefined {
    if (!props.geojson) return undefined
    try {
      const feature = new OlFormatGeoJSON().readFeature(JSON.parse(props.geojson), {
        dataProjection: 'EPSG:2056', featureProjection: 'EPSG:2056',
      })
      const source = new OlSourceVector({ features: Array.isArray(feature) ? feature : [feature] })
      return new OlLayerVector({ source, style })
    } catch {
      return undefined
    }
  }

  async function build () {
    const objects = objectLayer()
    const box = objects?.getSource()?.getExtent()
    extent.value = box
    if (!objects || !box) return
    if (!insideLausanne(box)) {
      outside.value = true
      return
    }
    try {
      baseLayers.value = await lausanneBaseLayers()
    } catch {
      failed.value = true
      return
    }
    if (!target.value || baseLayers.value.length === 0) {
      failed.value = baseLayers.value.length === 0
      return
    }
    const view = new OlView({ projection: swissProjection, center: [(box[0]! + box[2]!) / 2, (box[1]! + box[3]!) / 2], zoom: 8 })
    map.value = new OlMap({ target: target.value, layers: [...baseLayers.value, objects], view })
    view.fit(box, { padding: [48, 48, 48, 48], maxZoom: 11 })
  }

  watch(baseLayer, id => {
    for (const layer of baseLayers.value) layer.setVisible(layer.get('baseLayerId') === id)
  })

  onMounted(build)
  onBeforeUnmount(() => map.value?.setTarget(undefined))
</script>

<template>
  <div class="lausanne-map">
    <template v-if="geojson && !outside && !failed">
      <div
        ref="target"
        :aria-label="t('thingPage.map.label')"
        class="lausanne-map__canvas"
        role="img"
        :style="{ height: `${height}px` }"
      />

      <div class="lausanne-map__layers">
        <v-select
          v-model="baseLayer"
          bg-color="surface"
          density="compact"
          hide-details
          :items="layerItems"
          :label="t('thingPage.map.baseLayer')"
          variant="solo"
        />
      </div>
    </template>

    <div v-else class="d-flex flex-wrap align-start ga-4">
      <GeometryPreview :geojson="geojson" />

      <p class="text-body-2 text-medium-emphasis">
        {{ !geojson ? t('thingPage.map.noGeometry') : outside ? t('thingPage.map.outside') : t('thingPage.map.unavailable') }}
      </p>
    </div>

    <div class="d-flex flex-wrap align-center ga-3 mt-2 text-body-2">
      <a v-if="appUrl" :href="appUrl" rel="noopener noreferrer" target="_blank">{{ t('thingPage.map.openCityMap') }}<v-icon class="ml-1" icon="mdi-open-in-new" size="x-small" /></a>
      <span class="text-caption text-medium-emphasis ml-auto">© Ville de Lausanne</span>
    </div>
  </div>
</template>

<style scoped>
  .lausanne-map {
    position: relative;
  }

  .lausanne-map__canvas {
    width: 100%;
    border-radius: 8px;
    overflow: hidden;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    background: #f4f4f4;
  }

  .lausanne-map__layers {
    position: absolute;
    top: 10px;
    right: 10px;
    width: 220px;
    z-index: 2;
  }
</style>
