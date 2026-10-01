package access

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

// PostgresRepository stores grants and groups in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewPostgresRepository builds the repository on a shared pool. A nil logger
// falls back to slog.Default.
func NewPostgresRepository(pool *pgxpool.Pool, log *slog.Logger) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("access repository: pool is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &PostgresRepository{pool: pool, log: log}, nil
}

// MyAccess computes userID's effective level on subjectID.
func (r *PostgresRepository) MyAccess(ctx context.Context, userID string, subjectID uuid.UUID) (core.Access, error) {
	return core.EffectiveAccessTx(ctx, r.pool, userID, subjectID)
}

// ListGrants returns a subject's grants for a caller holding READ on it.
func (r *PostgresRepository) ListGrants(ctx context.Context, operatorID string, subjectID uuid.UUID, includeRevoked bool) ([]*Grant, error) {
	if err := core.EnsureAccessTx(ctx, r.pool, operatorID, subjectID, core.LevelRead); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, listGrantsSQL, pgx.NamedArgs{"subject_id": subjectID, "include_revoked": includeRevoked})
	if err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Grant])
}

// SetGrant gives the grantee a grant, or replaces its current one when the
// level differs (the old row is kept), with ACCESS_GRANTED / ACCESS_CHANGED.
func (r *PostgresRepository) SetGrant(ctx context.Context, in SetGrantInput) (*Grant, *core.AuditEvent, error) {
	var grant *Grant
	var ev *core.AuditEvent
	err := core.InTx(ctx, r.pool, "set grant", func(tx pgx.Tx) error {
		userID, subjectID, err := lockGrantChangeTx(ctx, tx, in)
		if err != nil {
			return err
		}
		current, err := collectGrant(tx.Query(ctx, currentGrantSQL, pgx.NamedArgs{
			"subject_id": in.SubjectID, "grantee_kind": in.GranteeKind, "grantee_user_id": userID, "grantee_subject_id": subjectID,
		}))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("current grant: %w", err)
		}
		if current != nil {
			if current.Level == in.Level {
				return fmt.Errorf("%w: the grantee already has %s", core.ErrConflict, in.Level)
			}
			if err := revokeTx(ctx, tx, current, in.OperatorID, "replaced: "+in.Reason); err != nil {
				return err
			}
		}
		grant, err = collectGrant(tx.Query(ctx, insertGrantSQL, pgx.NamedArgs{
			"subject_id": in.SubjectID, "grantee_kind": in.GranteeKind, "grantee_user_id": userID, "grantee_subject_id": subjectID,
			"level": int16(in.Level), "granted_by": in.OperatorID, "grant_reason": in.Reason,
		}))
		if err != nil {
			return fmt.Errorf("insert grant: %w", err)
		}
		ev, err = grantAuditTx(ctx, tx, current, grant, in.OperatorID, in.Reason)
		return err
	})
	return grant, ev, err
}

// lockGrantChangeTx checks the operator holds FULL_CONTROL on the subject,
// serializes the subject's grant changes and resolves the grantee (a user who
// signed in once, a live group or a live unit).
func lockGrantChangeTx(ctx context.Context, tx pgx.Tx, in SetGrantInput) (*string, *uuid.UUID, error) {
	if err := core.EnsureAccessTx(ctx, tx, in.OperatorID, in.SubjectID, core.LevelFullControl); err != nil {
		return nil, nil, err
	}
	if _, err := tx.Exec(ctx, lockSubjectGrantsSQL, pgx.NamedArgs{"subject_id": in.SubjectID}); err != nil {
		return nil, nil, fmt.Errorf("lock grants: %w", err)
	}
	if in.GranteeKind == GranteeUser {
		var known bool
		if err := tx.QueryRow(ctx, userKnownSQL, pgx.NamedArgs{"user_id": in.GranteeID}).Scan(&known); err != nil {
			return nil, nil, fmt.Errorf("check user: %w", err)
		}
		if !known {
			return nil, nil, fmt.Errorf("%w: user %s has never signed in", core.ErrNotFound, in.GranteeID)
		}
		return &in.GranteeID, nil, nil
	}
	id, err := uuid.Parse(in.GranteeID)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: grantee id must be a UUID", core.ErrInvalidInput)
	}
	if in.GranteeKind == GranteeOrgUnit {
		return nil, &id, core.EnsureLiveOrgUnitTx(ctx, tx, id)
	}
	return nil, &id, ensureLiveGroupTx(ctx, tx, id)
}

