package core

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reference catalogues administered through the API (GLD-040). Their rows are
// never deleted, only deactivated, and every change is logged in reference_change.
const (
	// CatalogueCaseType is the case_type catalogue.
	CatalogueCaseType = "case_type"
	// CatalogueRelationshipType is the relationship_type catalogue.
	CatalogueRelationshipType = "relationship_type"
	// CatalogueOrganizationCategory is the organization_category catalogue.
	CatalogueOrganizationCategory = "organization_category"
	// CatalogueDocumentType is the document_type catalogue.
	CatalogueDocumentType = "document_type"
	// CatalogueThingType is the thing_type catalogue.
	CatalogueThingType = "thing_type"

	// ReferenceCreated is the event type of a new catalogue entry.
	ReferenceCreated = "REFERENCE_CREATED"
	// ReferenceUpdated is the event type of a changed catalogue entry.
	ReferenceUpdated = "REFERENCE_UPDATED"

	// MaxReferenceLabelLength bounds a catalogue label (code points).
	MaxReferenceLabelLength = 200
	// MaxReferenceDescriptionLength bounds a catalogue description (code points).
	MaxReferenceDescriptionLength = 2000
)

// referenceCodePattern is the shape of a new catalogue code, e.g. OPC_DEMANDE_PC;
// it mirrors the protovalidate rule on the create requests.
var referenceCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,99}$`)

// ReferenceChange is one entry of the append-only reference_change log.
type ReferenceChange struct {
	// ID is the server-generated entry identity.
	ID uuid.UUID `db:"id"`
	// Catalogue is one of the Catalogue* constants.
	Catalogue string `db:"catalogue"`
	// Code is the changed entry's immutable code.
	Code string `db:"code"`
	// EventType is ReferenceCreated or ReferenceUpdated.
	EventType string `db:"event_type"`
	// ActorUserID is the operator (administrator) who made the change.
	ActorUserID string `db:"actor_user_id"`
	// OccurredAt is the database time of the change.
	OccurredAt time.Time `db:"occurred_at"`
	// BeforeState is the entry before an update; nil for a creation.
	BeforeState map[string]any `db:"before_state"`
	// AfterState is the entry after the change.
	AfterState map[string]any `db:"after_state"`
	// Reason is the operator's justification; empty when none was given.
	Reason string `db:"reason"`
}

// ReferenceFilter pages the reference_change log, newest first.
type ReferenceFilter struct {
	// Catalogue restricts the log to one catalogue; empty means all.
	Catalogue string
	// Limit is the page size, normalized like RelationshipFilter.Limit.
	Limit int
	// Offset is the zero-based number of rows to skip.
	Offset int
}

// ReferenceResult is a page of the reference_change log.
type ReferenceResult struct {
	// Changes is the requested page, newest first.
	Changes []*ReferenceChange
	// TotalSize is the number of matching entries across all pages.
	TotalSize int32
}

// ValidateReferenceCode checks the code of a new catalogue entry.
func ValidateReferenceCode(code string) error {
	if !referenceCodePattern.MatchString(code) {
		return fmt.Errorf("%w: code must be upper-case letters, digits and underscores (2-100), starting with a letter", ErrInvalidInput)
	}
	return nil
}

// NormalizeReferenceText trims a catalogue label or description and checks
// its length; required rejects an empty value.
func NormalizeReferenceText(field, value string, maxLen int, required bool) (string, error) {
	value = strings.TrimSpace(value)
	switch {
	case required && value == "":
		return "", fmt.Errorf("%w: %s is required", ErrInvalidInput, field)
	case utf8.RuneCountInString(value) > maxLen:
		return "", fmt.Errorf("%w: %s exceeds %d characters", ErrInvalidInput, field, maxLen)
	}
	return value, nil
}

// ReferenceMutation is one catalogue change run by MutateReference: Apply
// changes the row inside tx and returns its state before (nil for a creation)
// and after.
type ReferenceMutation[T any] struct {
	// Catalogue is one of the Catalogue* constants.
	Catalogue string
	// Code is the entry's code.
	Code string
	// OperatorID is the administrator making the change.
	OperatorID string
	// Reason is the justification recorded in the log.
	Reason string
	// Apply performs the change and returns the entry before and after it.
	Apply func(ctx context.Context, tx pgx.Tx) (before, after *T, err error)
	// State projects an entry into the logged state.
	State func(*T) map[string]any
}

// MutateReference runs m in one transaction together with its reference_change
// entry, so a catalogue never changes without a log line.
func MutateReference[T any](ctx context.Context, pool *pgxpool.Pool, m ReferenceMutation[T]) (*T, *ReferenceChange, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin %s change: %w", m.Catalogue, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	before, after, err := m.Apply(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	change := ReferenceChange{
		Catalogue: m.Catalogue, Code: m.Code, EventType: ReferenceCreated,
		ActorUserID: m.OperatorID, AfterState: m.State(after), Reason: strings.TrimSpace(m.Reason),
	}
	if before != nil {
		change.EventType, change.BeforeState = ReferenceUpdated, m.State(before)
	}
	recorded, err := insertReferenceChangeTx(ctx, tx, change)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit %s change: %w", m.Catalogue, err)
	}
	return after, recorded, nil
}

// insertReferenceChangeTx appends one entry to the reference_change log.
func insertReferenceChangeTx(ctx context.Context, q Querier, c ReferenceChange) (*ReferenceChange, error) {
	rows, err := q.Query(ctx, insertReferenceChangeSQL, pgx.NamedArgs{
		"catalogue":     c.Catalogue,
		"code":          c.Code,
		"event_type":    c.EventType,
		"actor_user_id": c.ActorUserID,
		"before_state":  c.BeforeState,
		"after_state":   c.AfterState,
		"reason":        c.Reason,
	})
	if err != nil {
		return nil, fmt.Errorf("insert reference_change: %w", err)
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[ReferenceChange])
}

// MapReferenceConflict translates a duplicate code into ErrConflict.
func MapReferenceConflict(err error, catalogue, code string) error {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" { // unique_violation
		return fmt.Errorf("%w: %s %q already exists", ErrConflict, catalogue, code)
	}
	return err
}

// NormalizeOptionalReferenceText applies NormalizeReferenceText to an optional
// update field, leaving nil untouched.
func NormalizeOptionalReferenceText(field string, value **string, maxLen int, required bool) error {
	if *value == nil {
		return nil
	}
	normalized, err := NormalizeReferenceText(field, **value, maxLen, required)
	if err != nil {
		return err
	}
	*value = &normalized
	return nil
}

// CollectReferenceRow reads exactly one catalogue row from a Query result,
// mapping no row to ErrNotFound.
func CollectReferenceRow[T any](rows pgx.Rows, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[T])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return row, err
}
