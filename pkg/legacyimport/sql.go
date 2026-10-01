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
