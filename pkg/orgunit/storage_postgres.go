package orgunit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// treeLockKey serializes the tree mutations: two concurrent moves could each
// pass the cycle check against the state the other one is changing.
const treeLockKey = "go-cloud-k8s-poc-2026:org_unit_tree"

// PostgresRepository implements Repository with pgx, composing core primitives.
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

// inTreeTx runs fn in a transaction holding the tree lock and commits it when
// fn succeeds.
func (r *PostgresRepository) inTreeTx(ctx context.Context, op string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", op, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, treeLockKey); err != nil {
		return fmt.Errorf("lock org unit tree: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", op, err)
	}
	return nil
}

// Create adds a unit: subject_ref, record_metadata, unit row and
// ORG_UNIT_CREATED audit event in one transaction.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*OrgUnit, *core.AuditEvent, error) {
	var created *OrgUnit
	var ev *core.AuditEvent
	err := r.inTreeTx(ctx, "create org unit", func(tx pgx.Tx) error {
		unitType, err := activeType(ctx, tx, in.TypeCode)
		if err != nil {
			return err
		}
		if in.ParentID != nil {
			if err := checkParentTx(ctx, tx, uuid.Nil, *in.ParentID); err != nil {
				return err
			}
		}
		label := DisplayLabel(in.Label, in.Abbreviation)
		ref, err := core.InsertSubjectRefTx(ctx, tx, core.SubjectKindOrgUnit, label, "")
		if err != nil {
			return fmt.Errorf("insert subject_ref: %w", err)
		}
		gov := core.CreateSubjectInput{Kind: core.SubjectKindOrgUnit, DisplayLabel: label, OperatorID: in.OperatorID, OwnerUserID: in.OperatorID}
		if _, err := core.InsertRecordMetadataTx(ctx, tx, gov, ref.ID); err != nil {
			return fmt.Errorf("insert record_metadata: %w", err)
		}
		created, err = collectUnit(tx.Query(ctx, insertUnitSQL, pgx.NamedArgs{
			"id":               ref.ID,
			"org_unit_type_id": unitType.ID,
			"abbreviation":     in.Abbreviation,
			"label":            in.Label,
			"description":      in.Description,
			"email":            in.Email,
			"parent_id":        in.ParentID,
			"external_ref":     in.ExternalRef,
			"created_by":       in.OperatorID,
		}))
		if err != nil {
			return err
		}
		ev, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: ref.ID, EventType: "ORG_UNIT_CREATED", ActorUserID: in.OperatorID, Reason: in.Reason,
			AfterState: unitState(created, unitType.Code),
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	u, err := r.get(ctx, created.ID)
	return u, ev, err
}

// Update replaces the editable fields of a live unit, moving it when the parent
// changes, keeps the subject label in sync and writes ORG_UNIT_UPDATED.
func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, in Input) (*OrgUnit, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTreeTx(ctx, "update org unit", func(tx pgx.Tx) error {
		var err error
		ev, err = updateTx(ctx, tx, id, in)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	u, err := r.get(ctx, id)
	return u, ev, err
}

// updateTx checks and applies an update inside the tree transaction.
func updateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, in Input) (*core.AuditEvent, error) {
	current, err := lockLiveUnit(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	currentType, unitType, err := resolveUpdateType(ctx, tx, current, in.TypeCode)
	if err != nil {
		return nil, err
	}
	if in.ParentID != nil {
		if err := checkParentTx(ctx, tx, id, *in.ParentID); err != nil {
			return nil, err
		}
	}
	updated, err := collectUnit(tx.Query(ctx, updateUnitSQL, pgx.NamedArgs{
		"id":               id,
		"org_unit_type_id": unitType.ID,
		"abbreviation":     in.Abbreviation,
		"label":            in.Label,
		"description":      in.Description,
		"email":            in.Email,
		"parent_id":        in.ParentID,
	}))
	if err != nil {
		return nil, err
	}
	if err := core.UpdateSubjectLabelTx(ctx, tx, id, DisplayLabel(updated.Label, updated.Abbreviation)); err != nil {
		return nil, fmt.Errorf("sync subject label: %w", err)
	}
	return core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID: id, EventType: "ORG_UNIT_UPDATED", ActorUserID: in.OperatorID, Reason: in.Reason,
		BeforeState: unitState(current, currentType.Code), AfterState: unitState(updated, unitType.Code),
	})
}

// resolveUpdateType returns the unit's current type and the requested one: the
// current type stays valid even when deactivated, a new type must be active.
func resolveUpdateType(ctx context.Context, q core.Querier, current *OrgUnit, code string) (*OrgUnitType, *OrgUnitType, error) {
	currentType, err := getType(ctx, q, getTypeByIDSQL, pgx.NamedArgs{"id": current.TypeID})
	if err != nil {
		return nil, nil, err
	}
	if code == currentType.Code {
		return currentType, currentType, nil
	}
	unitType, err := activeType(ctx, q, code)
	return currentType, unitType, err
}

