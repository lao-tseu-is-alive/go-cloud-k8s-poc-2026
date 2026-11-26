# Goéland POC demo script (synthetic data)

Status: **in progress** (GLD-079, see [ROADMAP.md](ROADMAP.md)). `goeland-server demo-seed`
(`pkg/demodata`) writes the catalogue, the org chart, the demo users and the six hero cases
(profile `small`, §7); the background data (profile `demo`) comes next. Decided 2026-11-26.

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
- **Reproducible:** the same content, dates and case references on every run; only the internal
  identifiers are drawn by the server. Each story gives its cases a reference in its year and type
  namespace (« OPC 2025-000031 »); the counters continue after them.
- **Through the API:** the generator sends every request as one of the demo users, so the data
  passes the same validation, rights, audit, outbox and timeline rules as the application, and
  every audit trail shown in the demo is a real one.
- **Story time:** each step of a story starts at a story time (`at("2025-03-03 09:10")`, Europe/
  Zurich), with realistic delays between steps. The API never backdates, so the server stamps
  everything at the time of the generation; `demo-seed` records when each step started and, once
  the demo is written, re-dates the whole database to the story times in one transaction, as the
  schema owner, the tables' triggers disabled for that transaction only (`demo_clock.go`): opening
  and closing times, audit, timeline, tasks, versions, then the retention dates derived from the
  closing. This happens only in the brand-new database it has just generated, before marking it
  as a demo; the server itself has no such path. The fictional municipality sets up its catalogue
  in January 2014.
- **A permanent banner** « Données fictives de démonstration » (GLD-081), so a screenshot is never
  mistaken for real records.
- **One language per demo:** the generator takes the language of the generated data (`--lang`,
  `fr` by default; `en` with GLD-079, `de` with GLD-083, `es` with GLD-084). Every text is in a
  catalogue per language; the cases, links, dates and seed are the same in every language. The
  French labels quoted in this script are those of the `fr` catalogue.
- **Photographs and places live in the repository** (`pkg/demodata`): the AI-generated
  photographs in `assets/` (their prompts in `assets/README.md`; no real photograph, person or
  brand) and the positions of the things in `geodata/things.geojson`, which anyone can edit with
  geojson.io or QGIS (`geodata/README.md`). The PDF documents are generated in the demo language.
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
| Emma | agent | Énergie | confidential subsidy (§4.4) |
| Félix | agent | Cohésion sociale | complaint (§4.3); does not see the police note |
| Hugo | agent | Parcs et promenades | answers the circulation of §4.1 |

Bruno, a member of Urbanisme (whose cases its members manage), approves the disposal item of §4.5:
an item is approved by a holder of MANAGE on the case's owning unit, never by the preparer.

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
- **Things:** the street section in front of number 12 and the building (owned by Lacrégie SA).
- **Story** (PLA 2025-000118):
  1. 15 September 2025: M. Tutu's letter, with two photographs: litter, syringes, a pavement
     hardly ever cleaned.
  2. 16 September: circulation to three services:
     - Voirie et propreté notes the facts (18 September); a task « nettoyage renforcé
       hebdomadaire » goes to the unit;
     - Police municipale reports on its patrols (23 September). The demo has no police account:
       Chloé, managing the case, records the answer the police sent her and files its report as an
       internal note in a **restricted** timeline entry;
     - Cohésion sociale (Félix, 25 September) sends its outreach social workers.
  3. 2 October: a coordinated reply letter; case closed.
  4. 17 November: M. Tutu writes again: « rien n'a changé ». The case is **reopened with a
     reason**, an on-site check is done on the 20th, a second reply is sent on the 24th and the
     case closed on the 25th.
- **Also:** M. Tutu's actor page shows an older complaint (PLA 2024-000052, June 2024, noise from
  a bar, owned by the police), closed.
- **Show:** the lifecycle with a reopening, several services on one case, Chloé's « mes tâches »,
  restricted visibility (Félix does not see the police note), a citizen's history on the actor page.

### 4.4 Confidential energy subsidy « Pompe à chaleur, chemin des Vignes 4 » (Emma)

- **Actors:** M. Marc and Mme Anne Berger (applicants); the house is a thing of the case.
- **Story** (SUB 2025-000009, August to November 2025): request form with the installer's quote
  and the income certificate (18 August), check (25 August), subsidy promise (10 September),
  photograph of the installed heat pump and invoice (3 November), payment letter (10 November),
  case closed (12 November). Emma read the case again in March 2026, so the access log already
  holds a read.
