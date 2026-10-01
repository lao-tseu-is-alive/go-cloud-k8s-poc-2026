package legacyimport

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// sourceEmployee is what a user needs from a legacy employee.
type sourceEmployee struct {
	// ID is the source column id.
	ID int64 `db:"id"`
	// FirstName is the source column first_name.
	FirstName string `db:"first_name"`
	// LastName is the source column last_name.
	LastName string `db:"last_name"`
	// Email is the source column email.
	Email string `db:"email"`
	// Active is the source column active.
	Active bool `db:"active"`
	// CreatedAt is the source column created_at.
	CreatedAt *time.Time `db:"created_at"`
	// UnitID is the source column unit_id.
	UnitID *int64 `db:"unit_id"`
}

// displayName is "<first> <last>", or a placeholder without a name.
func (e *sourceEmployee) displayName() string {
	name := strings.TrimSpace(e.FirstName + " " + e.LastName)
	if name == "" {
		return fmt.Sprintf("Employé %d", e.ID)
	}
	return name
}

var (
	appUserColumns      = []string{"user_id", "subject_id", "display_name", "email"}
	relationshipColumns = []string{"source_subject_id", "target_subject_id", "relationship_type_id", "valid_from", "valid_to", "created_at", "created_by"}
)

// importUsers writes every employee as a USER subject and an app_user whose
// user id is the legacy employee id; inactive employees are kept (they are
// creators, grantees and participants of past cases).
func (imp *Importer) importUsers(ctx context.Context, c *StageCounts) error {
	employees, err := querySource[sourceEmployee](ctx, imp, employeesSQL)
	if err != nil {
		return err
	}
	c.Read = len(employees)
	subjects := make([]subject, 0, len(employees))
	users := make([][]any, 0, len(employees))
	for _, e := range employees {
		s := newSubject(core.SubjectKindUser, "employe", e.ID, e.displayName())
		s.createdAt = e.CreatedAt
		subjects = append(subjects, s)
		users = append(users, []any{strconv.FormatInt(e.ID, 10), s.id, s.label, e.Email})
		imp.employees[e.ID] = true
		if e.Active {
			imp.activeEmployees[e.ID] = true
			if e.UnitID != nil {
				imp.employeeUnits[e.ID] = *e.UnitID
			}
		}
	}
	if err := imp.writeSubjects(ctx, subjects); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "app_user", appUserColumns, users); err != nil {
		return err
	}
	c.Written = len(users)
	return nil
}

// importUnitMemberships links every active employee to its direct unit.
func (imp *Importer) importUnitMemberships(ctx context.Context, c *StageCounts) error {
	typeID, err := imp.relType("USER_MEMBER_OF_ORG_UNIT")
	if err != nil {
		return err
	}
	rows := make([][]any, 0, len(imp.employeeUnits))
	for employee, unit := range imp.employeeUnits {
		c.Read++
		unitID := ID(string(core.SubjectKindOrgUnit), unit)
		if !imp.liveUnits[unitID] {
			c.skip("unit unknown or dissolved")
			continue
		}
		rows = append(rows, []any{ID(string(core.SubjectKindUser), employee), unitID, typeID, nil, nil, imp.now, OperatorID})
	}
	if err := imp.copyRows(ctx, "subject_relationship", relationshipColumns, rows); err != nil {
		return err
	}
	c.Written = len(rows)
	return nil
}

// sourceGroup is a legacy security group.
type sourceGroup struct {
	// ID is the source column id.
	ID int64 `db:"id"`
	// Name is the source column name.
	Name string `db:"name"`
	// Description is the source column description.
	Description string `db:"description"`
	// Active is the source column active.
	Active bool `db:"active"`
}

var securityGroupColumns = []string{"id", "name", "description", "archived_at", "archived_by"}

// importGroups writes the security groups; an inactive one is archived, and a
// live one whose name is taken (accent- and case-insensitively) gets its
// legacy id appended.
func (imp *Importer) importGroups(ctx context.Context, c *StageCounts) error {
	groups, err := querySource[sourceGroup](ctx, imp, groupsSQL)
	if err != nil {
		return err
	}
	c.Read = len(groups)
	taken := map[string]bool{}
	subjects := make([]subject, 0, len(groups))
	rows := make([][]any, 0, len(groups))
	for _, g := range groups {
		name, err := imp.uniqueGroupName(ctx, g, taken, c)
		if err != nil {
			return err
		}
		s := newSubject(core.SubjectKindGroup, "securite_groupe", g.ID, name)
		subjects = append(subjects, s)
		var archivedAt any
		archivedBy := ""
		if !g.Active {
			archivedAt, archivedBy = imp.now, OperatorID
		} else {
			imp.liveGroups[s.id] = true
		}
		rows = append(rows, []any{s.id, name, g.Description, archivedAt, archivedBy})
	}
	if err := imp.writeSubjects(ctx, subjects); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "security_group", securityGroupColumns, rows); err != nil {
		return err
	}
	c.Written = len(rows)
	return nil
}

// uniqueGroupName returns the name of a group, unique among the live groups.
func (imp *Importer) uniqueGroupName(ctx context.Context, g *sourceGroup, taken map[string]bool, c *StageCounts) (string, error) {
	name := g.Name
	if name == "" {
		name = fmt.Sprintf("Groupe %d", g.ID)
	}
	if !g.Active {
		return name, nil
	}
	var key string
	if err := imp.tx.QueryRow(ctx, `SELECT immutable_unaccent(lower($1))`, name).Scan(&key); err != nil {
		return "", fmt.Errorf("fold group name: %w", err)
	}
	if taken[key] {
		name = fmt.Sprintf("%s [%d]", name, g.ID)
		c.adjust("name taken by a live group, renamed")
	}
	taken[key] = true
	return name, nil
}

// importGroupMemberships links the active employees to their live groups;
// nested groups are not flattened in wave 1 (counted).
func (imp *Importer) importGroupMemberships(ctx context.Context, c *StageCounts) error {
	typeID, err := imp.relType("USER_MEMBER_OF_GROUP")
	if err != nil {
		return err
	}
	var nested int
	if err := imp.source.QueryRow(ctx, nestedGroupsSQL).Scan(&nested); err != nil {
		return fmt.Errorf("count nested groups: %w", err)
	}
	for range nested {
		c.skip("nested group (not flattened in wave 1)")
	}
	n, err := imp.copyFromSource(ctx, "subject_relationship", relationshipColumns, groupMembersSQL, func(rows pgx.Rows) ([]any, error) {
		var group, employee int64
		if err := rows.Scan(&group, &employee); err != nil {
			return nil, err
		}
		c.Read++
		groupID := ID(string(core.SubjectKindGroup), group)
		if !imp.activeEmployees[employee] {
			c.skip("employee unknown or inactive")
			return nil, nil
		}
		if !imp.liveGroups[groupID] {
			c.skip("group unknown or archived")
			return nil, nil
		}
		return []any{ID(string(core.SubjectKindUser), employee), groupID, typeID, nil, nil, imp.now, OperatorID}, nil
	})
	c.Written = n
	return err
}

// relType returns the id of a relationship type the target must hold.
func (imp *Importer) relType(code string) (uuid.UUID, error) {
	id, ok := imp.relTypes[code]
	if !ok {
		return uuid.Nil, fmt.Errorf("relationship type %s is missing from the target", code)
	}
	return id, nil
}