// RevokeGrant revokes a current grant (FULL_CONTROL on its subject) with ACCESS_REVOKED.
func (r *PostgresRepository) RevokeGrant(ctx context.Context, grantID uuid.UUID, operatorID, reason string) (*Grant, *core.AuditEvent, error) {
	var grant *Grant
	var ev *core.AuditEvent
	err := core.InTx(ctx, r.pool, "revoke grant", func(tx pgx.Tx) error {
		current, err := collectGrant(tx.Query(ctx, getGrantSQL, pgx.NamedArgs{"id": grantID}))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.RevokedAt != nil) {
			return fmt.Errorf("%w: no current grant %s", core.ErrNotFound, grantID)
		}
		if err != nil {
			return fmt.Errorf("get grant: %w", err)
		}
		if err := core.EnsureAccessTx(ctx, tx, operatorID, current.SubjectID, core.LevelFullControl); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, lockSubjectGrantsSQL, pgx.NamedArgs{"subject_id": current.SubjectID}); err != nil {
			return fmt.Errorf("lock grants: %w", err)
		}
		if err := revokeTx(ctx, tx, current, operatorID, reason); err != nil {
			return err
		}
		if grant, err = collectGrant(tx.Query(ctx, getGrantSQL, pgx.NamedArgs{"id": grantID})); err != nil {
			return fmt.Errorf("get grant: %w", err)
		}
		ev, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: grant.SubjectID, EventType: EventAccessRevoked, ActorUserID: operatorID, Reason: reason,
			BeforeState: grantState(current),
		})
		return err
	})
	return grant, ev, err
}

// revokeTx stamps the revocation; removing the last FULL_CONTROL is refused.
func revokeTx(ctx context.Context, tx pgx.Tx, g *Grant, operatorID, reason string) error {
	if _, err := tx.Exec(ctx, revokeGrantSQL, pgx.NamedArgs{"id": g.ID, "revoked_by": operatorID, "revoke_reason": reason}); err != nil {
		return fmt.Errorf("revoke grant: %w", err)
	}
	if g.Level != core.LevelFullControl {
		return nil
	}
	var left int
	if err := tx.QueryRow(ctx, countFullControlSQL, pgx.NamedArgs{"subject_id": g.SubjectID}).Scan(&left); err != nil {
		return fmt.Errorf("count FULL_CONTROL grants: %w", err)
	}
	if left == 0 {
		return fmt.Errorf("%w: the subject would lose its last FULL_CONTROL grant; give FULL_CONTROL to someone else first", core.ErrInvalidState)
	}
	return nil
}

// grantAuditTx writes ACCESS_GRANTED, or ACCESS_CHANGED when a grant was replaced.
func grantAuditTx(ctx context.Context, tx pgx.Tx, previous, grant *Grant, operatorID, reason string) (*core.AuditEvent, error) {
	ev := core.AuditEvent{SubjectID: grant.SubjectID, EventType: EventAccessGranted, ActorUserID: operatorID, Reason: reason, AfterState: grantState(grant)}
	if previous != nil {
		ev.EventType, ev.BeforeState = EventAccessChanged, grantState(previous)
	}
	return core.InsertAuditEventTx(ctx, tx, ev)
}

// grantState is the audited form of a grant.
func grantState(g *Grant) map[string]any {
	return map[string]any{"grant_id": g.ID.String(), "grantee_kind": string(g.GranteeKind), "grantee_id": g.GranteeID(), "level": g.Level.String()}
}

// ensureLiveGroupTx requires id to name a live security group (ErrInvalidInput otherwise).
func ensureLiveGroupTx(ctx context.Context, q core.Querier, id uuid.UUID) error {
	var live bool
	err := q.QueryRow(ctx, liveGroupSQL, pgx.NamedArgs{"id": id}).Scan(&live)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !live) {
		return fmt.Errorf("%w: group %s is unknown or archived", core.ErrInvalidInput, id)
	}
	return err
}

// --- groups -------------------------------------------------------------------------------

// ListGroups returns the groups matching query, by name.
func (r *PostgresRepository) ListGroups(ctx context.Context, query string, includeArchived bool) ([]*Group, error) {
	rows, err := r.pool.Query(ctx, listGroupsSQL, pgx.NamedArgs{"query": query, "include_archived": includeArchived})
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	groups, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Group])
	if err != nil {
		return nil, err
	}
	return groups, r.hydrate(ctx, groups)
}

