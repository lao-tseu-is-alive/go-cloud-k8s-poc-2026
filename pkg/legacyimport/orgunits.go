package legacyimport

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// UnitExternalRefPrefix prefixes the legacy IdOrgUnit in org_unit.external_ref.
const UnitExternalRefPrefix = "goeland:"

// UnitDissolutionReason is used when the source has no reason for an inactive unit.
const UnitDissolutionReason = "Inactive dans Goéland (import)"

// UnitTypeCodes maps the legacy TypeOrgUnit names to the seeded POC type codes.
var UnitTypeCodes = map[string]string{
	"Entreprise": "ENTERPRISE",
	"Direction":  "DIRECTION",
	"Service":    "SERVICE",
	"Office":     "OFFICE",
	"Division":   "DIVISION",
	"Bureau":     "BUREAU",
	"Unité":      "UNIT",
}

// UnitSourceSQL reads the structural columns of the legacy units only (never
// e-mails, managers or phones).
const UnitSourceSQL = `
SELECT o.idorgunit AS id, o.idparent AS parent_id, t.name AS type_name, btrim(o.name) AS label,
       -- a few legacy rows hold the literal text NULL instead of a missing sigle
       CASE WHEN upper(btrim(o.abreviation)) = 'NULL' THEN '' ELSE coalesce(btrim(o.abreviation), '') END AS abbreviation,
       o.isactive AS active,
       coalesce(btrim(o.reasondeath), '') AS reason
FROM org_unit o
JOIN type_org_unit t ON t.idtypeorgunit = o.idtypeorgunit;`

// SourceUnit is one legacy unit.
type SourceUnit struct {
	// ID is the legacy IdOrgUnit.
	ID int64 `db:"id"`
	// ParentID is the legacy parent id; nil for a root.
	ParentID *int64 `db:"parent_id"`
	// TypeName is the legacy type name (e.g. Service).
	TypeName string `db:"type_name"`
	// Label is the unit name.
	Label string `db:"label"`
	// Abbreviation is the unit sigle.
	Abbreviation string `db:"abbreviation"`
	// Active reports whether the unit is active in the source.
	Active bool `db:"active"`
	// Reason is the legacy dissolution reason.
	Reason string `db:"reason"`
}

// TopDown orders units parents first; among siblings, active units come first
// so they keep their label when an inactive homonym must be renamed. A unit
// whose parent is missing from the source becomes a root (its ParentID is
// cleared).
func TopDown(units []*SourceUnit) []*SourceUnit {
	byID := make(map[int64]bool, len(units))
	for _, u := range units {
		byID[u.ID] = true
	}
	children := map[int64][]*SourceUnit{}
	var queue []*SourceUnit
	for _, u := range units {
		if u.ParentID == nil || !byID[*u.ParentID] {
			u.ParentID = nil
			queue = append(queue, u)
		} else {
			children[*u.ParentID] = append(children[*u.ParentID], u)
		}
	}
	queue = activeFirst(queue)
	for i := 0; i < len(queue); i++ {
		queue = append(queue, activeFirst(children[queue[i].ID])...)
	}
	return queue
}

// activeFirst returns units with the active ones first, keeping their order.
func activeFirst(units []*SourceUnit) []*SourceUnit {
	out := make([]*SourceUnit, 0, len(units))
	for _, active := range []bool{true, false} {
		for _, u := range units {
			if u.Active == active {
				out = append(out, u)
			}
		}
	}
	return out
}

// UnitLabel is the label of a unit, a placeholder when the source has none.
func UnitLabel(u *SourceUnit) string {
	if u.Label == "" {
		return fmt.Sprintf("Unité %d", u.ID)
	}
	return u.Label
}

// liveUnits reports which units stay live: an active unit does, and so does an
// inactive one with a live sub-unit (the POC only dissolves a unit without
// live sub-units). ordered is parents first.
func liveUnits(ordered []*SourceUnit) map[int64]bool {
	live := make(map[int64]bool, len(ordered))
	for i := len(ordered) - 1; i >= 0; i-- {
		u := ordered[i]
		if u.Active {
			live[u.ID] = true
		}
		if live[u.ID] && u.ParentID != nil {
			live[*u.ParentID] = true
		}
	}
	return live
}

