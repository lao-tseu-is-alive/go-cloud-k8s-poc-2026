// Package legacyimport loads the POC from the legacy Goéland database, a
// read-only PostgreSQL replica of the production MSSQL database (GLD-051,
// GLD-052; rules in docs/IMPORT_MAPPING.md).
//
// The import is a transform, not a copy: each stage reads legacy rows, maps
// them onto the POC model and writes them set-based (COPY) into a brand-new
// target database, inside one transaction, so a run either loads everything or
// nothing, and a dry run is a full run rolled back. Subject ids are
// deterministic (SubjectID), every imported subject has a provenance row, and
// the run itself is one import_batch row holding the counts: no audit event is
// written per row, while the legacy creation dates and creators are kept.
//
// The package reads personal data only to write it into the local target; it
// reports counts, never values.
package legacyimport
