/**
 * The sort of a list (GLD-055): a sortable column and its direction, sent to
 * the server as order_by ("<field>" or "<field> desc"); undefined keeps the
 * list's default order.
 */
export interface ListSort {
  field: string
  desc: boolean
}

/** The order_by parameter of a sort, undefined for the default order. */
export function toOrderBy (sort?: ListSort): string | undefined {
  if (!sort) {
    return undefined
  }
  return sort.desc ? `${sort.field} desc` : sort.field
}

/** The next sort when a column is clicked: ascending first, then the other way. */
export function nextSort (current: ListSort | undefined, field: string): ListSort {
  if (current?.field === field) {
    return { field, desc: !current.desc }
  }
  return { field, desc: false }
}

/** Sorts rows already loaded (short lists) by a field value, as the server would. */
export function sortRows<T> (rows: T[], sort: ListSort | undefined, value: (row: T, field: string) => string | number | undefined): T[] {
  if (!sort) {
    return rows
  }
  const collator = new Intl.Collator('fr-CH', { numeric: true, sensitivity: 'base' })
  const sign = sort.desc ? -1 : 1
  return rows.toSorted((a, b) => {
    const x = value(a, sort.field)
    const y = value(b, sort.field)
    if (x === undefined || x === '') {
      return 1
    }
    if (y === undefined || y === '') {
      return -1
    }
    return sign * (typeof x === 'number' && typeof y === 'number' ? x - y : collator.compare(String(x), String(y)))
  })
}
