/**
 * Vuetify validation rule factories derived from the buf.validate constraints
 * declared in the protos. Messages come from i18n so callers pass `t`.
 */
import type { useI18n } from 'vue-i18n'

type TFn = ReturnType<typeof useI18n>['t']
export type Rule = (v: unknown) => true | string

const SHA256_RE = /^[a-f0-9]{64}$/i

export function required (t: TFn): Rule {
  return v => (v !== undefined && v !== null && String(v).trim() !== '') || t('validation.required')
}

export function maxLength (t: TFn, max: number): Rule {
  return v => v === undefined || v === null || String(v).length <= max || t('validation.maxLength', { max })
}

export function minLength (t: TFn, min: number): Rule {
  return v => v === undefined || v === null || String(v) === '' || String(v).length >= min || t('validation.minLength', { min })
}

export function sha256Rule (t: TFn): Rule {
  return v => {
    const s = String(v ?? '')
    return s === '' || SHA256_RE.test(s) || t('validation.sha256')
  }
}