// GetGroup returns a group (READ on it) and its current members.
func (r *PostgresRepository) GetGroup(ctx context.Context, operatorID string, id uuid.UUID) (*Group, []*Member, error) {
	if err := core.EnsureAccessTx(ctx, r.pool, operatorID, id, core.LevelRead); err != nil {
		return nil, nil, err
	}
	group, err := collectGroup(r.pool.Query(ctx, getGroupSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	if err := r.hydrate(ctx, []*Group{group}); err != nil {
		return nil, nil, err
	}
	rows, err := r.pool.Query(ctx, listMembersSQL, pgx.NamedArgs{"group_id": id})
	if err != nil {
		return nil, nil, fmt.Errorf("list members: %w", err)
	}
	members, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Member])
	return group, members, err
}

// CreateGroup creates the GROUP subject (its creator gets FULL_CONTROL), the
// group row and GROUP_CREATED.
func (r *PostgresRepository) CreateGroup(ctx context.Context, in GroupInput) (*Group, *core.AuditEvent, error) {
	var group *Group
	var ev *core.AuditEvent
	err := core.InTx(ctx, r.pool, "create group", func(tx pgx.Tx) error {
		ref, err := core.InsertSubjectRefTx(ctx, tx, core.SubjectKindGroup, in.Name, "")
		if err != nil {
			return fmt.Errorf("insert group subject: %w", err)
		}
		gov := core.CreateSubjectInput{Kind: core.SubjectKindGroup, DisplayLabel: in.Name, OperatorID: in.OperatorID, OwnerUserID: in.OperatorID}
		if _, err := core.InsertRecordMetadataTx(ctx, tx, gov, ref.ID); err != nil {
			return fmt.Errorf("insert group record_metadata: %w", err)
		}
		if group, err = collectGroup(tx.Query(ctx, insertGroupSQL, pgx.NamedArgs{"id": ref.ID, "name": in.Name, "description": in.Description})); err != nil {
			return mapDBError(err)
		}
		group.Subject = ref
		ev, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: ref.ID, EventType: EventGroupCreated, ActorUserID: in.OperatorID, AfterState: groupState(group),
		})
		return err
	})
	return group, ev, err
}

