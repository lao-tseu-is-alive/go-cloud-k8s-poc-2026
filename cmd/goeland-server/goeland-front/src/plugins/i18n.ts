import { createI18n } from 'vue-i18n'
import en from '@/locales/en.json'
import frCH from '@/locales/fr-CH.json'

// The POC default UI language is fr-CH (see the frontend brief §12.2). This is
// the *interface* language and is unrelated to Document.language (the language
// of a document's content).
export const SUPPORTED_LOCALES = ['fr-CH', 'en'] as const
export type AppLocale = typeof SUPPORTED_LOCALES[number]

export default createI18n({
  legacy: false,
  locale: 'fr-CH',
  fallbackLocale: 'en',
  messages: {
    'fr-CH': frCH,
    en,
  },
})
