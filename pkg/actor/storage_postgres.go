package actor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// PostgresRepository implements Repository with pgx, composing core primitives.
type PostgresRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewPostgresRepository builds a PostgresRepository from a connection pool.
func NewPostgresRepository(pool *pgxpool.Pool, log *slog.Logger) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &PostgresRepository{pool: pool, log: log}, nil
}

// Create inserts an actor + its subject_ref + record_metadata + contacts + audit
// event in a single transaction.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Actor, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin create actor: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ref, err := core.InsertSubjectRefTx(ctx, tx, core.SubjectKindActor, in.DisplayName, "")
	if err != nil {
		return nil, nil, fmt.Errorf("insert subject_ref: %w", err)
	}
	md, err := core.InsertRecordMetadataTx(ctx, tx, in.Governance, ref.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("insert record_metadata: %w", err)
	}

	var categoryID *uuid.UUID
	if in.ActorKind == KindOrganization {
		categoryID, err = resolveCategoryID(ctx, tx, in.CategoryCode)
		if err != nil {
			return nil, nil, err
		}
	}

	rows, err := tx.Query(ctx, insertActorSQL, pgx.NamedArgs{
		"id":                       ref.ID,
		"actor_kind":               int16(in.ActorKind),
		"display_name":             in.DisplayName,
		"name_for_search":          nameForSearch(in.DisplayName),
		"is_active":                true,
		"publication_code":         in.PublicationCode,
		"legal_name":               in.LegalName,
		"organization_category_id": categoryID,
		"org_complement":           in.OrgComplement,
		"is_ch_register":           in.IsCHRegister,
		"ch_register_ref":          in.CHRegisterRef,
		"created_by":               in.OperatorID,
	})
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	act, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Actor])
	if err != nil {
		return nil, nil, fmt.Errorf("insert actor: %w", err)
	}

	if err := insertContacts(ctx, tx, ref.ID, in.Contacts); err != nil {
		return nil, nil, err
	}

	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   ref.ID,
		EventType:   "ACTOR_CREATED",
		ActorUserID: in.OperatorID,
		AfterState:  map[string]any{"display_name": act.DisplayName, "actor_kind": int16(in.ActorKind)},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit create actor: %w", err)
	}
	act.Subject = ref
	act.RecordMetadata = md
	if err := r.hydrate(ctx, act); err != nil {
		return nil, nil, err
	}
	return act, ev, nil
}

// Get loads an actor with its subject, governance, category and contacts hydrated.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Actor, error) {
	rows, err := r.pool.Query(ctx, getActorSQL, pgx.NamedArgs{"id": id})
	if err != nil {
		return nil, fmt.Errorf("get actor: %w", err)
	}
	act, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Actor])
	if err != nil {
		return nil, mapDBError(err)
	}
	if err := r.hydrate(ctx, act); err != nil {
		return nil, err
	}
	return act, nil
}