// UpdateGroup renames or redescribes a live group (MANAGE on it), with GROUP_UPDATED.
func (r *PostgresRepository) UpdateGroup(ctx context.Context, id uuid.UUID, in GroupInput) (*Group, *core.AuditEvent, error) {
	var group *Group
	var ev *core.AuditEvent
	err := core.InTx(ctx, r.pool, "update group", func(tx pgx.Tx) error {
		before, err := lockLiveGroupTx(ctx, tx, in.OperatorID, id, core.LevelManage)
		if err != nil {
			return err
		}
		if group, err = collectGroup(tx.Query(ctx, updateGroupSQL, pgx.NamedArgs{"id": id, "name": in.Name, "description": in.Description})); err != nil {
			return mapDBError(err)
		}
		if err := core.UpdateSubjectLabelTx(ctx, tx, id, in.Name); err != nil {
			return fmt.Errorf("rename group subject: %w", err)
		}
		ev, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: id, EventType: EventGroupUpdated, ActorUserID: in.OperatorID, Reason: in.Reason,
			BeforeState: groupState(before), AfterState: groupState(group),
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return group, ev, r.hydrate(ctx, []*Group{group})
}

// ArchiveGroup archives a live group (FULL_CONTROL on it), with GROUP_ARCHIVED;
// its grants stop applying.
func (r *PostgresRepository) ArchiveGroup(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Group, *core.AuditEvent, error) {
	var group *Group
	var ev *core.AuditEvent
	err := core.InTx(ctx, r.pool, "archive group", func(tx pgx.Tx) error {
		before, err := lockLiveGroupTx(ctx, tx, operatorID, id, core.LevelFullControl)
		if err != nil {
			return err
		}
		if group, err = collectGroup(tx.Query(ctx, archiveGroupSQL, pgx.NamedArgs{"id": id, "archived_by": operatorID})); err != nil {
			return mapDBError(err)
		}
		ev, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: id, EventType: EventGroupArchived, ActorUserID: operatorID, Reason: reason,
			BeforeState: groupState(before), AfterState: groupState(group),
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return group, ev, r.hydrate(ctx, []*Group{group})
}

// AddMember links a known user to a live group (MANAGE on it); the
// RELATIONSHIP_LINKED event is on the user's subject.
func (r *PostgresRepository) AddMember(ctx context.Context, in MemberInput) (*Member, *core.AuditEvent, error) {
	var member *Member
	var ev *core.AuditEvent
	err := core.InTx(ctx, r.pool, "add group member", func(tx pgx.Tx) error {
		if _, err := lockLiveGroupTx(ctx, tx, in.OperatorID, in.GroupID, core.LevelManage); err != nil {
			return err
		}
		userSubject, err := userSubjectTx(ctx, tx, in.UserID)
		if err != nil {
			return err
		}
		link := core.LinkInput{SourceSubjectID: userSubject, TargetSubjectID: in.GroupID, RelationshipTypeCode: "USER_MEMBER_OF_GROUP", OperatorID: in.OperatorID}
		rel, err := core.LinkSubjectsTx(ctx, tx, link)
		if err != nil {
			return err
		}
		if ev, err = core.InsertAuditEventTx(ctx, tx, core.LinkedEvent(link, rel)); err != nil {
			return err
		}
		member, err = collectMember(tx.Query(ctx, getMemberSQL, pgx.NamedArgs{"group_id": in.GroupID, "user_id": in.UserID}))
		return err
	})
	return member, ev, err
}

// RemoveMember ends a membership (MANAGE on the group), kept as history.
func (r *PostgresRepository) RemoveMember(ctx context.Context, in MemberInput) (*core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := core.InTx(ctx, r.pool, "remove group member", func(tx pgx.Tx) error {
		if _, err := lockLiveGroupTx(ctx, tx, in.OperatorID, in.GroupID, core.LevelManage); err != nil {
			return err
		}
		member, err := collectMember(tx.Query(ctx, getMemberSQL, pgx.NamedArgs{"group_id": in.GroupID, "user_id": in.UserID}))
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %s is not a member", core.ErrNotFound, in.UserID)
		}
		if err != nil {
			return fmt.Errorf("get member: %w", err)
		}
		_, ev, err = core.EndRelationshipTx(ctx, tx, core.EndInput{RelationshipID: member.RelationshipID, OperatorID: in.OperatorID, Reason: in.Reason})
		return err
	})
	return ev, err
}

// lockLiveGroupTx checks the operator's level on the group and locks the live group row.
func lockLiveGroupTx(ctx context.Context, tx pgx.Tx, operatorID string, id uuid.UUID, need core.Level) (*Group, error) {
	if err := core.EnsureAccessTx(ctx, tx, operatorID, id, need); err != nil {
		return nil, err
	}
	group, err := collectGroup(tx.Query(ctx, lockGroupSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, mapDBError(err)
	}
	if group.ArchivedAt != nil {
		return nil, fmt.Errorf("%w: the group is archived", core.ErrInvalidState)
	}
	return group, nil
}

// userSubjectTx returns the USER subject of a user who signed in once.
func userSubjectTx(ctx context.Context, q core.Querier, userID string) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, userSubjectSQL, pgx.NamedArgs{"user_id": userID}).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("%w: user %s has never signed in", core.ErrNotFound, userID)
	}
	return id, err
}

// hydrate attaches each group's subject_ref.
func (r *PostgresRepository) hydrate(ctx context.Context, groups []*Group) error {
	if len(groups) == 0 {
		return nil
	}
	headers, err := core.GetSubjectHeadersTx(ctx, r.pool, core.IDsOf(groups, func(g *Group) uuid.UUID { return g.ID }))
	if err != nil {
		return fmt.Errorf("hydrate groups: %w", err)
	}
	for _, g := range groups {
		g.Subject = headers.Refs[g.ID]
	}
	return nil
}

// groupState is the audited form of a group.
func groupState(g *Group) map[string]any {
	return map[string]any{"name": g.Name, "description": g.Description, "archived": g.ArchivedAt != nil}
}

// collectGrant, collectGroup and collectMember read exactly one row.
func collectGrant(rows pgx.Rows, err error) (*Grant, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Grant])
}

func collectGroup(rows pgx.Rows, err error) (*Group, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Group])
}

func collectMember(rows pgx.Rows, err error) (*Member, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Member])
}

// mapDBError translates a missing row and a duplicate live group name.
func mapDBError(err error) error {
	return core.MapDBError(err, func(pgErr *pgconn.PgError) error {
		if pgErr.Code == core.PgUniqueViolation {
			return fmt.Errorf("%w: a live group already has this name", core.ErrConflict)
		}
		return nil
	})
}
