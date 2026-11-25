package blobstore

import (
	"context"
	"errors"
	"io"
)

// Chain writes new content to its primary store and reads or deletes a
// reference through the first of its stores that owns it (GLD-020). A
// deployment moving to another backend keeps its previous store behind the
// primary, so the bytes stored before the move stay readable and disposable.
type Chain struct {
	stores []Store
}

// Chain implements the blobstore contract.
var _ Store = (*Chain)(nil)

// NewChain builds a chain writing to primary and also reading from previous,
// in that order. Stores must own disjoint references (different schemes).
func NewChain(primary Store, previous ...Store) *Chain {
	return &Chain{stores: append([]Store{primary}, previous...)}
}

// Put writes to the primary store.
func (c *Chain) Put(ctx context.Context, r io.Reader, meta Metadata) (Stored, error) {
	return c.stores[0].Put(ctx, r, meta)
}

// Get opens ref in the first store that owns it; ErrInvalidRef when none does.
func (c *Chain) Get(ctx context.Context, ref string) (Object, error) {
	for _, store := range c.stores {
		obj, err := store.Get(ctx, ref)
		if !errors.Is(err, ErrInvalidRef) {
			return obj, err
		}
	}
	return nil, ErrInvalidRef
}

// Delete removes ref from the first store that owns it; ErrInvalidRef when none does.
func (c *Chain) Delete(ctx context.Context, ref string) error {
	for _, store := range c.stores {
		if err := store.Delete(ctx, ref); !errors.Is(err, ErrInvalidRef) {
			return err
		}
	}
	return ErrInvalidRef
}
