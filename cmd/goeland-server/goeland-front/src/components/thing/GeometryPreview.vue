<script setup lang="ts">
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { parseGeometry, svgGeometry } from '@/utils/geometry'

  // Lightweight SVG preview of a GeoJSON geometry in LV95 (north up), with its
  // extent in metres; no map library needed.
  const props = withDefaults(defineProps<{ geojson?: string, size?: number }>(), { size: 220 })
  const { t } = useI18n()
  const parsed = computed(() => parseGeometry(props.geojson))
  const drawn = computed(() => (parsed.value ? svgGeometry(parsed.value, props.size) : undefined))
</script>

<template>
  <div v-if="parsed && drawn" class="d-inline-flex flex-column align-center">
    <svg
      :aria-label="t('fields.thing.geometry')"
      class="geometry-preview"
      :height="size"
      role="img"
      :viewBox="`0 0 ${size} ${size}`"
      :width="size"
    >
      <template v-for="(path, i) in drawn.paths" :key="`p${i}`">
        <polygon v-if="parsed.filled" class="shape" :points="path" />
        <polyline v-else class="line" :points="path" />
      </template>

      <circle
        v-for="(p, i) in drawn.points"
        :key="`c${i}`"
        class="dot"
        :cx="p[0]"
        :cy="p[1]"
        r="5"
      />
    </svg>

    <span class="text-caption text-medium-emphasis">{{ parsed.type }} · {{ t('fields.thing.extent', { m: Math.round(drawn.spanMetres) }) }}</span>
  </div>

  <p v-else class="text-medium-emphasis text-caption">{{ t('messages.thing.noGeometry') }}</p>
</template>

<style scoped>
  .geometry-preview {
    background: rgba(var(--v-theme-on-surface), 0.04);
    border-radius: 4px;
  }

  .shape {
    fill: rgba(var(--v-theme-primary), 0.25);
    stroke: rgb(var(--v-theme-primary));
    stroke-width: 2;
  }

  .line {
    fill: none;
    stroke: rgb(var(--v-theme-primary));
    stroke-width: 2;
  }

  .dot {
    fill: rgb(var(--v-theme-primary));
  }
</style>
