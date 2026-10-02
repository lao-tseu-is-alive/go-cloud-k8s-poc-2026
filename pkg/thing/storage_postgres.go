package thing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// PostgresRepository implements Repository with pgx and PostGIS, composing core primitives.
type PostgresRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewPostgresRepository builds a PostgresRepository from a connection pool. A
// nil logger falls back to slog.Default.
func NewPostgresRepository(pool *pgxpool.Pool, log *slog.Logger) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &PostgresRepository{pool: pool, log: log}, nil
}

// Create registers a thing: subject_ref, record_metadata, thing row, detail
// block and THING_CREATED audit event in one transaction.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Thing, *core.AuditEvent, error) {
	thingType, err := getThingType(ctx, r.pool, getThingTypeByCodeSQL, pgx.NamedArgs{"code": in.TypeCode})
	if err != nil {
		return nil, nil, err
	}
	if !thingType.IsActive {
		return nil, nil, fmt.Errorf("%w: thing type %q is inactive", core.ErrInvalidInput, in.TypeCode)
	}
	if err := prepareCreate(&in, thingType.Specialization); err != nil {
		return nil, nil, err
	}
	if err := checkGeometry(ctx, r.pool, in.GeometryGeoJSON, thingType.Specialization); err != nil {
		return nil, nil, err
	}
	in.Governance.DisplayLabel = in.Name

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin create thing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ref, err := core.InsertSubjectRefTx(ctx, tx, core.SubjectKindThing, in.Name, "")
	if err != nil {
		return nil, nil, fmt.Errorf("insert subject_ref: %w", err)
	}
	if _, err := core.InsertRecordMetadataTx(ctx, tx, in.Governance, ref.ID); err != nil {
		return nil, nil, fmt.Errorf("insert record_metadata: %w", err)
	}
	t, err := collectThing(tx.Query(ctx, insertThingSQL, pgx.NamedArgs{
		"id": ref.ID, "thing_type_id": thingType.ID, "name": in.Name, "description": in.Description,
		"external_ref": in.ExternalRef, "geojson": in.GeometryGeoJSON, "metadata": jsonMap(in.Metadata), "created_by": in.OperatorID,
	}))
	if err != nil {
		return nil, nil, err
	}
	if err := saveDetails(ctx, tx, ref.ID, in.Parcel, in.Building); err != nil {
		return nil, nil, err
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID: ref.ID, EventType: "THING_CREATED", ActorUserID: in.OperatorID,
		AfterState: thingAuditState(t, thingType.Code, in.Parcel, in.Building),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit create thing: %w", err)
	}
	return t, ev, r.hydrate(ctx, t)
}

// Get loads a thing with its subject, governance, type and detail block.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Thing, error) {
	t, err := collectThing(r.pool.Query(ctx, getThingSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	return t, r.hydrate(ctx, t)
}

// Update replaces the editable fields of an unlocked, live thing, keeps the
// subject label in sync and writes THING_UPDATED.
func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Thing, *core.AuditEvent, error) {
	current, err := r.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if err := checkDetails(current.Type.Specialization, in.Parcel, in.Building, false); err != nil {
		return nil, nil, err
	}
	if err := checkGeometry(ctx, r.pool, in.GeometryGeoJSON, current.Type.Specialization); err != nil {
		return nil, nil, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin update thing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := core.EnsureMutableTx(ctx, tx, id, false); err != nil {
		return nil, nil, err
	}
	before, err := collectThing(tx.Query(ctx, getThingForUpdateSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, nil, err
	}
	t, err := collectThing(tx.Query(ctx, updateThingSQL, pgx.NamedArgs{
		"id": id, "name": in.Name, "description": in.Description, "external_ref": in.ExternalRef,
		"geojson": in.GeometryGeoJSON, "metadata": jsonMap(in.Metadata),
	}))
	if err != nil {
		return nil, nil, err
	}
	if err := saveDetails(ctx, tx, id, in.Parcel, in.Building); err != nil {
		return nil, nil, err
	}
	if err := core.UpdateSubjectLabelTx(ctx, tx, id, t.Name); err != nil {
		return nil, nil, fmt.Errorf("sync subject label: %w", err)
	}
	after := thingAuditState(t, current.Type.Code, in.Parcel, in.Building)
	after["geometry_changed"] = deref(before.GeometryGeoJSON) != deref(t.GeometryGeoJSON)
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID: id, EventType: "THING_UPDATED", ActorUserID: in.OperatorID, Reason: in.Reason,
		BeforeState: map[string]any{"name": before.Name}, AfterState: after,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit update thing: %w", err)
	}
	return t, ev, r.hydrate(ctx, t)
}

// SoftDelete logically deletes the thing via its governance record and writes THING_DELETED.
func (r *PostgresRepository) SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin delete thing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := core.EnsureMutableTx(ctx, tx, id, true); err != nil {
		return nil, err
	}
	if _, err := core.SoftDeleteRecordMetadataTx(ctx, tx, id, operatorID); err != nil {
		return nil, err
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{SubjectID: id, EventType: "THING_DELETED", ActorUserID: operatorID, Reason: reason})
	if err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delete thing: %w", err)
	}
	return ev, nil
}

