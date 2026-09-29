<script setup lang="ts">
  import { ref, watch } from 'vue'
  import { orgUnitLabel } from '@/components/orgunit/orgUnitForm'

  // An organizational unit shown by name and linking to its page (the id stays
  // in the tooltip).
  const props = defineProps<{ id?: string }>()
  const label = ref('')

  watch(() => props.id, async id => {
    label.value = id ? await orgUnitLabel(id) : ''
  }, { immediate: true })
</script>

<template>
  <router-link v-if="id" class="unit-link" :title="`#${id}`" :to="`/org-units/${encodeURIComponent(id)}`">
    <v-icon class="mr-1" icon="mdi-sitemap-outline" size="small" />{{ label || id }}
  </router-link>

  <span v-else>—</span>
</template>

<style scoped>
  .unit-link {
    color: rgb(var(--v-theme-primary));
    text-decoration: none;
  }

  .unit-link:hover,
  .unit-link:focus-visible {
    text-decoration: underline;
  }
</style>
