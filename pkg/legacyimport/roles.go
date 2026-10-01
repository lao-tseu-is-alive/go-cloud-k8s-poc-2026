package legacyimport

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// roleFamily describes how the legacy roles of one kind of participant on a
// subject (a case, a thing, a document) become relationship types and edges.
type roleFamily struct {
	// prefix starts the type codes (CASE_HAS_ACTOR_, ...).
	prefix string
	// sourceKind is the kind of the subject (CASE when unset); targetKind the participant's.
	sourceKind, targetKind core.SubjectKind
	// rolesSQL lists the legacy roles (id, name); edgesSQL the participations.
	rolesSQL, edgesSQL string
	// existing maps legacy role names to the seeded types they become.
	existing map[string]string
	// label and inverse build the labels of a created type from the role name.
	label, inverse string
}

var (
	actorRoles = roleFamily{
		prefix: "CASE_HAS_ACTOR_", targetKind: core.SubjectKindActor, rolesSQL: actorRolesSQL, edgesSQL: caseActorRolesSQL,
		existing: map[string]string{"Requérant": "CASE_HAS_ACTOR_REQUESTER", "Propriétaire": "CASE_HAS_ACTOR_OWNER", "Mandataire": "CASE_HAS_ACTOR_MANDATEE"},
		label:    "Affaire a %s", inverse: "Acteur %s dans affaire",
	}
	userRoles = roleFamily{
		prefix: "CASE_HAS_USER_", targetKind: core.SubjectKindUser, rolesSQL: userRolesSQL, edgesSQL: caseUserRolesSQL,
		label: "Affaire a %s (collaborateur)", inverse: "Collaborateur %s dans affaire",
	}
	unitRoles = roleFamily{
		prefix: "CASE_HAS_ORG_UNIT_", targetKind: core.SubjectKindOrgUnit, rolesSQL: unitRolesSQL, edgesSQL: caseUnitRolesSQL,
		existing: map[string]string{"Leader": "CASE_HAS_ORG_UNIT_LEADER", "Gestionnaire": "CASE_HAS_ORG_UNIT_MANAGER", "Participe": "CASE_HAS_ORG_UNIT_PARTICIPANT"},
		label:    "Affaire avec unité %s", inverse: "Unité %s de",
	}
)

// source is the kind of the subject the participants belong to.
func (f roleFamily) source() core.SubjectKind {
	if f.sourceKind == "" {
		return core.SubjectKindCase
	}
	return f.sourceKind
}

func (imp *Importer) importCaseActorRoles(ctx context.Context, c *StageCounts) error {
	return imp.importRoles(ctx, actorRoles, c)
}

func (imp *Importer) importCaseUserRoles(ctx context.Context, c *StageCounts) error {
	return imp.importRoles(ctx, userRoles, c)
}

func (imp *Importer) importCaseUnitRoles(ctx context.Context, c *StageCounts) error {
	return imp.importRoles(ctx, unitRoles, c)
}

// importRoles creates the relationship types of a family, then writes one edge
// per participation, from the case to the participant, with its period. Only
// one open edge may join a case, a participant and a role: further open
// duplicates are left out.
func (imp *Importer) importRoles(ctx context.Context, f roleFamily, c *StageCounts) error {
	types, err := imp.roleTypes(ctx, f, c)
	if err != nil {
		return err
	}
	open := map[[3]uuid.UUID]bool{}
	n, err := imp.copyFromSource(ctx, "subject_relationship", relationshipColumns, f.edgesSQL, func(rows pgx.Rows) ([]any, error) {
		var caseID, partyID, roleID int64
		var created, from, to *time.Time
		if err := rows.Scan(&caseID, &partyID, &roleID, &created, &from, &to); err != nil {
			return nil, err
		}
		c.Read++
		return imp.roleRow(f, types, open, edge{caseID, partyID, roleID, created, from, to}, c), nil
	})
	c.Written = n
	return err
}

// edge is one legacy participation.
type edge struct {
	caseID, partyID, roleID int64
	created, from, to       *time.Time
}

// roleRow maps one participation, or returns nil (counted) to leave it out.
func (imp *Importer) roleRow(f roleFamily, types map[int64]uuid.UUID, open map[[3]uuid.UUID]bool, e edge, c *StageCounts) []any {
	typeID, ok := types[e.roleID]
	if !ok {
		c.skip("unknown role")
		return nil
	}
	sourceKind := f.source()
	source, target := ID(string(sourceKind), e.caseID), ID(string(f.targetKind), e.partyID)
	if !imp.known(source, sourceKind) || !imp.known(target, f.targetKind) {
		c.skip("subject or participant not imported")
		return nil
	}
	to := e.to
	if f.targetKind == core.SubjectKindOrgUnit && !imp.liveUnits[target] && to == nil {
		to = imp.endOfParticipation(source)
		c.adjust("open participation of a dissolved unit, ended")
	}
	from := e.from
	if from != nil && to != nil && to.Before(*from) {
		from = nil
		c.adjust("ended before it started, start dropped")
	}
	if to == nil {
		key := [3]uuid.UUID{source, target, typeID}
		if open[key] {
			c.skip("duplicate open participation")
			return nil
		}
		open[key] = true
	}
	return []any{source, target, typeID, from, to, firstTime(e.created, &imp.now), OperatorID}
}

