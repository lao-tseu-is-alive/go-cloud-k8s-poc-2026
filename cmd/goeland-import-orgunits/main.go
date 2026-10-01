// Command goeland-import-orgunits imports the organizational unit tree of the
// legacy Goéland database (a read-only replica) into the POC (GLD-041).
//
// Only the structure is copied: label, abbreviation, type, parent and the
// active/dissolved state. E-mails, descriptions, managers, phone numbers and
// employee links are never read. Units are written through the org unit
// service, so every rule and audit event applies; each unit keeps its source
// id as external reference (goeland:<IdOrgUnit>), which makes the import
// idempotent: a unit already imported is skipped.
//
// It prints counts only, never unit values, and writes nothing unless -apply
// is given:
//
//	GOELAND_IMPORT_SOURCE_URL='postgres://goeland_read:<password>@localhost/goeland' \
//	DATABASE_URL='postgres://<user>:<password>@localhost/goeland_poc_db' \
//	    go run ./cmd/goeland-import-orgunits [-apply]
//
// The target database must already be migrated (start the server once).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/legacyimport"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/orgunit"
)

const (
	// operatorID is recorded as the author of the imported units.
	operatorID = "import:goeland"
	// importTimeout bounds the whole run.
	importTimeout = 10 * time.Minute
)

// report counts what the import did or would do.
type report struct {
	read, skipped, created, renamed, dissolved, keptLive, unknownType int
}

func main() {
	apply := flag.Bool("apply", false, "write to the target database (default: dry run)")
	flag.Parse()
	if err := run(*apply); err != nil {
		fmt.Fprintln(os.Stderr, "goeland-import-orgunits:", err)
		os.Exit(1)
	}
}

// run reads the source tree and imports it, printing the counts.
func run(apply bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), importTimeout)
	defer cancel()
	source, err := connect(ctx, "GOELAND_IMPORT_SOURCE_URL")
	if err != nil {
		return err
	}
	defer source.Close()
	units, err := readSource(ctx, source)
	if err != nil {
		return err
	}
	target, err := connect(ctx, "DATABASE_URL")
	if err != nil {
		return err
	}
	defer target.Close()
	imp, err := newImporter(target, apply)
	if err != nil {
		return err
	}
	rep, err := imp.importTree(ctx, units)
	fmt.Printf("mode=%s read=%d already_imported=%d created=%d renamed=%d dissolved=%d kept_live_with_live_children=%d unknown_type=%d\n",
		map[bool]string{true: "apply", false: "dry-run"}[apply], rep.read, rep.skipped, rep.created, rep.renamed, rep.dissolved, rep.keptLive, rep.unknownType)
	return err
}

// connect opens a pool on the DSN held by the environment variable env.
func connect(ctx context.Context, env string) (*pgxpool.Pool, error) {
	dsn := strings.TrimSpace(os.Getenv(env))
	if dsn == "" {
		return nil, fmt.Errorf("%s is required", env)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", env, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping %s: %w", env, err)
	}
	return pool, nil
}

// readSource loads the legacy units.
func readSource(ctx context.Context, source *pgxpool.Pool) ([]*legacyimport.SourceUnit, error) {
	rows, err := source.Query(ctx, legacyimport.UnitSourceSQL)
	if err != nil {
		return nil, fmt.Errorf("read source units: %w", err)
	}
	units, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[legacyimport.SourceUnit])
	if err != nil {
		return nil, fmt.Errorf("scan source units: %w", err)
	}
	return units, nil
}

// importer writes the tree through the org unit service.
type importer struct {
	pool  *pgxpool.Pool
	svc   *orgunit.Service
	apply bool
	// ids maps a legacy id to the POC unit id (existing or created).
	ids map[int64]uuid.UUID
	rep report
}

// newImporter wires the core and org unit services on the target pool.
func newImporter(target *pgxpool.Pool, apply bool) (*importer, error) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	coreRepo, err := core.NewPostgresRepository(target, log)
	if err != nil {
		return nil, err
	}
	coreSvc, err := core.NewService(coreRepo, log)
	if err != nil {
		return nil, err
	}
	repo, err := orgunit.NewPostgresRepository(target, log)
	if err != nil {
		return nil, err
	}
	svc, err := orgunit.NewService(repo, coreSvc, log)
	if err != nil {
		return nil, err
	}
	return &importer{pool: target, svc: svc, apply: apply, ids: map[int64]uuid.UUID{}}, nil
}

