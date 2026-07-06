package document

import (
	"time"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"google.golang.org/protobuf/types/known/structpb"
)

// isoDate is the layout used for the probative official_date field on the wire.
const isoDate = "2006-01-02"

// DomainTypeToProto converts a document type to its proto representation.
func DomainTypeToProto(t *DocumentType) *goelandv1.DocumentType {
	if t == nil {
		return nil
	}
	return &goelandv1.DocumentType{
		Id:          t.ID.String(),
		Code:        t.Code,
		Label:       t.Label,
		Description: t.Description,
		Category:    t.Category,
		IsActive:    t.IsActive,
	}
}

// DomainToProto converts a domain Document (with hydrated associations) to its proto representation.
func DomainToProto(doc *Document) *goelandv1.Document {
	if doc == nil {
		return nil
	}
	sha := ""
	if doc.SHA256 != nil {
		sha = *doc.SHA256
	}
	previous := ""
	if doc.PreviousVersionID != nil {
		previous = doc.PreviousVersionID.String()
	}
	officialDate := ""
	if doc.OfficialDate != nil {
		officialDate = doc.OfficialDate.Format(isoDate)
	}
	return &goelandv1.Document{
		SubjectRef:        core.DomainSubjectRefToProto(doc.Subject),
		DocumentType:      DomainTypeToProto(doc.Type),
		Title:             doc.Title,
		Description:       doc.Description,
		OfficialDate:      officialDate,
		StorageRef:        doc.StorageRef,
		ExternalSystem:    doc.ExternalSystem,
		ExternalId:        doc.ExternalID,
		ExternalUrl:       doc.ExternalURL,
		MimeType:          doc.MimeType,
		FileSizeBytes:     doc.FileSizeBytes,
		Sha256:            sha,
		Sha256VerifiedAt:  core.TimestampPtrOrNil(doc.SHA256VerifiedAt),
		Version:           doc.Version,
		PreviousVersionId: previous,
		IsFinal:           doc.IsFinal,
		IsRecord:          doc.IsRecord,
		Language:          doc.Language,
		PageCount:         doc.PageCount,
		Status:            goelandv1.DocumentStatus(doc.Status),
		Metadata:          structFromMap(doc.Metadata),
		CreatedAt:         core.TimestampOrNil(doc.CreatedAt),
		CreatedBy:         doc.CreatedBy,
		UpdatedAt:         core.TimestampOrNil(doc.UpdatedAt),
		RecordMetadata:    core.DomainRecordMetadataToProto(doc.RecordMetadata),
	}
}

// DomainsToProto maps a slice of documents.
func DomainsToProto(docs []*Document) []*goelandv1.Document {
	out := make([]*goelandv1.Document, 0, len(docs))
	for _, d := range docs {
		out = append(out, DomainToProto(d))
	}
	return out
}

// parseOfficialDate parses an ISO date string into a *time.Time, tolerating empty input.
func parseOfficialDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(isoDate, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// structFromMap converts a map to a proto Struct, returning nil for empty/invalid input.
func structFromMap(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
}
