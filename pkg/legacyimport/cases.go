package legacyimport

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const (
	// CaseRefNamespace holds the legacy case numbers as business references,
	// so a case is found by the number users know.
	CaseRefNamespace = "GOELAND"
	// confidentialTypeShare is the share of confidential cases from which a
	// case type gets the default confidentiality level (GLD-050).
	confidentialTypeShare = 0.95
	// closureReason is the reason of the cases closed in the legacy.
	closureReason = "Clôturée dans Goéland (import)"
)

// Case statuses (casefile.Status, kept as numbers to avoid importing the domain).
const (
	statusOpen      = 1
	statusSuspended = 3
	statusClosed    = 4
)

// grantLevels maps the legacy IdDroit to the POC levels; 5 (Aucun accès) has
// no equivalent (there is no deny level) and is left out.
var grantLevels = map[int16]core.Level{
	1: core.LevelFullControl, // Contrôle total
	2: core.LevelManage,      // Edition
	3: core.LevelContribute,  // Ajout suivis/documents
	4: core.LevelRead,        // Consultation affaire confidentielle
}

// sourceCaseType is a legacy case type with its share of confidential cases.
type sourceCaseType struct {
	// ID is the source column id.
	ID int64 `db:"id"`
	// Name is the source column name.
	Name string `db:"name"`
	// Description is the source column description.
	Description string `db:"description"`
	// Active is the source column active.
	Active bool `db:"active"`
	// ConfidentialShare is the source column confidential_share.
	ConfidentialShare float64 `db:"confidential_share"`
}

var caseTypeColumns = []string{"id", "code", "label", "description", "is_active", "default_confidentiality_level"}

// importCaseTypes writes the case types, coded LEG_<IdTypeAffaire>; a type
// whose cases are nearly all confidential becomes confidential by default.
func (imp *Importer) importCaseTypes(ctx context.Context, c *StageCounts) error {
	types, err := querySource[sourceCaseType](ctx, imp, caseTypesSQL)
	if err != nil {
		return err
	}
	c.Read = len(types)
	rows := make([][]any, 0, len(types))
	for _, t := range types {
		id := ID(keyCaseType, t.ID)
		label := t.Name
		if label == "" {
			label = fmt.Sprintf("Type %d", t.ID)
		}
		level := 0
		if t.ConfidentialShare > confidentialTypeShare {
			level = core.ConfidentialLevel
			c.adjust("confidential by default")
		}
		rows = append(rows, []any{id, "LEG_" + strconv.FormatInt(t.ID, 10), label, t.Description, t.Active, level})
		imp.caseTypes[t.ID] = id
	}
	if err := imp.copyRows(ctx, "case_type", caseTypeColumns, rows); err != nil {
		return err
	}
	c.Written = len(rows)
	return nil
}

// sourceCase is a legacy case.
type sourceCase struct {
	// ID is the source column id.
	ID int64 `db:"id"`
	// TypeID is the source column type_id.
	TypeID int64 `db:"type_id"`
	// Title is the source column title.
	Title string `db:"title"`
	// Description is the source column description.
	Description string `db:"description"`
	// Comment is the source column comment.
	Comment string `db:"comment"`
	// CreatedAt is the source column created_at.
	CreatedAt *time.Time `db:"created_at"`
	// UpdatedAt is the source column updated_at.
	UpdatedAt *time.Time `db:"updated_at"`
	// BeginAt is the source column begin_at.
	BeginAt *time.Time `db:"begin_at"`
	// EndAt is the source column end_at.
	EndAt *time.Time `db:"end_at"`
	// Suspended is the source column suspended.
	Suspended bool `db:"suspended"`
	// Terminated is the source column terminated.
	Terminated bool `db:"terminated"`
	// CreatorID is the source column creator_id.
	CreatorID *int64 `db:"creator_id"`
	// Confidential is the source column confidential.
	Confidential bool `db:"confidential"`
}

var caseFileColumns = []string{
	"id", "case_type_id", "title", "description", "status", "opened_at", "closed_at", "closed_by",
	"closure_reason", "metadata", "created_at", "created_by",
}

// importCases writes the cases with their status, confidentiality, dates and
// creator; the legacy number is the business reference (namespace GOELAND).
func (imp *Importer) importCases(ctx context.Context, c *StageCounts) error {
	cases, err := querySource[sourceCase](ctx, imp, casesSQL)
	if err != nil {
		return err
	}
	c.Read = len(cases)
	subjects := make([]subject, 0, len(cases))
	rows := make([][]any, 0, len(cases))
	for _, a := range cases {
		typeID, ok := imp.caseTypes[a.TypeID]
		if !ok {
			c.skip("unknown case type")
			continue
		}
		s := imp.caseSubject(a)
		subjects = append(subjects, s)
		rows = append(rows, imp.caseRow(s, typeID, a))
	}
	if err := imp.writeSubjects(ctx, subjects); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "case_file", caseFileColumns, rows); err != nil {
		return err
	}
	c.Written = len(rows)
	return nil
}

