/**
 * Maps ApiError (Connect codes + buf.validate violations) to human, translated
 * messages, and surfaces them through the shared snackbar store. Technical
 * protovalidate strings are never shown directly.
 */
import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'
import { useUiStore } from '@/stores/ui'

// buf.validate constraint id → i18n validation key.
const CONSTRAINT_KEYS: Record<string, string> = {
  'required': 'validation.required',
  'string.min_len': 'validation.minLength',
  'string.max_len': 'validation.maxLength',
  'string.uuid': 'validation.uuid',
  'string.pattern': 'validation.pattern',
}

export function useApiErrors () {
  const { t, te } = useI18n()
  const ui = useUiStore()

  function toMessage (err: unknown): string {
    if (err instanceof ApiError) {
      if (err.violations.length > 0) {
        const first = err.violations[0]
        const key = first.constraint ? CONSTRAINT_KEYS[first.constraint] : undefined
        if (key && te(key)) {
          return t(key, { min: '', max: '' })
        }
        if (first.message) {
          return first.message
        }
      }
      // Business errors: prefer a mapped message by Connect code when we have one.
      const codeKey = err.code ? `errors.${err.code}` : ''
      if (codeKey && te(codeKey)) {
        return t(codeKey)
      }
      return err.message || t('messages.common.error')
    }
    if (err instanceof Error) {
      return err.message
    }
    return t('messages.common.error')
  }

  /** Shows the error in a snackbar and returns the resolved message. */
  function report (err: unknown): string {
    const message = toMessage(err)
    ui.notify(message, 'error')
    return message
  }

  return { toMessage, report }
}