// Update applies a partial update after verifying the record is not locked.
func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Actor, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin update actor: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Reject the mutation atomically if the record is locked or soft-deleted.
	if _, err := core.EnsureMutableTx(ctx, tx, id, false); err != nil {
		return nil, nil, err
	}

	var categoryID *uuid.UUID
	if in.CategoryCode != nil {
		categoryID, err = resolveCategoryID(ctx, tx, *in.CategoryCode)
		if err != nil {
			return nil, nil, err
		}
	}

	displayName := deref(in.DisplayName)
	rows, err := tx.Query(ctx, updateActorSQL, pgx.NamedArgs{
		"id":                       id,
		"set_display_name":         in.DisplayName != nil,
		"display_name":             displayName,
		"name_for_search":          nameForSearch(displayName),
		"set_is_active":            in.IsActive != nil,
		"is_active":                derefBool(in.IsActive),
		"set_publication_code":     in.PublicationCode != nil,
		"publication_code":         derefInt32(in.PublicationCode),
		"set_legal_name":           in.LegalName != nil,
		"legal_name":               deref(in.LegalName),
		"set_category":             in.CategoryCode != nil,
		"organization_category_id": categoryID,
		"set_org_complement":       in.OrgComplement != nil,
		"org_complement":           deref(in.OrgComplement),
		"set_is_ch_register":       in.IsCHRegister != nil,
		"is_ch_register":           derefBool(in.IsCHRegister),
		"set_ch_register_ref":      in.CHRegisterRef != nil,
		"ch_register_ref":          deref(in.CHRegisterRef),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("update actor: %w", err)
	}
	act, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Actor])
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	// Keep the canonical subject label in sync with the display name.
	if in.DisplayName != nil {
		if err := core.UpdateSubjectLabelTx(ctx, tx, id, act.DisplayName); err != nil {
			return nil, nil, fmt.Errorf("sync subject label: %w", err)
		}
	}
	if in.ReplaceContacts {
		if _, err := tx.Exec(ctx, deleteContactsSQL, pgx.NamedArgs{"actor_id": id}); err != nil {
			return nil, nil, fmt.Errorf("clear contacts: %w", err)
		}
		if err := insertContacts(ctx, tx, id, in.Contacts); err != nil {
			return nil, nil, err
		}
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "ACTOR_UPDATED",
		ActorUserID: in.OperatorID,
		Reason:      in.Reason,
		AfterState:  map[string]any{"display_name": act.DisplayName},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit update actor: %w", err)
	}
	if err := r.hydrate(ctx, act); err != nil {
		return nil, nil, err
	}
	return act, ev, nil
}

// actorListRow adds the window total to the actor columns for search scanning.
type actorListRow struct {
	Actor
	TotalSize int32 `db:"total_count"`
}

// Search runs accent-insensitive + filtered search and hydrates the results.
func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	rows, err := r.pool.Query(ctx, searchActorsSQL, pgx.NamedArgs{
		"query":           filter.Query,
		"actor_kind":      int16(filter.ActorKind),
		"category_code":   filter.OrganizationCatCode,
		"only_active":     filter.OnlyActive,
		"include_deleted": filter.IncludeDeleted,
		"limit":           filter.Limit,
		"offset":          filter.Offset,
	})
	if err != nil {
		return SearchResult{}, fmt.Errorf("search actors: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[actorListRow])
	if err != nil {
		return SearchResult{}, fmt.Errorf("read actors: %w", err)
	}
	result := SearchResult{Actors: make([]*Actor, len(listRows))}
	for i := range listRows {
		act := listRows[i].Actor
		result.Actors[i] = &act
		result.TotalSize = listRows[i].TotalSize
	}
	for _, act := range result.Actors {
		if err := r.hydrate(ctx, act); err != nil {
			return SearchResult{}, err
		}
	}
	return result, nil
}

// SoftDelete logically deletes the actor via its governance record and writes an audit event.
func (r *PostgresRepository) SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin delete actor: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Reject deleting an already soft-deleted actor (locked is allowed to be retired).
	if _, err := core.EnsureMutableTx(ctx, tx, id, true); err != nil {
		return nil, err
	}
	if _, err := core.SoftDeleteRecordMetadataTx(ctx, tx, id, operatorID); err != nil {
		return nil, err
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "ACTOR_DELETED",
		ActorUserID: operatorID,
		Reason:      reason,
	})
	if err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delete actor: %w", err)
	}
	return ev, nil
}

// ListCategories returns the organization category catalogue.
func (r *PostgresRepository) ListCategories(ctx context.Context, onlyActive bool) ([]*OrganizationCategory, error) {
	rows, err := r.pool.Query(ctx, listCategoriesSQL, pgx.NamedArgs{"only_active": onlyActive})
	if err != nil {
		return nil, fmt.Errorf("list organization categories: %w", err)
	}
	cats, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[OrganizationCategory])
	if err != nil {
		return nil, fmt.Errorf("read organization categories: %w", err)
	}
	return cats, nil
}

