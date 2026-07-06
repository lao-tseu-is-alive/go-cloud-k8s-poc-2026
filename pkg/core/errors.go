package core

import "errors"

var (
	// ErrInvalidInput is returned when caller-supplied data fails validation.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound is returned when a subject, relationship or reference row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrUnauthenticated is returned when a request carries no recognised identity.
	ErrUnauthenticated = errors.New("authenticated user is required")
	// ErrConflict is returned when an active relationship already exists (uniqueness violation).
	ErrConflict = errors.New("conflict")
	// ErrKindMismatch is returned when the source/target subject kinds do not match the relationship type.
	ErrKindMismatch = errors.New("subject kind does not match relationship type")
	// ErrLocked is returned when a mutation targets a locked (immutable) record.
	ErrLocked = errors.New("record is locked")
)
