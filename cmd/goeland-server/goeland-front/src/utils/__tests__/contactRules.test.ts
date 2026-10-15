import type { ContactType } from '@/api/types'
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { contactHref, contactValueRule, formatContactValue, normalizePhone, phoneDirectoryHref } from '../contactRules'
import { t } from './i18nStub'

interface SharedCase {
  name: string
  type: ContactType
  value: string
  valid: boolean
  normalized?: string
}

// The fixture the server's pkg/actor/contacts_test.go also reads: a rule that
// changes on one side only fails here or there.
const fixture = JSON.parse(readFileSync(new URL('../../../../../../pkg/actor/testdata/contact_values.json', import.meta.url), 'utf8')) as { cases: SharedCase[] }

const PHONE_TYPES = new Set<ContactType>(['CONTACT_TYPE_PHONE', 'CONTACT_TYPE_PHONE_PRIVATE', 'CONTACT_TYPE_PHONE_PRO', 'CONTACT_TYPE_MOBILE', 'CONTACT_TYPE_FAX'])

describe('contact rules shared with the server', () => {
  it('has cases', () => {
    expect(fixture.cases.length).toBeGreaterThan(0)
  })

  for (const c of fixture.cases) {
    it(`${c.valid ? 'accepts' : 'rejects'} ${c.name}`, () => {
      const verdict = contactValueRule(t, c.type)(c.value)
      expect(verdict === true).toBe(c.valid)
      if (c.valid && PHONE_TYPES.has(c.type)) {
        expect(normalizePhone(c.value)).toBe(c.normalized)
      }
    })
  }

  it('accepts an empty value (required is a separate rule)', () => {
    expect(contactValueRule(t, 'CONTACT_TYPE_EMAIL')('')).toBe(true)
  })

  it('names the kind in the message', () => {
    expect(contactValueRule(t, 'CONTACT_TYPE_IDE_FEDERAL')('CHE-1')).toBe('validation.contact.ide')
  })
})

describe('contact display', () => {
  it('groups Swiss E.164 numbers', () => {
    expect(formatContactValue('CONTACT_TYPE_PHONE', '+41213152222')).toBe('+41 21 315 22 22')
    expect(formatContactValue('CONTACT_TYPE_PHONE', '+33123456789')).toBe('+33123456789')
    expect(formatContactValue('CONTACT_TYPE_EMAIL', '+41213152222')).toBe('+41213152222')
  })

  it('links what is linkable', () => {
    expect(contactHref('CONTACT_TYPE_MOBILE', '+41791234567')).toBe('tel:+41791234567')
    expect(contactHref('CONTACT_TYPE_EMAIL', 'a@b.ch')).toBe('mailto:a@b.ch')
    expect(contactHref('CONTACT_TYPE_WEBSITE', 'https://x.ch')).toBe('https://x.ch')
    expect(contactHref('CONTACT_TYPE_COMMERCIAL_REGISTER', 'CH-550.1.012.345-6')).toBeUndefined()
    expect(contactHref('CONTACT_TYPE_VAT_NUMBER', 'CHE-105.851.230 TVA')).toBeUndefined()
  })
})

describe('contact links (GLD-063)', () => {
  it('links an IDE number to the Swiss business register', () => {
    expect(contactHref('CONTACT_TYPE_IDE_FEDERAL', 'CHE-105.851.230')).toBe('https://www.uid.admin.ch/Detail.aspx?uid_id=CHE-105.851.230')
  })

  it('links an ABACUS debtor number to the debtor application', () => {
    expect(contactHref('CONTACT_TYPE_ABACUS_DEBTOR', '20000506')).toBe('https://debiteur.lausanne.ch/debiteur-ui/details-debiteur/20000506')
  })

  it('searches only Swiss phone numbers in the Swiss directory', () => {
    expect(phoneDirectoryHref('+41218071515')).toBe('https://map.search.ch/?tel=0218071515')
    expect(phoneDirectoryHref('021 807 15 15')).toBe('https://map.search.ch/?tel=0218071515')
    expect(phoneDirectoryHref('+33142000000')).toBeUndefined()
  })
})
