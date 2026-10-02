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
		"salutation":               int16(in.Salutation),
		"last_name":                in.LastName,
		"first_name":               in.FirstName,
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
	if err := insertAddresses(ctx, tx, ref.ID, in.Addresses, in.OperatorID); err != nil {
		return nil, nil, err
	}

	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   ref.ID,
		EventType:   "ACTOR_CREATED",
		ActorUserID: in.OperatorID,
		AfterState:  actorAuditState(act),
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

	act, err := updateActorRow(ctx, tx, id, in)
	if err != nil {
		return nil, nil, err
	}
	// Keep the canonical subject label in sync with the display name.
	if in.DisplayName != nil {
		if err := core.UpdateSubjectLabelTx(ctx, tx, id, act.DisplayName); err != nil {
			return nil, nil, fmt.Errorf("sync subject label: %w", err)
		}
	}
	if in.ReplaceContacts {
		if err := replaceContacts(ctx, tx, id, in.Contacts); err != nil {
			return nil, nil, err
		}
	}
	if in.ReplaceAddresses {
		if err := replaceAddresses(ctx, tx, id, in.Addresses, in.OperatorID); err != nil {
			return nil, nil, err
		}
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "ACTOR_UPDATED",
		ActorUserID: in.OperatorID,
		Reason:      in.Reason,
		AfterState:  updateAuditState(act, in),
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

// updateActorRow applies the non-nil fields of in to the actor row.
func updateActorRow(ctx context.Context, tx pgx.Tx, id uuid.UUID, in UpdateInput) (*Actor, error) {
	var categoryID *uuid.UUID
	if in.CategoryCode != nil {
		var err error
		if categoryID, err = resolveCategoryID(ctx, tx, *in.CategoryCode); err != nil {
			return nil, err
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
		"set_salutation":           in.Salutation != nil,
		"salutation":               derefSalutation(in.Salutation),
		"set_last_name":            in.LastName != nil,
		"last_name":                deref(in.LastName),
		"set_first_name":           in.FirstName != nil,
		"first_name":               deref(in.FirstName),
	})
	if err != nil {
		return nil, fmt.Errorf("update actor: %w", err)
	}
	act, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Actor])
	if err != nil {
		return nil, mapDBError(err)
	}
	return act, nil
}

// replaceContacts swaps the whole contact list of an actor (see GLD-007 about
// keeping the replaced contacts).
func replaceContacts(ctx context.Context, tx pgx.Tx, id uuid.UUID, contacts []ContactInput) error {
	if _, err := tx.Exec(ctx, deleteContactsSQL, pgx.NamedArgs{"actor_id": id}); err != nil {
		return fmt.Errorf("clear contacts: %w", err)
	}
	return insertContacts(ctx, tx, id, contacts)
}

// actorListRow adds the window total to the actor columns for search scanning.
type actorListRow struct {
	Actor
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// Search runs accent-insensitive + filtered search and hydrates the results.
func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	rows, err := r.pool.Query(ctx, core.SortedQuery(searchActorsSQL, filter.Sort, defaultActorSort), filter.Viewer.AddTo(pgx.NamedArgs{
		"count_limit":     core.CountLimit,
		"query":           filter.Query,
		"actor_kind":      int16(filter.ActorKind),
		"category_code":   filter.OrganizationCatCode,
		"only_active":     filter.OnlyActive,
		"include_deleted": filter.IncludeDeleted,
		"limit":           filter.Limit,
		"offset":          filter.Offset,
	}))
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
	result.TotalSize, result.TotalCapped = core.CapTotal(result.TotalSize)
	return result, r.hydrateAll(ctx, result.Actors)
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
	return r.hydrateAll(ctx, []*Actor{act})
}

