-- migrate:up

-- Goéland POC — relationship roles read from the current page (roadmap GLD-063).
--
-- A relationship type had a label read from the source ("Affaire a client") and
-- an inverse label read from the target, both written in the third person, and
-- the SPA always showed the first. Users read a relationship from the subject
-- they are looking at: on an actor page "Client des affaires suivantes", on the
-- case page "Clients". So a type now names the ROLE one end plays for the other:
--
--   role_label         the role, singular ("Client", "Personne de contact");
--   role_label_plural  its plural when not regular ("Bureaux d'étude"); empty
--                      means the SPA derives it;
--   role_end           which end plays it: TARGET (an actor client of a case)
--                      or SOURCE (a person contact of an organization);
--   role_preposition   how the role joins the other end's kind in a heading:
--                      "de" (Client de l'affaire suivante), "par" (Objet
--                      concerné par l'affaire suivante), "sur", "à", "dans".
--
-- Seen from the end the role belongs to, a group is titled role + preposition +
-- the other kind ("Client des affaires suivantes"); seen from the other end, by
-- the plural role ("Clients"). A type without a role keeps its labels.

ALTER TABLE relationship_type
    ADD COLUMN role_label        TEXT NOT NULL DEFAULT '',
    ADD COLUMN role_label_plural TEXT NOT NULL DEFAULT '',
    ADD COLUMN role_end          TEXT NOT NULL DEFAULT 'TARGET',
    ADD COLUMN role_preposition  TEXT NOT NULL DEFAULT 'de',
    ADD CONSTRAINT relationship_type_role_end_valid CHECK (role_end IN ('SOURCE', 'TARGET')),
    ADD CONSTRAINT relationship_type_role_preposition_valid
        CHECK (role_preposition IN ('de', 'à', 'par', 'sur', 'dans', 'avec', 'pour'));

-- Seeded types (a code absent from this database is simply not updated).
UPDATE relationship_type AS rt
SET role_label = v.role, role_label_plural = v.plural, role_end = v.role_end, role_preposition = v.prep
FROM (VALUES
    ('CASE_HAS_ACTOR_REQUESTER',             'Requérant',                 '',                           'TARGET', 'de'),
    ('CASE_HAS_ACTOR_MANDATEE',              'Mandataire',                '',                           'TARGET', 'de'),
    ('CASE_HAS_ACTOR_OWNER',                 'Propriétaire',              '',                           'TARGET', 'de'),
    ('CASE_HAS_ACTOR_ARCHITECT',             'Architecte',                '',                           'TARGET', 'de'),
    ('CASE_HAS_ACTOR_CONTRACTOR',            'Entreprise',                '',                           'TARGET', 'de'),
    ('DOCUMENT_AUTHORED_BY_ACTOR',           'Auteur',                    '',                           'TARGET', 'de'),
    ('DOCUMENT_SENT_TO_ACTOR',               'Destinataire',              '',                           'TARGET', 'de'),
    ('THING_HAS_ACTOR_OWNER',                'Propriétaire',              '',                           'TARGET', 'de'),
    ('THING_HAS_ACTOR_TENANT',               'Locataire',                 '',                           'TARGET', 'de'),
    ('THING_HAS_ACTOR_SURFACE_RIGHT_HOLDER', 'Superficiaire',             '',                           'TARGET', 'de'),
    ('THING_HAS_ACTOR_FARMER',               'Fermier',                   '',                           'TARGET', 'de'),
    ('THING_HAS_ACTOR_EASEMENT_HOLDER',      'Bénéficiaire de servitude', 'Bénéficiaires de servitude', 'TARGET', 'sur'),
    ('ACTOR_BRANCH_OF_ACTOR',                'Succursale',                '',                           'SOURCE', 'de'),
    ('ACTOR_CONTACT_PERSON_OF_ACTOR',        'Personne de contact',       'Personnes de contact',       'SOURCE', 'de'),
    ('CASE_HAS_ORG_UNIT_LEADER',             'Unité pilote',              'Unités pilotes',             'TARGET', 'de'),
    ('CASE_HAS_ORG_UNIT_MANAGER',            'Unité gestionnaire',        'Unités gestionnaires',       'TARGET', 'de'),
    ('CASE_HAS_ORG_UNIT_PARTICIPANT',        'Unité participante',        'Unités participantes',       'TARGET', 'à'),
    ('CASE_HAS_DOCUMENT',                    'Document',                  '',                           'TARGET', 'de'),
    ('CASE_CONCERNS_THING',                  'Objet concerné',            'Objets concernés',           'TARGET', 'par'),
    ('DOCUMENT_REPRESENTS_THING',            'Objet représenté',          'Objets représentés',         'TARGET', 'par'),
    ('DOCUMENT_PREVIOUS_VERSION',            'Version précédente',        'Versions précédentes',       'TARGET', 'de'),
    ('THING_CONTAINS_THING',                 'Objet contenu',             'Objets contenus',            'TARGET', 'dans'),
    ('CASE_PARENT_OF_CASE',                  'Sous-affaire',              '',                           'TARGET', 'de'),
    ('CASE_RELATED_TO_CASE',                 'Affaire liée',              'Affaires liées',             'TARGET', 'à'),
    ('USER_MEMBER_OF_GROUP',                 'Membre',                    '',                           'SOURCE', 'de'),
    ('USER_MEMBER_OF_ORG_UNIT',              'Membre',                    '',                           'SOURCE', 'de')
) AS v (code, role, plural, role_end, prep)
WHERE rt.code = v.code;

-- Roles created by the legacy import (GLD-052): the participant (target) plays
-- them. Their description keeps the legacy role name as written
-- ("Rôle « Client » de Goéland (import)"); their label has it in lower case
-- ("Affaire a client", "Affaire avec unité ...", "Objet a ...", "Document a ...",
-- "... (collaborateur)"), used with an upper-case initial when the description
-- does not match.
UPDATE relationship_type
SET role_label = CASE
        WHEN description ~ '^Rôle « .+ » de Goéland \(import\)$'
            THEN btrim(regexp_replace(description, '^Rôle « (.+) » de Goéland \(import\)$', '\1'))
        ELSE upper(left(btrim(regexp_replace(label, '^(Affaire a |Affaire avec unité |Objet a |Document a )(.*?)( \(collaborateur\))?$', '\2')), 1))
             || substr(btrim(regexp_replace(label, '^(Affaire a |Affaire avec unité |Objet a |Document a )(.*?)( \(collaborateur\))?$', '\2')), 2)
    END
WHERE role_label = ''
  AND code ~ '^(CASE_HAS_ACTOR_|CASE_HAS_USER_|CASE_HAS_ORG_UNIT_|THING_HAS_ACTOR_|DOCUMENT_HAS_ACTOR_)'
  AND (description ~ '^Rôle « .+ » de Goéland \(import\)$' OR label ~ '^(Affaire a |Affaire avec unité |Objet a |Document a )');

-- migrate:down

ALTER TABLE relationship_type
    DROP CONSTRAINT IF EXISTS relationship_type_role_preposition_valid,
    DROP CONSTRAINT IF EXISTS relationship_type_role_end_valid,
    DROP COLUMN IF EXISTS role_preposition,
    DROP COLUMN IF EXISTS role_end,
    DROP COLUMN IF EXISTS role_label_plural,
    DROP COLUMN IF EXISTS role_label;
