// Package integration holds database-backed integration tests for the Goéland POC.
//
// These tests are the highest-value coverage the SQL-heavy code can have: they run
// the embedded migrations against a real PostgreSQL and exercise the full document
// lifecycle (create, search, finalize+lock, locked-update rejection, soft delete,
// audit listing) through the same repositories/services the server wires up.
//
// They are gated on the GOELAND_TEST_DATABASE_URL environment variable and skip
// cleanly when it is unset, so `go test ./...` stays green without a database.
// The target database must have the PostGIS, pgcrypto, pg_trgm and unaccent
// extensions available (see docs/PRODUCTION_READINESS.md).
package integration