// hydrateAll fills Subject, RecordMetadata, Category, Contacts and Addresses on a
// page of actors in at most five queries, whatever the page size.
func (r *PostgresRepository) hydrateAll(ctx context.Context, actors []*Actor) error {
	if len(actors) == 0 {
		return nil
	}
	ids := core.IDsOf(actors, func(a *Actor) uuid.UUID { return a.ID })
	headers, err := core.GetSubjectHeadersTx(ctx, r.pool, ids)
	if err != nil {
		return fmt.Errorf("hydrate actors: %w", err)
	}
	var categoryIDs []uuid.UUID
	for _, act := range actors {
		if act.CategoryID != nil {
			categoryIDs = append(categoryIDs, *act.CategoryID)
		}
	}
	categories, err := core.CollectIndexedTx(ctx, r.pool, getCategoriesByIDsSQL, categoryIDs, func(c *OrganizationCategory) uuid.UUID { return c.ID })
	if err != nil {
		return fmt.Errorf("hydrate categories: %w", err)
	}
	contacts, err := core.CollectGroupedTx(ctx, r.pool, listContactsByActorsSQL, ids, func(c *Contact) uuid.UUID { return c.ActorID })
	if err != nil {
		return fmt.Errorf("hydrate contacts: %w", err)
	}
	addresses, err := core.CollectGroupedTx(ctx, r.pool, listAddressesByActorsSQL, ids, func(a *Address) uuid.UUID { return a.ActorID })
	if err != nil {
		return fmt.Errorf("hydrate addresses: %w", err)
	}
	for _, act := range actors {
		act.Subject, act.RecordMetadata = headers.Refs[act.ID], headers.Metadata[act.ID]
		if act.CategoryID != nil {
			act.Category = categories[*act.CategoryID]
		}
		act.Contacts, act.Addresses = contacts[act.ID], addresses[act.ID]
	}
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
	return core.MapDBError(err, nil)
}

// derefSalutation returns the salutation as its column value, 0 when nil.
func derefSalutation(s *Salutation) int16 {
	if s == nil {
		return 0
	}
	return int16(*s)
}

// actorAuditState is the audited identity of an actor: its names (a person's
// minimal identity included) and kind; contacts are not copied into the log.
func actorAuditState(act *Actor) map[string]any {
	state := map[string]any{"display_name": act.DisplayName, "actor_kind": int16(act.ActorKind)}
	if act.ActorKind == KindPerson {
		state["salutation"] = int16(act.Salutation)
		state["last_name"] = act.LastName
		state["first_name"] = act.FirstName
	}
	return state
}

// insertAddresses creates one address row per input and links it to the actor.
func insertAddresses(ctx context.Context, q core.Querier, actorID uuid.UUID, addresses []AddressInput, operatorID string) error {
	for _, a := range addresses {
		var addressID uuid.UUID
		if err := q.QueryRow(ctx, insertAddressSQL, pgx.NamedArgs{
			"street":        a.Street,
			"house_number":  a.HouseNumber,
			"address_line2": a.AddressLine2,
			"postal_code":   a.PostalCode,
			"locality":      a.Locality,
			"country_code":  a.CountryCode,
			"created_by":    operatorID,
		}).Scan(&addressID); err != nil {
			return fmt.Errorf("insert address: %w", mapDBError(err))
		}
		if _, err := q.Exec(ctx, insertActorAddressSQL, pgx.NamedArgs{
			"actor_id":     actorID,
			"address_id":   addressID,
			"address_type": int16(a.AddressType),
			"is_principal": a.IsPrincipal,
			"label":        a.Label,
			"created_by":   operatorID,
		}); err != nil {
			return fmt.Errorf("insert actor_address: %w", mapDBError(err))
		}
	}
	return nil
}

// replaceAddresses ends the actor's current address links (kept as history)
// and links the new set.
func replaceAddresses(ctx context.Context, q core.Querier, actorID uuid.UUID, addresses []AddressInput, operatorID string) error {
	if _, err := q.Exec(ctx, endActorAddressesSQL, pgx.NamedArgs{"actor_id": actorID, "operator_id": operatorID}); err != nil {
		return fmt.Errorf("end actor addresses: %w", err)
	}
	return insertAddresses(ctx, q, actorID, addresses, operatorID)
}

// updateAuditState is the actor's audited identity plus which collections the
// update replaced.
func updateAuditState(act *Actor, in UpdateInput) map[string]any {
	state := actorAuditState(act)
	if in.ReplaceContacts {
		state["contacts_replaced"] = len(in.Contacts)
	}
	if in.ReplaceAddresses {
		state["addresses_replaced"] = len(in.Addresses)
	}
	return state
}
