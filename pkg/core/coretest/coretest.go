// Package coretest provides test doubles for the core domain, shared by the
// unit tests of the domains built on top of it (document, case, ...).
package coretest

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// StubRepository is a no-op core.Repository: every method succeeds with zero
// values. It lets a core.Service be constructed in tests that never exercise
// the core persistence paths.
type StubRepository struct{}

// StubRepository implements the core repository contract.
var _ core.Repository = StubRepository{}

// NewService returns a core.Service backed by StubRepository.
func NewService(t *testing.T) *core.Service {
	t.Helper()
	svc, err := core.NewService(StubRepository{}, nil)
	if err != nil {
		t.Fatalf("core service: %v", err)
	}
	return svc
}

// CreateSubject returns zero values.
func (StubRepository) CreateSubject(context.Context, core.CreateSubjectInput) (*core.SubjectRef, *core.RecordMetadata, *core.AuditEvent, error) {
	return nil, nil, nil, nil
}

// GetSubject returns zero values.
func (StubRepository) GetSubject(context.Context, uuid.UUID) (*core.SubjectRef, error) {
	return nil, nil
}

// AssignBusinessRef returns zero values.
func (StubRepository) AssignBusinessRef(context.Context, uuid.UUID, core.BusinessRefRequest, string, string) (*core.SubjectRef, *core.AuditEvent, error) {
	return nil, nil, nil
}

// LookupSubjects returns zero values.
func (StubRepository) LookupSubjects(context.Context, core.LookupFilter, int) ([]*core.SubjectRef, error) {
	return nil, nil
}

// GetRecordMetadata returns zero values.
func (StubRepository) GetRecordMetadata(context.Context, uuid.UUID) (*core.RecordMetadata, error) {
	return nil, nil
}

// LinkSubjects returns zero values.
func (StubRepository) LinkSubjects(context.Context, core.LinkInput) (*core.SubjectRelationship, *core.AuditEvent, error) {
	return nil, nil, nil
}

// UnlinkSubjects returns zero values.
func (StubRepository) UnlinkSubjects(context.Context, uuid.UUID, string, string) (*core.SubjectRelationship, *core.AuditEvent, error) {
	return nil, nil, nil
}

// ListRelationships returns an empty result.
func (StubRepository) ListRelationships(context.Context, core.RelationshipFilter) (core.RelationshipResult, error) {
	return core.RelationshipResult{}, nil
}

// ListRelationshipTypes returns zero values.
func (StubRepository) ListRelationshipTypes(context.Context, bool, core.SubjectKind, core.SubjectKind) ([]*core.RelationshipType, error) {
	return nil, nil
}

// AppendAuditEvent returns zero values.
func (StubRepository) AppendAuditEvent(context.Context, core.AuditEvent) (*core.AuditEvent, error) {
	return nil, nil
}

// ListAuditEvents returns an empty result.
func (StubRepository) ListAuditEvents(context.Context, core.AuditFilter) (core.AuditResult, error) {
	return core.AuditResult{}, nil
}
