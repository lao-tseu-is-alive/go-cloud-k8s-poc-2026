# Goéland POC demo script (synthetic data)

Status: **specification, not built yet.** The data this script walks through will be written by
the synthetic dataset generator (GLD-079, see [ROADMAP.md](ROADMAP.md)); until then nothing here
exists in a database. Decided 2026-11-26.

This document has two readers:

- the **generator** (GLD-079): the hero cases below are its specification, case by case;
- the **presenter**: the same cases, in order, are the thread of the demo.

Labels quoted from the application are those of the French demo; the prose is in English like
every document of this repository.

## 1. Principles

- **Everything is fictional.** Every value is generated, never sampled or anonymized from real
  data. Only aggregate distributions already profiled (actor kind ratio, organization category
  histogram, contact-type mix) may shape the background data.
- **A fictional municipality on Swiss territory:** « Commune de Goéland-sur-Lac ». Geometries may
  come from Swiss open geodata (official survey parcels, building footprints) so they match the
  cadastral background map (swisstopo, GLD-080); their real identifiers and owners are never used.
- **Fictional identifiers:** parcel numbers, EGRID, EGID and IDE numbers have valid check digits in
  ranges that cannot be real ones; e-mails are `@example.ch`; phone numbers are in fictional ranges
  that still pass E.164 validation; names come from generic lists.
- **Reproducible:** fixed seed, UUIDv5 identifiers and **absolute dates** (2014 to 2026), so a reset
  gives back exactly the same cases with the same references, and the case due for disposal stays
  due.
- **Through the domain services:** the data passes the same validation, rights, audit, outbox and
  timeline rules as the application, so every audit trail shown in the demo is a real one.
- **A permanent banner** « Données fictives de démonstration » (GLD-081), so a screenshot is never
  mistaken for real records.
- **One language per demo:** the generator takes the language of the generated data (`--lang`,
  `fr` by default; `en` with GLD-079, `de` with GLD-083, `es` with GLD-084). Every text is in a
  catalogue per language; the cases, links, dates and seed are the same in every language. The
  French labels quoted in this script are those of the `fr` catalogue.
- **Third parties are not actors.** When a citizen writes about other people (the complaint of
  §4.3), they are never created as actors, and what the police reports about them is an internal
  note with restricted visibility.

## 2. Catalogue

A demo with hundreds of case types and document families is unreadable, so the catalogue is
reduced to what the hero cases need.

### Case types (8, plus the generic request)

| Type | What it shows |
| --- | --- |
| Demande de permis de construire | the full hero: map, roles, circulation, versions, decision |
| Opposition / recours | links between cases, incoming and outgoing letters |
| Autorisation d'abattage d'arbre | a point thing (tree) on the map, a short circulation |
| Procédé de réclame | small repetitive cases, volume for lists and filters; retention |
| Occupation du domaine public | a street thing, dates, tasks |
| Signalement citoyen | a one-off defect fixed by one task, then closed |
| Plaintes et requêtes | a recurring grievance followed by several services, reopened |
| Subvention énergétique | the confidential case: read filtering and sensitive read audit |
| Demande générique | everything else |

Each type has a retention policy, so the disposal circuit can be shown (§4.5).

### Document types (12)

Formulaire de demande, Courrier entrant, Courrier sortant, Plan, Photographie, Rapport,
Décision / autorisation, Préavis de service, Avis d'enquête publique, Opposition, Pièce
justificative (attestation, devis, facture), Note interne — filed in the existing categories
(entrée, sortie, plan, justificatif, décision).

## 3. Cast

### Organizational units

Urbanisme (police des constructions), Voirie et propreté (with the unit Éclairage public), Parcs et
promenades, Police municipale, Cohésion sociale, Énergie, Archives communales, Greffe.

### Demo users

| User | Role | Unit | Used in |
| --- | --- | --- | --- |
| Alice | administrator | Greffe | sensitive read audit (§4.4) |
| Bruno | agent | Urbanisme | building permit, opposition (§4.1, §4.2); does not see §4.4 |
| Chloé | agent | Voirie et propreté | complaint, citizen report (§4.3, §4.6) |
| David | records manager | Archives communales | disposal (§4.5) |
| Emma | reader | Énergie | confidential subsidy (§4.4) |
| Félix | agent | Cohésion sociale | complaint (§4.3); does not see the police note |

The head of Urbanisme (a user holding MANAGE on that unit) approves the disposal item of §4.5.

