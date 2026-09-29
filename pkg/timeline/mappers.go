package timeline

import (
	"github.com/google/uuid"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// DomainToProto converts an entry (with its document links) to its proto
// representation; nil stays nil.
func DomainToProto(e *Entry) *goelandv1.TimelineEntry {
	if e == nil {
		return nil
	}
	documents := make([]*goelandv1.TimelineDocumentLink, 0, len(e.Documents))
	for _, link := range e.Documents {
		documents = append(documents, LinkToProto(link))
	}
	return &goelandv1.TimelineEntry{
		Id:                 e.ID.String(),
		CaseId:             e.CaseID.String(),
		EntryType:          goelandv1.TimelineEntryType(e.Type),
		Status:             goelandv1.TimelineEntryStatus(e.Status),
		Title:              e.Title,
		Body:               e.Body,
		Visibility:         goelandv1.TimelineVisibility(e.Visibility),
		OccurredAt:         core.TimestampOrNil(e.OccurredAt),
		CorrectsEntryId:    uuidString(e.CorrectsEntryID),
		CorrectedByEntryId: uuidString(e.CorrectedByEntryID),
		Documents:          documents,
		CreatedAt:          core.TimestampOrNil(e.CreatedAt),
		CreatedBy:          e.CreatedBy,
		UpdatedAt:          core.TimestampPtrOrNil(e.UpdatedAt),
		UpdatedBy:          e.UpdatedBy,
		ValidatedAt:        core.TimestampPtrOrNil(e.ValidatedAt),
		ValidatedBy:        e.ValidatedBy,
		LockedAt:           core.TimestampPtrOrNil(e.LockedAt),
		LockedBy:           e.LockedBy,
		WithdrawnAt:        core.TimestampPtrOrNil(e.WithdrawnAt),
		WithdrawnBy:        e.WithdrawnBy,
		WithdrawalReason:   e.WithdrawalReason,
		Metadata:           core.StructFromMap(e.Metadata),
	}
}

// DomainsToProto maps a slice of entries.
func DomainsToProto(entries []*Entry) []*goelandv1.TimelineEntry {
	out := make([]*goelandv1.TimelineEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, DomainToProto(e))
	}
	return out
}

// LinkToProto converts a document link to its proto representation.
func LinkToProto(link *DocumentLink) *goelandv1.TimelineDocumentLink {
	if link == nil {
		return nil
	}
	var versionNo int32
	if link.DocumentVersionNo != nil {
		versionNo = *link.DocumentVersionNo
	}
	return &goelandv1.TimelineDocumentLink{
		Id:                link.ID.String(),
		DocumentId:        link.DocumentID.String(),
		DocumentLabel:     link.DocumentLabel,
		DocumentVersionId: uuidString(link.DocumentVersionID),
		DocumentVersionNo: versionNo,
		CreatedAt:         core.TimestampOrNil(link.CreatedAt),
		CreatedBy:         link.CreatedBy,
	}
}

// uuidString renders an optional id; nil becomes the empty string.
func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
