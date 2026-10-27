/** Display formatters. All accept the proto-JSON string shapes. */
import { ref } from 'vue'

// Dates follow the interface language, not the browser's: the FR/EN switch
// changes them too, and English uses day/month/year as in Switzerland.
const DATE_LOCALES: Record<string, string> = { 'fr-CH': 'fr-CH', 'en': 'en-GB' }
// Reactive, so the dates on screen change with the language.
const interfaceLocale = ref('fr-CH')

/** Sets the interface language the dates are written in (App.vue follows vue-i18n). */
export function setDateLocale (locale: string): void {
  interfaceLocale.value = locale
}

/** The locale dates are written in, from the interface language. */
export function dateLocale (): string {
  return DATE_LOCALES[interfaceLocale.value] ?? 'fr-CH'
}

export function formatDate (value?: string): string {
  if (!value) {
    return '—'
  }
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {
    return value
  }
  return d.toLocaleDateString(dateLocale())
}

export function formatDateTime (value?: string): string {
  if (!value) {
    return '—'
  }
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {
    return value
  }
  return d.toLocaleString(dateLocale())
}

/** Formats an int64-as-string (or number) byte count into a human size. */
export function formatBytes (value?: string | number): string {
  const raw = value ?? ''
  if (raw === '') {
    return '—'
  }
  let n = typeof raw === 'string' ? Number(raw) : raw
  if (Number.isNaN(n)) {
    return String(value)
  }
  if (n < 1024) {
    return `${n} B`
  }
  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  let i = -1
  do {
    n /= 1024
    i++
  } while (n >= 1024 && i < units.length - 1)
  return `${n.toFixed(1)} ${units[i]}`
}

/** Short SHA-256 display (first/last 8 hex chars). */
export function shortHash (sha?: string): string {
  if (!sha) {
    return '—'
  }
  return sha.length > 20 ? `${sha.slice(0, 8)}…${sha.slice(-8)}` : sha
}

/** A total for display: "10 000+" when it is a lower bound (counting stops, GLD-053). */
export function formatTotal (total?: number, capped?: boolean): string {
  const text = (total ?? 0).toLocaleString('fr-CH')
  return capped ? `${text}+` : text
}