var orgUnitColumns = []string{
	"id", "org_unit_type_id", "abbreviation", "label", "parent_id", "external_ref",
	"dissolved_at", "dissolved_by", "dissolution_reason", "created_by",
}

// importOrgUnits writes the unit tree parents first, dissolving the inactive
// units without a live sub-unit; a live unit whose label is taken by a live
// sibling gets its legacy id appended.
func (imp *Importer) importOrgUnits(ctx context.Context, c *StageCounts) error {
	units, err := querySource[SourceUnit](ctx, imp, UnitSourceSQL)
	if err != nil {
		return err
	}
	c.Read = len(units)
	typeIDs, err := imp.codeIDs(ctx, `SELECT code, id FROM org_unit_type`)
	if err != nil {
		return err
	}
	ordered := TopDown(units)
	live := liveUnits(ordered)
	taken := map[string]bool{}
	subjects := make([]subject, 0, len(ordered))
	rows := make([][]any, 0, len(ordered))
	for _, u := range ordered {
		typeCode, ok := UnitTypeCodes[u.TypeName]
		if !ok {
			typeCode = "UNIT"
			c.adjust("unknown type, imported as UNIT")
		}
		label := imp.uniqueUnitLabel(u, live[u.ID], taken, c)
		s := newSubject(core.SubjectKindOrgUnit, "org_unit", u.ID, label)
		subjects = append(subjects, s)
		if live[u.ID] {
			imp.liveUnits[s.id] = true
		}
		rows = append(rows, imp.orgUnitRow(s.id, typeIDs[typeCode], u, label, live[u.ID], c))
	}
	if err := imp.writeSubjects(ctx, subjects); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "org_unit", orgUnitColumns, rows); err != nil {
		return err
	}
	c.Written = len(rows)
	return nil
}

// uniqueUnitLabel returns the label of a unit, made unique among the live
// siblings (the POC rule) by appending the legacy id.
func (imp *Importer) uniqueUnitLabel(u *SourceUnit, live bool, taken map[string]bool, c *StageCounts) string {
	label := UnitLabel(u)
	if !live {
		return label
	}
	parent := ""
	if u.ParentID != nil {
		parent = strconv.FormatInt(*u.ParentID, 10)
	}
	key := parent + "/" + strings.ToLower(label)
	if taken[key] {
		label = fmt.Sprintf("%s [%d]", label, u.ID)
		c.adjust("label taken by a live sibling, renamed")
	}
	taken[parent+"/"+strings.ToLower(label)] = true
	return label
}

// orgUnitRow is the org_unit row of a legacy unit.
func (imp *Importer) orgUnitRow(id, typeID uuid.UUID, u *SourceUnit, label string, live bool, c *StageCounts) []any {
	var parent *uuid.UUID
	if u.ParentID != nil {
		p := ID(string(core.SubjectKindOrgUnit), *u.ParentID)
		parent = &p
	}
	var dissolvedAt any
	dissolvedBy, reason := "", ""
	if !live {
		dissolvedAt, dissolvedBy, reason = imp.now, OperatorID, u.Reason
		if reason == "" {
			reason = UnitDissolutionReason
		}
	} else if !u.Active {
		c.adjust("inactive with live sub-units, kept live")
	}
	return []any{id, typeID, u.Abbreviation, label, parent, UnitExternalRefPrefix + strconv.FormatInt(u.ID, 10), dissolvedAt, dissolvedBy, reason, OperatorID}
}

// codeIDs reads a code → id catalogue from the target.
func (imp *Importer) codeIDs(ctx context.Context, query string) (map[string]uuid.UUID, error) {
	rows, err := imp.tx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read catalogue: %w", err)
	}
	defer rows.Close()
	out := map[string]uuid.UUID{}
	for rows.Next() {
		var code string
		var id uuid.UUID
		if err := rows.Scan(&code, &id); err != nil {
			return nil, err
		}
		out[code] = id
	}
	return out, rows.Err()
}
