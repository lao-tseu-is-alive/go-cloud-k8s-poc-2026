package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// storyMark pairs the database clock when a story step started (gen) with
// the story time of that step.
type storyMark struct {
	gen, story time.Time
}

// storyClock records the story steps of a demo generation (demodata.Options
// Clock), to re-date the generated history afterwards (GLD-079): the API
// stamps every record with the time of the generation and never backdates.
type storyClock struct {
	pool  *pgxpool.Pool
	marks []storyMark
}

// mark records that the step of story time story starts now.
func (c *storyClock) mark(ctx context.Context, story time.Time) error {
	var gen time.Time
	if err := c.pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&gen); err != nil {
		return err
	}
	c.marks = append(c.marks, storyMark{gen: gen, story: story})
	return nil
}

// errDemoSeedNotOwner explains why re-dating the demo needs the schema owner.
var errDemoSeedNotOwner = errors.New("demo-seed re-dates the generated history and must connect as the schema owner (the role that ran the migrations)")

// redateDemo moves every timestamp the generation wrote to the story time of
// the step that wrote it (the story time plus the delay since the step
// started), then derives what follows from those times: the retention dates
// and the reference counters. It runs in one transaction as the schema owner,
// with the tables' triggers disabled for the transaction only (the audit and
// timeline are immutable otherwise), in the brand-new demo database demo-seed
// just generated, before it is marked as a demo: it never runs on any other
// database, and the server has no such path.
func redateDemo(ctx context.Context, owner *pgxpool.Pool, marks []storyMark) error {
	if len(marks) == 0 {
		return nil
	}
	tx, err := owner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := createRedateFunction(ctx, tx, marks); err != nil {
		return err
	}
	tables, err := timestampColumns(ctx, tx)
	if err != nil {
		return err
	}
	if err := redateTables(ctx, tx, tables); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, redateDerivedSQL); err != nil {
		return fmt.Errorf("derive retention dates and reference counters: %w", err)
	}
	return tx.Commit(ctx)
}

// createRedateFunction loads the marks and defines pg_temp.demo_redate(t):
// the story time of the last step started at or before t plus the delay
// since, for any time from a day before the first step (the migrations of the
// run) to an hour after the last; other times (none expected) stay.
func createRedateFunction(ctx context.Context, tx pgx.Tx, marks []storyMark) error {
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE demo_marks (gen TIMESTAMPTZ PRIMARY KEY, story TIMESTAMPTZ NOT NULL) ON COMMIT DROP`); err != nil {
		return err
	}
	rows := make([][]any, len(marks))
	for i, m := range marks {
		rows[i] = []any{m.gen, m.story}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"demo_marks"}, []string{"gen", "story"}, pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("load the story marks: %w", err)
	}
	_, err := tx.Exec(ctx, `
CREATE FUNCTION pg_temp.demo_redate(t TIMESTAMPTZ) RETURNS TIMESTAMPTZ LANGUAGE sql STABLE AS $$
    SELECT CASE
        WHEN t IS NULL OR t < (SELECT min(gen) FROM pg_temp.demo_marks) - interval '1 day'
                       OR t > (SELECT max(gen) FROM pg_temp.demo_marks) + interval '1 hour' THEN t
        ELSE coalesce(
            (SELECT m.story + (t - m.gen) FROM pg_temp.demo_marks m WHERE m.gen <= t ORDER BY m.gen DESC LIMIT 1),
            (SELECT m.story + (t - m.gen) FROM pg_temp.demo_marks m ORDER BY m.gen LIMIT 1))
    END
$$`)
	return err
}

// timestampColumns lists the timestamptz columns of every table of the
// schema, by table (the migration bookkeeping and the demo marker excluded).
func timestampColumns(ctx context.Context, tx pgx.Tx) (map[string][]string, error) {
	rows, err := tx.Query(ctx, `
SELECT c.table_name, c.column_name
  FROM information_schema.columns c
  JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
 WHERE c.table_schema = 'public' AND t.table_type = 'BASE TABLE'
   AND c.data_type = 'timestamp with time zone' AND c.is_generated = 'NEVER'
   AND c.table_name NOT IN ('schema_migrations', 'demo_dataset')
 ORDER BY c.table_name, c.ordinal_position`)
	if err != nil {
		return nil, err
	}
	tables := map[string][]string{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			return nil, err
		}
		tables[table] = append(tables[table], column)
	}
	return tables, rows.Err()
}

// redateTables rewrites every timestamp column, one statement per table (so
// the per-row checks see consistent times), triggers disabled meanwhile.
func redateTables(ctx context.Context, tx pgx.Tx, tables map[string][]string) error {
	for table, columns := range tables {
		name := pgx.Identifier{table}.Sanitize()
		sets := make([]string, len(columns))
		for i, column := range columns {
			col := pgx.Identifier{column}.Sanitize()
			sets[i] = col + " = pg_temp.demo_redate(" + col + ")"
		}
		statements := []string{
			"ALTER TABLE " + name + " DISABLE TRIGGER USER",
			"UPDATE " + name + " SET " + strings.Join(sets, ", "),
			"ALTER TABLE " + name + " ENABLE TRIGGER USER",
		}
		for _, statement := range statements {
			if _, err := tx.Exec(ctx, statement); err != nil {
				if strings.Contains(err.Error(), "must be owner") {
					return errDemoSeedNotOwner
				}
				return fmt.Errorf("re-date %s: %w", table, err)
			}
		}
	}
	return nil
}

// redateDerivedSQL derives from the re-dated times what was computed from
// them: a retention starts on the (Zurich) day of the closing that opened it
// and lasts its duration; each reference counter continues after the highest
// number of its namespace and year (the stories give explicit references).
const redateDerivedSQL = `
UPDATE subject_retention
   SET starts_on = (created_at AT TIME ZONE 'Europe/Zurich')::date,
       until_on  = ((created_at AT TIME ZONE 'Europe/Zurich')::date + make_interval(years => duration_years))::date;

INSERT INTO business_ref_counter AS c (namespace, period, last_value)
SELECT business_ref_namespace, split_part(business_ref, '-', 1), max(split_part(business_ref, '-', 2)::bigint)
  FROM subject_ref
 WHERE business_ref_namespace <> '' AND business_ref ~ '^[0-9]{4}-[0-9]{6}$'
 GROUP BY 1, 2
ON CONFLICT (namespace, period) DO UPDATE SET last_value = greatest(c.last_value, EXCLUDED.last_value);`
