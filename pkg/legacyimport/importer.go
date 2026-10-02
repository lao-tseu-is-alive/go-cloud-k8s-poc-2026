package legacyimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ErrTargetNotEmpty refuses a target that already holds imported data: an
// import always goes into a brand-new database (scripts/import_rebuild.sh).
var ErrTargetNotEmpty = errors.New("the target database already holds imported data; rebuild it first")

// Options control a run.
type Options struct {
	// Apply commits the import; otherwise everything is written and rolled back
	// (a dry run with the real counts and every constraint checked).
	Apply bool
	// StartedBy names who ran the import, recorded on the import batch.
	StartedBy string
}

// Importer runs the stages of one import inside one target transaction.
type Importer struct {
	source  *pgxpool.Pool
	tx      pgx.Tx
	batchID uuid.UUID
	report  *Report
	now     time.Time
	// subjects holds the ids written so far, so that grants and relationships
	// never point at a subject left out.
	subjects map[uuid.UUID]core.SubjectKind
	// employees are the legacy employee ids imported as users.
	employees map[int64]bool
	// activeEmployees are the active ones; employeeUnits their direct unit.
	activeEmployees map[int64]bool
	employeeUnits   map[int64]int64
	// liveUnits and liveGroups are the units not dissolved and the groups not archived.
	liveUnits  map[uuid.UUID]bool
	liveGroups map[uuid.UUID]bool
	// caseTypes maps a legacy IdTypeAffaire to the imported case type id.
	caseTypes map[int64]uuid.UUID
	// allEmployeeUnits is the direct unit of every employee (active or not);
	// unitParents the parent of each legacy unit.
	allEmployeeUnits map[int64]int64
	unitParents      map[int64]int64
	// unitTypes is the POC type code of each legacy unit.
	unitTypes map[int64]string
	// entries are the imported timeline entries.
	entries map[uuid.UUID]bool
	// thingTypes maps a legacy IdTypeThing to the POC thing type id.
	thingTypes map[int64]uuid.UUID
	// caseClosedAt holds the closing date of the closed cases.
	caseClosedAt map[uuid.UUID]time.Time
	// relTypes maps relationship type codes to their ids in the target.
	relTypes map[string]uuid.UUID
}

// stage is one step of the import.
type stage struct {
	name string
	run  func(context.Context, *StageCounts) error
}

// Run imports the source into the target and returns the counts.
func Run(ctx context.Context, source, target *pgxpool.Pool, opts Options) (*Report, error) {
	if err := ensureEmptyTarget(ctx, target); err != nil {
		return nil, err
	}
	var snapshot *time.Time
	if err := source.QueryRow(ctx, snapshotSQL).Scan(&snapshot); err != nil {
		return nil, fmt.Errorf("read the source snapshot date: %w", err)
	}
	tx, err := target.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin import: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	imp := &Importer{
		source: source, tx: tx, report: &Report{}, now: time.Now(),
		subjects: map[uuid.UUID]core.SubjectKind{}, employees: map[int64]bool{}, relTypes: map[string]uuid.UUID{},
		activeEmployees: map[int64]bool{}, employeeUnits: map[int64]int64{}, liveUnits: map[uuid.UUID]bool{},
		liveGroups: map[uuid.UUID]bool{}, caseTypes: map[int64]uuid.UUID{}, caseClosedAt: map[uuid.UUID]time.Time{}, thingTypes: map[int64]uuid.UUID{},
		allEmployeeUnits: map[int64]int64{}, unitParents: map[int64]int64{}, unitTypes: map[int64]string{}, entries: map[uuid.UUID]bool{},
	}
	if err := tx.QueryRow(ctx, insertBatchSQL, pgx.NamedArgs{
		"source_system": SourceSystem, "snapshot_at": snapshot, "started_by": opts.StartedBy,
	}).Scan(&imp.batchID); err != nil {
		return nil, fmt.Errorf("insert import batch: %w", err)
	}
	if err := imp.loadRelationshipTypes(ctx); err != nil {
		return nil, err
	}
	for _, s := range imp.stages() {
		if err := s.run(ctx, imp.report.stage(s.name)); err != nil {
			return imp.report, fmt.Errorf("stage %s: %w", s.name, err)
		}
	}
	if err := imp.finish(ctx); err != nil {
		return imp.report, err
	}
	if !opts.Apply {
		return imp.report, nil // rolled back by the deferred Rollback
	}
	if err := tx.Commit(ctx); err != nil {
		return imp.report, fmt.Errorf("commit import: %w", err)
	}
	// A bulk load leaves the planner without statistics until autovacuum runs:
	// the first requests then get very poor plans (a 10 s timeout was seen).
	if _, err := target.Exec(ctx, "ANALYZE"); err != nil {
		return imp.report, fmt.Errorf("analyze the target: %w", err)
	}
	return imp.report, nil
}

