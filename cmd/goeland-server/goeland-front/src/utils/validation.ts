/**
 * Vuetify validation rule factories derived from the buf.validate constraints
 * declared in the protos. Messages come from i18n so callers pass `t`.
 */
import type { useI18n } from 'vue-i18n'

type TFn = ReturnType<typeof useI18n>['t']
export type Rule = (v: unknown) => true | string

const SHA256_RE = /^[a-f0-9]{64}$/i

/**
 * Text of a form value: strings as-is, numbers as their decimal form, anything
 * else (absent, objects) as '' so it never renders as "[object Object]".
 */
function asText (v: unknown): string {
  if (typeof v === 'string') {
    return v
  }
  return typeof v === 'number' ? String(v) : ''
}

export function required (t: TFn): Rule {
  return v => asText(v).trim() !== '' || t('validation.required')
}

export function maxLength (t: TFn, max: number): Rule {
  return v => asText(v).length <= max || t('validation.maxLength', { max })
}

export function minLength (t: TFn, min: number): Rule {
  return v => {
    const s = asText(v)
    return s === '' || s.length >= min || t('validation.minLength', { min })
  }
}

export function sha256Rule (t: TFn): Rule {
  return v => {
    const s = asText(v)
    return s === '' || SHA256_RE.test(s) || t('validation.sha256')
  }
}