// Dissolve dissolves a live unit without live children and writes ORG_UNIT_DISSOLVED.
func (r *PostgresRepository) Dissolve(ctx context.Context, id uuid.UUID, operatorID, reason string) (*OrgUnit, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTreeTx(ctx, "dissolve org unit", func(tx pgx.Tx) error {
		if _, err := lockLiveUnit(ctx, tx, id); err != nil {
			return err
		}
		var children int
		if err := tx.QueryRow(ctx, countLiveChildrenSQL, pgx.NamedArgs{"id": id}).Scan(&children); err != nil {
			return fmt.Errorf("count live children: %w", err)
		}
		if children > 0 {
			return fmt.Errorf("%w: the unit still has %d live sub-units; move or dissolve them first", core.ErrInvalidState, children)
		}
		if _, err := collectUnit(tx.Query(ctx, dissolveUnitSQL, pgx.NamedArgs{"id": id, "operator_id": operatorID, "reason": reason})); err != nil {
			return err
		}
		var err error
		ev, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: id, EventType: "ORG_UNIT_DISSOLVED", ActorUserID: operatorID, Reason: reason,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	u, err := r.get(ctx, id)
	return u, ev, err
}

// lockLiveUnit rejects a locked or soft-deleted subject and a dissolved unit,
// and returns the unit row locked for the rest of the transaction.
func lockLiveUnit(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*OrgUnit, error) {
	if _, err := core.EnsureMutableTx(ctx, tx, id, false); err != nil {
		return nil, err
	}
	u, err := collectUnit(tx.Query(ctx, getUnitForUpdateSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	if u.Dissolved() {
		return nil, fmt.Errorf("%w: the unit is dissolved", core.ErrInvalidState)
	}
	return u, nil
}

// checkParentTx requires parentID to be a live unit, and, when a unit id is
// given, neither that unit nor one of its descendants.
func checkParentTx(ctx context.Context, q core.Querier, id, parentID uuid.UUID) error {
	if parentID == id {
		return fmt.Errorf("%w: a unit cannot be its own parent", core.ErrInvalidInput)
	}
	parent, err := collectUnit(q.Query(ctx, getUnitForUpdateSQL, pgx.NamedArgs{"id": parentID}))
	if errors.Is(err, core.ErrNotFound) {
		return fmt.Errorf("%w: parent unit %s does not exist", core.ErrInvalidInput, parentID)
	}
	if err != nil {
		return err
	}
	if parent.Dissolved() {
		return fmt.Errorf("%w: parent unit %s is dissolved", core.ErrInvalidInput, parentID)
	}
	if id == uuid.Nil {
		return nil
	}
	var below bool
	if err := q.QueryRow(ctx, isDescendantSQL, pgx.NamedArgs{"id": id, "candidate": parentID}).Scan(&below); err != nil {
		return fmt.Errorf("check descendants: %w", err)
	}
	if below {
		return fmt.Errorf("%w: a unit cannot be moved under one of its own sub-units", core.ErrInvalidInput)
	}
	return nil
}

// Get loads a unit with its ancestors and children.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Detail, error) {
	u, err := r.get(ctx, id)
	if err != nil {
		return nil, err
	}
	ancestors, err := collectNodes(r.pool.Query(ctx, ancestorsSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	children, err := collectNodes(r.pool.Query(ctx, childrenSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	return &Detail{Unit: u, Ancestors: ancestors, Children: children}, nil
}

// get loads one hydrated unit.
func (r *PostgresRepository) get(ctx context.Context, id uuid.UUID) (*OrgUnit, error) {
	u, err := collectUnit(r.pool.Query(ctx, getUnitSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	return u, r.hydrate(ctx, u)
}

// List returns the whole tree as a flat list.
func (r *PostgresRepository) List(ctx context.Context, includeDissolved bool) ([]*Node, error) {
	return collectNodes(r.pool.Query(ctx, listNodesSQL, pgx.NamedArgs{"include_dissolved": includeDissolved}))
}

// unitListRow adds the window total to the unit columns for search scanning.
type unitListRow struct {
	OrgUnit
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// Search runs the filtered search and hydrates the results.
func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	rows, err := r.pool.Query(ctx, searchUnitsSQL, pgx.NamedArgs{
		"query":             filter.Query,
		"include_dissolved": filter.IncludeDissolved,
		"limit":             filter.Limit,
		"offset":            filter.Offset,
	})
	if err != nil {
		return SearchResult{}, fmt.Errorf("search org units: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[unitListRow])
	if err != nil {
		return SearchResult{}, fmt.Errorf("read org units: %w", err)
	}
	result := SearchResult{Units: make([]*OrgUnit, len(listRows))}
	for i := range listRows {
		u := listRows[i].OrgUnit
		result.Units[i] = &u
		result.TotalSize = listRows[i].TotalSize
		if err := r.hydrate(ctx, &u); err != nil {
			return SearchResult{}, err
		}
	}
	return result, nil
}

// ListTypes returns the unit type catalogue by sort order then code.
func (r *PostgresRepository) ListTypes(ctx context.Context, onlyActive bool) ([]*OrgUnitType, error) {
	rows, err := r.pool.Query(ctx, listTypesSQL, pgx.NamedArgs{"only_active": onlyActive})
	if err != nil {
		return nil, fmt.Errorf("list org unit types: %w", err)
	}
	types, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[OrgUnitType])
	if err != nil {
		return nil, fmt.Errorf("read org unit types: %w", err)
	}
	return types, nil
}

// hydrate fills Subject, RecordMetadata and Type on a unit.
func (r *PostgresRepository) hydrate(ctx context.Context, u *OrgUnit) error {
	ref, err := core.GetSubjectRefTx(ctx, r.pool, u.ID)
	if err != nil {
		return fmt.Errorf("hydrate subject: %w", err)
	}
	md, err := core.GetRecordMetadataTx(ctx, r.pool, u.ID)
	if err != nil {
		return fmt.Errorf("hydrate metadata: %w", err)
	}
	unitType, err := getType(ctx, r.pool, getTypeByIDSQL, pgx.NamedArgs{"id": u.TypeID})
	if err != nil {
		return fmt.Errorf("hydrate type: %w", err)
	}
	u.Subject, u.RecordMetadata, u.Type = ref, md, unitType
	return nil
}

// activeType loads a unit type by code and requires it to be active.
func activeType(ctx context.Context, q core.Querier, code string) (*OrgUnitType, error) {
	unitType, err := getType(ctx, q, getTypeByCodeSQL, pgx.NamedArgs{"code": code})
	if errors.Is(err, core.ErrNotFound) {
		return nil, fmt.Errorf("%w: unknown org unit type %q", core.ErrInvalidInput, code)
	}
	if err != nil {
		return nil, err
	}
	if !unitType.IsActive {
		return nil, fmt.Errorf("%w: org unit type %q is inactive", core.ErrInvalidInput, code)
	}
	return unitType, nil
}

// unitState is the audited snapshot of a unit.
func unitState(u *OrgUnit, typeCode string) map[string]any {
	parent := ""
	if u.ParentID != nil {
		parent = u.ParentID.String()
	}
	return map[string]any{"abbreviation": u.Abbreviation, "label": u.Label, "type": typeCode, "email": u.Email, "parent_id": parent, "external_ref": u.ExternalRef}
}

// getType loads one unit type with the given query and arguments.
func getType(ctx context.Context, q core.Querier, sql string, args pgx.NamedArgs) (*OrgUnitType, error) {
	rows, err := q.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("query org unit type: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[OrgUnitType])
	if err != nil {
		return nil, mapDBError(err)
	}
	return t, nil
}

// collectUnit reads exactly one unit row from a query result.
func collectUnit(rows pgx.Rows, err error) (*OrgUnit, error) {
	if err != nil {
		return nil, fmt.Errorf("query org unit: %w", err)
	}
	u, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[OrgUnit])
	if err != nil {
		return nil, mapDBError(err)
	}
	return u, nil
}

// collectNodes reads tree nodes from a query result.
func collectNodes(rows pgx.Rows, err error) ([]*Node, error) {
	if err != nil {
		return nil, fmt.Errorf("query org unit nodes: %w", err)
	}
	nodes, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Node])
	if err != nil {
		return nil, fmt.Errorf("read org unit nodes: %w", err)
	}
	return nodes, nil
}

// mapDBError translates pgx.ErrNoRows to core.ErrNotFound, a sibling label or
// external reference already in use to core.ErrConflict and the tree
// trigger's cycle refusal to core.ErrInvalidInput.
func mapDBError(err error) error {
	return core.MapDBError(err, func(pgErr *pgconn.PgError) error {
		switch pgErr.Code {
		case core.PgUniqueViolation:
			if pgErr.ConstraintName == "idx_org_unit_external_ref_unique" {
				return fmt.Errorf("%w: an org unit already has this external reference", core.ErrConflict)
			}
			return fmt.Errorf("%w: a live unit with this label already exists under the same parent", core.ErrConflict)
		case core.PgCheckViolation: // cycle refused by guard_org_unit
			return fmt.Errorf("%w: %s", core.ErrInvalidInput, pgErr.Message)
		}
		return nil
	})
}
