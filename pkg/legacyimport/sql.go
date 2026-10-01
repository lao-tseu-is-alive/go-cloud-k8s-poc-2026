package legacyimport

// --- target -------------------------------------------------------------------------------

// targetUsedSQL tells whether the target already holds an import or a case.
const targetUsedSQL = `SELECT EXISTS (SELECT 1 FROM import_batch) OR EXISTS (SELECT 1 FROM case_file);`

const insertBatchSQL = `
INSERT INTO import_batch (source_system, snapshot_at, started_by)
VALUES (@source_system, @snapshot_at, @started_by)
RETURNING id;`

const finishBatchSQL = `UPDATE import_batch SET finished_at = now(), counts = @counts WHERE id = @id;`

const insertRelationshipTypeSQL = `
INSERT INTO relationship_type (code, label, source_kind, target_kind, is_directed, inverse_label, description)
VALUES (@code, @label, @source_kind, @target_kind, true, @inverse_label, @description)
RETURNING id;`

// createThingStagingSQL holds the thing rows with their location as EWKT text.
const createThingStagingSQL = `
CREATE TEMP TABLE import_thing (
    id UUID, thing_type_id UUID, name TEXT, description TEXT, external_ref TEXT, geom TEXT,
    metadata JSONB, created_at TIMESTAMPTZ, created_by TEXT
) ON COMMIT DROP;`

const insertThingsFromStagingSQL = `
INSERT INTO thing (id, thing_type_id, name, description, external_ref, geom, metadata, created_at, created_by)
SELECT id, thing_type_id, name, description, external_ref, ST_GeomFromEWKT(geom), metadata, created_at, created_by
FROM import_thing;`

const insertLegacyDocumentTypeSQL = `
INSERT INTO document_type (code, label, description, category)
VALUES (@code, 'Document Goéland', 'Documents importés de Goéland (le type legacy est un format de fichier)', 'Goéland')
RETURNING id;`

// setCurrentVersionsSQL points each imported document at its single version.
const setCurrentVersionsSQL = `
UPDATE document d SET current_version_id = v.id
FROM document_version v
WHERE v.document_id = d.id AND v.version_no = 1 AND d.current_version_id IS NULL
  AND d.id IN (SELECT subject_id FROM subject_provenance WHERE import_batch_id = @batch_id);`

// createDocumentGrantStagingSQL collects the computed and listed document
// grants before they are written once per grantee.
const createDocumentGrantStagingSQL = `
CREATE TEMP TABLE import_document_grant (
    subject_id UUID, grantee_kind TEXT, grantee_user_id TEXT, grantee_subject_id UUID,
    level SMALLINT, granted_by TEXT, grant_reason TEXT
) ON COMMIT DROP;`

const insertDocumentGrantsFromStagingSQL = `
INSERT INTO access_grant (subject_id, grantee_kind, grantee_user_id, grantee_subject_id, level, granted_by, grant_reason)
SELECT DISTINCT ON (subject_id, grantee_kind, coalesce(grantee_user_id, ''), grantee_subject_id)
       subject_id, grantee_kind, grantee_user_id, grantee_subject_id, level, granted_by, grant_reason
FROM import_document_grant
ORDER BY subject_id, grantee_kind, coalesce(grantee_user_id, ''), grantee_subject_id, level DESC;`

const createEntryFinalStagingSQL = `
CREATE TEMP TABLE import_entry_final (id UUID, status SMALLINT, at TIMESTAMPTZ, by TEXT) ON COMMIT DROP;`

// applyEntryFinalStatusSQL moves the drafts to their final status (an allowed
// transition: the guard only refuses changes to an entry no longer a draft).
const applyEntryFinalStatusSQL = `
UPDATE case_timeline_entry e
SET status = f.status,
    validated_at = CASE WHEN f.status = 2 THEN f.at END,
    validated_by = CASE WHEN f.status = 2 THEN f.by ELSE e.validated_by END,
    locked_at = CASE WHEN f.status = 3 THEN f.at END,
    locked_by = CASE WHEN f.status = 3 THEN f.by ELSE e.locked_by END
FROM import_entry_final f
WHERE f.id = e.id;`

// --- source (the legacy replica) -------------------------------------------------------------

// snapshotSQL dates the source data: the last change of a case (the instant T).
const snapshotSQL = `SELECT max(greatest(datecreated, datelastmodif)) FROM affaire;`

// employeesSQL reads what a user needs only: never birth date, private address or phones.
const employeesSQL = `
SELECT e.idemploye AS id, coalesce(btrim(e.prenom), '') AS first_name, coalesce(btrim(e.nom), '') AS last_name,
       lower(coalesce(btrim(e.email), '')) AS email, e.isactive AS active, e.datecreated AS created_at,
       (SELECT eo.idorgunit FROM employe__org_unit eo WHERE eo.idemploye = e.idemploye AND eo.levelou = 0 LIMIT 1) AS unit_id
FROM employe e
ORDER BY e.idemploye;`