// stages lists the stages of waves 1 and 2 in dependency order.
func (imp *Importer) stages() []stage {
	return []stage{
		{"org_units", imp.importOrgUnits},
		{"users", imp.importUsers},
		{"unit_memberships", imp.importUnitMemberships},
		{"groups", imp.importGroups},
		{"group_memberships", imp.importGroupMemberships},
		{"case_types", imp.importCaseTypes},
		{"cases", imp.importCases},
		{"grants", imp.importGrants},
		{"actors", imp.importActors},
		{"actor_contacts", imp.importActorContacts},
		{"actor_addresses", imp.importActorAddresses},
		{"case_actor_roles", imp.importCaseActorRoles},
		{"case_user_roles", imp.importCaseUserRoles},
		{"case_unit_roles", imp.importCaseUnitRoles},
		// wave 2
		{"thing_types", imp.importThingTypes},
		{"things", imp.importThings},
		{"thing_actor_roles", imp.roleStage(thingActorRoles)},
		{"case_things", imp.linkStage(caseThingLink)},
		{"parent_cases", imp.linkStage(parentCaseLink)},
		{"related_cases", imp.linkStage(relatedCaseLink)},
		{"documents", imp.importDocuments},
		{"document_grants", imp.importDocumentGrants},
		{"document_access_lists", imp.importDocumentAccessLists},
		{"case_documents", imp.linkStage(caseDocumentLink)},
		{"thing_documents", imp.linkStage(thingDocLink)},
		{"document_actor_roles", imp.roleStage(documentActorRoles)},
		{"timeline_entries", imp.importTimelineEntries},
		{"timeline_documents", imp.importTimelineDocuments},
		{"timeline_status", imp.importTimelineFinalStatus},
	}
}

// ensureEmptyTarget refuses a target holding an import batch or a case.
func ensureEmptyTarget(ctx context.Context, target *pgxpool.Pool) error {
	var used bool
	if err := target.QueryRow(ctx, targetUsedSQL).Scan(&used); err != nil {
		return fmt.Errorf("inspect the target: %w", err)
	}
	if used {
		return ErrTargetNotEmpty
	}
	return nil
}

// finish stores the counts on the import batch.
func (imp *Importer) finish(ctx context.Context) error {
	counts, err := json.Marshal(imp.report.Counts())
	if err != nil {
		return fmt.Errorf("encode counts: %w", err)
	}
	if _, err := imp.tx.Exec(ctx, finishBatchSQL, pgx.NamedArgs{"id": imp.batchID, "counts": counts}); err != nil {
		return fmt.Errorf("finish import batch: %w", err)
	}
	return nil
}

// loadRelationshipTypes reads the relationship types already in the target (seeds).
func (imp *Importer) loadRelationshipTypes(ctx context.Context) error {
	rows, err := imp.tx.Query(ctx, `SELECT code, id FROM relationship_type`)
	if err != nil {
		return fmt.Errorf("read relationship types: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var id uuid.UUID
		if err := rows.Scan(&code, &id); err != nil {
			return err
		}
		imp.relTypes[code] = id
	}
	return rows.Err()
}

// operator returns the user id of a legacy employee, or the import operator
// when the employee is unknown.
func (imp *Importer) operator(employeeID *int64) string {
	if employeeID != nil && imp.employees[*employeeID] {
		return strconv.FormatInt(*employeeID, 10)
	}
	return OperatorID
}

// known reports whether id was written as a subject of kind.
func (imp *Importer) known(id uuid.UUID, kind core.SubjectKind) bool {
	return imp.subjects[id] == kind
}

// copyRows copies rows into table.
func (imp *Importer) copyRows(ctx context.Context, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	if _, err := imp.tx.CopyFrom(ctx, pgx.Identifier{table}, columns, pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("copy %s: %w", table, err)
	}
	return nil
}

// copyFromSource streams the rows of query on the source into table; convert
// maps a source row to the target values, or nil to leave it out.
func (imp *Importer) copyFromSource(ctx context.Context, table string, columns []string, query string, convert func(pgx.Rows) ([]any, error)) (int, error) {
	rows, err := imp.source.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("read source for %s: %w", table, err)
	}
	defer rows.Close()
	n, err := imp.tx.CopyFrom(ctx, pgx.Identifier{table}, columns, pgx.CopyFromFunc(func() ([]any, error) {
		for rows.Next() {
			values, err := convert(rows)
			if err != nil || values != nil {
				return values, err
			}
		}
		return nil, rows.Err()
	}))
	if err != nil {
		return int(n), fmt.Errorf("copy %s: %w", table, err)
	}
	return int(n), nil
}

// querySource runs query on the source and collects its rows into T (by column name).
func querySource[T any](ctx context.Context, imp *Importer, query string) ([]*T, error) {
	rows, err := imp.source.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[T])
	if err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}
	return out, nil
}

// copyFromSourceMulti is copyFromSource where one source row may give several
// target rows (or none).
func (imp *Importer) copyFromSourceMulti(ctx context.Context, table string, columns []string, query string, convert func(pgx.Rows) ([][]any, error)) (int, error) {
	rows, err := imp.source.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("read source for %s: %w", table, err)
	}
	defer rows.Close()
	var pending [][]any
	n, err := imp.tx.CopyFrom(ctx, pgx.Identifier{table}, columns, pgx.CopyFromFunc(func() ([]any, error) {
		for len(pending) == 0 {
			if !rows.Next() {
				return nil, rows.Err()
			}
			out, err := convert(rows)
			if err != nil {
				return nil, err
			}
			pending = out
		}
		next := pending[0]
		pending = pending[1:]
		return next, nil
	}))
	if err != nil {
		return int(n), fmt.Errorf("copy %s: %w", table, err)
	}
	return int(n), nil
}
