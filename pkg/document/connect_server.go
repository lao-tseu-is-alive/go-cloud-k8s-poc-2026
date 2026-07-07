package document

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"google.golang.org/protobuf/types/known/structpb"
)

// ConnectServer exposes Service through the generated DocumentService contract.
type ConnectServer struct {
	service *Service
	log     *slog.Logger
	goelandv1connect.UnimplementedDocumentServiceHandler
}

// NewConnectServer builds a DocumentService ConnectServer. A nil logger falls back to slog.Default.
func NewConnectServer(service *Service, log *slog.Logger) (*ConnectServer, error) {
	if service == nil {
		return nil, errors.New("document service is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ConnectServer{service: service, log: log}, nil
}

// CreateDocument registers a new document.
func (s *ConnectServer) CreateDocument(ctx context.Context, req *connect.Request[goelandv1.CreateDocumentRequest]) (*connect.Response[goelandv1.CreateDocumentResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	officialDate, err := parseOfficialDate(msg.OfficialDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("official_date must be an ISO date (YYYY-MM-DD)"))
	}
	previous, err := optionalUUID(msg.PreviousVersionId)
	if err != nil {
		return nil, err
	}
	linkCase, err := optionalUUID(msg.LinkToCaseId)
	if err != nil {
		return nil, err
	}
	in := CreateInput{
		DocumentTypeCode:  msg.DocumentTypeCode,
		Title:             msg.Title,
		Description:       msg.Description,
		OfficialDate:      officialDate,
		StorageRef:        msg.StorageRef,
		ExternalSystem:    msg.ExternalSystem,
		ExternalID:        msg.ExternalId,
		ExternalURL:       msg.ExternalUrl,
		MimeType:          msg.MimeType,
		FileSizeBytes:     msg.FileSizeBytes,
		SHA256:            msg.Sha256,
		Version:           msg.Version,
		PreviousVersionID: previous,
		IsFinal:           msg.IsFinal,
		IsRecord:          msg.IsRecord,
		Language:          msg.Language,
		PageCount:         msg.PageCount,
		Metadata:          structToMap(msg.Metadata),
		OperatorID:        core.OperatorID(user),
		LinkToCaseID:      linkCase,
	}
	if gov := msg.InitialGovernance; gov != nil {
		in.Governance.OwnerUserID = gov.OwnerUserId
		in.Governance.OwnerOrgID = gov.OwnerOrgId
		in.Governance.ConfidentialityLevel = gov.ConfidentialityLevel
		in.Governance.RetentionUntil = gov.RetentionUntil
		in.Governance.SortFinal = gov.SortFinal
		in.Governance.Metadata = gov.Metadata
	}
	doc, ev, rel, err := s.service.Create(ctx, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.CreateDocumentResponse{
		Document:     DomainToProto(doc),
		CreatedEvent: core.DomainAuditEventToProto(ev),
	}
	if rel != nil {
		resp.InitialRelationship = core.DomainRelationshipToProto(rel)
	}
	return connect.NewResponse(resp), nil
}

// GetDocument retrieves a document with optional relationships + audit.
func (s *ConnectServer) GetDocument(ctx context.Context, req *connect.Request[goelandv1.GetDocumentRequest]) (*connect.Response[goelandv1.GetDocumentResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	doc, err := s.service.Get(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.GetDocumentResponse{Document: DomainToProto(doc)}
	if req.Msg.IncludeRelationships {
		rels, err := s.service.Relationships(ctx, id)
		if err != nil {
			return nil, s.mapError(err)
		}
		resp.Relationships = core.DomainRelationshipsToProto(rels)
	}
	if req.Msg.IncludeAudit {
		audit, err := s.service.RecentAudit(ctx, id)
		if err != nil {
			return nil, s.mapError(err)
		}
		resp.RecentAudit = core.DomainAuditEventsToProto(audit)
	}
	return connect.NewResponse(resp), nil
}

// UpdateDocumentMetadata updates mutable metadata (respects locking).
func (s *ConnectServer) UpdateDocumentMetadata(ctx context.Context, req *connect.Request[goelandv1.UpdateDocumentMetadataRequest]) (*connect.Response[goelandv1.UpdateDocumentMetadataResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	officialDate, err := parseOfficialDate(req.Msg.OfficialDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("official_date must be an ISO date (YYYY-MM-DD)"))
	}
	doc, ev, err := s.service.UpdateMetadata(ctx, id, UpdateInput{
		Title:        req.Msg.Title,
		Description:  req.Msg.Description,
		OfficialDate: officialDate,
		Language:     req.Msg.Language,
		Metadata:     structToMap(req.Msg.Metadata),
		OperatorID:   core.OperatorID(user),
		Reason:       req.Msg.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateDocumentMetadataResponse{
		Document:    DomainToProto(doc),
		UpdateEvent: core.DomainAuditEventToProto(ev),
	}), nil
}

// FinalizeDocument marks a document final and optionally locks it.
func (s *ConnectServer) FinalizeDocument(ctx context.Context, req *connect.Request[goelandv1.FinalizeDocumentRequest]) (*connect.Response[goelandv1.FinalizeDocumentResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	doc, ev, err := s.service.Finalize(ctx, id, core.OperatorID(user), req.Msg.Reason, req.Msg.AlsoLockGovernance)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.FinalizeDocumentResponse{
		Document:      DomainToProto(doc),
		FinalizeEvent: core.DomainAuditEventToProto(ev),
	}), nil
}

// VerifyDocumentIntegrity performs a non-mutating, non-probative stored-hash
// comparison (it does not read bytes from storage). See Service.Verify. It runs
// under the read scope precisely because it writes nothing.
func (s *ConnectServer) VerifyDocumentIntegrity(ctx context.Context, req *connect.Request[goelandv1.VerifyDocumentIntegrityRequest]) (*connect.Response[goelandv1.VerifyDocumentIntegrityResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	doc, verified, err := s.service.Verify(ctx, id, req.Msg.ExpectedSha256)
	if err != nil {
		return nil, s.mapError(err)
	}
	actual := ""
	if doc.SHA256 != nil {
		actual = *doc.SHA256
	}
	// storage_ref_checked is left empty on purpose: no storage bytes were read.
	return connect.NewResponse(&goelandv1.VerifyDocumentIntegrityResponse{
		Verified:          verified,
		ActualSha256:      actual,
		VerifiedAt:        core.TimestampPtrOrNil(doc.SHA256VerifiedAt),
		StorageRefChecked: "",
	}), nil
}

// SearchDocuments runs a full-text + filtered search.
func (s *ConnectServer) SearchDocuments(ctx context.Context, req *connect.Request[goelandv1.SearchDocumentsRequest]) (*connect.Response[goelandv1.SearchDocumentsResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	offset, err := core.ParsePageToken(req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	caseID, err := optionalUUID(req.Msg.CaseId)
	if err != nil {
		return nil, err
	}
	thingID, err := optionalUUID(req.Msg.ThingId)
	if err != nil {
		return nil, err
	}
	result, err := s.service.Search(ctx, SearchFilter{
		Query:              req.Msg.Query,
		DocumentTypeCode:   req.Msg.DocumentTypeCode,
		CaseID:             caseID,
		ThingID:            thingID,
		ConfidentialityMax: req.Msg.ConfidentialityMax,
		OnlyRecords:        req.Msg.OnlyRecords,
		OnlyFinal:          req.Msg.OnlyFinal,
		IncludeDeleted:     req.Msg.IncludeDeleted,
		Limit:              int(req.Msg.PageSize),
		Offset:             offset,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.SearchDocumentsResponse{
		Documents:     DomainsToProto(result.Documents),
		NextPageToken: core.NextPageToken(offset, len(result.Documents), result.TotalSize),
		TotalSize:     result.TotalSize,
	}), nil
}

// LinkDocument creates a typed relationship from a document to another subject.
func (s *ConnectServer) LinkDocument(ctx context.Context, req *connect.Request[goelandv1.LinkDocumentRequest]) (*connect.Response[goelandv1.LinkDocumentResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	docID, err := parseUUID(req.Msg.DocumentId)
	if err != nil {
		return nil, err
	}
	targetID, err := parseUUID(req.Msg.TargetSubjectId)
	if err != nil {
		return nil, err
	}
	rel, ev, err := s.service.Link(ctx, core.LinkInput{
		SourceSubjectID:      docID,
		TargetSubjectID:      targetID,
		RelationshipTypeCode: req.Msg.RelationshipTypeCode,
		RoleDetail:           req.Msg.RoleDetail,
		OperatorID:           core.OperatorID(user),
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.LinkDocumentResponse{
		Relationship: core.DomainRelationshipToProto(rel),
		AuditEvent:   core.DomainAuditEventToProto(ev),
	}), nil
}

// DeleteDocument logically deletes a document.
func (s *ConnectServer) DeleteDocument(ctx context.Context, req *connect.Request[goelandv1.DeleteDocumentRequest]) (*connect.Response[goelandv1.DeleteDocumentResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := parseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	ev, err := s.service.SoftDelete(ctx, id, core.OperatorID(user), req.Msg.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.DeleteDocumentResponse{
		DeletedDocumentId: id.String(),
		DeleteEvent:       core.DomainAuditEventToProto(ev),
	}), nil
}

// ListDocumentTypes returns the document type catalogue.
func (s *ConnectServer) ListDocumentTypes(ctx context.Context, req *connect.Request[goelandv1.ListDocumentTypesRequest]) (*connect.Response[goelandv1.ListDocumentTypesResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	types, err := s.service.ListTypes(ctx, req.Msg.OnlyActive)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.DocumentType, 0, len(types))
	for _, t := range types {
		out = append(out, DomainTypeToProto(t))
	}
	return connect.NewResponse(&goelandv1.ListDocumentTypesResponse{DocumentTypes: out}), nil
}

// mapError converts domain errors to Connect status codes, logging unexpected ones.
func (s *ConnectServer) mapError(err error) *connect.Error {
	if mapped := core.MapError(err); mapped != nil {
		return mapped
	}
	s.log.Error("document request failed", "error", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// parseUUID parses a required UUID field, returning a Connect InvalidArgument error on failure.
func parseUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	return id, nil
}

// optionalUUID parses an optional UUID field: empty string yields nil.
func optionalUUID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	return &id, nil
}

// structToMap converts a proto Struct to a Go map (nil when unset).
func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}