const groupsSQL = `
SELECT g.idgroupe AS id, coalesce(btrim(g.groupname), '') AS name, coalesce(btrim(g.commentaire), '') AS description,
       coalesce(g.bactif, false) AS active
FROM securite_groupe g
ORDER BY g.idgroupe;`

const groupMembersSQL = `SELECT DISTINCT idgroupe, idemploye FROM securite_groupe_liste_employe;`

const nestedGroupsSQL = `SELECT count(*) FROM securite_groupe_liste_groupe;`

// caseTypesSQL reads the types with the share of confidential cases of each.
const caseTypesSQL = `
SELECT t.idtypeaffaire AS id, coalesce(btrim(t.name), '') AS name, coalesce(btrim(t.description), '') AS description,
       coalesce(t.isactive, false) AS active,
       coalesce((SELECT avg(CASE WHEN a.isconfidential THEN 1.0 ELSE 0.0 END) FROM affaire a WHERE a.idtypeaffaire = t.idtypeaffaire), 0)::float8 AS confidential_share
FROM type_affaire t
ORDER BY t.idtypeaffaire;`

const casesSQL = `
SELECT a.idaffaire AS id, a.idtypeaffaire AS type_id, coalesce(btrim(a.name), '') AS title,
       coalesce(a.description, '') AS description, coalesce(btrim(a.commentaire), '') AS comment,
       a.datecreated AS created_at, a.datelastmodif AS updated_at, a.datebegin AS begin_at, a.dateend AS end_at,
       coalesce(a.issuspended, false) AS suspended, coalesce(a.isterminated, false) AS terminated,
       a.idcreator AS creator_id, coalesce(a.isconfidential, false) AS confidential
FROM affaire a
ORDER BY a.idaffaire;`

// grantsSQL keeps one grant per case and grantee (E and e are the same
// employees), the strongest one (the lowest IdDroit).
const grantsSQL = `
SELECT DISTINCT ON (r.idaffaire, upper(r.emporou), r.idemporou)
       r.idaffaire AS case_id, upper(r.emporou) AS grantee_kind, r.idemporou AS grantee_id, r.iddroit AS right_id
FROM affaire_droit_emp_or_ou r
ORDER BY r.idaffaire, upper(r.emporou), r.idemporou, r.iddroit;`

// actorsSQL joins the person and organization specializations; the register
// link is a flag and an opaque reference only.
const actorsSQL = `
SELECT a.idacteur AS id, coalesce(a.isphysique, false) AS person, coalesce(btrim(a.name), '') AS name,
       coalesce(btrim(a.nameforsearch), '') AS name_for_search, coalesce(a.isactive, false) AS active,
       greatest(coalesce(a.codepublication, 0), 0) AS publication_code, a.datecreated AS created_at, a.idcreator AS creator_id,
       p.idtitre AS title_id, coalesce(btrim(p.lastname), '') AS last_name, coalesce(btrim(p.firstname), '') AS first_name,
       coalesce(btrim(m.raisonsociale), '') AS legal_name, coalesce(btrim(c.category), '') AS category,
       coalesce(btrim(m.complement), '') AS org_complement, ch.idch AS register_id
FROM acteur a
LEFT JOIN act_physique p ON p.idacteur = a.idacteur
LEFT JOIN act_moral m ON m.idacteur = a.idacteur
LEFT JOIN dico_act_moral_category c ON c.idcategory = m.idcategory
LEFT JOIN LATERAL (SELECT f.idch FROM act_phys_from_ch f WHERE f.idacteur = a.idacteur ORDER BY f.idch LIMIT 1) ch ON true
ORDER BY a.idacteur;`

const actorContactsSQL = `
SELECT c.idacteur AS actor_id, c.idtypecomplement AS type_id, coalesce(btrim(d.typecomplcomplet), '') AS type_label,
       coalesce(c.complement, '') AS value
FROM acteur_complement c
LEFT JOIN dico_acteur_type_complement d ON d.idtypecomplement = c.idtypecomplement
ORDER BY c.idacteur, c.idtypecomplement;`

// actorAddressesSQL reads the correspondence address the legacy keeps per actor.
const actorAddressesSQL = `
SELECT d.idacteur AS actor_id, coalesce(btrim(d.rue), '') AS street, coalesce(btrim(d.numero), '') AS house_number,
       coalesce(btrim(d.casepostale), '') AS postal_box, coalesce(btrim(d.codepostal), '') AS postal_code,
       coalesce(btrim(d.ville), '') AS locality, coalesce(btrim(d.pays), '') AS country
FROM acteur_adresse_corresp_denormalise d
ORDER BY d.idacteur;`

