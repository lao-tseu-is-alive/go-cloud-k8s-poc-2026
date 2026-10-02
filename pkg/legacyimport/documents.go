package legacyimport

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const (
	keyDocumentVersion = "DOCUMENT_VERSION"
	keyBlob            = "BLOB"
	// legacyDocumentTypeCode is the single document type of the imported
	// documents: the legacy type is a file format (pdf, tif, ...), not a
	// business type, and it gives the content's media type instead.
	legacyDocumentTypeCode = "LEG_DOCUMENT"
	// maxDocumentTitle is the POC limit on a document title (document.MaxTitleLength).
	maxDocumentTitle = 500
	// documentStatusDraft and documentStatusFinal are document.Status values.
	documentStatusDraft = 1
	documentStatusFinal = 2
	// The legacy confidentiality levels of a document (selNivConf in the
	// legacy UI): 0 public, 1 internal to the City, 2 the poster's direction,
	// 3 the poster's service, 4 the poster's unit, 5 the authorized security
	// groups, 6 the authorized employees (the last two: the access lists).
	levelInternal     = 1
	levelDirection    = 2
	levelService      = 3
	levelUnit         = 4
	explicitListLevel = 5
	// internalConfidentiality is the POC level of an internal document: read by
	// every employee (below core.ConfidentialLevel).
	internalConfidentiality = 1
)

// levelUnitTypes is the type of the poster's unit (or ancestor) a level opens to.
var levelUnitTypes = map[int32]string{levelDirection: "DIRECTION", levelService: "SERVICE"}

// sourceDocument is a legacy document with the metadata of its current content.
type sourceDocument struct {
	// ID is the source column iddocument.
	ID int64
	// Title is the source column doctitle.
	Title string
	// Description is the source column docdescription.
	Description string
	// Subject is the source column docsubject.
	Subject string
	// Comment is the source column doccomment.
	Comment string
	// OfficialDate is the source column docdateofficielle.
	OfficialDate *time.Time
	// CreatedAt is the source column datecreated.
	CreatedAt *time.Time
	// UpdatedAt is the source column datelastmodif.
	UpdatedAt *time.Time
	// PosterID is the source column iduserpost.
	PosterID *int64
	// Definitive is the source column docisdefinitive.
	Definitive bool
	// Confidential is the source column docisconfidential.
	Confidential bool
	// Level is the source column doclevelconfidential (0-6, relative to the poster's unit).
	Level int32
	// SHA256 is the source column sha256hash.
	SHA256 string
	// Size is the source column docsizeinbyte.
	Size int64
	// Pages is the source column nbrpage.
	Pages int32
	// MimeType is the media type of the legacy file format.
	MimeType string
	// FileName is the legacy file name with its extension.
	FileName string
	// Version is the legacy version number (docnumver).
	Version int32
}

// scanDocument reads one row of documentsSQL.
func scanDocument(rows pgx.Rows) (*sourceDocument, error) {
	d := &sourceDocument{}
	err := rows.Scan(&d.ID, &d.Title, &d.Description, &d.Subject, &d.Comment, &d.OfficialDate, &d.CreatedAt, &d.UpdatedAt,
		&d.PosterID, &d.Definitive, &d.Confidential, &d.Level, &d.SHA256, &d.Size, &d.Pages, &d.MimeType, &d.FileName, &d.Version)
	return d, err
}

// documentPass is one COPY of the documents stage: the rows of one table.
type documentPass struct {
	table   string
	columns []string
	values  func(*sourceDocument) []any
}

// importDocuments writes the documents as metadata with a version known by
// its digest only (the bytes stay in the legacy store): the subjects, the
// documents, the content blobs (one per digest), the versions, then the
// current version of each document. The source is streamed once per table so
// the 2.3M documents never sit in memory.
func (imp *Importer) importDocuments(ctx context.Context, c *StageCounts) error {
	typeID, err := imp.ensureLegacyDocumentType(ctx)
	if err != nil {
		return err
	}
	blobs := map[string]bool{}
	passes := []documentPass{
		{"subject_ref", subjectRefColumns, func(d *sourceDocument) []any { s := imp.documentSubject(d); return imp.subjectRefRow(s) }},
		{"record_metadata", recordMetadataColumns, func(d *sourceDocument) []any { return imp.metadataRow(imp.documentSubject(d)) }},
		{"subject_provenance", provenanceColumns, func(d *sourceDocument) []any { return imp.provenanceRow(imp.documentSubject(d)) }},
		{"document", documentColumns, func(d *sourceDocument) []any { return imp.documentRow(d, typeID) }},
		{"content_blob", blobColumns, func(d *sourceDocument) []any { return imp.blobRow(d, blobs) }},
		{"document_version", versionColumns, func(d *sourceDocument) []any { return imp.versionRow(d) }},
	}
	for i, p := range passes {
		n, err := imp.copyFromSource(ctx, p.table, p.columns, documentsSQL, func(rows pgx.Rows) ([]any, error) {
			d, err := scanDocument(rows)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				c.Read++
				imp.subjects[ID(string(core.SubjectKindDocument), d.ID)] = core.SubjectKindDocument
				imp.countDocumentAdjustments(d, c)
			}
			return p.values(d), nil
		})
		if err != nil {
			return err
		}
		if p.table == "document" {
			c.Written = n
		}
	}
	if _, err := imp.tx.Exec(ctx, setCurrentVersionsSQL, pgx.NamedArgs{"batch_id": imp.batchID}); err != nil {
		return fmt.Errorf("set current versions: %w", err)
	}
	return nil
}