- **Access:** confidentiality level 2 (the case type's minimum): its creator and the Énergie unit
  only; Alice, although administrator, does not read it (an application role never opens a
  confidential subject) but consults its audit.
- **Show:** Bruno finds it neither in the case list nor in the global search; Emma opens it; Alice
  then reads the access log (« Consultations ») and sees Emma's read; on an ordinary case, any
  reader sees who consulted it.

### 4.5 Closed case due for disposal « Réclame, Café du Lac (2014) » (David)

- **Cases:** REC 2014-000023, the Café du Lac sign, authorized in April 2014 and closed in January
  2015 under the 10-year destruction policy, so due since January 2025; next to it REC 2014-000031,
  the Garage du Port sign, closed in February 2015 and due as well, but kept by a **legal hold**
  David placed in June 2025 (a dispute on its brightness). The generator stops there: the steps
  below are done live.
- **Story:**
  1. David finds the case in the due list (the one under hold is not offered).
  2. He prepares an elimination bordereau.
  3. Bruno (Urbanisme) approves the item from « mes tâches ».
  4. David runs the **simulation** and reads its counts. The real elimination stays disabled in the
     demo (`GOELAND_DISPOSAL_ENABLED` unset).
- **Show:** records governance end to end without destroying anything.

### 4.6 Citizen report « Lampadaire éteint, avenue des Cèdres » (Chloé)

- **Story:** on Monday 9 February 2026 Mme Sofia Keller's e-mail arrives with a photo (the
  original e-mail is filed as an incoming letter); a task goes to the unit Éclairage public, which
  replaces the lamp on Thursday 12; Chloé closes the case on Friday 13.
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

## 7. Generating the demo

From the repository root. The simplest is a demo settings file: copy `.env_demo.sample` to a
Git-ignored `.env_demo_<profile>_<lang>` file (e.g. `.env_demo_small_fr`), which names the demo
database, the language and a separate document directory, then:

```bash
scripts/demo_seed.sh -e .env_demo_small_fr
scripts/demo_seed.sh -r -e .env_demo_small_fr   # regenerate: drop the previous demo first
scripts/demo_seed.sh goeland_demo en            # without a file: database and language as arguments
scripts/demo_seed.sh -e .env_demo_small_fr other_db en   # arguments win over the file
```

Settings come from the environment, then from the `-e` file, then from `.env`: the demo file
only holds what differs and **no secret** (the database role and its password stay in `.env`).
The database is the argument, else `DB_NAME` of the `-e` file itself, never the development
database of `.env`; the language is the argument, else `GOELAND_DEMO_LANG`, else `fr`.

`demo-seed` must connect as the schema owner (the role that ran the migrations, `DB_USER` with the
script) to re-date the history it generated. The script creates the database when it does not
exist, owned by the application role
(`DB_USER`), with the extensions enabled as the PostgreSQL superuser (`GOELAND_DEMO_ADMIN_URL`,
by default `postgres@DB_HOST:DB_PORT`), then runs `goeland-server demo-seed`, which migrates it
and writes the demo. An existing database is used as it is, but the generator only writes into
a **brand-new** one: it refuses a database holding any subject. To generate it again (a newer
generator, another language), `-r` first drops the database, only when it carries the demo marker
`demo-seed` writes (`demo_dataset`, see below; any other database is refused), and the documents
of the file store when they are under `./demo_documents`. Serve the demo with the same blob store
settings, so its documents are found.

The generator replaces the authentication: the demo users get tokens drawn for the run, used
in-process only (it never listens on a network), and Alice (user id 1) is bootstrapped as
administrator.

To look at the demo, serve it with the same settings: `-s` creates and writes nothing, it checks
that the database exists and runs the server on it (`DB_NAME` set, `DATABASE_URL` unset).

```bash
scripts/demo_seed.sh -s -e .env_demo_small_fr
```

`.env_demo.sample` serves it in the **demo mode** (`GOELAND_AUTH_MODE=demo`, GLD-085): the
sign-in screen lists the demo users below with their role and what each shows, one click signs in,
« Changer d'utilisateur » in the user menu brings the list back (the browser remembers the demo user
picked, so a reload or a new tab keeps it; only its key, the demo tokens being public), a banner says on every screen that
the data is fictional, and the interface opens in the language of the generated data. Nobody types
a token: the demo users' tokens are public on purpose. Because anyone can sign in, the server
refuses this mode unless the database holds the marker `demo-seed` writes once it has generated
the demo into a brand-new database (`demo_dataset`, migration `0040`): a database holding real data
can never be served this way.

| User | Id | Role |
| --- | --- | --- |
| Alice Martin | 1 | administrator |
| Bruno Favre | 2 | agent, Urbanisme |
| Chloé Bonvin | 3 | agent, Voirie et propreté |
| David Perret | 4 | records manager, Archives communales |
| Emma Jaquet | 5 | agent, Énergie |
| Félix Morel | 6 | agent, Cohésion sociale |
| Hugo Delessert | 7 | agent, Parcs et promenades |

The server applies pending migrations at startup, as locally: a newer POC updates the schema of
the demo database, never its data.