const countriesSQL = `SELECT btrim(nomfrancais), upper(btrim(codeiso)) FROM dico_pays WHERE codeiso ~ '^[A-Za-z]{2}$';`

const actorRolesSQL = `
SELECT d.idrole AS id, btrim(d.role) AS name FROM dico_acteur_role d
WHERE EXISTS (SELECT 1 FROM acteur_role r WHERE r.objecttablename = 'Affaire' AND r.idrole = d.idrole);`

const userRolesSQL = `SELECT idrole AS id, btrim(role) AS name FROM dico_employe_role;`

const unitRolesSQL = `SELECT idrole AS id, btrim(role) AS name FROM dico_org_unit_role;`

// caseActorRolesSQL keeps one edge per case, actor and role.
const caseActorRolesSQL = `
SELECT DISTINCT ON (r.idobject, r.idacteur, r.idrole)
       r.idobject AS case_id, r.idacteur AS party_id, r.idrole AS role_id, r.datecreated AS created_at,
       NULL::timestamp AS valid_from, NULL::timestamp AS valid_to
FROM acteur_role r
WHERE r.objecttablename = 'Affaire'
ORDER BY r.idobject, r.idacteur, r.idrole, r.datecreated;`

const caseUserRolesSQL = `
SELECT e.idaffaire AS case_id, e.idemploye AS party_id, e.idroleemp AS role_id, e.datecreation AS created_at,
       e.datebeginparticipate AS valid_from, e.dateendparticipate AS valid_to
FROM affaire_employe e
ORDER BY e.idaffaire, e.idemploye, e.idroleemp, e.dateendparticipate NULLS FIRST;`

const caseUnitRolesSQL = `
SELECT o.idaffaire AS case_id, o.idorgunit AS party_id, o.idroleou AS role_id, NULL::timestamp AS created_at,
       o.datebeginparticipate AS valid_from, o.dateendparticipate AS valid_to
FROM affaire_org_unit o
ORDER BY o.idaffaire, o.idorgunit, o.idroleou, o.dateendparticipate NULLS FIRST;`

const thingTypesSQL = `
SELECT t.idtypething AS id, coalesce(btrim(t.name), '') AS name, coalesce(btrim(t.description), '') AS description,
       coalesce(t.isactive, false) AS active
FROM type_thing t
ORDER BY t.idtypething;`

// thingsSQL reads the things with their extent, parcel and building details.
const thingsSQL = `
SELECT t.idthing AS id, t.idtypething AS type_id, coalesce(btrim(t.name), '') AS name,
       coalesce(btrim(t.description), '') AS description, t.datecreated AS created_at, t.idcreator AS creator_id,
       p.mineo AS min_e, p.maxeo AS max_e, p.minsn AS min_n, p.maxsn AS max_n,
       pa.idcommune AS commune, coalesce(btrim(pa.numparcelle), '') AS parcel_number,
       coalesce(upper(btrim(pa.egrid)), '') AS egrid, pa.surface AS surface,
       (SELECT min(e.egid) FROM thi_building_egid e WHERE e.idthing = t.idthing) AS egid
FROM thing t
LEFT JOIN thing_position p ON p.idthing = t.idthing
LEFT JOIN parcelle pa ON pa.idthing = t.idthing AND t.idtypething = 3
ORDER BY t.idthing;`

// documentsSQL reads the documents with the metadata of their current content
// (scanDocument order); the media type comes from the legacy file format.
const documentsSQL = `
SELECT d.iddocument, coalesce(btrim(d.doctitle), ''), coalesce(d.docdescription, ''), coalesce(btrim(d.docsubject), ''),
       coalesce(btrim(d.doccomment), ''), d.docdateofficielle, d.datecreated, d.datelastmodif, d.iduserpost,
       coalesce(d.docisdefinitive, false), coalesce(d.docisconfidential, false), coalesce(d.doclevelconfidential, 0),
       coalesce(btrim(d.sha256hash), ''), coalesce(d.docsizeinbyte, 0)::bigint, coalesce(d.nbrpage, 0),
       coalesce(btrim(t.mimetype), ''), coalesce(btrim(d.locfilename), '') || coalesce(btrim(d.locfileext), ''),
       coalesce(d.docnumver, 1)
FROM document d
LEFT JOIN type_document t ON t.idtypedocument = d.idtypedocument
ORDER BY d.iddocument;`

const documentGrantsSQL = `
SELECT d.iddocument, d.iduserpost, coalesce(d.doclevelconfidential, 0)
FROM document d
WHERE d.docisconfidential AND d.iduserpost IS NOT NULL
ORDER BY d.iddocument;`