// countDocumentAdjustments counts, once per document, what the mapping changes.
func (imp *Importer) countDocumentAdjustments(d *sourceDocument, c *StageCounts) {
	if utf8.RuneCountInString(d.Title) > maxDocumentTitle {
		c.adjust("title longer than 500 characters, shortened")
	}
	if d.Title == "" {
		c.adjust("document without a title, placeholder used")
	}
	if d.Version > 1 {
		c.adjust("earlier versions not imported (current one only)")
	}
}

// ensureLegacyDocumentType creates the document type of the imported documents.
func (imp *Importer) ensureLegacyDocumentType(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := imp.tx.QueryRow(ctx, insertLegacyDocumentTypeSQL, pgx.NamedArgs{"code": legacyDocumentTypeCode}).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create the legacy document type: %w", err)
	}
	return id, nil
}

// documentSubject is the identity and governance of a legacy document.
func (imp *Importer) documentSubject(d *sourceDocument) subject {
	s := newSubject(core.SubjectKindDocument, "document", d.ID, documentTitle(d))
	s.createdAt, s.updatedAt, s.createdBy = d.CreatedAt, d.UpdatedAt, imp.operator(d.PosterID)
	switch {
	case d.Level >= levelDirection:
		s.confidentiality = core.ConfidentialLevel
	case d.Level == levelInternal:
		s.confidentiality = internalConfidentiality
	}
	return s
}

// documentTitle is the title within the POC limit, or a placeholder.
func documentTitle(d *sourceDocument) string {
	title := strings.TrimSpace(d.Title)
	if title == "" {
		return fmt.Sprintf("Document %d", d.ID)
	}
	if utf8.RuneCountInString(title) > maxDocumentTitle {
		runes := []rune(title)
		return string(runes[:maxDocumentTitle-1]) + "…"
	}
	return title
}

var (
	documentColumns = []string{
		"id", "document_type_id", "title", "description", "official_date", "external_system", "external_id",
		"status", "metadata", "created_at", "created_by",
	}
	blobColumns    = []string{"id", "sha256", "storage_ref", "mime_type", "file_size_bytes", "created_at", "created_by"}
	versionColumns = []string{
		"id", "document_id", "version_no", "content_blob_id", "page_count", "is_final", "validated_at", "validated_by",
		"metadata", "created_at", "created_by",
	}
)

// documentRow is the document row of a legacy document.
func (imp *Importer) documentRow(d *sourceDocument, typeID uuid.UUID) []any {
	s := imp.documentSubject(d)
	status := documentStatusDraft
	if d.Definitive {
		status = documentStatusFinal
	}
	metadata := map[string]any{"legacy_file_name": d.FileName, "legacy_version": d.Version, "legacy_level": d.Level, "legacy_confidential_flag": d.Confidential}
	if d.Subject != "" {
		metadata["legacy_subject"] = d.Subject
	}
	if d.Comment != "" {
		metadata["legacy_comment"] = d.Comment
	}
	var official any
	if d.OfficialDate != nil {
		official = d.OfficialDate.Format(time.DateOnly)
	}
	return []any{s.id, typeID, s.label, d.Description, official, SourceSystem, strconv.FormatInt(d.ID, 10),
		int16(status), metadata, firstTime(d.CreatedAt, &imp.now), s.createdBy}
}

// blobRow is the content blob of a digest seen for the first time, or nil:
// the content is known by its digest only (no storage reference).
func (imp *Importer) blobRow(d *sourceDocument, seen map[string]bool) []any {
	sha := strings.ToLower(strings.TrimSpace(d.SHA256))
	if sha == "" || seen[sha] {
		return nil
	}
	seen[sha] = true
	return []any{blobID(sha), sha, "", d.MimeType, max(d.Size, 0), firstTime(d.CreatedAt, &imp.now), imp.operator(d.PosterID)}
}

// blobID is the deterministic id of the content blob of a digest.
func blobID(sha string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte(keyBlob+":"+sha))
}

