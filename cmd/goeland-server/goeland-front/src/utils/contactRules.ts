import type { Rule } from './validation'
/**
 * Per-type rules for actor complements ("contacts"), mirroring the server's
 * authoritative validation and normalization (pkg/actor/contacts.go) so the form
 * reacts immediately; the server still normalizes what it stores.
 */
import type { ContactType } from '@/api/types'
import type { useI18n } from 'vue-i18n'

type TFn = ReturnType<typeof useI18n>['t']

type Kind = 'phone' | 'email' | 'website' | 'postalBox' | 'ide' | 'vat' | 'abacus' | 'register' | 'other'

const KINDS: Record<ContactType, Kind> = {
  CONTACT_TYPE_UNSPECIFIED: 'other',
  CONTACT_TYPE_PHONE: 'phone',
  CONTACT_TYPE_PHONE_PRIVATE: 'phone',
  CONTACT_TYPE_PHONE_PRO: 'phone',
  CONTACT_TYPE_MOBILE: 'phone',
  CONTACT_TYPE_FAX: 'phone',
  CONTACT_TYPE_EMAIL: 'email',
  CONTACT_TYPE_WEBSITE: 'website',
  CONTACT_TYPE_POSTAL_BOX: 'postalBox',
  CONTACT_TYPE_IDE_FEDERAL: 'ide',
  CONTACT_TYPE_VAT_NUMBER: 'vat',
  CONTACT_TYPE_ABACUS_DEBTOR: 'abacus',
  CONTACT_TYPE_COMMERCIAL_REGISTER: 'register',
  CONTACT_TYPE_OTHER: 'other',
}

const PLACEHOLDERS: Record<Kind, string> = {
  phone: '021 315 22 22',
  email: 'nom@exemple.ch',
  website: 'www.exemple.ch',
  postalBox: 'Case postale 1234',
  ide: 'CHE-123.456.788',
  vat: 'CHE-123.456.788 TVA',
  abacus: '123456',
  register: 'CH-550.1.012.345-6',
  other: '',
}

const E164 = /^\+[1-9]\d{6,14}$/
const PHONE_SEPARATORS = /[\s\-./()]/g
const EMAIL = /^[^\s@<>()]+@[^\s()<>@][^\s().<>@]*\.[^\s()<>@]+$/
const POSTAL_BOX = /^(?:case postale|c\.?p\.?|postfach|pf|po box|p\.o\. box)?\s*\d{1,6}$/i
const ID_SEPARATORS = /[\s.-]/g
const IDE = /^CHE(\d{9})$/
const VAT_SUFFIX = /(?:MWST|TVA|IVA)$/i
const ABACUS = /^\d{1,10}$/
const REGISTER_ID = /^CH\d{11}$/
const IDE_WEIGHTS = [5, 4, 3, 2, 7, 6, 5, 4]

export function contactKind (type?: ContactType): Kind {
  return type ? KINDS[type] : 'other'
}

export function contactPlaceholder (type?: ContactType): string {
  return PLACEHOLDERS[contactKind(type)]
}

/** E.164 form of a phone number, or undefined when it is not one. */
export function normalizePhone (value: string): string | undefined {
  let n = value.replace(PHONE_SEPARATORS, '')
  if (n.startsWith('00')) {
    n = `+${n.slice(2)}`
  } else if (n.length === 10 && n.startsWith('0')) {
    n = `+41${n.slice(1)}`
  }
  return E164.test(n) ? n : undefined
}

function validIde (value: string): boolean {
  const m = IDE.exec(value.replace(ID_SEPARATORS, '').toUpperCase())
  if (!m?.[1]) {
    return false
  }
  const digits = m[1]
  const sum = IDE_WEIGHTS.reduce((acc, w, i) => acc + Number(digits[i]) * w, 0)
  const check = (11 - (sum % 11)) % 11
  return check !== 10 && check === Number(digits[8])
}

function validWebsite (value: string): boolean {
  try {
    const url = new URL(value.includes('://') ? value : `https://${value}`)
    return ['http:', 'https:'].includes(url.protocol) && url.hostname.includes('.') && !url.username
  } catch {
    return false
  }
}

const CHECKS: Record<Kind, (v: string) => boolean> = {
  phone: v => normalizePhone(v) !== undefined,
  email: v => EMAIL.test(v) && v.length <= 254,
  website: validWebsite,
  postalBox: v => POSTAL_BOX.test(v),
  ide: validIde,
  vat: v => {
    const suffix = VAT_SUFFIX.exec(v)
    return !!suffix && validIde(v.slice(0, suffix.index))
  },
  abacus: v => ABACUS.test(v.replaceAll(' ', '')),
  register: v => REGISTER_ID.test(v.replace(ID_SEPARATORS, '').toUpperCase()) || (v.includes('://') && validWebsite(v)),
  other: () => true,
}

/** Vuetify rule checking a complement value against its type. */
export function contactValueRule (t: TFn, type?: ContactType): Rule {
  const kind = contactKind(type)
  return v => {
    const s = typeof v === 'string' ? v.trim() : ''
    return s === '' || CHECKS[kind](s) || t(`validation.contact.${kind}`, { example: PLACEHOLDERS[kind] })
  }
}

/** Display form: Swiss E.164 numbers grouped as +41 21 315 22 22. */
export function formatContactValue (type: ContactType | undefined, value: string): string {
  const m = contactKind(type) === 'phone' ? /^\+41(\d{2})(\d{3})(\d{2})(\d{2})$/.exec(value) : null
  return m ? `+41 ${m[1]} ${m[2]} ${m[3]} ${m[4]}` : value
}

/** Link for a complement (tel:, mailto:, web), or undefined when not linkable. */
export function contactHref (type: ContactType | undefined, value: string): string | undefined {
  switch (contactKind(type)) {
    case 'phone': {
      return `tel:${value}`
    }
    case 'email': {
      return `mailto:${value}`
    }
    case 'website': {
      return value
    }
    case 'register': {
      return value.includes('://') ? value : undefined
    }
    default: {
      return undefined
    }
  }
}
