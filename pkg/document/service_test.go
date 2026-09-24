package document

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// fakeRepo is a minimal document.Repository capturing inputs for assertions.
type fakeRepo struct {
	lastCreate  CreateInput
	createCalls int
	createErr   error
	updateErr   error
	// registerReused / registerErr drive RegisterBlob.
	registerReused bool
	registerErr    error
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput) (CreateResult, error) {
	f.lastCreate = in
	f.createCalls++
	if f.createErr != nil {
		return CreateResult{}, f.createErr
	}
	return CreateResult{Document: &Document{ID: uuid.New(), Title: in.Title}, Event: &core.AuditEvent{}}, nil
}
func (f *fakeRepo) AddVersion(context.Context, uuid.UUID, VersionInput) (*Document, *Version, *core.AuditEvent, error) {
	return &Document{}, &Version{VersionNo: 2}, &core.AuditEvent{}, nil
}
func (f *fakeRepo) ListVersions(context.Context, uuid.UUID) ([]*Version, error) { return nil, nil }
func (f *fakeRepo) RegisterBlob(_ context.Context, blob ContentBlob) (*ContentBlob, bool, error) {
	if f.registerErr != nil {
		return nil, false, f.registerErr
	}
	blob.ID = uuid.New()
	return &blob, f.registerReused, nil
}
func (f *fakeRepo) FindBlobBySHA256(context.Context, string) (*ContentBlob, error) {
	return nil, core.ErrNotFound
}

// fakeStore records saved and removed references.
type fakeStore struct {
	saved   []string
	removed []string
}

func (f *fakeStore) Put(_ context.Context, r io.Reader, meta blobstore.Metadata) (blobstore.Stored, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return blobstore.Stored{}, err
	}
	ref := "fake://" + uuid.NewString()
	f.saved = append(f.saved, ref)
	return blobstore.Stored{Ref: ref, SHA256: strings.Repeat("a", 64), Size: int64(len(data)), Filename: meta.Filename}, nil
}
func (f *fakeStore) Get(context.Context, string) (blobstore.Object, error) {
	return nil, blobstore.ErrNotFound
}
func (f *fakeStore) Delete(_ context.Context, ref string) error {
	f.removed = append(f.removed, ref)
	return nil
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
func (stubCoreRepo) AssignBusinessRef(context.Context, uuid.UUID, core.BusinessRefRequest, string, string) (*core.SubjectRef, *core.AuditEvent, error) {
	return nil, nil, nil
}
func (stubCoreRepo) LookupSubjects(context.Context, core.LookupFilter, int) ([]*core.SubjectRef, error) {
	return nil, nil
}
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

func newTestCoreService(t *testing.T) *core.Service {
	t.Helper()
	coreSvc, err := core.NewService(stubCoreRepo{}, nil)
	if err != nil {
		t.Fatalf("core service: %v", err)
	}
	return coreSvc
}

func newTestService(t *testing.T, repo Repository) *Service {
	t.Helper()
	svc, err := NewService(repo, newTestCoreService(t), nil, nil)
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
			_, err := svc.Create(context.Background(), tt.in)
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

	_, err := svc.Create(context.Background(), CreateInput{
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

func TestIngestContent(t *testing.T) {
	coreSvc := newTestCoreService(t)
	tests := []struct {
		name        string
		repo        *fakeRepo
		wantErr     bool
		wantReused  bool
		wantRemoved bool
	}{
		{name: "new content is kept", repo: &fakeRepo{}},
		{name: "duplicate bytes are removed", repo: &fakeRepo{registerReused: true}, wantReused: true, wantRemoved: true},
		{name: "unregistered bytes are removed on failure", repo: &fakeRepo{registerErr: errors.New("db down")}, wantErr: true, wantRemoved: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{}
			svc, err := NewService(tt.repo, coreSvc, store, nil)
			if err != nil {
				t.Fatalf("service: %v", err)
			}
			res, err := svc.IngestContent(context.Background(), strings.NewReader("pdf bytes"), "plan.pdf", "application/pdf", "42")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (res.Reused != tt.wantReused || res.Blob.FileSizeBytes != 9 || res.Blob.MimeType != "application/pdf" || res.Blob.CreatedBy != "42") {
				t.Fatalf("unexpected result %+v / %+v", res, res.Blob)
			}
			if (len(store.removed) == 1 && store.removed[0] == store.saved[0]) != tt.wantRemoved {
				t.Fatalf("saved %v, removed %v, want removed=%v", store.saved, store.removed, tt.wantRemoved)
			}
		})
	}
}

func TestIngestContentWithoutStore(t *testing.T) {
	svc := newTestService(t, &fakeRepo{})
	if _, err := svc.IngestContent(context.Background(), strings.NewReader("x"), "x", "", "42"); !errors.Is(err, ErrNoContentStore) {
		t.Fatalf("expected ErrNoContentStore, got %v", err)
	}
}

func TestAddVersionValidation(t *testing.T) {
	svc := newTestService(t, &fakeRepo{})
	if _, _, _, err := svc.AddVersion(context.Background(), uuid.Nil, VersionInput{}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("nil id should be ErrInvalidInput, got %v", err)
	}
	if _, _, _, err := svc.AddVersion(context.Background(), uuid.New(), VersionInput{PageCount: -1}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("negative page count should be ErrInvalidInput, got %v", err)
	}
}
