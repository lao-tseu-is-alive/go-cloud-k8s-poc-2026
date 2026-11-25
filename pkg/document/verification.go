package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// VerificationOutcome is the result of rereading a version's stored bytes (GLD-021).
type VerificationOutcome string

// Verification outcomes.
const (
	// OutcomeVerified means the bytes have the registered SHA-256 and size.
	OutcomeVerified VerificationOutcome = "VERIFIED"
	// OutcomeMismatch means the bytes differ from the registered digest or size.
	OutcomeMismatch VerificationOutcome = "MISMATCH"
	// OutcomeMissing means Goéland should hold the bytes, but the store has none.
	OutcomeMissing VerificationOutcome = "MISSING"
	// OutcomeNotHeld means Goéland holds no bytes for the version (no content,
	// content known only by its digest, or destroyed bytes); nothing is
	// read nor recorded.
	OutcomeNotHeld VerificationOutcome = "NOT_HELD"
)

// Audit event types written by VerifyContent.
const (
	eventContentVerified           = "DOCUMENT_CONTENT_VERIFIED"
	eventContentVerificationFailed = "DOCUMENT_CONTENT_VERIFICATION_FAILED"
)

// ContentVerification reports a probative verification of a version's bytes.
type ContentVerification struct {
	// Outcome is the result.
	Outcome VerificationOutcome
	// Version is the version whose content was checked.
	Version *Version
	// ComputedSHA256 is the digest of the bytes read; empty unless bytes were read.
	ComputedSHA256 string
	// ComputedSize is the number of bytes read.
	ComputedSize int64
	// StorageRefChecked is the reference whose bytes were read; empty when NOT_HELD.
	StorageRefChecked string
	// VerifiedAt is the content's last successful verification after this one:
	// now when VERIFIED, nil after a MISMATCH or a MISSING outcome.
	VerifiedAt *time.Time
	// Event is the audit event recorded on the document; nil when NOT_HELD.
	Event *core.AuditEvent
}

// VerifyContent streams the stored bytes of a document version (the current
// one when versionID is nil), recomputes their SHA-256 and size and compares
// them with what the upload registered (spec v2 §49, GLD-021). A match sets the
// content's verified_at; a mismatch or missing bytes clear it, since the content
// is no longer known to be intact. Either way the result is audited on the
// document. A version whose bytes Goéland does not hold reports NOT_HELD and
// writes nothing. The caller's access is checked by the adapter.
func (s *Service) VerifyContent(ctx context.Context, documentID uuid.UUID, versionID *uuid.UUID, operatorID string) (ContentVerification, error) {
	if documentID == uuid.Nil {
		return ContentVerification{}, fmt.Errorf("%w: document id is required", core.ErrInvalidInput)
	}
	if s.store == nil {
		return ContentVerification{}, ErrNoContentStore
	}
	version, err := s.repo.VersionContent(ctx, documentID, versionID)
	if err != nil {
		return ContentVerification{}, err
	}
	res := ContentVerification{Outcome: OutcomeNotHeld, Version: version}
	blob := version.Content
	if blob == nil || blob.StorageRef == "" {
		return res, nil
	}
	if res.Outcome, res.ComputedSHA256, res.ComputedSize, err = s.rehash(ctx, blob); err != nil {
		return ContentVerification{}, err
	}
	res.StorageRefChecked = blob.StorageRef
	res.VerifiedAt, res.Event, err = s.repo.RecordContentVerification(ctx, documentID, res, operatorID)
	if err != nil {
		return ContentVerification{}, err
	}
	if res.Outcome != OutcomeVerified {
		s.log.Warn("document content verification failed", "document_id", documentID, "version_id", version.ID,
			"content_blob_id", blob.ID, "outcome", res.Outcome)
	}
	return res, nil
}

// rehash streams the blob's bytes through SHA-256 and compares the digest and
// size with the registered ones. A store failure other than missing bytes is
// an error: nothing can be concluded about the content.
func (s *Service) rehash(ctx context.Context, blob *ContentBlob) (VerificationOutcome, string, int64, error) {
	obj, err := s.store.Get(ctx, blob.StorageRef)
	if errors.Is(err, blobstore.ErrNotFound) {
		return OutcomeMissing, "", 0, nil
	}
	if err != nil {
		return "", "", 0, fmt.Errorf("open stored content: %w", err)
	}
	defer func() { _ = obj.Close() }()
	h := sha256.New()
	size, err := io.Copy(h, obj)
	if err != nil {
		return "", "", 0, fmt.Errorf("read stored content: %w", err)
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if digest != blob.SHA256 || size != blob.FileSizeBytes {
		return OutcomeMismatch, digest, size, nil
	}
	return OutcomeVerified, digest, size, nil
}

// verificationAuditState is the after state of a verification audit event.
func verificationAuditState(res ContentVerification) map[string]any {
	blob := res.Version.Content
	state := map[string]any{
		"outcome":         string(res.Outcome),
		"version_id":      res.Version.ID.String(),
		"version_no":      res.Version.VersionNo,
		"content_blob_id": blob.ID.String(),
		"sha256":          blob.SHA256,
		"size_bytes":      blob.FileSizeBytes,
	}
	if res.Outcome != OutcomeVerified {
		state["computed_sha256"] = res.ComputedSHA256
		state["computed_size_bytes"] = res.ComputedSize
	}
	return state
}

// verificationEventType is the audit event type of an outcome.
func verificationEventType(outcome VerificationOutcome) string {
	if outcome == OutcomeVerified {
		return eventContentVerified
	}
	return eventContentVerificationFailed
}
