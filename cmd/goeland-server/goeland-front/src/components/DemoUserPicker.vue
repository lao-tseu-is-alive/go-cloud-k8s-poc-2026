<script setup lang="ts">
  import type { DemoUser } from '@/api/types'
  import { storeToRefs } from 'pinia'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useAuthStore } from '@/stores/auth'

  // demo mode (GLD-085): one entry per demo user, with its role and what it
  // shows in the demo script; a click signs in as that user.
  const { t, te } = useI18n()
  const auth = useAuthStore()
  const { demo } = storeToRefs(auth)
  const busy = ref('')
  const rejected = ref(false)

  // The role and hint of a user the messages know; a new demo user still shows its name.
  function line (u: DemoUser, part: 'role' | 'hint'): string {
    const key = `demo.users.${u.key}.${part}`
    return te(key) ? t(key) : ''
  }

  async function pick (u: DemoUser) {
    busy.value = u.key
    rejected.value = !(await auth.signInAsDemo(u))
    busy.value = ''
  }
</script>

<template>
  <div>
    <v-alert
      v-if="rejected"
      class="mb-2"
      density="compact"
      type="error"
      variant="tonal"
    >
      {{ t('auth.demoRejected') }}
    </v-alert>

    <v-list class="py-0" lines="three">
      <v-list-item
        v-for="u in demo?.users ?? []"
        :key="u.key"
        :disabled="busy !== '' && busy !== u.key"
        prepend-icon="mdi-account-circle-outline"
        rounded
        :subtitle="line(u, 'role')"
        :title="u.displayName"
        @click="pick(u)"
      >
        <div class="text-caption text-medium-emphasis">{{ line(u, 'hint') }}</div>

        <template #append>
          <v-progress-circular v-if="busy === u.key" indeterminate size="20" width="2" />
          <v-icon v-else icon="mdi-login" />
        </template>
      </v-list-item>
    </v-list>
  </div>
</template>
