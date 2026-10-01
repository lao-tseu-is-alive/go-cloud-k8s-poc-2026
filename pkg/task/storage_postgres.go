package task

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

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

// inTx runs fn in a transaction on the repository's pool (core.InTx).
func (r *PostgresRepository) inTx(ctx context.Context, op string, fn func(pgx.Tx) error) error {
	return core.InTx(ctx, r.pool, op, fn)
}

// Create adds a manual task to an open case, with its first assignment when an
// assignee is given, and writes TASK_CREATED on the case.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Task, *core.AuditEvent, error) {
	var id uuid.UUID
	var ev *core.AuditEvent
	err := r.inTx(ctx, "create task", func(tx pgx.Tx) error {
		t, created, err := CreateTx(ctx, tx, in, OriginManual, "")
		if err == nil {
			id, ev = t.ID, created
		}
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	t, err := r.Get(ctx, id)
	return t, ev, err
}

// Update replaces the content of a pending task and writes TASK_UPDATED.
func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Task, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "update task", func(tx pgx.Tx) error {
		current, err := lockManualPendingTx(ctx, tx, id, in.OperatorID)
		if err != nil {
			return err
		}
		currentType, taskType, err := resolveType(ctx, tx, current.TypeID, in.TypeCode)
		if err != nil {
			return err
		}
		t, err := collectTask(tx.Query(ctx, updateTaskSQL, pgx.NamedArgs{
			"id": id, "task_type_id": taskType.ID, "title": in.Title, "description": in.Description,
			"due_at": in.DueAt, "operator_id": in.OperatorID,
		}))
		if err != nil {
			return err
		}
		ev, err = auditTx(ctx, tx, t, "TASK_UPDATED", in.OperatorID, in.Reason, taskState(current, currentType.Code), taskState(t, taskType.Code))
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	t, err := r.Get(ctx, id)
	return t, ev, err
}

// Assign (re)assigns or unassigns a pending task, keeps the history and
// writes TASK_ASSIGNED.
func (r *PostgresRepository) Assign(ctx context.Context, id uuid.UUID, to Assignee, operatorID, reason string) (*Task, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "assign task", func(tx pgx.Tx) error {
		current, err := lockManualPendingTx(ctx, tx, id, operatorID)
		if err != nil {
			return err
		}
		if sameAssignee(current, to) {
			return fmt.Errorf("%w: the task is already assigned this way", core.ErrInvalidInput)
		}
		if err := CheckAssigneeTx(ctx, tx, to); err != nil {
			return err
		}
		t, err := collectTask(tx.Query(ctx, setAssigneeSQL, pgx.NamedArgs{
			"id": id, "assignee_user_id": to.UserID, "assignee_org_unit_id": to.OrgUnitID, "operator_id": operatorID,
		}))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, endCurrentAssignmentSQL, pgx.NamedArgs{"task_id": id}); err != nil {
			return fmt.Errorf("end current assignment: %w", err)
		}
		if err := recordAssignmentTx(ctx, tx, id, to, operatorID, reason); err != nil {
			return err
		}
		ev, err = auditTx(ctx, tx, t, "TASK_ASSIGNED", operatorID, reason, assigneeState(current), assigneeState(t))
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	t, err := r.Get(ctx, id)
	return t, ev, err
}

// ChangeStatus applies one lifecycle move (see moves) and writes its audit
// event; completion and cancellation also record a SYSTEM timeline entry.
func (r *PostgresRepository) ChangeStatus(ctx context.Context, id uuid.UUID, move Move, operatorID, note string) (*Task, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "change task status", func(tx pgx.Tx) error {
		var err error
		_, ev, err = MoveTx(ctx, tx, id, move, operatorID, note, MoveOptions{})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	t, err := r.Get(ctx, id)
	return t, ev, err
}

// Get loads a task with its type and assignment history.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Task, error) {
	t, err := collectTask(r.pool.Query(ctx, getTaskSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	if err := r.hydrate(ctx, []*Task{t}); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, listAssignmentsSQL, pgx.NamedArgs{"task_id": id})
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	if t.Assignments, err = pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Assignment]); err != nil {
		return nil, fmt.Errorf("read assignments: %w", err)
	}
	return t, nil
}