// caseSubject is the identity and governance of a legacy case.
func (imp *Importer) caseSubject(a *sourceCase) subject {
	title := a.Title
	if title == "" {
		title = fmt.Sprintf("Affaire %d", a.ID)
	}
	s := newSubject(core.SubjectKindCase, "affaire", a.ID, title)
	s.createdAt, s.updatedAt, s.createdBy = a.CreatedAt, a.UpdatedAt, imp.operator(a.CreatorID)
	s.businessRef, s.businessRefNS = strconv.FormatInt(a.ID, 10), CaseRefNamespace
	if a.Confidential {
		s.confidentiality = core.ConfidentialLevel
	}
	return s
}

// caseRow is the case_file row of a legacy case: terminated → CLOSED (also when
// suspended too), suspended → SUSPENDED, otherwise OPEN.
func (imp *Importer) caseRow(s subject, typeID uuid.UUID, a *sourceCase) []any {
	status, closedBy, reason := statusOpen, "", ""
	var closedAt *time.Time
	switch {
	case a.Terminated:
		status, closedBy, reason = statusClosed, OperatorID, closureReason
		closedAt = firstTime(a.EndAt, a.UpdatedAt, a.CreatedAt, &imp.now)
		imp.caseClosedAt[s.id] = *closedAt
	case a.Suspended:
		status = statusSuspended
	}
	metadata := map[string]any{}
	if a.Comment != "" {
		metadata["legacy_comment"] = a.Comment
	}
	return []any{
		s.id, typeID, s.label, a.Description, int16(status), firstTime(a.BeginAt, a.CreatedAt, &imp.now), closedAt, closedBy,
		reason, metadata, firstTime(a.CreatedAt, &imp.now), s.createdBy,
	}
}

// firstTime returns the first non-nil time.
func firstTime(times ...*time.Time) *time.Time {
	for _, t := range times {
		if t != nil {
			return t
		}
	}
	return nil
}

var accessGrantColumns = []string{"subject_id", "grantee_kind", "grantee_user_id", "grantee_subject_id", "level", "granted_by", "grant_reason"}

// grantReason is recorded on every imported grant.
const grantReason = "Importé de Goéland"

// importGrants writes the case grants: E/e → USER (the employee id), O →
// ORG_UNIT, G → GROUP, at the mapped level.
func (imp *Importer) importGrants(ctx context.Context, c *StageCounts) error {
	n, err := imp.copyFromSource(ctx, "access_grant", accessGrantColumns, grantsSQL, func(rows pgx.Rows) ([]any, error) {
		var caseID, granteeID int64
		var kind string
		var right int16
		if err := rows.Scan(&caseID, &kind, &granteeID, &right); err != nil {
			return nil, err
		}
		c.Read++
		return imp.grantRow(ID(string(core.SubjectKindCase), caseID), kind, granteeID, right, c), nil
	})
	c.Written = n
	return err
}

// grantRow maps one legacy grant, or returns nil (counted) to leave it out.
func (imp *Importer) grantRow(caseID uuid.UUID, kind string, granteeID int64, right int16, c *StageCounts) []any {
	level, ok := grantLevels[right]
	if !ok {
		c.skip(fmt.Sprintf("right %d (Aucun accès: no deny level)", right))
		return nil
	}
	if !imp.known(caseID, core.SubjectKindCase) {
		c.skip("case not imported")
		return nil
	}
	switch kind {
	case "E":
		if !imp.employees[granteeID] {
			c.skip("unknown employee")
			return nil
		}
		return []any{caseID, string(core.GranteeUser), strconv.FormatInt(granteeID, 10), nil, int16(level), OperatorID, grantReason}
	case "O", "G":
		subjectKind, granteeKind := core.SubjectKindOrgUnit, core.GranteeOrgUnit
		if kind == "G" {
			subjectKind, granteeKind = core.SubjectKindGroup, core.GranteeGroup
		}
		grantee := ID(string(subjectKind), granteeID)
		if !imp.known(grantee, subjectKind) {
			c.skip("unknown " + string(subjectKind))
			return nil
		}
		return []any{caseID, string(granteeKind), nil, grantee, int16(level), OperatorID, grantReason}
	default:
		c.skip("unknown grantee kind")
		return nil
	}
}