// hydrate fills Subject, RecordMetadata, Category and Contacts on an actor.
func (r *PostgresRepository) hydrate(ctx context.Context, act *Actor) error {
	if act.Subject == nil {
		ref, err := core.GetSubjectRefTx(ctx, r.pool, act.ID)
		if err != nil {
			return fmt.Errorf("hydrate subject: %w", err)
		}
		act.Subject = ref
	}
	if act.RecordMetadata == nil {
		md, err := core.GetRecordMetadataTx(ctx, r.pool, act.ID)
		if err != nil {
			return fmt.Errorf("hydrate metadata: %w", err)
		}
		act.RecordMetadata = md
	}
	if act.CategoryID != nil {
		cat, err := getCategoryByID(ctx, r.pool, *act.CategoryID)
		if err != nil {
			return fmt.Errorf("hydrate category: %w", err)
		}
		act.Category = cat
	}
	contacts, err := listContacts(ctx, r.pool, act.ID)
	if err != nil {
		return fmt.Errorf("hydrate contacts: %w", err)
	}
	act.Contacts = contacts
	return nil
}

// --- helpers -----------------------------------------------------------------

// resolveCategoryID maps an organization category code to its id. An empty code
// yields nil (no category). An unknown code is an invalid-input error.
func resolveCategoryID(ctx context.Context, q core.Querier, code string) (*uuid.UUID, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	cat, err := getCategoryByCode(ctx, q, code)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, fmt.Errorf("%w: unknown organization category %q", core.ErrInvalidInput, code)
		}
		return nil, err
	}
	id := cat.ID
	return &id, nil
}

func getCategoryByCode(ctx context.Context, q core.Querier, code string) (*OrganizationCategory, error) {
	rows, err := q.Query(ctx, getCategoryByCodeSQL, pgx.NamedArgs{"code": code})
	if err != nil {
		return nil, err
	}
	cat, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[OrganizationCategory])
	if err != nil {
		return nil, mapDBError(err)
	}
	return cat, nil
}

func getCategoryByID(ctx context.Context, q core.Querier, id uuid.UUID) (*OrganizationCategory, error) {
	rows, err := q.Query(ctx, getCategoryByIDSQL, pgx.NamedArgs{"id": id})
	if err != nil {
		return nil, err
	}
	cat, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[OrganizationCategory])
	if err != nil {
		return nil, mapDBError(err)
	}
	return cat, nil
}

// insertContacts writes the given contacts for an actor using q.
func insertContacts(ctx context.Context, q core.Querier, actorID uuid.UUID, contacts []ContactInput) error {
	for _, c := range contacts {
		if _, err := q.Exec(ctx, insertContactSQL, pgx.NamedArgs{
			"actor_id":     actorID,
			"contact_type": int16(c.ContactType),
			"value":        c.Value,
			"is_primary":   c.IsPrimary,
			"label":        c.Label,
		}); err != nil {
			return fmt.Errorf("insert actor_contact: %w", mapDBError(err))
		}
	}
	return nil
}

func listContacts(ctx context.Context, q core.Querier, actorID uuid.UUID) ([]*Contact, error) {
	rows, err := q.Query(ctx, listContactsSQL, pgx.NamedArgs{"actor_id": actorID})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Contact])
}

// nameForSearch normalizes a display name into the stored search accelerator.
// Accent-insensitive matching itself is handled by the generated search_vector
// (immutable_unaccent); this is a lightweight lower-cased copy of the name.
func nameForSearch(displayName string) string {
	return strings.ToLower(strings.TrimSpace(displayName))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool { return b != nil && *b }

func derefInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

// mapDBError translates pgx.ErrNoRows to core.ErrNotFound.
func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return core.ErrNotFound
	}
	return err
}