// documentAccessListsSQL reads the access lists of the confidential documents
// as (document, E/G/O, grantee).
const documentAccessListsSQL = `
SELECT a.iddocument, 'E', a.idemploye FROM document_employe_acces a JOIN document d ON d.iddocument = a.iddocument AND d.docisconfidential
UNION ALL
SELECT a.iddocument, 'G', a.idgroupe FROM document_groupe_acces a JOIN document d ON d.iddocument = a.iddocument AND d.docisconfidential
UNION ALL
SELECT a.iddocument, 'O', a.idorgunit FROM document_org_unit_acces a JOIN document d ON d.iddocument = a.iddocument AND d.docisconfidential;`

const timelineEntriesSQL = `
SELECT s.idaffairesuivi, s.idaffaire, s.idcreator, coalesce(s.commentaire, ''), s.dateofficielle, s.datecreated,
       coalesce(btrim(s.color), '')
FROM affaire_suivi s
ORDER BY s.idaffairesuivi;`

const timelineDocumentsSQL = `SELECT idaffairesuivi, iddocument FROM lien_affaire_suivi_document;`

// timelineStatusSQL reads the validation and the first lock of each follow-up.
const timelineStatusSQL = `
SELECT s.idaffairesuivi, s.idaffaire, s.datevalidation, s.idvalideur,
       CASE WHEN v.idaffairesuivi IS NOT NULL THEN coalesce(v.dateverrou, s.datecreated) END, v.idemploye
FROM affaire_suivi s
LEFT JOIN LATERAL (
    SELECT x.idaffairesuivi, x.dateverrou, x.idemploye FROM affaire_suivi_verrou x
    WHERE x.idaffairesuivi = s.idaffairesuivi ORDER BY x.dateverrou NULLS LAST LIMIT 1) v ON true;`

const caseThingsSQL = `SELECT idaffaire, idthing FROM lien_thing_affaire;`

const caseDocumentsSQL = `SELECT idaffaire, iddocument FROM lien_affaire_document;`

const thingDocumentsSQL = `SELECT iddocument, idthing FROM lien_thing_document;`

// parentCasesSQL: a "Parent" row says that case 1 is the parent of case 2 (it
// is the older one in 86% of the rows); the "Enfant" rows are its mirror.
const parentCasesSQL = `
SELECT l.idaffaire1, l.idaffaire2 FROM lien_affaire_affaire l
JOIN dico_type_lien_affaire_affaire t ON t.id = l.idtypelien12
WHERE t.typelien = 'Parent' AND l.idaffaire1 <> l.idaffaire2;`

// relatedCasesSQL: a "Lien" is stored in both directions (kept once), a "Lien
// unidirectionnel" in its own direction.
const relatedCasesSQL = `
SELECT least(l.idaffaire1, l.idaffaire2), greatest(l.idaffaire1, l.idaffaire2) FROM lien_affaire_affaire l
JOIN dico_type_lien_affaire_affaire t ON t.id = l.idtypelien12
WHERE t.typelien = 'Lien' AND l.idaffaire1 <> l.idaffaire2
UNION
SELECT l.idaffaire1, l.idaffaire2 FROM lien_affaire_affaire l
JOIN dico_type_lien_affaire_affaire t ON t.id = l.idtypelien12
WHERE t.typelien = 'Lien unidirectionnel' AND l.idaffaire1 <> l.idaffaire2;`

const thingActorRoleNamesSQL = `
SELECT d.idrole AS id, btrim(d.role) AS name FROM dico_acteur_role d
WHERE EXISTS (SELECT 1 FROM acteur_role r WHERE r.objecttablename = 'Thing' AND r.idrole = d.idrole);`

const thingActorRolesSQL = `
SELECT DISTINCT ON (r.idobject, r.idacteur, r.idrole)
       r.idobject, r.idacteur, r.idrole, r.datecreated, NULL::timestamp, NULL::timestamp
FROM acteur_role r
WHERE r.objecttablename = 'Thing'
ORDER BY r.idobject, r.idacteur, r.idrole, r.datecreated;`

const documentActorRoleNamesSQL = `
SELECT d.idrole AS id, btrim(d.role) AS name FROM dico_acteur_role d
WHERE EXISTS (SELECT 1 FROM acteur_role r WHERE r.objecttablename = 'Document' AND r.idrole = d.idrole);`

const documentActorRolesSQL = `
SELECT DISTINCT ON (r.idobject, r.idacteur, r.idrole)
       r.idobject, r.idacteur, r.idrole, r.datecreated, NULL::timestamp, NULL::timestamp
FROM acteur_role r
WHERE r.objecttablename = 'Document'
ORDER BY r.idobject, r.idacteur, r.idrole, r.datecreated;`