// thingListRow adds the window total to the thing columns for search scanning.
type thingListRow struct {
	Thing
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// Search runs the filtered search and hydrates the results.
func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	box := BBox{}
	if filter.BBox != nil {
		box = *filter.BBox
	}
	rows, err := r.pool.Query(ctx, core.SortedQuery(searchThingsSQL, filter.Sort, defaultThingSort), filter.Viewer.AddTo(pgx.NamedArgs{
		"count_limit": core.CountLimit,
		"query":       filter.Query, "thing_type_code": filter.TypeCode, "has_bbox": filter.BBox != nil,
		"e_min": box.EMin, "n_min": box.NMin, "e_max": box.EMax, "n_max": box.NMax,
		"include_deleted": filter.IncludeDeleted, "limit": filter.Limit, "offset": filter.Offset,
	}))
	if err != nil {
		return SearchResult{}, fmt.Errorf("search things: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[thingListRow])
	if err != nil {
		return SearchResult{}, fmt.Errorf("read things: %w", err)
	}
	result := SearchResult{Things: make([]*Thing, len(listRows))}
	for i := range listRows {
		t := listRows[i].Thing
		result.Things[i] = &t
		result.TotalSize = listRows[i].TotalSize
	}
	result.TotalSize, result.TotalCapped = core.CapTotal(result.TotalSize)
	return result, r.hydrateAll(ctx, result.Things)
}

// ListTypes returns the thing type catalogue ordered by code.
func (r *PostgresRepository) ListTypes(ctx context.Context, onlyActive bool) ([]*ThingType, error) {
	rows, err := r.pool.Query(ctx, listThingTypesSQL, pgx.NamedArgs{"only_active": onlyActive})
	if err != nil {
		return nil, fmt.Errorf("list thing types: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[ThingType])
}

// saveDetails writes (or replaces) the parcel or building block.
func saveDetails(ctx context.Context, tx pgx.Tx, id uuid.UUID, parcel *Parcel, building *Building) error {
	switch {
	case parcel != nil:
		_, err := tx.Exec(ctx, upsertParcelSQL, pgx.NamedArgs{
			"thing_id": id, "commune_ofs": parcel.CommuneOFS, "parcel_number": parcel.ParcelNumber,
			"egrid": parcel.EGRID, "surface_m2": parcel.SurfaceM2,
		})
		return mapDetailConflict(err, "a parcel with this commune and number, or this EGRID, already exists")
	case building != nil:
		_, err := tx.Exec(ctx, upsertBuildingSQL, pgx.NamedArgs{
			"thing_id": id, "egid": building.EGID, "eca_number": building.ECANumber,
			"construction_year": building.ConstructionYear, "building_status": int16(building.Status),
		})
		return mapDetailConflict(err, "a building with this EGID already exists")
	}
	return nil
}

// mapDetailConflict turns a unique violation of a detail block into ErrConflict.
func mapDetailConflict(err error, message string) error {
	if _, ok := core.PgErrorWithCode(err, core.PgUniqueViolation); ok {
		return fmt.Errorf("%w: %s", core.ErrConflict, message)
	}
	if err != nil {
		return fmt.Errorf("save thing details: %w", err)
	}
	return nil
}

// hydrate fills Subject, RecordMetadata, Type and the detail block of a thing.
func (r *PostgresRepository) hydrate(ctx context.Context, t *Thing) error {
	return r.hydrateAll(ctx, []*Thing{t})
}

// parcelRow and buildingRow are detail rows keyed by their thing, for batch loading.
type parcelRow struct {
	// ThingID is the parcel's thing.
	ThingID uuid.UUID `db:"thing_id"`
	Parcel
}

type buildingRow struct {
	// ThingID is the building's thing.
	ThingID uuid.UUID `db:"thing_id"`
	Building
}

// hydrateAll fills Subject, RecordMetadata, Type and the parcel or building
// details on a page of things in at most five queries, whatever the page size.
func (r *PostgresRepository) hydrateAll(ctx context.Context, things []*Thing) error {
	if len(things) == 0 {
		return nil
	}
	headers, err := core.GetSubjectHeadersTx(ctx, r.pool, core.IDsOf(things, func(t *Thing) uuid.UUID { return t.ID }))
	if err != nil {
		return fmt.Errorf("hydrate things: %w", err)
	}
	types, err := core.CollectIndexedTx(ctx, r.pool, getThingTypesByIDsSQL,
		core.IDsOf(things, func(t *Thing) uuid.UUID { return t.TypeID }), func(tt *ThingType) uuid.UUID { return tt.ID })
	if err != nil {
		return fmt.Errorf("hydrate thing types: %w", err)
	}
	var parcelIDs, buildingIDs []uuid.UUID
	for _, t := range things {
		t.Subject, t.RecordMetadata, t.Type = headers.Refs[t.ID], headers.Metadata[t.ID], types[t.TypeID]
		switch t.Type.Specialization {
		case SpecializationParcel:
			parcelIDs = append(parcelIDs, t.ID)
		case SpecializationBuilding:
			buildingIDs = append(buildingIDs, t.ID)
		}
	}
	return r.hydrateDetails(ctx, things, parcelIDs, buildingIDs)
}

// hydrateDetails attaches the parcel and building rows (one query each) to the
// things of the matching specialization.
func (r *PostgresRepository) hydrateDetails(ctx context.Context, things []*Thing, parcelIDs, buildingIDs []uuid.UUID) error {
	parcels, err := core.CollectGroupedTx(ctx, r.pool, getParcelsSQL, parcelIDs, func(p *parcelRow) uuid.UUID { return p.ThingID })
	if err != nil {
		return fmt.Errorf("hydrate parcels: %w", err)
	}
	buildings, err := core.CollectGroupedTx(ctx, r.pool, getBuildingsSQL, buildingIDs, func(b *buildingRow) uuid.UUID { return b.ThingID })
	if err != nil {
		return fmt.Errorf("hydrate buildings: %w", err)
	}
	for _, t := range things {
		if rows := parcels[t.ID]; len(rows) > 0 {
			t.Parcel = &rows[0].Parcel
		}
		if rows := buildings[t.ID]; len(rows) > 0 {
			t.Building = &rows[0].Building
		}
	}
	return nil
}

// collectThing reads exactly one thing row from a query result.
func collectThing(rows pgx.Rows, err error) (*Thing, error) {
	if err != nil {
		return nil, fmt.Errorf("query thing: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Thing])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	return t, err
}

// getThingType loads one thing type with the given query and arguments.
func getThingType(ctx context.Context, q core.Querier, sql string, args pgx.NamedArgs) (*ThingType, error) {
	return core.CollectReferenceRow[ThingType](q.Query(ctx, sql, args))
}

// thingAuditState is the audited identity of a thing: name, type, whether it is
// georeferenced and its official identifiers.
func thingAuditState(t *Thing, typeCode string, parcel *Parcel, building *Building) map[string]any {
	state := map[string]any{"name": t.Name, "thing_type": typeCode, "has_geometry": t.GeometryGeoJSON != nil}
	if parcel != nil {
		state["commune_ofs"], state["parcel_number"], state["egrid"] = parcel.CommuneOFS, parcel.ParcelNumber, parcel.EGRID
	}
	if building != nil && building.EGID != nil {
		state["egid"] = *building.EGID
	}
	return state
}

// jsonMap turns a nil map into an empty JSON object.
func jsonMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
