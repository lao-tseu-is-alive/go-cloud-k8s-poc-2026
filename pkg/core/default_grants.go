package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MaxDefaultGrants bounds a default grant template.
const MaxDefaultGrants = 50

// DefaultGrant is one line of a default grant template (GLD-050): the grants
// copied once onto a new subject, next to its creator's FULL_CONTROL and its
// owning unit's MANAGE. GranteeCreatorUnits stands for the units the creator
// is a direct member of at that moment.
type DefaultGrant struct {
	// GranteeKind is a user, a group, an org unit or GranteeCreatorUnits.
	GranteeKind GranteeKind `db:"grantee_kind"`
	// GranteeUserID is the user of a USER line.
	GranteeUserID *string `db:"grantee_user_id"`
	// GranteeSubjectID is the group or unit of a GROUP or ORG_UNIT line.
	GranteeSubjectID *uuid.UUID `db:"grantee_subject_id"`
	// Level is the level given (READ to FULL_CONTROL).
	Level Level `db:"level"`
	// GranteeLabel is the grantee's display name on read paths.
	GranteeLabel string `db:"grantee_label"`
}

// GranteeID is the user id or the subject id of the line ("" for creator units).
func (g DefaultGrant) GranteeID() string {
	switch {
	case g.GranteeUserID != nil:
		return *g.GranteeUserID
	case g.GranteeSubjectID != nil:
		return g.GranteeSubjectID.String()
	default:
		return ""
	}
}

// State is the logged form of the line.
func (g DefaultGrant) State() map[string]any {
	return map[string]any{"grantee_kind": string(g.GranteeKind), "grantee_id": g.GranteeID(), "level": g.Level.String()}
}

// NewDefaultGrant builds a template line from an API grantee kind and id,
// checking its shape (not that the grantee exists, see ValidateDefaultGrantsTx).
func NewDefaultGrant(kind GranteeKind, granteeID string, level Level) (DefaultGrant, error) {
	g := DefaultGrant{GranteeKind: kind, Level: level}
	if level < LevelRead || level > LevelFullControl {
		return g, fmt.Errorf("%w: a default grant gives READ to FULL_CONTROL", ErrInvalidInput)
	}
	granteeID = strings.TrimSpace(granteeID)
	switch kind {
	case GranteeUser:
		if granteeID == "" {
			return g, fmt.Errorf("%w: a user default grant names the user", ErrInvalidInput)
		}
		g.GranteeUserID = &granteeID
	case GranteeGroup, GranteeOrgUnit:
		id, err := uuid.Parse(granteeID)
		if err != nil {
			return g, fmt.Errorf("%w: invalid %s id %q", ErrInvalidInput, strings.ToLower(string(kind)), granteeID)
		}
		g.GranteeSubjectID = &id
	case GranteeCreatorUnits:
		if granteeID != "" {
			return g, fmt.Errorf("%w: the creator's units name no grantee", ErrInvalidInput)
		}
	default:
		return g, fmt.Errorf("%w: unknown grantee kind %q", ErrInvalidInput, kind)
	}
	return g, nil
}

// ValidateDefaultGrantsTx checks a whole template: at most MaxDefaultGrants
// lines, one per grantee, each naming a known user, a live group or a live
// org unit.
func ValidateDefaultGrantsTx(ctx context.Context, q Querier, template []DefaultGrant) error {
	if len(template) > MaxDefaultGrants {
		return fmt.Errorf("%w: at most %d default grants", ErrInvalidInput, MaxDefaultGrants)
	}
	seen := make(map[string]bool, len(template))
	for _, g := range template {
		key := string(g.GranteeKind) + ":" + g.GranteeID()
		if seen[key] {
			return fmt.Errorf("%w: grantee %s appears twice", ErrInvalidInput, key)
		}
		seen[key] = true
		if err := ensureLiveGranteeTx(ctx, q, g); err != nil {
			return err
		}
	}
	return nil
}

// ensureLiveGranteeTx requires the grantee of a template line to exist and be live.
func ensureLiveGranteeTx(ctx context.Context, q Querier, g DefaultGrant) error {
	var query string
	var args pgx.NamedArgs
	switch g.GranteeKind {
	case GranteeUser:
		query, args = knownUserSQL, pgx.NamedArgs{"user_id": *g.GranteeUserID}
	case GranteeGroup:
		query, args = liveGroupGranteeSQL, pgx.NamedArgs{"id": *g.GranteeSubjectID}
	case GranteeOrgUnit:
		query, args = liveUnitGranteeSQL, pgx.NamedArgs{"id": *g.GranteeSubjectID}
	default:
		return nil
	}
	var ok bool
	if err := q.QueryRow(ctx, query, args).Scan(&ok); err != nil {
		return fmt.Errorf("check grantee: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: grantee %s %s is unknown or inactive", ErrInvalidInput, g.GranteeKind, g.GranteeID())
	}
	return nil
}

// ApplyDefaultGrantsTx copies template onto a new subject created by
// operatorID and returns the grants written, for the creation's audit event.
// Creator units are resolved now; a grantee already holding a grant (the
// creator, the owning unit) keeps the highest level; a group archived or a
// unit dissolved since the template was set gets nothing.
func ApplyDefaultGrantsTx(ctx context.Context, q Querier, subjectID uuid.UUID, operatorID, reason string, template []DefaultGrant) ([]map[string]any, error) {
	resolved, err := resolveCreatorUnitsTx(ctx, q, operatorID, template)
	if err != nil {
		return nil, err
	}
	applied := make([]map[string]any, 0, len(resolved))
	for _, g := range resolved {
		args := pgx.NamedArgs{
			"subject_id": subjectID, "grantee_kind": string(g.GranteeKind), "level": int16(g.Level),
			"granted_by": operatorID, "reason": reason,
		}
		query := upsertDefaultSubjectGrantSQL
		if g.GranteeKind == GranteeUser {
			query, args["user_id"] = upsertDefaultUserGrantSQL, *g.GranteeUserID
		} else {
			args["grantee_subject_id"] = *g.GranteeSubjectID
		}
		tag, err := q.Exec(ctx, query, args)
		if err != nil {
			return nil, fmt.Errorf("apply default grant: %w", err)
		}
		if tag.RowsAffected() > 0 {
			applied = append(applied, g.State())
		}
	}
	return applied, nil
}

// resolveCreatorUnitsTx replaces a GranteeCreatorUnits line by one ORG_UNIT
// line per live unit the creator is a direct member of.
func resolveCreatorUnitsTx(ctx context.Context, q Querier, operatorID string, template []DefaultGrant) ([]DefaultGrant, error) {
	out := make([]DefaultGrant, 0, len(template))
	for _, g := range template {
		if g.GranteeKind != GranteeCreatorUnits {
			out = append(out, g)
			continue
		}
		rows, err := q.Query(ctx, creatorUnitsSQL, pgx.NamedArgs{"user_id": operatorID})
		if err != nil {
			return nil, fmt.Errorf("creator units: %w", err)
		}
		units, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return nil, fmt.Errorf("creator units: %w", err)
		}
		for _, id := range units {
			out = append(out, DefaultGrant{GranteeKind: GranteeOrgUnit, GranteeSubjectID: &id, Level: g.Level})
		}
	}
	return out, nil
}
