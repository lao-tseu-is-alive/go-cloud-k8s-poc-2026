-- migrate:up

-- Goéland POC — seed reference data (spec §14).
--
-- Reference/controlled data required for the domain to function:
--   * subject kinds
--   * catalogue of relationship types (typed edges)
--   * catalogue of document types (classification)
-- All inserts are idempotent (ON CONFLICT DO NOTHING) so re-running is safe.

-- Subject kinds --------------------------------------------------------------
INSERT INTO subject_kind (code) VALUES
    ('CASE'),
    ('DOCUMENT'),
    ('THING'),
    ('ACTOR'),
    ('USER'),
    ('ORG_UNIT')
ON CONFLICT (code) DO NOTHING;

-- Relationship types ---------------------------------------------------------
INSERT INTO relationship_type (code, label, source_kind, target_kind, inverse_label, description) VALUES
    ('CASE_HAS_DOCUMENT',          'Affaire contient document',   'CASE',     'DOCUMENT', 'Document appartient à affaire',   'Un document rattaché à une affaire'),
    ('CASE_CONCERNS_THING',        'Affaire concerne objet',      'CASE',     'THING',    'Objet concerné par affaire',     'Objet métier ou territorial concerné par une affaire'),
    ('CASE_HAS_ACTOR_REQUESTER',   'Affaire a demandeur',         'CASE',     'ACTOR',    'Acteur demandeur dans affaire',  'Acteur jouant le rôle de demandeur'),
    ('CASE_HAS_ACTOR_MANDATEE',    'Affaire a mandataire',        'CASE',     'ACTOR',    'Acteur mandataire dans affaire', 'Acteur jouant le rôle de mandataire'),
    ('DOCUMENT_REPRESENTS_THING',  'Document représente objet',   'DOCUMENT', 'THING',    'Objet représenté par document',  'Un plan ou une photo représentant un objet'),
    ('DOCUMENT_AUTHORED_BY_ACTOR', 'Document a auteur acteur',    'DOCUMENT', 'ACTOR',    'Acteur auteur de document',      'Acteur ayant produit le document'),
    ('DOCUMENT_SENT_TO_ACTOR',     'Document envoyé à acteur',    'DOCUMENT', 'ACTOR',    'Acteur destinataire de document','Acteur destinataire du document'),
    ('DOCUMENT_PREVIOUS_VERSION',  'Version précédente du document','DOCUMENT','DOCUMENT', 'Version suivante du document',    'Chaîne de versions entre documents'),
    ('THING_CONTAINS_THING',       'Objet contient objet',        'THING',    'THING',    'Objet contenu par objet',        'Composition territoriale ou métier'),
    ('CASE_PARENT_OF_CASE',        'Affaire parente affaire',     'CASE',     'CASE',     'Affaire enfant de affaire',      'Hiérarchie entre affaires')
ON CONFLICT (code) DO NOTHING;

-- Document types -------------------------------------------------------------
INSERT INTO document_type (code, label, category) VALUES
    ('INCOMING_LETTER', 'Courrier entrant',   'ENTREE'),
    ('OUTGOING_LETTER', 'Courrier sortant',   'SORTIE'),
    ('PLAN',            'Plan',               'PLAN'),
    ('PHOTO',           'Photographie',       'JUSTIFICATIF'),
    ('FORM',            'Formulaire',         'ENTREE'),
    ('REPORT',          'Rapport',            'JUSTIFICATIF'),
    ('DECISION',        'Décision',           'DECISION')
ON CONFLICT (code) DO NOTHING;

-- migrate:down

DELETE FROM document_type WHERE code IN
    ('INCOMING_LETTER','OUTGOING_LETTER','PLAN','PHOTO','FORM','REPORT','DECISION');
DELETE FROM relationship_type WHERE code IN
    ('CASE_HAS_DOCUMENT','CASE_CONCERNS_THING','CASE_HAS_ACTOR_REQUESTER','CASE_HAS_ACTOR_MANDATEE',
     'DOCUMENT_REPRESENTS_THING','DOCUMENT_AUTHORED_BY_ACTOR','DOCUMENT_SENT_TO_ACTOR','DOCUMENT_PREVIOUS_VERSION',
     'THING_CONTAINS_THING','CASE_PARENT_OF_CASE');
DELETE FROM subject_kind WHERE code IN ('CASE','DOCUMENT','THING','ACTOR','USER','ORG_UNIT');