// importTree creates the units parents first, then dissolves the inactive ones
// children first (a unit is dissolved only once it has no live sub-unit).
func (imp *importer) importTree(ctx context.Context, units []*legacyimport.SourceUnit) (report, error) {
	imp.rep.read = len(units)
	ordered := legacyimport.TopDown(units)
	for _, u := range ordered {
		if err := imp.importUnit(ctx, u); err != nil {
			return imp.rep, fmt.Errorf("import unit %s%d: %w", legacyimport.UnitExternalRefPrefix, u.ID, err)
		}
	}
	// Children come before their parents in reverse order, so the number of
	// sub-units that stay live is known when a unit is reached.
	liveChildren := map[int64]int{}
	for i := len(ordered) - 1; i >= 0; i-- {
		u := ordered[i]
		live, err := imp.settle(ctx, u, liveChildren[u.ID])
		if err != nil {
			return imp.rep, fmt.Errorf("dissolve unit %s%d: %w", legacyimport.UnitExternalRefPrefix, u.ID, err)
		}
		if live && u.ParentID != nil {
			liveChildren[*u.ParentID]++
		}
	}
	return imp.rep, nil
}

// importUnit creates one unit unless it was imported before. An inactive unit
// whose label is taken by a live sibling gets its legacy id appended.
func (imp *importer) importUnit(ctx context.Context, u *legacyimport.SourceUnit) error {
	ref := legacyimport.UnitExternalRefPrefix + strconv.FormatInt(u.ID, 10)
	existing, err := imp.findByRef(ctx, ref)
	if err != nil {
		return err
	}
	if existing != uuid.Nil {
		imp.ids[u.ID] = existing
		imp.rep.skipped++
		return nil
	}
	in := imp.createInput(u, ref)
	if !imp.apply {
		imp.ids[u.ID] = uuid.New()
		imp.rep.created++
		return nil
	}
	created, _, err := imp.svc.Create(ctx, in)
	if errors.Is(err, core.ErrConflict) && !u.Active {
		in.Label = fmt.Sprintf("%s [%d]", in.Label, u.ID)
		imp.rep.renamed++
		created, _, err = imp.svc.Create(ctx, in)
	}
	if err != nil {
		return err
	}
	imp.ids[u.ID] = created.ID
	imp.rep.created++
	return nil
}

// createInput maps a legacy unit to a POC creation request.
func (imp *importer) createInput(u *legacyimport.SourceUnit, ref string) orgunit.CreateInput {
	typeCode, ok := legacyimport.UnitTypeCodes[u.TypeName]
	if !ok {
		typeCode = "UNIT"
		imp.rep.unknownType++
	}
	label := legacyimport.UnitLabel(u)
	in := orgunit.CreateInput{
		ExternalRef: ref,
		Input:       orgunit.Input{TypeCode: typeCode, Abbreviation: u.Abbreviation, Label: label, OperatorID: operatorID},
	}
	if u.ParentID != nil {
		if parent, ok := imp.ids[*u.ParentID]; ok {
			in.ParentID = &parent
		}
	}
	return in
}

// settle gives a unit its final state and reports whether it stays live: an
// active unit does, and so does an inactive one that still has live sub-units
// (the POC only dissolves a unit without live sub-units; those are counted).
func (imp *importer) settle(ctx context.Context, u *legacyimport.SourceUnit, liveChildren int) (bool, error) {
	switch {
	case u.Active:
		return true, nil
	case liveChildren > 0:
		imp.rep.keptLive++
		return true, nil
	}
	return false, imp.dissolve(ctx, u)
}

// dissolve dissolves an inactive unit that has no live sub-unit left.
func (imp *importer) dissolve(ctx context.Context, u *legacyimport.SourceUnit) error {
	if !imp.apply {
		imp.rep.dissolved++
		return nil
	}
	id := imp.ids[u.ID]
	detail, err := imp.svc.Get(ctx, id)
	if err != nil {
		return err
	}
	if detail.Unit.Dissolved() {
		return nil
	}
	reason := u.Reason
	if reason == "" {
		reason = legacyimport.UnitDissolutionReason
	}
	if _, _, err := imp.svc.Dissolve(ctx, id, operatorID, reason); err != nil {
		return err
	}
	imp.rep.dissolved++
	return nil
}

// findByRef returns the unit already imported with this external reference,
// or uuid.Nil.
func (imp *importer) findByRef(ctx context.Context, ref string) (uuid.UUID, error) {
	var id uuid.UUID
	err := imp.pool.QueryRow(ctx, `SELECT id FROM org_unit WHERE external_ref = $1`, ref).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	return id, err
}