// versionRow is the single imported version of a legacy document (its current
// content); a definitive document's version is final.
func (imp *Importer) versionRow(d *sourceDocument) []any {
	var blob any
	if sha := strings.ToLower(strings.TrimSpace(d.SHA256)); sha != "" {
		blob = blobID(sha)
	}
	var validatedAt any
	validatedBy := ""
	if d.Definitive {
		validatedAt, validatedBy = firstTime(d.UpdatedAt, d.CreatedAt, &imp.now), imp.operator(d.PosterID)
	}
	return []any{ID(keyDocumentVersion, d.ID), ID(string(core.SubjectKindDocument), d.ID), int32(1), blob, max(d.Pages, 0), d.Definitive,
		validatedAt, validatedBy, map[string]any{"legacy_version": d.Version}, firstTime(d.CreatedAt, &imp.now), imp.operator(d.PosterID)}
}

// importDocumentGrants gives the readers of the confidential documents (legacy
// levels 2 to 6; 0 and 1 are read by every employee): the poster gets
// FULL_CONTROL; level 2 opens the document to the poster's direction, 3 to its
// service, 4 to its unit (a unit grant covers its sub-units); levels 5 and 6
// to the explicit access lists only (importDocumentAccessLists).
func (imp *Importer) importDocumentGrants(ctx context.Context, c *StageCounts) error {
	if _, err := imp.tx.Exec(ctx, createDocumentGrantStagingSQL); err != nil {
		return fmt.Errorf("create document grant staging: %w", err)
	}
	n, err := imp.copyFromSourceMulti(ctx, "import_document_grant", accessGrantColumns, documentGrantsSQL, func(rows pgx.Rows) ([][]any, error) {
		var docID, posterID int64
		var level int32
		if err := rows.Scan(&docID, &posterID, &level); err != nil {
			return nil, err
		}
		c.Read++
		return imp.documentGrantRows(ID(string(core.SubjectKindDocument), docID), posterID, level, c), nil
	})
	c.Written = n
	return err
}

// documentGrantRows are the grants of one confidential document.
func (imp *Importer) documentGrantRows(docID uuid.UUID, posterID int64, level int32, c *StageCounts) [][]any {
	if !imp.employees[posterID] {
		c.skip("unknown poster (no grant)")
		return nil
	}
	rows := [][]any{{docID, string(core.GranteeUser), strconv.FormatInt(posterID, 10), nil, int16(core.LevelFullControl), OperatorID, grantReason}}
	if level >= explicitListLevel {
		return rows
	}
	unit, ok := imp.posterUnit(posterID, levelUnitTypes[level], c)
	if !ok {
		c.adjust("poster without a known unit, poster only")
		return rows
	}
	return append(rows, []any{docID, string(core.GranteeOrgUnit), nil, unit, int16(core.LevelRead), OperatorID, grantReason})
}

// posterUnit is the poster's direct unit, or for a direction or service
// level its nearest ancestor of that type (the poster's own unit when none is
// found above it), when that unit was imported. The legacy keeps the poster's
// current unit only, not the one at the time of posting.
func (imp *Importer) posterUnit(posterID int64, unitType string, c *StageCounts) (uuid.UUID, bool) {
	unit, ok := imp.allEmployeeUnits[posterID]
	if !ok {
		return uuid.Nil, false
	}
	if unitType != "" {
		unit = imp.ancestorOfType(unit, unitType, c)
	}
	id := ID(string(core.SubjectKindOrgUnit), unit)
	return id, imp.known(id, core.SubjectKindOrgUnit)
}

// ancestorOfType walks up from unit to the first unit of unitType.
func (imp *Importer) ancestorOfType(unit int64, unitType string, c *StageCounts) int64 {
	for current, depth := unit, 0; depth < 64; depth++ {
		if imp.unitTypes[current] == unitType {
			return current
		}
		parent, ok := imp.unitParents[current]
		if !ok {
			break
		}
		current = parent
	}
	c.adjust("no " + strings.ToLower(unitType) + " above the poster's unit, its unit used")
	return unit
}

// importDocumentAccessLists turns the legacy per-document access lists of the
// confidential documents (employees, groups, units; all read-only) into READ
// grants, then writes every document grant once per grantee, the strongest
// (an access list may name the poster or its unit again).
func (imp *Importer) importDocumentAccessLists(ctx context.Context, c *StageCounts) error {
	n, err := imp.copyFromSource(ctx, "import_document_grant", accessGrantColumns, documentAccessListsSQL, func(rows pgx.Rows) ([]any, error) {
		var docID, granteeID int64
		var kind string
		if err := rows.Scan(&docID, &kind, &granteeID); err != nil {
			return nil, err
		}
		c.Read++
		return imp.grantRow(ID(string(core.SubjectKindDocument), docID), core.SubjectKindDocument, kind, granteeID, 4, c), nil
	})
	c.Written = n
	if err != nil {
		return err
	}
	if _, err := imp.tx.Exec(ctx, insertDocumentGrantsFromStagingSQL); err != nil {
		return fmt.Errorf("insert document grants: %w", err)
	}
	return nil
}
