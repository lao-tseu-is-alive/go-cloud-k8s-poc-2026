/** Shared UI state: a single app-wide snackbar for success/error/info toasts. */
import { defineStore } from 'pinia'
import { ref } from 'vue'

type Tone = 'success' | 'error' | 'info' | 'warning'

export const useUiStore = defineStore('ui', () => {
  const snackbar = ref<{ show: boolean, message: string, tone: Tone }>({
    show: false,
    message: '',
    tone: 'info',
  })

  function notify (message: string, tone: Tone = 'info'): void {
    snackbar.value = { show: true, message, tone }
  }

  return { snackbar, notify }
})