## 4. Hero cases

### 4.1 Building permit « Villa Les Tilleuls » (Bruno)

- **Actors:** Mme Claire Fontaine (owner), Atelier Molard SA (architect), Bâtisud Sàrl (contractor).
- **Things:** a parcel and an existing building, on the map.
- **Story:**
  1. Request form and plans (version 1).
  2. Public inquiry notice.
  3. Circulation for opinions to Voirie (access), Parcs (two trees on the boundary) and Énergie;
     each answer lands in the timeline.
  4. The Parcs opinion asks for a change: the architect files the plans in version 2.
  5. Decision granting the permit, outgoing letter, case closed.
- **Show:** the map, the roles grouped by kind, the circulation and its answers, the versions of a
  plan, the integrity verification of a document, the full timeline.

### 4.2 Opposition « Voisins c/ Villa Les Tilleuls » (Bruno)

- **Actor:** M. Paul Rochat (neighbour); the case is linked to §4.1.
- **Story:** opposition letter during the public inquiry, acknowledgement of receipt, conciliation
  meeting (internal note), opposition lifted in the permit decision, case closed with a reason.
- **Show:** the link between cases and the navigation between them, incoming and outgoing letters,
  a closing with its reason.

### 4.3 Complaint « Propreté et nuisances, rue du Port 12 » (Chloé, Félix)

- **Actors:** M. Jean Tutu (complainant), Lacrégie SA (property manager of the building).
- **Thing:** the street section in front of number 12.
- **Story:**
  1. M. Tutu writes: litter, syringes, a pavement hardly ever cleaned.
  2. Circulation to three services:
     - Voirie et propreté notes the facts; a task « nettoyage renforcé hebdomadaire » is created;
     - Police municipale reports on its patrols in an internal note with restricted visibility;
     - Cohésion sociale sends its outreach social workers.
  3. A coordinated reply letter; case closed.
  4. Six weeks later M. Tutu writes again: « rien n'a changé ». The case is **reopened with a
     reason**, a new task is created and a second reply letter is sent.
- **Also:** M. Tutu's actor page shows an older complaint (2024, noise from a bar), closed.
- **Show:** the lifecycle with a reopening, several services on one case, Chloé's « mes tâches »,
  restricted visibility (Félix does not see the police note), a citizen's history on the actor page.

### 4.4 Confidential energy subsidy « Pompe à chaleur, chemin des Vignes 4 » (Emma)

- **Actors:** M. and Mme Berger (applicants).
- **Documents:** request form, supporting documents (quote, income certificate), granting decision.
- **Access:** confidentiality level 2, granted to Énergie only.
- **Show:** Bruno finds it neither in the case list nor in the global search; Emma opens it; Alice
  then reads the sensitive read audit and sees Emma's read.

### 4.5 Closed case due for disposal « Réclame, Café du Lac (2014) » (David)

- **Case:** an advertising sign authorization closed in 2015 whose policy destroys it after 10
  years; next to it, a similar case under a **legal hold**.
- **Story:**
  1. David finds the case in the due list (the one under hold is not offered).
  2. He prepares an elimination bordereau.
  3. The head of Urbanisme approves the item from « mes tâches ».
  4. David runs the **simulation** and reads its counts. The real elimination stays disabled in the
     demo (`GOELAND_DISPOSAL_ENABLED` unset).
- **Show:** records governance end to end without destroying anything.

### 4.6 Citizen report « Lampadaire éteint, avenue des Cèdres » (Chloé)

- **Story:** report with a photo, a task assigned to the unit Éclairage public, completed the same
  day, case closed.
- **Show:** the shortest case, in contrast with the complaint of §4.3.

## 5. Background data

Generated around the heroes so lists, filters, search and counts mean something (profile `demo`,
orders of magnitude): about 300 actors, 400 cases spread over the 8 types, 1,500 documents and
200 things, including a tree to fell and a café terrace on the public domain. The profile `small`
keeps the heroes and a minimal background for the tests.

## 6. Suggested demo order

1. Log in as Bruno: the building permit (§4.1), then the opposition through the case link (§4.2).
2. As Chloé: « mes tâches », the complaint (§4.3) and its reopening; the citizen report (§4.6).
3. As Félix: the same complaint without the police note.
4. As Bruno, then Emma, then Alice: the confidential subsidy and its audit (§4.4).
5. As David: the due list, the bordereau and the simulation (§4.5).
