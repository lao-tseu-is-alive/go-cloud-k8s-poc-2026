package actor

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Replacing an actor's contacts or addresses is non-destructive (GLD-007,
// v2 §35): the rows that left the list are ended (ended_at, ended_by) and
// kept, the new ones inserted, and a row identical to a wanted item stays
// untouched, so saving an actor without changing its contacts writes nothing.

// replaceCounts says how many rows a replacement ended and added.
type replaceCounts struct {
	// Ended is the number of current rows that were ended.
	Ended int
	// Added is the number of rows inserted.
	Added int
}

// diffByKey matches wanted against the current rows by key: each wanted item
// keeps at most one identical row. It returns the ids of the unmatched rows
// (to end) and the unmatched items (to insert), in their input order.
func diffByKey[R any, K comparable](current []R, wanted []K, key func(R) K, id func(R) uuid.UUID) ([]uuid.UUID, []K) {
	available := make(map[K][]uuid.UUID, len(current))
	for _, row := range current {
		k := key(row)
		available[k] = append(available[k], id(row))
	}
	added := make([]K, 0, len(wanted))
	for _, item := range wanted {
		if ids := available[item]; len(ids) > 0 {
			available[item] = ids[1:]
			continue
		}
		added = append(added, item)
	}
	unmatched := make(map[uuid.UUID]bool, len(current))
	for _, ids := range available {
		for _, rowID := range ids {
			unmatched[rowID] = true
		}
	}
	ended := make([]uuid.UUID, 0, len(unmatched))
	for _, row := range current {
		if unmatched[id(row)] {
			ended = append(ended, id(row))
		}
	}
	return ended, added
}

// contactKey is the content of a current contact, compared with the wanted ones.
func contactKey(c *Contact) ContactInput {
	return ContactInput{ContactType: c.ContactType, Value: c.Value, IsPrimary: c.IsPrimary, Label: c.Label}
}

// addressKey is the content of a current address link, compared with the wanted ones.
func addressKey(a *Address) AddressInput {
	return AddressInput{
		AddressType: a.AddressType, IsPrincipal: a.IsPrincipal, Street: a.Street, HouseNumber: a.HouseNumber,
		AddressLine2: a.AddressLine2, PostalCode: a.PostalCode, Locality: a.Locality, CountryCode: a.CountryCode, Label: a.Label,
	}
}

// replaceContacts makes contacts the actor's current contact list.
func replaceContacts(ctx context.Context, q core.Querier, actorID uuid.UUID, contacts []ContactInput, operatorID string) (replaceCounts, error) {
	current, err := core.CollectGroupedTx(ctx, q, listContactsByActorsSQL, []uuid.UUID{actorID}, func(c *Contact) uuid.UUID { return c.ActorID })
	if err != nil {
		return replaceCounts{}, fmt.Errorf("read current contacts: %w", err)
	}
	ended, added := diffByKey(current[actorID], contacts, contactKey, func(c *Contact) uuid.UUID { return c.ID })
	if err := endRows(ctx, q, endContactsSQL, ended, operatorID); err != nil {
		return replaceCounts{}, fmt.Errorf("end contacts: %w", err)
	}
	return replaceCounts{Ended: len(ended), Added: len(added)}, insertContacts(ctx, q, actorID, added)
}

// replaceAddresses makes addresses the actor's current address list. The
// ended links are ended before the new ones are inserted, so a new principal
// address never meets the old one.
func replaceAddresses(ctx context.Context, q core.Querier, actorID uuid.UUID, addresses []AddressInput, operatorID string) (replaceCounts, error) {
	current, err := core.CollectGroupedTx(ctx, q, listAddressesByActorsSQL, []uuid.UUID{actorID}, func(a *Address) uuid.UUID { return a.ActorID })
	if err != nil {
		return replaceCounts{}, fmt.Errorf("read current addresses: %w", err)
	}
	ended, added := diffByKey(current[actorID], addresses, addressKey, func(a *Address) uuid.UUID { return a.ID })
	if err := endRows(ctx, q, endActorAddressesSQL, ended, operatorID); err != nil {
		return replaceCounts{}, fmt.Errorf("end actor addresses: %w", err)
	}
	return replaceCounts{Ended: len(ended), Added: len(added)}, insertAddresses(ctx, q, actorID, added, operatorID)
}

// endRows runs an "end these rows" statement taking @ids and @operator_id.
func endRows(ctx context.Context, q core.Querier, sql string, ids []uuid.UUID, operatorID string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := q.Exec(ctx, sql, pgx.NamedArgs{"ids": ids, "operator_id": operatorID})
	return err
}
