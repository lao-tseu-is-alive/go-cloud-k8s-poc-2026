/**
 * Shared model + mappers for the actor create/edit forms.
 *
 * The model is a flat, UI-friendly shape; buildCreateRequest / buildUpdateRequest
 * fold it back into the proto3-JSON request shapes (only the specialization block
 * matching the kind is sent). Enum codes travel untranslated.
 */
import type {
  ActorContact,
  ActorKind,
  ContactType,
  CreateActorRequest,
  GoActor,
  Salutation,
  UpdateActorRequest,
} from '@/api/types'

export interface ActorFormModel {
  actorKind: ActorKind
  displayName: string
  publicationCode: number
  // organization specialization
  legalName: string
  categoryCode: string | undefined
  orgComplement: string
  // person specialization: minimal identity + register link
  salutation: Salutation
  lastName: string
  firstName: string
  isChRegister: boolean
  chRegisterRef: string
  contacts: ActorContact[]
}

/** The persistable contact types offered in the editor (excludes UNSPECIFIED). */
export const CONTACT_TYPES: ContactType[] = [
  'CONTACT_TYPE_PHONE',
  'CONTACT_TYPE_PHONE_PRIVATE',
  'CONTACT_TYPE_PHONE_PRO',
  'CONTACT_TYPE_MOBILE',
  'CONTACT_TYPE_FAX',
  'CONTACT_TYPE_EMAIL',
  'CONTACT_TYPE_WEBSITE',
  'CONTACT_TYPE_POSTAL_BOX',
  'CONTACT_TYPE_IDE_FEDERAL',
  'CONTACT_TYPE_VAT_NUMBER',
  'CONTACT_TYPE_ABACUS_DEBTOR',
  'CONTACT_TYPE_COMMERCIAL_REGISTER',
  'CONTACT_TYPE_OTHER',
]

export const ACTOR_KINDS: ActorKind[] = ['ACTOR_KIND_PERSON', 'ACTOR_KIND_ORGANIZATION']

export const SALUTATIONS: Salutation[] = ['SALUTATION_UNSPECIFIED', 'SALUTATION_MADAME', 'SALUTATION_MONSIEUR', 'SALUTATION_NEUTRAL']

/** A person's usual name derived from the identity, as the server does: "<first> <last>". */
export function personDisplayName (firstName: string, lastName: string): string {
  return `${firstName.trim()} ${lastName.trim()}`.trim()
}

export function emptyActorForm (): ActorFormModel {
  return {
    actorKind: 'ACTOR_KIND_PERSON',
    displayName: '',
    publicationCode: 0,
    legalName: '',
    categoryCode: undefined,
    orgComplement: '',
    salutation: 'SALUTATION_UNSPECIFIED',
    lastName: '',
    firstName: '',
    isChRegister: false,
    chRegisterRef: '',
    contacts: [],
  }
}

/** Projects an existing actor into the flat form model for editing. */
export function actorToForm (actor: GoActor): ActorFormModel {
  return {
    actorKind: actor.actorKind,
    displayName: actor.displayName ?? '',
    publicationCode: actor.publicationCode ?? 0,
    legalName: actor.organization?.legalName ?? '',
    categoryCode: actor.organization?.categoryCode || undefined,
    orgComplement: actor.organization?.complement ?? '',
    salutation: actor.person?.salutation ?? 'SALUTATION_UNSPECIFIED',
    lastName: actor.person?.lastName ?? '',
    firstName: actor.person?.firstName ?? '',
    isChRegister: actor.person?.isChRegister ?? false,
    chRegisterRef: actor.person?.chRegisterRef ?? '',
    contacts: (actor.contacts ?? []).map(c => ({ ...c })),
  }
}

function cleanContacts (contacts: ActorContact[]): ActorContact[] {
  return contacts
    .filter(c => c.value.trim() !== '')
    .map(c => ({
      contactType: c.contactType,
      value: c.value.trim(),
      isPrimary: c.isPrimary || undefined,
      label: c.label?.trim() || undefined,
    }))
}

export function buildCreateRequest (m: ActorFormModel): CreateActorRequest {
  const req: CreateActorRequest = {
    actorKind: m.actorKind,
    displayName: m.displayName.trim(),
    publicationCode: m.publicationCode || undefined,
    contacts: cleanContacts(m.contacts),
  }
  if (m.actorKind === 'ACTOR_KIND_ORGANIZATION') {
    req.organization = {
      legalName: m.legalName.trim(),
      categoryCode: m.categoryCode || undefined,
      complement: m.orgComplement.trim() || undefined,
    }
  } else {
    req.person = personDetails(m)
  }
  return req
}

export function buildUpdateRequest (m: ActorFormModel, reason: string): UpdateActorRequest {
  const req: UpdateActorRequest = {
    displayName: m.displayName.trim() || undefined,
    isActive: undefined, // toggled separately on the detail page
    publicationCode: m.publicationCode || undefined,
    replaceContacts: true,
    contacts: cleanContacts(m.contacts),
    reason: reason.trim() || undefined,
  }
  if (m.actorKind === 'ACTOR_KIND_ORGANIZATION') {
    req.organization = {
      legalName: m.legalName.trim(),
      categoryCode: m.categoryCode || undefined,
      complement: m.orgComplement.trim() || undefined,
    }
  } else {
    req.person = personDetails(m)
  }
  return req
}

/** The PERSON block of a request (the whole block is replaced on update). */
function personDetails (m: ActorFormModel) {
  return {
    salutation: m.salutation === 'SALUTATION_UNSPECIFIED' ? undefined : m.salutation,
    lastName: m.lastName.trim(),
    firstName: m.firstName.trim() || undefined,
    isChRegister: m.isChRegister || undefined,
    chRegisterRef: m.chRegisterRef.trim() || undefined,
  }
}
