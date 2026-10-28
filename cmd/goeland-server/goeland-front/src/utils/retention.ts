/** Retention helpers (GLD-032): days are YYYY-MM-DD in Europe/Zurich, like the server. */

/** Today in Europe/Zurich as YYYY-MM-DD. */
export function zurichToday (now: Date = new Date()): string {
  // en-CA writes ISO-like dates (2026-10-28).
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Europe/Zurich', year: 'numeric', month: '2-digit', day: '2-digit' }).format(now)
}

/** Whether a retention ending on untilOn is due on day (both YYYY-MM-DD). */
export function isDue (untilOn: string | undefined, day: string): boolean {
  return !!untilOn && untilOn <= day
}

/** Chip colour of a final disposition. */
export function dispositionColor (disposition: string | undefined): string {
  switch (disposition) {
    case 'FINAL_DISPOSITION_ARCHIVE': {
      return 'primary'
    }
    case 'FINAL_DISPOSITION_DESTROY': {
      return 'error'
    }
    case 'FINAL_DISPOSITION_REVIEW': {
      return 'warning'
    }
    default: {
      return 'default'
    }
  }
}
