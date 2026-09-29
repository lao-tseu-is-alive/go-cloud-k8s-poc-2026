/** Conversions between RFC3339 values and <input type="datetime-local"> values (local time). */

const pad = (n: number) => String(n).padStart(2, '0')

/** RFC3339 → value of an <input type="datetime-local"> in local time. */
export function toLocalInput (iso?: string): string {
  const d = iso ? new Date(iso) : new Date()
  if (Number.isNaN(d.getTime())) {
    return ''
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** Value of an <input type="datetime-local"> → RFC3339 (undefined when empty or invalid). */
export function fromLocalInput (value: string): string | undefined {
  if (!value) {
    return undefined
  }
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString()
}
