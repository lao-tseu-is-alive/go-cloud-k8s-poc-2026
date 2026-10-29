# Retention and final disposition of cases

Reference for records managers (archivistes), data protection officers, case managers and
operators. It states what Goéland does with a case once it is closed, who decides what, what is
kept as proof, and what is not built yet. Roadmap: GLD-076 (retention, holds, transfer) and
GLD-077 (elimination, rebuilding a transfer package). Spec: v2 §35-36.

> **Status.** Retention, legal holds, disposition bordereaux, the transfer to the archives and
> the **elimination** are built. An elimination is **irreversible**: it erases the content of the
> approved cases and destroys the files only they hold (§5b). It is **off by default**: a server
> only simulates eliminations until its operators set `GOELAND_DISPOSAL_ENABLED=true` (§5c). The
> three retention policies shipped are **examples**: the real schedule must come from the City
> archives before any use on real cases.

## 1. Vocabulary

| Goéland (fr-CH UI) | Meaning |
|---|---|
| Politique de conservation (*retention policy*) | An entry of the retention schedule: how many years a case is kept after its closing, its legal basis and its final disposition. |
| Sort final (*final disposition*) | What happens when the retention ends: **versement** (transfer to the archives), **élimination** (destruction) or **tri** (the records manager decides case by case). |
| Conservation (*retention*) | The retention of one closed case: the policy applied, its start (closing day) and its end. |
| Échéance (*deadline*) | The day the retention ends; from that day the case is due for its final disposition. |
| Gel (*legal hold*) | A block placed on a case (litigation, inquiry...): it cannot enter a bordereau while the hold lasts. |
| Bordereau | A list of due cases of one final disposition, prepared by a records manager, approved case by case, then executed. |
| Archiviste (*records manager*) | The application role `RECORDS_MANAGER`. |

## 2. Lifecycle of a case

```text
 OPEN / IN_PROGRESS / SUSPENDED
        │ closing (the case type has a policy)
        ▼
 CLOSED, retention running ──── reopening ───▶ retention ended (kept as history)
        │ deadline reached                       (closing again starts a new one)
        ▼
 DUE ── legal hold ──▶ held (not due for disposition until released)
        │ put in a bordereau by a records manager
        ▼
 PROPOSED ── rejected by the approver ──▶ back to DUE
        │ approved by the owning unit's manager
        ▼
 executed:  ARCHIVE ──▶ ARCHIVED: transfer package built, case frozen
            DESTROY ──▶ DISPOSED: notice kept, content erased, files destroyed
```

The case is the unit of retention. Its documents follow it; actors and things (reference data)
have no retention of their own yet.

## 3. Rules

