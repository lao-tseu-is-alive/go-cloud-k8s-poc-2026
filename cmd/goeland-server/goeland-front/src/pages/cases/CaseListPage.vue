<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import CaseSearchList from '@/components/case/CaseSearchList.vue'

  const { t } = useI18n()
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center justify-space-between mb-4 flex-wrap ga-2">
      <h1 class="text-h5">{{ t('pages.cases.list.title') }}</h1>

      <v-btn color="primary" prepend-icon="mdi-plus" to="/cases/new" variant="flat">
        {{ t('nav.createCase') }}
      </v-btn>
    </div>

    <CaseSearchList />
  </v-container>
</template>
