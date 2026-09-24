<script setup lang="ts">
  import { computed, watchEffect } from 'vue'
  import { useUsersStore } from '@/stores/users'

  // Displays an internal user (operator) by name; the raw id stays visible in
  // the tooltip, and alone when the user is unknown.
  const props = defineProps<{ id?: string }>()
  const users = useUsersStore()

  watchEffect(() => users.request(props.id))
  const user = computed(() => users.get(props.id))
  const label = computed(() => user.value?.displayName || user.value?.email || props.id || '—')
  const tooltip = computed(() => [user.value?.email, props.id ? `#${props.id}` : ''].filter(Boolean).join(' · '))
</script>

<template>
  <span :title="tooltip">
    {{ label }}
    <v-icon v-if="user?.isAdmin" class="ml-1" icon="mdi-shield-account" size="x-small" />
  </span>
</template>
