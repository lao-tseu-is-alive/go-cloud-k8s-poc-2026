/**
 * Relationship headings read from the subject on screen (GLD-063). A
 * relationship type names the role one end plays for the other ("Client",
 * played by the actor in a case). Seen from the end playing it, a group of
 * edges is titled by the role, its preposition and the other kind
 * ("Client des affaires suivantes", "Client de l’affaire suivante"); seen from
 * the other end, by the role itself ("Clients", "Client"). A type without a
 * role falls back to its labels.
 */
import type { RelationshipGroup, RelationshipType, SubjectKind } from '@/api/types'

/** Past this many edges, a case group also filters by case type and period. */
export const GROUP_FILTER_THRESHOLD = 10

/** A section of a roles panel: the groups whose other ends are of one kind. */
export interface RoleSection {
  kind: SubjectKind
  /** The id of the section, target of the page's jump links. */
  anchor: string
  /** The number of edges of its groups. */
  total: number
  /** The section title, read from the subject on screen ("Pièces du dossier"). */
  title: string
  /** The short name of the section in the page's jump links. */
  label: string
  groups: { key: string, heading: string, group: RelationshipGroup }[]
}

/** The end of an edge the subject on screen stands on. */
export type ViewedFrom = 'SOURCE' | 'TARGET'

/** Languages the headings are composed in. */
export type HeadingLang = 'fr' | 'en'

interface KindPhrase { one: string, many: string }

const FR_KINDS: Partial<Record<SubjectKind, KindPhrase>> = {
  SUBJECT_KIND_CASE: { one: 'l’affaire suivante', many: 'les affaires suivantes' },
  SUBJECT_KIND_DOCUMENT: { one: 'le document suivant', many: 'les documents suivants' },
  SUBJECT_KIND_THING: { one: 'l’objet suivant', many: 'les objets suivants' },
  SUBJECT_KIND_ACTOR: { one: 'l’acteur suivant', many: 'les acteurs suivants' },
  SUBJECT_KIND_USER: { one: 'l’utilisateur suivant', many: 'les utilisateurs suivants' },
  SUBJECT_KIND_ORG_UNIT: { one: 'l’unité suivante', many: 'les unités suivantes' },
  SUBJECT_KIND_GROUP: { one: 'le groupe suivant', many: 'les groupes suivants' },
}

const EN_KINDS: Partial<Record<SubjectKind, KindPhrase>> = {
  SUBJECT_KIND_CASE: { one: 'the following case', many: 'the following cases' },
  SUBJECT_KIND_DOCUMENT: { one: 'the following document', many: 'the following documents' },
  SUBJECT_KIND_THING: { one: 'the following object', many: 'the following objects' },
  SUBJECT_KIND_ACTOR: { one: 'the following actor', many: 'the following actors' },
  SUBJECT_KIND_USER: { one: 'the following user', many: 'the following users' },
  SUBJECT_KIND_ORG_UNIT: { one: 'the following unit', many: 'the following units' },
  SUBJECT_KIND_GROUP: { one: 'the following group', many: 'the following groups' },
}

const EN_PREPOSITIONS: Record<string, string> = { de: 'of', à: 'to', par: 'by', sur: 'on', dans: 'in', avec: 'with', pour: 'for' }

/** Joins a French preposition and a phrase starting with its article ("de les" → "des"). */
function joinFrench (preposition: string, phrase: string): string {
  if (preposition === 'de' || preposition === 'à') {
    const contracted = preposition === 'de' ? { les: 'des', le: 'du' } : { les: 'aux', le: 'au' }
    for (const [article, merged] of Object.entries(contracted)) {
      if (phrase.startsWith(article + ' ')) {
        return merged + phrase.slice(article.length)
      }
    }
  }
  return preposition + ' ' + phrase
}

/**
 * The regular French plural of a role: its first word takes the mark
 * ("Bureau d’étude" → "Bureaux d’étude", "Responsable projet" → "Responsables
 * projet"); a type stores an irregular plural itself.
 */
export function pluralizeRole (role: string): string {
  const space = role.indexOf(' ')
  const first = space === -1 ? role : role.slice(0, space)
  const rest = space === -1 ? '' : role.slice(space)
  let plural = first + 's'
  if (/[sxz]$/i.test(first)) {
    plural = first
  } else if (/(?:eau|au|eu)$/i.test(first)) {
    plural = first + 'x'
  } else if (/al$/i.test(first)) {
    plural = first.slice(0, -2) + 'aux'
  }
  return plural + rest
}

/** The end playing the role of a type (TARGET unless SOURCE). */
function roleEnd (type: RelationshipType): ViewedFrom {
  return type.roleEnd === 'ROLE_END_SOURCE' ? 'SOURCE' : 'TARGET'
}

/**
 * The heading of `count` edges of `type` seen from the subject on screen,
 * standing on the `from` end of each edge.
 */
export function roleHeading (type: RelationshipType, from: ViewedFrom, count: number, lang: HeadingLang = 'fr'): string {
  const role = type.roleLabel?.trim()
  if (!role) {
    return from === 'SOURCE' ? type.label : (type.inverseLabel || type.label)
  }
  if (from !== roleEnd(type)) {
    return count === 1 ? role : (type.roleLabelPlural?.trim() || pluralizeRole(role))
  }
  const otherKind = from === 'TARGET' ? type.sourceKind : type.targetKind
  const phrases = (lang === 'en' ? EN_KINDS : FR_KINDS)[otherKind ?? 'SUBJECT_KIND_UNSPECIFIED']
  if (!phrases) {
    return role
  }
  const phrase = count === 1 ? phrases.one : phrases.many
  const preposition = type.rolePreposition || 'de'
  if (lang === 'en') {
    return `${role} ${EN_PREPOSITIONS[preposition] ?? 'of'} ${phrase}`
  }
  return `${role} ${joinFrench(preposition, phrase)}`
}
