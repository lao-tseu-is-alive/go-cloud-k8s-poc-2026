<script setup lang="ts">
  import type { SubjectRef } from '@/api/types'
  import { computed } from 'vue'
  import { useRoute } from 'vue-router'
  import { kindIcon, subjectRoute } from '@/utils/subjects'

  // A subject label that links to its detail page (a real link: mouse, keyboard,
  // middle-click). The subject of the current page stays plain text.
  const props = defineProps<{ subject?: SubjectRef }>()
  const route = useRoute()
  const to = computed(() => {
    const target = subjectRoute(props.subject)
    return target && target !== route.path ? target : undefined
  })
</script>

<template>
  <router-link v-if="to" class="subject-link" :to="to">
    <v-icon class="mr-1" :icon="kindIcon(subject?.kind)" size="small" />{{ subject?.displayLabel }}
  </router-link>

  <span v-else>{{ subject?.displayLabel }}</span>
</template>

<style scoped>
  .subject-link {
    color: rgb(var(--v-theme-primary));
    text-decoration: none;
  }

  .subject-link:hover,
  .subject-link:focus-visible {
    text-decoration: underline;
  }
</style>
