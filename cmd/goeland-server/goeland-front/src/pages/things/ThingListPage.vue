<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import ThingSearchList from '@/components/thing/ThingSearchList.vue'

  const { t } = useI18n()
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center justify-space-between mb-4 flex-wrap ga-2">
      <h1 class="text-h5">{{ t('pages.things.list.title') }}</h1>

      <v-btn color="primary" prepend-icon="mdi-plus" to="/things/new" variant="flat">
        {{ t('nav.createThing') }}
      </v-btn>
    </div>

    <ThingSearchList />
  </v-container>
</template>
