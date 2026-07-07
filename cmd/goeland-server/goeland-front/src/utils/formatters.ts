/** Display formatters. All accept the proto-JSON string shapes. */

export function formatDate (value?: string): string {
  if (!value) {
    return '—'
  }
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {
    return value
  }
  return d.toLocaleDateString()
}

export function formatDateTime (value?: string): string {
  if (!value) {
    return '—'
  }
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {
    return value
  }
  return d.toLocaleString()
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
