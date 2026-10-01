package legacyimport

import (
	"fmt"
	"io"
	"maps"
	"slices"
)

// StageCounts are the counts of one stage: rows read from the source, rows
// written and rows left out by reason.
type StageCounts struct {
	// Read is the number of source rows read.
	Read int `json:"read"`
	// Written is the number of rows written to the target.
	Written int `json:"written"`
	// Skipped counts the rows left out, by reason.
	Skipped map[string]int `json:"skipped,omitempty"`
	// Adjusted counts the rows written with an adjustment (renamed, defaulted, ...), by kind.
	Adjusted map[string]int `json:"adjusted,omitempty"`
}

// adjust counts one row written with an adjustment.
func (c *StageCounts) adjust(kind string) {
	if c.Adjusted == nil {
		c.Adjusted = map[string]int{}
	}
	c.Adjusted[kind]++
}

// skip counts one row left out for reason.
func (c *StageCounts) skip(reason string) {
	if c.Skipped == nil {
		c.Skipped = map[string]int{}
	}
	c.Skipped[reason]++
}

// Report holds the counts of every stage, in the order they ran.
type Report struct {
	order  []string
	stages map[string]*StageCounts
}

// stage returns the counts of name, created on first use.
func (r *Report) stage(name string) *StageCounts {
	if r.stages == nil {
		r.stages = map[string]*StageCounts{}
	}
	if c, ok := r.stages[name]; ok {
		return c
	}
	c := &StageCounts{}
	r.stages[name] = c
	r.order = append(r.order, name)
	return c
}

// Counts returns the stage counts keyed by stage name (stored in import_batch.counts).
func (r *Report) Counts() map[string]*StageCounts {
	return maps.Clone(r.stages)
}

// Print writes one line per stage and per skip reason (counts only, never values).
func (r *Report) Print(w io.Writer) {
	for _, name := range r.order {
		c := r.stages[name]
		_, _ = fmt.Fprintf(w, "%-22s read=%-9d written=%d\n", name, c.Read, c.Written)
		for _, reason := range slices.Sorted(maps.Keys(c.Skipped)) {
			_, _ = fmt.Fprintf(w, "%-22s   skipped  %-46s %d\n", "", reason, c.Skipped[reason])
		}
		for _, kind := range slices.Sorted(maps.Keys(c.Adjusted)) {
			_, _ = fmt.Fprintf(w, "%-22s   adjusted %-46s %d\n", "", kind, c.Adjusted[kind])
		}
	}
}
