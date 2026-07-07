/**
 * Translates a stored enum *code* into a human label via i18n, falling back to
 * the raw code when no translation exists. Codes are never sent translated to
 * the API — this is display-only.
 */
import { useI18n } from 'vue-i18n'

export function useI18nEnum () {
  const { t, te } = useI18n()

  function enumLabel (enumName: string, value: string | undefined): string {
    if (!value) {
      return ''
    }
    const key = `enums.${enumName}.${value}`
    return te(key) ? t(key) : value
  }

  return { enumLabel }
}
