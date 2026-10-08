// Package integration holds database-backed integration tests for the Goéland POC.
//
// These tests are the highest-value coverage the SQL-heavy code can have: they run
// the embedded migrations against a real PostgreSQL and exercise every domain
// (document, actor, case, thing, timeline, org unit, task, circulation, access)
// through the same repositories and services the server wires up, together with
// the cross-cutting rules: relationships, append-only logs, business references,
// reference administration, roles, grants, the read filter and list sorting.
//
// They are gated on the GOELAND_TEST_DATABASE_URL environment variable and skip
// cleanly when it is unset, so `go test ./...` stays green without a database.
// The target database must have the PostGIS, pgcrypto, pg_trgm and unaccent
// extensions available (see docs/PRODUCTION_READINESS.md).
package integration
