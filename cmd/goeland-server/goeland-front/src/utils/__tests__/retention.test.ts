import { describe, expect, it } from 'vitest'
import { bordereauStatusColor, decisionColor, dispositionColor, isDue, zurichToday } from '../retention'

describe('retention', () => {
  it('takes today in Zurich, not in UTC', () => {
    // 23:30 UTC on 28 October is already 29 October in Zurich (UTC+1 in winter).
    expect(zurichToday(new Date('2026-10-28T23:30:00Z'))).toBe('2026-10-29')
    expect(zurichToday(new Date('2026-07-01T10:00:00Z'))).toBe('2026-07-01')
  })

  it('is due from the day the retention ends', () => {
    expect(isDue('2026-10-28', '2026-10-28')).toBe(true)
    expect(isDue('2026-10-29', '2026-10-28')).toBe(false)
    expect(isDue(undefined, '2026-10-28')).toBe(false)
  })

  it('colours the dispositions', () => {
    expect(dispositionColor('FINAL_DISPOSITION_DESTROY')).toBe('error')
    expect(dispositionColor(undefined)).toBe('default')
    expect(bordereauStatusColor('BORDEREAU_STATUS_EXECUTED')).toBe('success')
    expect(decisionColor('DISPOSITION_DECISION_PENDING')).toBe('warning')
  })
})