// taskListRow adds the window total to the task columns for list scanning.
type taskListRow struct {
	Task
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// ListCase returns a page of the tasks of a case and its pending count.
func (r *PostgresRepository) ListCase(ctx context.Context, filter CaseFilter) (ListResult, error) {
	if _, err := core.GetRecordMetadataTx(ctx, r.pool, filter.CaseID); err != nil {
		return ListResult{}, err
	}
	res, err := r.list(ctx, listCaseTasksSQL, pgx.NamedArgs{
		"case_id": filter.CaseID, "statuses": statusCodes(filter.Statuses), "limit": filter.Limit, "offset": filter.Offset,
	})
	if err != nil {
		return ListResult{}, err
	}
	if res.OpenCount, err = countPendingTx(ctx, r.pool, filter.CaseID); err != nil {
		return ListResult{}, err
	}
	return res, nil
}

// ListMine returns a page of the caller's tasks across live cases.
func (r *PostgresRepository) ListMine(ctx context.Context, filter MineFilter) (ListResult, error) {
	return r.list(ctx, listMyTasksSQL, pgx.NamedArgs{
		"user_id": filter.UserID, "include_units": filter.IncludeUnits, "statuses": statusCodes(filter.Statuses),
		"limit": filter.Limit, "offset": filter.Offset,
	})
}

// list runs a paged task query and hydrates the types.
func (r *PostgresRepository) list(ctx context.Context, sql string, args pgx.NamedArgs) (ListResult, error) {
	rows, err := r.pool.Query(ctx, sql, args)
	if err != nil {
		return ListResult{}, fmt.Errorf("list tasks: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[taskListRow])
	if err != nil {
		return ListResult{}, fmt.Errorf("read tasks: %w", err)
	}
	res := ListResult{Tasks: make([]*Task, len(listRows))}
	for i := range listRows {
		t := listRows[i].Task
		res.Tasks[i] = &t
		res.TotalSize = listRows[i].TotalSize
	}
	return res, r.hydrate(ctx, res.Tasks)
}

// ListTypes returns the task type catalogue by code.
func (r *PostgresRepository) ListTypes(ctx context.Context, onlyActive bool) ([]*TaskType, error) {
	rows, err := r.pool.Query(ctx, listTypesSQL, pgx.NamedArgs{"only_active": onlyActive})
	if err != nil {
		return nil, fmt.Errorf("list task types: %w", err)
	}
	types, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[TaskType])
	if err != nil {
		return nil, fmt.Errorf("read task types: %w", err)
	}
	return types, nil
}

// hydrate fills the type of each task (the catalogue is small: one query per
// distinct type).
func (r *PostgresRepository) hydrate(ctx context.Context, tasks []*Task) error {
	types := map[uuid.UUID]*TaskType{}
	for _, t := range tasks {
		if types[t.TypeID] == nil {
			taskType, err := getType(ctx, r.pool, getTypeByIDSQL, pgx.NamedArgs{"id": t.TypeID})
			if err != nil {
				return fmt.Errorf("hydrate task type: %w", err)
			}
			types[t.TypeID] = taskType
		}
		t.Type = types[t.TypeID]
	}
	return nil
}

// statusCodes converts statuses to the smallint array the queries expect.
func statusCodes(statuses []Status) []int16 {
	codes := make([]int16, 0, len(statuses))
	for _, s := range statuses {
		codes = append(codes, int16(s))
	}
	return codes
}

// mapDBError translates pgx.ErrNoRows to core.ErrNotFound and an unknown
// assigned user (foreign key) to core.ErrInvalidInput.
func mapDBError(err error) error {
	return core.MapDBError(err, func(pgErr *pgconn.PgError) error {
		if pgErr.Code == core.PgForeignKeyViolation {
			return fmt.Errorf("%w: unknown assignee or reference (%s)", core.ErrInvalidInput, pgErr.ConstraintName)
		}
		return nil
	})
}
