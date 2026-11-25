package document

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/filestore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/coretest"
)

// TestVerifyContentOutcomes covers GLD-021: the stored bytes are reread and
// compared with the registered digest and size; only bytes read are recorded.
func TestVerifyContentOutcomes(t *testing.T) {
	store, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stored, err := store.Put(context.Background(), strings.NewReader("plan de situation"), blobMeta())
	if err != nil {
		t.Fatal(err)
	}
	held := func(sha string, size int64, ref string) *ContentBlob {
		return &ContentBlob{ID: uuid.New(), SHA256: sha, FileSizeBytes: size, StorageRef: ref}
	}
	tests := []struct {
		name    string
		content *ContentBlob
		want    VerificationOutcome
	}{
		{"intact bytes", held(stored.SHA256, stored.Size, stored.Ref), OutcomeVerified},
		{"other digest", held(strings.Repeat("b", 64), stored.Size, stored.Ref), OutcomeMismatch},
		{"other size", held(stored.SHA256, stored.Size+1, stored.Ref), OutcomeMismatch},
		{"bytes gone", held(stored.SHA256, stored.Size, "internal://absent.pdf"), OutcomeMissing},
		{"known by digest only", held(stored.SHA256, stored.Size, ""), OutcomeNotHeld},
		{"no content", nil, OutcomeNotHeld},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{version: &Version{ID: uuid.New(), VersionNo: 1, Content: tt.content}}
			svc, err := NewService(repo, coretest.NewService(t), store, nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := svc.VerifyContent(context.Background(), uuid.New(), nil, "42")
			if err != nil || res.Outcome != tt.want {
				t.Fatalf("outcome %q, want %q (%v)", res.Outcome, tt.want, err)
			}
			if tt.want == OutcomeNotHeld {
				if len(repo.recorded) != 0 || res.Event != nil || res.StorageRefChecked != "" {
					t.Fatalf("nothing is recorded without bytes: %+v", repo.recorded)
				}
				return
			}
			if len(repo.recorded) != 1 || res.Event == nil || res.StorageRefChecked != tt.content.StorageRef {
				t.Fatalf("the verification is recorded once: %+v", repo.recorded)
			}
			if (res.VerifiedAt != nil) != (tt.want == OutcomeVerified) {
				t.Fatalf("verified_at only after a match: %v", res.VerifiedAt)
			}
			if tt.want == OutcomeVerified && (res.ComputedSHA256 != stored.SHA256 || res.ComputedSize != stored.Size) {
				t.Fatalf("computed digest and size reported: %+v", res)
			}
		})
	}
}

// blobMeta describes the test upload.
func blobMeta() blobstore.Metadata {
	return blobstore.Metadata{Filename: "plan.pdf", ContentType: "application/pdf"}
}

// TestVerifyContentNeedsStoreAndDocument covers the refusals.
func TestVerifyContentNeedsStoreAndDocument(t *testing.T) {
	svc := newTestService(t, &fakeRepo{})
	if _, err := svc.VerifyContent(context.Background(), uuid.New(), nil, "42"); !errors.Is(err, ErrNoContentStore) {
		t.Fatalf("a service without a store cannot verify: %v", err)
	}
	store, _ := filestore.New(t.TempDir())
	withStore, _ := NewService(&fakeRepo{}, coretest.NewService(t), store, nil)
	if _, err := withStore.VerifyContent(context.Background(), uuid.Nil, nil, "42"); err == nil {
		t.Fatal("a document id is required")
	}
}
