import type { useI18n } from 'vue-i18n'

/** A translate function returning the message key, so rules can be asserted without i18n. */
export const t = ((key: string) => key) as unknown as ReturnType<typeof useI18n>['t']