**Policies.** Administrators manage the schedule (Administration → Politiques de conservation):
code (immutable), label, legal basis, duration in years (0-200), final disposition. Entries are
never deleted, only deactivated; every change is logged in the reference change log. A case type
names its default policy (Administration → Types d'affaire); only an active policy can be chosen.

**Computation.** When a case is closed, in the same transaction:
- the retention starts on the closing day (Europe/Zurich) and ends *duration* years later;
- it is a **snapshot** of the policy: changing the policy later does not change the retentions
  already running, only those of cases closed afterwards;
- a case whose type has no policy gets no retention ("Clôturée sans politique de conservation").

Reopening a case ends its retention (kept as history with the reason); closing it again computes
a new one. For cases closed before their type had a policy (imported data in particular),
administrators apply the type's policy from the Conservation page, at most 1000 cases per call,
each audited; the page reports how many remain.

**Legal hold.** A records manager places a hold with a reason and releases it with a reason; one
active hold per case; holds are kept as history. A held case leaves the deadline list (it can be
shown on request) and cannot be put in a bordereau. A hold placed after an approval blocks the
execution of the bordereau until the case is rejected or the hold released.

**Bordereau.** A records manager selects due cases (Conservation → Échéances) and prepares a
bordereau with a reason. A case enters only if: its retention has ended, it is not on hold, it is
in no other open bordereau, and its policy's disposition is the bordereau's (a *tri* policy fits
either). A bordereau holds 1 to 500 cases.

**Approval.** Each case is approved or rejected, with a reason, by a holder of the **Gérer**
(MANAGE) right on the case's owning unit; for a case without a live owning unit, by a holder of
MANAGE on the case itself. **The preparer of a bordereau never approves it.** Approvers find the
cases waiting for them on "Mes tâches".

**Execution.** A records manager executes the bordereau once every case is decided; only the
approved cases are handled. For a transfer (ARCHIVE), for each approved case:
1. the complete transfer package is queued (see §5);
2. the transfer is recorded as EXPORT in the sensitive read audit;
3. the case is **frozen** (locked: it can no longer change nor be reopened);
4. its retention is marked ARCHIVED.

For an elimination (DESTROY), see §5b; it runs only on a server where it is enabled (§5c). The
bordereau is then closed with its proof (§4). An open bordereau can be cancelled with a reason;
its cases become due again. The execution page asks for an explicit confirmation before an
elimination.

## 4. Proof

An executed bordereau keeps a **manifest** (JSON) and the **SHA-256** of exactly those bytes. The
manifest lists the bordereau (id, disposition, reason, preparer and date, executor and date) and,
for every case, its reference, type, policy, retention start and end, the decision, its approver,
date and reason, and the transfer package id. Both are stored forever in
`disposition_bordereau` and shown on the bordereau page, where the manifest can be saved.

To check a proof: save the manifest, then `sha256sum bordereau-<id>-manifest.json` must print
the SHA-256 shown on the page. Each transfer package also shows its own SHA-256.

The case keeps its own trail in its audit log (`audit_event`, append-only):
`RETENTION_STARTED`, `RETENTION_ENDED`, `RETENTION_HOLD_PLACED`, `RETENTION_HOLD_RELEASED`,
`RETENTION_DISPOSITION_PROPOSED`, `RETENTION_DISPOSITION_APPROVED`, `RETENTION_DISPOSITION_REJECTED`,
`RETENTION_DISPOSITION_WITHDRAWN` (cancelled bordereau), `RETENTION_ARCHIVED`,
`RETENTION_TRANSFER_REBUILT`, `RETENTION_DISPOSED`. After an elimination these events stay (who,
when, what) but their states and reasons are erased (§5b), except `RETENTION_DISPOSED`.

## 5. Transfer package

The package has the format of the case export (GLD-029): a ZIP with `manifest.json` (the case,
its timeline, tasks, circulations, relationships, documents with all their versions, audit
summary) and every document version's bytes once under `blobs/<sha256>`. Unlike an ordinary
export, it is built **by the system without read filter**: it holds the whole case, confidential
content included. Therefore:
- only records managers (and administrators) download it, from the bordereau page;
- **every download is recorded** as DOWNLOAD in the sensitive read audit;
- it stays downloadable **90 days**, then its bytes are removed like any export.

The handover to the archives is manual: a records manager downloads each package and delivers it
(with the bordereau manifest) according to the archives' procedure. A package that failed or
expired is rebuilt from the bordereau page (Reconstruire le paquet): a new package is queued,
recorded as EXPORT and audited (`RETENTION_TRANSFER_REBUILT`); the case stays frozen.

## 5b. Elimination

Executing an approved DESTROY bordereau, for each approved case, in one transaction:

| Kept | Erased |
|---|---|
| The case as a **notice**: id, kind, type, business reference, dates (opening, closing, retention), owning unit, relationships (which subjects it was linked to) | Its title (replaced by « Affaire éliminée <référence> »), description, closure reason, metadata |
| The timeline entries, tasks, circulations and answers as rows (type, status, dates, people) | Their titles, bodies, descriptions, notes, reasons, answers and metadata |
| Its audit events: type, author, date | Their before/after states and reasons (replaced by `{"eliminated": true}`) |
| The bordereau, its manifest and SHA-256 (§4), the `RETENTION_DISPOSED` event | The ordinary exports of the case (archives removed) |

Its **documents** go with it unless still linked to a **thing** or to a **case that stays** (not
in this elimination, not already eliminated): such a document is kept untouched. A document that
goes is blanked (« Document éliminé », no description, metadata or external reference), its
version metadata erased, and it is logically deleted (it leaves every list; its content is no
longer served). Its **files** are destroyed when no other document version references them; a
file shared with a kept document stays. The bytes are deleted from the blob store just after the
transaction commits; should a deletion fail, the server logs the storage reference to remove by
hand (the database already records the file as destroyed). Uploading the same file again later
stores it anew.

The case is frozen and its retention marked DISPOSED. The protected tables (audit log, validated
timeline entries, validated or record document versions) accept this erasure only inside the
elimination transaction (`SET LOCAL goeland.disposal = 'on'`, migration `0037`); outside it they
stay append-only or immutable, and deletions are always refused.

What an elimination does **not** reach: the bordereau's own manifest (it names the case's
reference, type and the approvers' reasons, as proof), the transfer packages already handed over,
backups of the database and of the blob store (their retention is an operations matter), the
access log (`access_audit_event`, references only), and the outbox (references only).

## 5c. Enabling the elimination; simulation

Destroying records is a decision of the deployment, not a default. The server reads
`GOELAND_DISPOSAL_ENABLED`:

| Value | Effect |
|---|---|
| unset, empty or `false` (default) | Elimination **disabled**: executing a DESTROY bordereau is refused (`FAILED_PRECONDITION`); it can only be simulated. Transfers (ARCHIVE) are not affected. |
| `true` (exactly) | Elimination **enabled**: an approved DESTROY bordereau is executed as in §5b. |
| anything else (`1`, `yes`, `True`, a typo) | The server **does not start**: a misconfiguration is never guessed. |

The server logs its mode at startup (a warning when the elimination is enabled), and the bordereau
page says when it is disabled.

**Simulation.** On a fully decided DESTROY bordereau, « Simuler l'élimination » (`POST
/api/disposition-bordereaux/{id}/simulate`, records managers) runs the whole execution in a
transaction that is rolled back, and reports for each approved case the documents that would be
eliminated and kept, the files that would be destroyed and the exports that would be removed.
Nothing is written (neither the data nor the audit) and no file is touched: bytes are only deleted
after a commit. The simulation is available whether the elimination is enabled or not; it fails
for the same reasons as the execution would (a legal hold placed since the approval, for example).

Recommended procedure: run the first bordereaux on the production data with the elimination
disabled, check their simulations with the archives, then enable it.

## 6. Roles and rights

| Who | May |
|---|---|
| Administrator (`ADMIN`) | Manage the policies and the case types' default policy; apply a type's policy to its closed cases; everything a records manager may (the administrator scope satisfies every check). |
| Records manager (`RECORDS_MANAGER`, scope `goeland:records`) | Read every non-confidential case; read the retention (dates, policy, holds) of any case, confidential ones included; place and release holds; list deadlines; prepare, execute and cancel bordereaux; download transfer packages. |
| Manager of the owning unit, or of the case without one (MANAGE) | Approve or reject the cases of a bordereau, except one they prepared. |
| Anyone who may read a case | See its retention on the case page (Gouvernance). |

The role is granted like ADMIN (Administration → Rôles), with a reason, audited on the user; a
token claiming the scope is ignored. In lists, the title of a case the caller may not read is left
out; its reference stays visible.

## 7. Operations

- Before use on real cases: replace the example policies (`EXAMPLE_ARCHIVE_10`,
  `EXAMPLE_DESTROY_5`, `EXAMPLE_REVIEW_10`) with the archives' schedule and give each case type its
  policy; then apply it to the already closed cases.
- Leave `GOELAND_DISPOSAL_ENABLED` unset until the archives have validated the schedule and the
  simulations of the first bordereaux (§5c); then set it to `true` explicitly.
- Grant `RECORDS_MANAGER` to the archivists; make sure each case has an owning unit whose
  managers can approve (otherwise the case's own managers approve).
- Transfer packages are written to the blob store (`GOELAND_DOCUMENT_PATH`) like exports; size the
  storage for 90 days of packages.
- Metrics: the export backlog (`goeland_case_export_jobs`) includes the packages being built.

## 8. Known limitations

- **Backups keep eliminated data** until they expire: align the backup retention of the
  database and the blob store with the elimination schedule.
- The approvers' reasons stay in the bordereau manifest (the proof): they must not contain
  personal data.
- The handover to the archives (format, receipt) is manual; no archival system is connected.
- Retention applies to cases only; actors (personal data under the nLPD) have none yet.
