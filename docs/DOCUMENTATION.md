# Documentation quality contract

This document is the normative documentation contract for
`go-cloud-k8s-poc-2026` (Goéland POC). It applies equally to human contributors
and coding agents. Its purpose is to keep the repository understandable without
relying on conversational memory and to turn detectable documentation drift
into a build or release failure.

The words **MUST**, **MUST NOT**, **SHOULD** and **MAY** express requirement
levels. Automated checks prove structure, coverage and selected factual links;
review remains responsible for clarity and semantic accuracy.

The model is ported from `go-pdf-forge`; the local conventions it depends on
are listed in [Local conventions](#local-conventions).

## Outcomes

A compliant change leaves a future reader able to answer four questions:

1. What contract does each package, exported Go API and RPC provide?
2. Which file owns each behavior, decision, generated artifact or operation?
3. Which documented operational and security claims are enforced by code?
4. Which controls ran before the change could become a release?

Documentation is part of the implementation. It MUST be updated in the same
change as the behavior, file inventory or release state it describes.

## Sources of truth and responsibilities

| Component | Responsibility | Enforcement |
| --- | --- | --- |
| `docs/DOCUMENTATION.md` | Defines this normative contract. | Durable references from `AGENTS.md` and `README.md`; executable assertions. |
| Go source | Documents package purpose and exported contracts beside the implementation. | `cmd/doccheck --scope go`, exposed as `make godoc-check`. |
| `proto/goeland/v1/*.proto` | Defines the authoritative network contracts (Go, ConnectRPC and OpenAPI are generated from it). | `buf lint`, including the `COMMENTS` category configured in `buf.yaml`. |
| `docs/atlas.md` | Gives every repository file one discoverable responsibility and authority note. | `cmd/doccheck --scope atlas`, exposed as `make atlas-check`. |
| `scripts/check_documentation_claims.sh` | Connects selected stable prose claims to source and automation. | `make docs-assert`. |
| `requirements/goeland_poc_domain_model_agent.md` | Immutable statement of intent (the spec); cited, never rewritten to match reality. | Review. |
| `requirements/IMPLEMENTATION_STATUS.md` | Living state against the spec: what is built, decided enhancements (🚀), deviations and known gaps. | Review; updated at the end of each slice. |
| `docs/ROADMAP.md` | Owns implementation order, scope and `GLD-NNN` task completion state. | Roadmap and release-traceability guards. |
| `CHANGELOG.md` | Records what a released version actually delivered. | Version, changelog and bidirectional task traceability guards. |
| `Makefile` | Composes local documentation, quality and release gates. | `make docs-check`, `make check`, `make release-check`. |
| `.github/workflows/*.yml` | Runs the same repository-owned gates in CI and during publication. | `ci.yml` runs `make release-check`; no separate reduced CI policy. |
| `AGENTS.md` | Makes this contract mandatory for any coding agent entering the repository. | Repository instruction loading plus an executable reference assertion. |

No summary in another file supersedes this document. Short references SHOULD
point here instead of copying rules that could later diverge.

The roadmap owns *order and task state*; `IMPLEMENTATION_STATUS.md` owns *how
the built system relates to the spec*. A fact belongs to exactly one of them.
Untracked scratch notes MUST NOT be the only record of planned work: anything
worth doing later is a roadmap task.

## Go source contract

### Package documentation

Every non-generated Go package MUST contain at least one package comment:

- a library package comment begins with `Package <package-name> `;
- a `main` package comment begins with `Command ` or `Package main `.

The comment explains the package boundary and responsibility, not its
directory name. The domain packages keep it in `doc.go`; a smaller package MAY
put it in its natural main file.

```go
// Package core implements the transversal Goéland primitives: subject
// identity, governance metadata, typed relationships and the audit trail.
package core
```

### Exported API documentation

Every exported type, function, constant and variable MUST have a GoDoc comment
whose first word is its exact identifier. Exported methods on exported receiver
types follow the same rule. Every named exported struct field MUST have a field
or trailing comment beginning with its exact field name.

```go
// Document is the GED entity row; its identity and governance live in the
// core subject_ref and record_metadata rows sharing its ID.
type Document struct {
	// FileSizeBytes is the stored blob size in bytes; nil until bytes exist.
	FileSizeBytes *int64 `db:"file_size_bytes"`
}
```

A useful contract comment states the facts callers need that the Go type
system cannot express. Depending on the API, that includes:

- units, bounds, default values and whether deadlines are inclusive;
- nullability (pointer fields), the column or enum values a field maps to, and
  whether Postgres computes it (e.g. a `GENERATED ALWAYS` column);
- ownership, secret handling and authorization boundaries (operator vs domain
  actor, required scope);
- transaction scope, audit side effects, idempotence and partial failure;
- concurrency safety, ordering, locking and retry semantics;
- resource lifecycle, cancellation and cleanup responsibilities;
- sentinel errors and meaningful failure conditions;
- whether data is streamed or may be retained in memory.

Comments MUST NOT merely restate the declaration. Private helpers SHOULD be
commented when they carry a non-obvious invariant, security decision or
algorithmic constraint. Otherwise, clear naming and small functions are
preferred.

Source comments are written in English to match Go identifiers, Go tooling and
the public API. Domain identifiers keep their established names.

### Mechanical scope and review scope

`cmd/doccheck` examines every non-ignored `.go` file known to Git, including
new untracked files. It mechanically excludes `_test.go` files and files that
Go identifies as generated (`gen/`). It also excludes exported-looking methods
on private receiver types because those methods are not a public package API.
Git-ignored trees such as `node_modules/` are outside the inventory.

These exclusions only remove a mechanical GoDoc requirement. Tests MUST remain
readable, generated files MUST identify their generator, and complex private
behavior still requires human review. Generated files MUST be changed through
their authoritative source and generator, never by hand.

## Protobuf contract

`proto/goeland/v1/*.proto` is authoritative. `gen/goeland/v1/` (Go and
ConnectRPC) and `api/openapi/goeland.swagger.yaml` are derived artifacts and
MUST NOT be edited manually.

`buf.yaml` enables `STANDARD` and `COMMENTS`. Services, RPCs, messages, enums,
enum values, oneofs and fields therefore require comments. Those comments MUST
describe relevant units, optionality, required scope, audit and idempotence
semantics, ownership and error conditions; they also become the OpenAPI
descriptions. Every RPC also carries a `google.api.http` annotation so it has a
REST binding. Regenerate with `make generate` after a contract change and
commit the source plus generated result together.

## Frontend contract

The embedded SPA (`cmd/goeland-server/goeland-front`) is a first-class part of
the repository:

- every non-ignored frontend file has an atlas entry; `dist/` and
  `node_modules/` are git-ignored build outputs;
- `src/api/types.ts` is hand-maintained against the proto contract and MUST be
  updated in the same change as any RPC the SPA uses;
- `make front-check` (frozen install, `vue-tsc` type-check, ESLint, build) is
  part of `make check`.

## Repository atlas contract

[`atlas.md`](atlas.md) is a controlled, file-by-file index. It MUST contain
exactly one entry for every tracked or new non-ignored repository file,
including documentation, migrations, generated bindings, frontend sources and
automation.

The canonical inventory is the NUL-delimited result of:

```bash
git ls-files -z --cached --others --exclude-standard
```

Each entry uses this machine-readable form and a canonical path relative to the
repository root:

```text
- `path/from/repository/root` — One-line responsibility and authority note.
```

Descriptions SHOULD distinguish an authoritative input from a generated
artifact, test, example or operational wrapper. `doccheck` compares paths in
both directions and rejects missing, stale, duplicated or non-canonical
entries. It also requires the `Tracked version: **vX.Y.Z**.` banner to match
the `Version` constant in `pkg/version/version.go`.

When a file is added, add its atlas entry in the same change. When a file is
removed, remove its entry. When a file is renamed, update its path and review
its description rather than treating the operation as a blind text rename.

## Executable documentation claims

Some prose is load-bearing: a stale default, security boundary or release rule
could mislead an operator even when compilation succeeds.
`scripts/check_documentation_claims.sh` uses explicit literal assertions to
link selected claims across their authoritative implementation, agent
instructions and operator-facing documentation. It reports every failing
claim, not only the first.

Add or update an assertion when all of these conditions hold:

- the fact is stable and operationally or security relevant;
- two or more repository surfaces must agree;
- silent drift would be worse than an explicit maintenance failure;
- a deterministic assertion can identify the expected source text.

Literal assertions are intentionally simple and visible. They SHOULD NOT cover
every sentence, and MUST NOT replace unit, integration or security tests. If a
refactor intentionally changes asserted text (including a `gofmt` realignment),
update the implementation, prose and assertion together after verifying that
the underlying contract still holds.

## Roadmap, changelog and version traceability

Documentation quality also covers the claim that work has shipped:

- `docs/ROADMAP.md` is authoritative for `GLD-NNN` task IDs, scope, order and
  status;
- a completed `GLD-*` task MUST appear in a dated, versioned changelog section;
- a task named by a released changelog section MUST be marked complete in the
  roadmap;
- the `Version` constant in `pkg/version/version.go`, the README
  `Current version:` banner, the roadmap and atlas `Tracked version:` banners
  and the release changelog section MUST agree for a release.

`Version` is a Go constant, not a variable, so `-ldflags -X` cannot make a
binary report a version different from the audited source; only `Revision` and
`BuildStamp` are injected at build time. The `Unreleased` changelog section may
describe ongoing work but does not prove that a task was delivered.

## Control chain

The repository deliberately composes one set of controls rather than defining
different local, CI and release standards:

```text
make docs-check
  ├─ make godoc-check  -> cmd/doccheck --scope go
  ├─ make atlas-check  -> cmd/doccheck --scope atlas
  └─ make docs-assert  -> scripts/check_documentation_claims.sh

make check
  ├─ make front-check  (frozen bun install, type-check, lint, build dist/)
  ├─ make fmt-check    (gofmt, buf format)
  ├─ make lint         (go vet, buf lint incl. COMMENTS)
  ├─ make test         (race detector, coverage)
  ├─ make docs-check
  └─ git diff --check

make release-check
  ├─ make check
  ├─ version, changelog and scripts consistency
  └─ binary build

GitHub CI (ci.yml)   -> make release-check on every push to main and every PR
```

A contributor or agent MUST NOT bypass a failing documentation gate. Fix the
authoritative source, its documentation or the checker as appropriate. A
checker change requires tests demonstrating both the accepted and rejected
behavior.

## Change workflow

Before considering a change complete, apply every relevant row:

| Change | Required documentation work |
| --- | --- |
| Add or change a Go package/API | Update package and API contract comments; run `make godoc-check`. |
| Change a Protobuf API | Update `.proto` comments and the HTTP annotation, regenerate, update `src/api/types.ts` if the SPA uses it, run `make lint`. |
| Add, remove or rename any non-ignored file | Synchronize `docs/atlas.md`; run `make atlas-check`. |
| Change a stable default or security/operational promise | Update all owning surfaces and the executable claim when appropriate. |
| Finish a slice | Update `requirements/IMPLEMENTATION_STATUS.md` (a few lines). |
| Start, complete, reorder or rescope a roadmap task | Update `docs/ROADMAP.md` in the same change. |
| Prepare a release | Synchronize version banners and changelog, then run `make release-check`. |

For any documentation-sensitive change, the minimum local command is:

```bash
make docs-check
```

Before normal handoff, run `make check`. Before a release commit, run
`make release-check`.

## Definition of done

Documentation work is complete only when:

- public contracts describe caller-relevant semantics and invariants;
- authoritative and generated files are clearly distinguished;
- the atlas matches the complete non-ignored file inventory;
- stable cross-file claims agree and have assertions where warranted;
- roadmap, changelog, status and version claims are honest for the current
  lifecycle;
- `make docs-check` and the broader gate appropriate to the change pass;
- the review confirms meaning and clarity beyond mechanical coverage.

## Local conventions

`cmd/doccheck` is a parameterized copy of the `go-pdf-forge` checker. This
repository uses its defaults:

| Convention | Value | `doccheck` flag |
| --- | --- | --- |
| Version source | `pkg/version/version.go` | `--version-file` |
| Version constant | `Version` (a `const`) | `--version-name` |
| Atlas path | `docs/atlas.md` | `--atlas` |
| Atlas banner | `Tracked version: **vX.Y.Z**.` | `--banner-prefix` |
| Roadmap task IDs | `GLD-NNN` | (roadmap scripts) |
| Document language | English | — |

Changing a convention MUST change the checker, its tests and this document
together. If a third repository adopts the checker, extract it into a shared
module used via a `go.mod` `tool` directive instead of copying it again.

## Adoption status

This contract is being adopted in five slices; this section is removed when
the last one lands.

| Slice | Scope | State |
| --- | --- | --- |
| 1 | This contract, `cmd/doccheck`, constant `Version`, Make gates, `ci.yml`, claims script | Done |
| 2 | `docs/atlas.md` for the complete inventory | Pending — `make atlas-check` fails until then |
| 3 | GoDoc for every package and exported API | Pending — `make godoc-check` fails until then |
| 4 | Protobuf `COMMENTS` lint and regenerated bindings/OpenAPI | Pending |
| 5 | `docs/ROADMAP.md` with `GLD-NNN`, traceability, guarded release script and gated publication workflows | Pending |

Until slice 3 lands, `make check`, `make release-check` and CI fail on the
documentation gate by design; the failure lists the remaining work.
