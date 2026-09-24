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

  /** Translated message of the first validation violation, if any. */
  function violationMessage (err: ApiError): string | undefined {
    const first = err.violations[0]
    if (!first) {
      return undefined
    }
    const key = first.constraint ? CONSTRAINT_KEYS[first.constraint] : undefined
    if (key && te(key)) {
      return t(key, { min: '', max: '' })
    }
    return first.message || undefined
  }

  /** Translated message mapped from the Connect code (errors.<code>), if any. */
  function codeMessage (err: ApiError): string | undefined {
    const codeKey = err.code ? `errors.${err.code}` : ''
    return codeKey && te(codeKey) ? t(codeKey) : undefined
  }

  function toMessage (err: unknown): string {
    if (err instanceof ApiError) {
      // Validation first, then business errors by Connect code, then the raw message.
      return violationMessage(err) ?? codeMessage(err) ?? (err.message || t('messages.common.error'))
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
