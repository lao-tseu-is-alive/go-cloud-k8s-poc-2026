package legacyimport

import (
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// heartbeatRows is how many rows a COPY writes between two progress records.
const heartbeatRows = 250_000

// progress logs the steps of an import run: each step's start and end with its
// duration, each COPY's rows and rate, and a heartbeat inside long COPYs.
// Records carry names, durations and counts only, never source values.
type progress struct {
	log   *slog.Logger
	begin time.Time
	now   func() time.Time
}

// newProgress returns a progress logging to log (nil: discarded).
func newProgress(log *slog.Logger) *progress {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &progress{log: log, begin: time.Now(), now: time.Now}
}

// elapsed is the time since the run began, rounded for reading.
func (p *progress) elapsed() time.Duration {
	return p.now().Sub(p.begin).Round(time.Second)
}

// info logs one record with the run's elapsed time.
func (p *progress) info(message string, attrs ...any) {
	p.log.Info(message, append([]any{"elapsed", p.elapsed()}, attrs...)...)
}

// step logs the start of a step and returns the function that logs its end,
// with its duration and the attributes given then.
func (p *progress) step(name string, attrs ...any) func(endAttrs ...any) {
	p.info("step started", append([]any{"step", name}, attrs...)...)
	started := p.now()
	return func(endAttrs ...any) {
		base := []any{"step", name, "duration", p.now().Sub(started).Round(time.Millisecond)}
		p.info("step done", append(base, endAttrs...)...)
	}
}

// copySource wraps src so that the COPY into table logs a heartbeat every
// heartbeatRows rows; call done once the COPY returns.
func (p *progress) copySource(table string, src pgx.CopyFromSource) (wrapped pgx.CopyFromSource, done func(rows int64)) {
	c := &countingSource{CopyFromSource: src, p: p, table: table, started: p.now()}
	return c, func(rows int64) {
		duration := p.now().Sub(c.started)
		p.info("copy done", "table", table, "rows", rows,
			"duration", duration.Round(time.Millisecond), "rows_per_s", rate(rows, duration))
	}
}

// countingSource counts the rows a COPY pulls and logs a heartbeat.
type countingSource struct {
	pgx.CopyFromSource
	p       *progress
	table   string
	started time.Time
	rows    int64
}

// Next advances the wrapped source and logs every heartbeatRows rows.
func (c *countingSource) Next() bool {
	if !c.CopyFromSource.Next() {
		return false
	}
	c.rows++
	if c.rows%heartbeatRows == 0 {
		duration := c.p.now().Sub(c.started)
		c.p.info("copying", "table", c.table, "rows", c.rows, "rows_per_s", rate(c.rows, duration))
	}
	return true
}

// rate is rows per second over duration, 0 for an instant.
func rate(rows int64, duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	return int64(float64(rows) / duration.Seconds())
}