// endOfParticipation is when an open participation of a dissolved unit ends:
// the closing of its case, or the import for a case still open.
func (imp *Importer) endOfParticipation(caseID uuid.UUID) *time.Time {
	if closed, ok := imp.caseClosedAt[caseID]; ok {
		return &closed
	}
	return &imp.now
}

// roleTypes maps each legacy role of a family to a relationship type: a
// seeded one when listed in existing, otherwise one created as
// <prefix><ROLE NAME IN CAPITALS>.
func (imp *Importer) roleTypes(ctx context.Context, f roleFamily, c *StageCounts) (map[int64]uuid.UUID, error) {
	rows, err := imp.source.Query(ctx, f.rolesSQL)
	if err != nil {
		return nil, fmt.Errorf("read roles: %w", err)
	}
	roles, err := pgx.CollectRows(rows, pgx.RowToStructByPos[struct {
		ID   int64
		Name string
	}])
	if err != nil {
		return nil, fmt.Errorf("scan roles: %w", err)
	}
	out := make(map[int64]uuid.UUID, len(roles))
	for _, r := range roles {
		code := f.existing[r.Name]
		if code == "" {
			code = f.prefix + roleCode(r.Name, r.ID)
		}
		id, err := imp.ensureRelationshipType(ctx, f, code, r.Name, c)
		if err != nil {
			return nil, err
		}
		out[r.ID] = id
	}
	return out, nil
}

// ensureRelationshipType returns the id of code, creating the type from the role name.
func (imp *Importer) ensureRelationshipType(ctx context.Context, f roleFamily, code, role string, c *StageCounts) (uuid.UUID, error) {
	if id, ok := imp.relTypes[code]; ok {
		return id, nil
	}
	name := strings.ToLower(role)
	var id uuid.UUID
	if err := imp.tx.QueryRow(ctx, insertRelationshipTypeSQL, pgx.NamedArgs{
		"code": code, "label": fmt.Sprintf(f.label, name), "source_kind": string(f.source()),
		"target_kind": string(f.targetKind), "inverse_label": fmt.Sprintf(f.inverse, name),
		"description": "Rôle « " + role + " » de Goéland (import)",
	}).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("create relationship type %s: %w", code, err)
	}
	imp.relTypes[code] = id
	c.adjust("relationship type created")
	return id, nil
}

// roleCode turns a role name into a code part: capitals, digits and
// underscores, accents folded; the legacy id when nothing is left.
func roleCode(name string, id int64) string {
	folded := strings.NewReplacer(
		"à", "a", "â", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e", "î", "i", "ï", "i",
		"ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c",
	).Replace(strings.ToLower(name))
	var b strings.Builder
	underscore := false
	for _, r := range folded {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
			underscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			underscore = false
		case !underscore && b.Len() > 0:
			b.WriteByte('_')
			underscore = true
		}
	}
	code := strings.TrimSuffix(b.String(), "_")
	if code == "" {
		return fmt.Sprintf("ROLE_%d", id)
	}
	return code
}

// link describes a legacy link table that becomes edges of one relationship
// type: edgesSQL returns (source id, target id) pairs.
type link struct {
	typeCode               string
	sourceKind, targetKind core.SubjectKind
	edgesSQL               string
}

// importLink writes one open edge per pair of a link table; pairs naming a
// subject left out and repeated pairs are counted.
func (imp *Importer) importLink(ctx context.Context, l link, c *StageCounts) error {
	typeID, err := imp.relType(l.typeCode)
	if err != nil {
		return err
	}
	seen := map[[2]uuid.UUID]bool{}
	n, err := imp.copyFromSource(ctx, "subject_relationship", relationshipColumns, l.edgesSQL, func(rows pgx.Rows) ([]any, error) {
		var sourceID, targetID int64
		if err := rows.Scan(&sourceID, &targetID); err != nil {
			return nil, err
		}
		c.Read++
		source, target := ID(string(l.sourceKind), sourceID), ID(string(l.targetKind), targetID)
		if !imp.known(source, l.sourceKind) || !imp.known(target, l.targetKind) {
			c.skip("subject not imported")
			return nil, nil
		}
		key := [2]uuid.UUID{source, target}
		if seen[key] {
			c.skip("repeated link")
			return nil, nil
		}
		seen[key] = true
		return []any{source, target, typeID, nil, nil, imp.now, OperatorID}, nil
	})
	c.Written = n
	return err
}
