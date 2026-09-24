import type { Rule } from './validation'
/** Postal address helpers: checks mirroring pkg/actor/addresses.go and display. */
import type { ActorAddress } from '@/api/types'
import type { useI18n } from 'vue-i18n'

type TFn = ReturnType<typeof useI18n>['t']

const SWISS_POSTAL_CODE = /^[1-9]\d{3}$/
const COUNTRY_CODE = /^[A-Z]{2}$/i

function isSwiss (countryCode?: string): boolean {
  return (countryCode?.trim() || 'CH').toUpperCase() === 'CH'
}

/** Rule for a postal code: four digits (1000-9999) in Switzerland. */
export function postalCodeRule (t: TFn, countryCode?: string): Rule {
  return v => {
    const s = typeof v === 'string' ? v.trim() : ''
    return s === '' || !isSwiss(countryCode) || SWISS_POSTAL_CODE.test(s) || t('validation.postalCodeCH')
  }
}

/** Rule for an ISO 3166-1 alpha-2 country code (empty means CH). */
export function countryCodeRule (t: TFn): Rule {
  return v => {
    const s = typeof v === 'string' ? v.trim() : ''
    return s === '' || COUNTRY_CODE.test(s) || t('validation.countryCode')
  }
}

/** The address as display lines: street and number, complement, postal code and locality. */
export function addressLines (a: ActorAddress): string[] {
  const country = isSwiss(a.countryCode) ? '' : ` (${a.countryCode})`
  return [
    [a.street, a.houseNumber].filter(Boolean).join(' '),
    a.addressLine2 ?? '',
    `${a.postalCode} ${a.locality}${country}`,
  ].filter(line => line.trim() !== '')
}

/** A map.geo.admin.ch search for a Swiss address, or undefined abroad. */
export function swissMapUrl (a: ActorAddress): string | undefined {
  if (!isSwiss(a.countryCode)) {
    return undefined
  }
  const query = `${a.street} ${a.houseNumber ?? ''} ${a.postalCode} ${a.locality}`.replaceAll(/\s+/g, ' ').trim()
  return `https://map.geo.admin.ch/?swisssearch=${encodeURIComponent(query)}`
}
