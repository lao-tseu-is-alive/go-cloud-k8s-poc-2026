package caseexport

import (
	"time"

	"github.com/google/uuid"
)

// Status is the state of an export job; it mirrors case_export.status.
type Status string

// Export job states.
const (
	// StatusQueued is a requested export waiting for a worker.
	StatusQueued Status = "QUEUED"
	// StatusRunning is an export a worker is building.
	StatusRunning Status = "RUNNING"
	// StatusDone is a built archive, downloadable until ExpiresAt.
	StatusDone Status = "DONE"
	// StatusFailed is an export that could not be built.
	StatusFailed Status = "FAILED"
	// StatusExpired is a built archive whose bytes were removed.
	StatusExpired Status = "EXPIRED"
)

const (
	// FormatVersion is the version of the archive layout written in the manifest.
	FormatVersion = 1
	// ManifestName is the manifest file at the root of an archive.
	ManifestName = "manifest.json"
	// blobDir is the archive directory holding the content files.
	blobDir = "blobs/"
	// maxAttempts bounds the builds of one job (a worker crash leaves it RUNNING).
	maxAttempts = 3
	// listLimit bounds the exports listed for one case.
	listLimit = 50
	// maxReasonLength bounds the justification of a request (code points).
	maxReasonLength = 2000
)

// Export is one export job (a row of case_export).
type Export struct {
	// ID is the job identity.
	ID uuid.UUID `db:"id"`
	// CaseID is the exported case.
	CaseID uuid.UUID `db:"case_id"`
	// RequestedBy is the operator id of the requester, the only downloader.
	RequestedBy string `db:"requested_by"`
	// Reason is the justification given with the request.
	Reason string `db:"reason"`
	// Status is the job state.
	Status Status `db:"status"`
	// RequestedAt is when the export was requested.
	RequestedAt time.Time `db:"requested_at"`
	// StartedAt is when the last build started.
	StartedAt *time.Time `db:"started_at"`
	// FinishedAt is when the job was built or failed.
	FinishedAt *time.Time `db:"finished_at"`
	// ExpiresAt is when a built archive is removed.
	ExpiresAt *time.Time `db:"expires_at"`
	// Attempts is the number of builds started.
	Attempts int16 `db:"attempts"`
	// StorageRef is the blob store reference of the archive; empty unless DONE.
	StorageRef string `db:"storage_ref"`
	// SHA256 is the lower-case hex digest of the archive once built.
	SHA256 string `db:"sha256"`
	// SizeBytes is the archive size once built.
	SizeBytes int64 `db:"size_bytes"`
	// Error explains a failed export.
	Error string `db:"error"`
}

// RequestInput holds a request for an export.
type RequestInput struct {
	// CaseID is the case to export.
	CaseID uuid.UUID
	// Reason is the optional justification (at most 2000 code points).
	Reason string
	// OperatorID is the requester (server-side, never from the request).
	OperatorID string
}

// Config tunes the export worker.
type Config struct {
	// TTL is how long a built archive stays downloadable; defaults to 24 h.
	TTL time.Duration
	// PollInterval is how often the worker looks for queued jobs and expired
	// archives; defaults to 5 s.
	PollInterval time.Duration
	// StaleAfter is how long a RUNNING job may go without finishing before
	// another worker retries it (its worker presumably died); defaults to 30 min.
	StaleAfter time.Duration
}

// withDefaults fills the zero fields of c.
func (c Config) withDefaults() Config {
	if c.TTL <= 0 {
		c.TTL = 24 * time.Hour
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 5 * time.Second
	}
	if c.StaleAfter <= 0 {
		c.StaleAfter = 30 * time.Minute
	}
	return c
}
