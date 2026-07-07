package document

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// fakeRepo is a minimal document.Repository capturing inputs for assertions.
type fakeRepo struct {
	lastCreate  CreateInput
	createCalls int
	createErr   error
	updateErr   error
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput) (*Document, *core.AuditEvent, *core.SubjectRelationship, error) {
	f.lastCreate = in
	f.createCalls++
	if f.createErr != nil {
		return nil, nil, nil, f.createErr
	}
	return &Document{ID: uuid.New(), Title: in.Title}, &core.AuditEvent{}, nil, nil
}
func (f *fakeRepo) Get(context.Context, uuid.UUID) (*Document, error) { return &Document{}, nil }
func (f *fakeRepo) UpdateMetadata(_ context.Context, _ uuid.UUID, _ UpdateInput) (*Document, *core.AuditEvent, error) {
	if f.updateErr != nil {
		return nil, nil, f.updateErr
	}
	return &Document{}, &core.AuditEvent{}, nil
}
func (f *fakeRepo) Finalize(context.Context, uuid.UUID, string, string, bool) (*Document, *core.AuditEvent, error) {
	return &Document{}, &core.AuditEvent{}, nil
}
func (f *fakeRepo) Verify(context.Context, uuid.UUID, string) (*Document, bool, error) {
	return &Document{}, false, nil
}
func (f *fakeRepo) Search(context.Context, SearchFilter) (SearchResult, error) {
	return SearchResult{}, nil
}
func (f *fakeRepo) Link(context.Context, core.LinkInput) (*core.SubjectRelationship, *core.AuditEvent, error) {
	return &core.SubjectRelationship{}, &core.AuditEvent{}, nil
}
func (f *fakeRepo) SoftDelete(context.Context, uuid.UUID, string, string) (*core.AuditEvent, error) {
	return &core.AuditEvent{}, nil
}
func (f *fakeRepo) ListTypes(context.Context, bool) ([]*DocumentType, error) { return nil, nil }

// stubCoreRepo satisfies core.Repository so a core.Service can be constructed; the
// document tests below never exercise the core paths.
type stubCoreRepo struct{}

func (stubCoreRepo) CreateSubject(context.Context, core.CreateSubjectInput) (*core.SubjectRef, *core.RecordMetadata, *core.AuditEvent, error) {
	return nil, nil, nil, nil
}
func (stubCoreRepo) GetSubject(context.Context, uuid.UUID) (*core.SubjectRef, error) { return nil, nil }
func (stubCoreRepo) GetRecordMetadata(context.Context, uuid.UUID) (*core.RecordMetadata, error) {
	return nil, nil
}
func (stubCoreRepo) LinkSubjects(context.Context, core.LinkInput) (*core.SubjectRelationship, *core.AuditEvent, error) {
	return nil, nil, nil
}
func (stubCoreRepo) UnlinkSubjects(context.Context, uuid.UUID, string, string) (*core.SubjectRelationship, *core.AuditEvent, error) {
	return nil, nil, nil
}
func (stubCoreRepo) ListRelationships(context.Context, core.RelationshipFilter) (core.RelationshipResult, error) {
	return core.RelationshipResult{}, nil
}
func (stubCoreRepo) ListRelationshipTypes(context.Context, bool, core.SubjectKind, core.SubjectKind) ([]*core.RelationshipType, error) {
	return nil, nil
}
func (stubCoreRepo) AppendAuditEvent(context.Context, core.AuditEvent) (*core.AuditEvent, error) {
	return nil, nil
}
func (stubCoreRepo) ListAuditEvents(context.Context, core.AuditFilter) (core.AuditResult, error) {
	return core.AuditResult{}, nil
}

func newTestService(t *testing.T, repo Repository) *Service {
	t.Helper()
	coreSvc, err := core.NewService(stubCoreRepo{}, nil)
	if err != nil {
		t.Fatalf("core service: %v", err)
	}
	svc, err := NewService(repo, coreSvc, nil)
	if err != nil {
		t.Fatalf("document service: %v", err)
	}
	return svc
}

func TestCreateValidation(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(t, repo)

	bad := []struct {
		name string
		in   CreateInput
	}{
		{"empty title", CreateInput{DocumentTypeCode: "PLAN"}},
		{"blank title", CreateInput{Title: "   ", DocumentTypeCode: "PLAN"}},
		{"empty type", CreateInput{Title: "x"}},
		{"title too long", CreateInput{Title: strings.Repeat("a", MaxTitleLength+1), DocumentTypeCode: "PLAN"}},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := svc.Create(context.Background(), tt.in)
			if !errors.Is(err, core.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
	if repo.createCalls != 0 {
		t.Fatalf("repository.Create must not be called for invalid input, got %d calls", repo.createCalls)
	}
}

func TestCreatePopulatesGovernanceAndOperator(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(t, repo)

	_, _, _, err := svc.Create(context.Background(), CreateInput{
		Title:            "  Plan de masse  ",
		DocumentTypeCode: "PLAN",
		OperatorID:       "42",
		ExternalURL:      "https://x/y",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.lastCreate
	if got.Title != "Plan de masse" {
		t.Fatalf("title should be trimmed, got %q", got.Title)
	}
	if got.Governance.Kind != core.SubjectKindDocument {
		t.Fatalf("governance kind = %q, want DOCUMENT", got.Governance.Kind)
	}
	if got.Governance.DisplayLabel != "Plan de masse" {
		t.Fatalf("governance label = %q, want the (trimmed) title", got.Governance.DisplayLabel)
	}
	if got.Governance.OperatorID != "42" {
		t.Fatalf("governance operator = %q, want 42", got.Governance.OperatorID)
	}
	if got.Governance.OwnerUserID != "42" {
		t.Fatalf("owner should default to the operator, got %q", got.Governance.OwnerUserID)
	}
	if got.Governance.CanonicalURL != "https://x/y" {
		t.Fatalf("canonical url = %q, want external url", got.Governance.CanonicalURL)
	}
}

func TestUpdateMetadataPropagatesLock(t *testing.T) {
	repo := &fakeRepo{updateErr: core.ErrLocked}
	svc := newTestService(t, repo)

	_, _, err := svc.UpdateMetadata(context.Background(), uuid.New(), UpdateInput{Title: "x"})
	if !errors.Is(err, core.ErrLocked) {
		t.Fatalf("expected ErrLocked to propagate, got %v", err)
	}
}

func TestUpdateMetadataValidation(t *testing.T) {
	svc := newTestService(t, &fakeRepo{})
	if _, _, err := svc.UpdateMetadata(context.Background(), uuid.Nil, UpdateInput{Title: "x"}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("nil id should be ErrInvalidInput, got %v", err)
	}
	if _, _, err := svc.UpdateMetadata(context.Background(), uuid.New(), UpdateInput{Title: "  "}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("blank title should be ErrInvalidInput, got %v", err)
	}
}

func TestHashMatches(t *testing.T) {
	const h = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	tests := []struct {
		name           string
		stored, expect string
		want           bool
	}{
		{"match", h, h, true},
		{"case-insensitive", h, strings.ToUpper(h), true},
		{"mismatch", h, "0000", false},
		{"blank expected is never verified", h, "", false},
		{"blank stored is never verified", "", h, false},
		{"both blank", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hashMatches(tt.stored, tt.expect); got != tt.want {
				t.Fatalf("hashMatches(%q,%q) = %v, want %v", tt.stored, tt.expect, got, tt.want)
			}
		})
	}
}
