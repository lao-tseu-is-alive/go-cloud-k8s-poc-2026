package document

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
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
	blobID, err := core.OptionalUUID(msg.ContentBlobId)
	if err != nil {
		return nil, err
	}
	linkCase, err := core.OptionalUUID(msg.LinkToCaseId)
	if err != nil {
		return nil, err
	}
	in := CreateInput{
		DocumentTypeCode: msg.DocumentTypeCode,
		Title:            msg.Title,
		Description:      msg.Description,
		OfficialDate:     officialDate,
		ExternalSystem:   msg.ExternalSystem,
		ExternalID:       msg.ExternalId,
		ExternalURL:      msg.ExternalUrl,
		ContentBlobID:    blobID,
		IsFinal:          msg.IsFinal,
		IsRecord:         msg.IsRecord,
		Language:         msg.Language,
		PageCount:        msg.PageCount,
		Metadata:         core.StructToMap(msg.Metadata),
		OperatorID:       core.OperatorID(user),
		LinkToCaseID:     linkCase,
	}
	if gov := msg.InitialGovernance; gov != nil {
		in.Governance.OwnerUserID = gov.OwnerUserId
		in.Governance.OwnerOrgID = gov.OwnerOrgId
		in.Governance.ConfidentialityLevel = gov.ConfidentialityLevel
		in.Governance.RetentionUntil = gov.RetentionUntil
		in.Governance.SortFinal = gov.SortFinal
		in.Governance.Metadata = gov.Metadata
	}
	res, err := s.service.Create(ctx, in)
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.CreateDocumentResponse{
		Document:     DomainToProto(res.Document),
		CreatedEvent: core.DomainAuditEventToProto(res.Event),
		Reused:       res.Reused,
	}
	if res.Relationship != nil {
		resp.InitialRelationship = core.DomainRelationshipToProto(res.Relationship)
	}
	return connect.NewResponse(resp), nil
}

// AddDocumentVersion appends a new current version to a document.
func (s *ConnectServer) AddDocumentVersion(ctx context.Context, req *connect.Request[goelandv1.AddDocumentVersionRequest]) (*connect.Response[goelandv1.AddDocumentVersionResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeWrite)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.DocumentId)
	if err != nil {
		return nil, err
	}
	blobID, err := core.OptionalUUID(req.Msg.ContentBlobId)
	if err != nil {
		return nil, err
	}
	doc, version, ev, err := s.service.AddVersion(ctx, id, VersionInput{
		ContentBlobID: blobID,
		IsFinal:       req.Msg.IsFinal,
		IsRecord:      req.Msg.IsRecord,
		PageCount:     req.Msg.PageCount,
		Metadata:      core.StructToMap(req.Msg.Metadata),
		OperatorID:    core.OperatorID(user),
		Reason:        req.Msg.Reason,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.AddDocumentVersionResponse{
		Document:   DomainToProto(doc),
		Version:    VersionToProto(version),
		AuditEvent: core.DomainAuditEventToProto(ev),
	}), nil
}

// ListDocumentVersions lists the versions of a document, newest first.
func (s *ConnectServer) ListDocumentVersions(ctx context.Context, req *connect.Request[goelandv1.ListDocumentVersionsRequest]) (*connect.Response[goelandv1.ListDocumentVersionsResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.DocumentId)
	if err != nil {
		return nil, err
	}
	versions, err := s.service.ListVersions(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	out := make([]*goelandv1.DocumentVersion, 0, len(versions))
	for _, v := range versions {
		out = append(out, VersionToProto(v))
	}
	return connect.NewResponse(&goelandv1.ListDocumentVersionsResponse{Versions: out}), nil
}

// GetDocument retrieves a document with optional relationships + audit.
func (s *ConnectServer) GetDocument(ctx context.Context, req *connect.Request[goelandv1.GetDocumentRequest]) (*connect.Response[goelandv1.GetDocumentResponse], error) {
	if _, err := core.RequireCaller(ctx, core.ScopeRead); err != nil {
		return nil, err
	}
	id, err := core.ParseUUID(req.Msg.Id)
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
	id, err := core.ParseUUID(req.Msg.Id)
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
		Metadata:     core.StructToMap(req.Msg.Metadata),
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
	id, err := core.ParseUUID(req.Msg.Id)
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
	id, err := core.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, err
	}
	doc, verified, err := s.service.Verify(ctx, id, req.Msg.ExpectedSha256)
	if err != nil {
		return nil, s.mapError(err)
	}
	resp := &goelandv1.VerifyDocumentIntegrityResponse{Verified: verified}
	if doc.CurrentVersion != nil && doc.CurrentVersion.Content != nil {
		resp.ActualSha256 = doc.CurrentVersion.Content.SHA256
		resp.VerifiedAt = core.TimestampPtrOrNil(doc.CurrentVersion.Content.VerifiedAt)
	}
	// storage_ref_checked is left empty on purpose: no storage bytes were read.
	return connect.NewResponse(resp), nil
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
	caseID, err := core.OptionalUUID(req.Msg.CaseId)
	if err != nil {
		return nil, err
	}
	thingID, err := core.OptionalUUID(req.Msg.ThingId)
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
	docID, err := core.ParseUUID(req.Msg.DocumentId)
	if err != nil {
		return nil, err
	}
	targetID, err := core.ParseUUID(req.Msg.TargetSubjectId)
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
	id, err := core.ParseUUID(req.Msg.Id)
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
	return core.ToConnectError(s.log, "document", err)
}

// CreateDocumentType adds a document type (administrators only).
func (s *ConnectServer) CreateDocumentType(ctx context.Context, req *connect.Request[goelandv1.CreateDocumentTypeRequest]) (*connect.Response[goelandv1.CreateDocumentTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.CreateDocumentType(ctx, DocumentTypeInput{Code: m.Code, Label: m.Label, Description: m.Description, Category: m.Category, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.CreateDocumentTypeResponse{DocumentType: DomainTypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}

// UpdateDocumentType changes a document type (administrators only).
func (s *ConnectServer) UpdateDocumentType(ctx context.Context, req *connect.Request[goelandv1.UpdateDocumentTypeRequest]) (*connect.Response[goelandv1.UpdateDocumentTypeResponse], error) {
	user, err := core.RequireCaller(ctx, core.ScopeAdmin)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	entry, change, err := s.service.UpdateDocumentType(ctx, m.Code, DocumentTypeUpdate{Label: m.Label, Description: m.Description, Category: m.Category, IsActive: m.IsActive, OperatorID: core.OperatorID(user), Reason: m.Reason})
	if err != nil {
		return nil, s.mapError(err)
	}
	return connect.NewResponse(&goelandv1.UpdateDocumentTypeResponse{DocumentType: DomainTypeToProto(entry), Change: core.DomainReferenceChangeToProto(change)}), nil
}
