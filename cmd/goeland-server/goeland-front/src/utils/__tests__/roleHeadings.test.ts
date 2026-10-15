import type { RelationshipType } from '@/api/types'
import { describe, expect, it } from 'vitest'
import { pluralizeRole, roleHeading } from '../roleHeadings'

const caseClient: RelationshipType = {
  code: 'CASE_HAS_ACTOR_CLIENT', label: 'Affaire a client', inverseLabel: 'Acteur client dans affaire',
  sourceKind: 'SUBJECT_KIND_CASE', targetKind: 'SUBJECT_KIND_ACTOR', roleLabel: 'Client', roleEnd: 'ROLE_END_TARGET', rolePreposition: 'de',
}

describe('role headings (GLD-063)', () => {
  it('reads from the end playing the role, plural and singular', () => {
    expect(roleHeading(caseClient, 'TARGET', 36)).toBe('Client des affaires suivantes')
    expect(roleHeading(caseClient, 'TARGET', 1)).toBe('Client de l’affaire suivante')
  })

  it('reads the role itself from the other end', () => {
    expect(roleHeading(caseClient, 'SOURCE', 3)).toBe('Clients')
    expect(roleHeading(caseClient, 'SOURCE', 1)).toBe('Client')
  })

  it('contracts French prepositions with the article', () => {
    const author: RelationshipType = { ...caseClient, code: 'DOCUMENT_AUTHORED_BY_ACTOR', label: '', sourceKind: 'SUBJECT_KIND_DOCUMENT', roleLabel: 'Auteur' }
    expect(roleHeading(author, 'TARGET', 760)).toBe('Auteur des documents suivants')
    expect(roleHeading(author, 'TARGET', 1)).toBe('Auteur du document suivant')
    const related: RelationshipType = { ...caseClient, targetKind: 'SUBJECT_KIND_CASE', roleLabel: 'Affaire liée', roleLabelPlural: 'Affaires liées', rolePreposition: 'à' }
    expect(roleHeading(related, 'TARGET', 2)).toBe('Affaire liée aux affaires suivantes')
    expect(roleHeading(related, 'SOURCE', 2)).toBe('Affaires liées')
  })

  it('keeps other prepositions as written', () => {
    const easement: RelationshipType = { ...caseClient, sourceKind: 'SUBJECT_KIND_THING', roleLabel: 'Bénéficiaire de servitude', roleLabelPlural: 'Bénéficiaires de servitude', rolePreposition: 'sur' }
    expect(roleHeading(easement, 'TARGET', 1)).toBe('Bénéficiaire de servitude sur l’objet suivant')
    expect(roleHeading(easement, 'SOURCE', 4)).toBe('Bénéficiaires de servitude')
  })

  it('handles a role played by the source', () => {
    const contact: RelationshipType = { ...caseClient, sourceKind: 'SUBJECT_KIND_ACTOR', roleLabel: 'Personne de contact', roleLabelPlural: 'Personnes de contact', roleEnd: 'ROLE_END_SOURCE' }
    expect(roleHeading(contact, 'SOURCE', 1)).toBe('Personne de contact de l’acteur suivant')
    expect(roleHeading(contact, 'TARGET', 2)).toBe('Personnes de contact')
  })

  it('falls back to the labels without a role', () => {
    const legacy: RelationshipType = { ...caseClient, roleLabel: '' }
    expect(roleHeading(legacy, 'SOURCE', 2)).toBe('Affaire a client')
    expect(roleHeading(legacy, 'TARGET', 2)).toBe('Acteur client dans affaire')
  })

  it('composes in English', () => {
    expect(roleHeading(caseClient, 'TARGET', 2, 'en')).toBe('Client of the following cases')
  })

  it('pluralizes the first word of a role', () => {
    expect(pluralizeRole('Bureau d’étude')).toBe('Bureaux d’étude')
    expect(pluralizeRole('Responsable projet')).toBe('Responsables projet')
    expect(pluralizeRole('Locataire')).toBe('Locataires')
    expect(pluralizeRole('Fils')).toBe('Fils')
    expect(pluralizeRole('Local')).toBe('Locaux')
  })
})
