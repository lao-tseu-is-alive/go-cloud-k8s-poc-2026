package document

import (
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Status mirrors the document.status column and the DocumentStatus proto enum.
type Status int16

const (
	StatusUnspecified Status = 0
	StatusDraft       Status = 1
	StatusFinal       Status = 2
	StatusSuperseded  Status = 3
	StatusArchived    Status = 4
)

// DocumentType is a controlled classification of documents.
type DocumentType struct {
	ID          uuid.UUID `db:"id"`
	Code        string    `db:"code"`
	Label       string    `db:"label"`
	Description string    `db:"description"`
	Category    string    `db:"category"`
	IsActive    bool      `db:"is_active"`
}

// Document is the document-specific projection (1:1 with a DOCUMENT subject_ref).
// Nullable columns use pointers so pgx can scan SQL NULLs.
type Document struct {
	ID                uuid.UUID      `db:"id"`
	DocumentTypeID    uuid.UUID      `db:"document_type_id"`
	Title             string         `db:"title"`
	Description       string         `db:"description"`
	OfficialDate      *time.Time     `db:"official_date"`
	StorageRef        string         `db:"storage_ref"`
	ExternalSystem    string         `db:"external_system"`
	ExternalID        string         `db:"external_id"`
	ExternalURL       string         `db:"external_url"`
	MimeType          string         `db:"mime_type"`
	FileSizeBytes     int64          `db:"file_size_bytes"`
	SHA256            *string        `db:"sha256"`
	SHA256VerifiedAt  *time.Time     `db:"sha256_verified_at"`
	Version           int32          `db:"version"`
	PreviousVersionID *uuid.UUID     `db:"previous_version_id"`
	IsFinal           bool           `db:"is_final"`
	IsRecord          bool           `db:"is_record"`
	Language          string         `db:"language"`
	PageCount         int32          `db:"page_count"`
	Status            Status         `db:"status"`
	Metadata          map[string]any `db:"metadata"`
	CreatedAt         time.Time      `db:"created_at"`
	CreatedBy         string         `db:"created_by"`
	UpdatedAt         time.Time      `db:"updated_at"`

	// Hydrated associations (nil on write paths).
	Subject        *core.SubjectRef     `db:"-"`
	RecordMetadata *core.RecordMetadata `db:"-"`
	Type           *DocumentType        `db:"-"`
}

// CreateInput holds the client-controlled fields for a new document.
type CreateInput struct {
	DocumentTypeCode  string
	Title             string
	Description       string
	OfficialDate      *time.Time
	StorageRef        string
	ExternalSystem    string
	ExternalID        string
	ExternalURL       string
	MimeType          string
	FileSizeBytes     int64
	SHA256            string
	Version           int32
	PreviousVersionID *uuid.UUID
	IsFinal           bool
	IsRecord          bool
	Language          string
	PageCount         int32
	Metadata          map[string]any
	OperatorID        string
	Governance        core.CreateSubjectInput // owner/confidentiality carried from the request
	LinkToCaseID      *uuid.UUID
}

// UpdateInput holds the mutable metadata of a document.
type UpdateInput struct {
	Title        string
	Description  string
	OfficialDate *time.Time
	Language     string
	Metadata     map[string]any
	OperatorID   string
	Reason       string
}

// SearchFilter controls document search.
type SearchFilter struct {
	Query              string
	DocumentTypeCode   string
	CaseID             *uuid.UUID
	ThingID            *uuid.UUID
	ConfidentialityMax int32
	OnlyRecords        bool
	OnlyFinal          bool
	IncludeDeleted     bool
	Limit              int
	Offset             int
}

// SearchResult holds a page of documents and the total count before pagination.
type SearchResult struct {
	Documents []*Document
	TotalSize int32
}
