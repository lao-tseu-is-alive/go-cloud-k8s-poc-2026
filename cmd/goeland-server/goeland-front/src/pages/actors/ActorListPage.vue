<script setup lang="ts">
  import { useI18n } from 'vue-i18n'
  import ActorSearchList from '@/components/actor/ActorSearchList.vue'

  const { t } = useI18n()
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center justify-space-between mb-4 flex-wrap ga-2">
      <h1 class="text-h5">{{ t('pages.actors.list.title') }}</h1>

      <v-btn color="primary" prepend-icon="mdi-plus" to="/actors/new" variant="flat">
        {{ t('nav.createActor') }}
      </v-btn>
    </div>

    <ActorSearchList />
  </v-container>
</template>
