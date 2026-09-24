package core

import (
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

// Shared helpers for the ConnectRPC adapters of every domain (core, document,
// actor, case, ...), so request parsing and error mapping behave identically.

// errInvalidID is the client-facing message of a malformed UUID field.
var errInvalidID = errors.New("invalid id")

// ParseUUID parses a required UUID request field, returning a Connect
// InvalidArgument error on failure.
func ParseUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errInvalidID)
	}
	return id, nil
}

// OptionalUUID parses an optional UUID request field: an empty string yields
// nil, a malformed one a Connect InvalidArgument error.
func OptionalUUID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := ParseUUID(raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// StructFromMap converts a map to a proto Struct, returning nil for an empty or
// non-JSON-representable map.
func StructFromMap(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
}

// StructToMap converts a proto Struct to a Go map; nil stays nil.
func StructToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}

// ToConnectError maps a domain error to its Connect status (see MapError). An
// unexpected error is logged with its domain and hidden behind a generic
// Internal error so no internal detail leaks to clients.
func ToConnectError(log *slog.Logger, domain string, err error) *connect.Error {
	if mapped := MapError(err); mapped != nil {
		return mapped
	}
	log.Error(domain+" request failed", "error", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}
