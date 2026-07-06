package core

import (
	"errors"
	"testing"
)

func TestParsePageToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		want    int
		wantErr bool
	}{
		{name: "empty is first page", token: "", want: 0},
		{name: "valid offset", token: "50", want: 50},
		{name: "whitespace trimmed", token: "  25 ", want: 25},
		{name: "negative rejected", token: "-1", wantErr: true},
		{name: "non-numeric rejected", token: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePageToken(tt.token)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNextPageToken(t *testing.T) {
	tests := []struct {
		name     string
		offset   int
		returned int
		total    int32
		want     string
	}{
		{name: "no rows returns empty", offset: 0, returned: 0, total: 0, want: ""},
		{name: "more pages available", offset: 0, returned: 25, total: 100, want: "25"},
		{name: "last page returns empty", offset: 75, returned: 25, total: 100, want: ""},
		{name: "exact end returns empty", offset: 0, returned: 10, total: 10, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextPageToken(tt.offset, tt.returned, tt.total); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizePageSize(t *testing.T) {
	if got, err := NormalizePageSize(0); err != nil || got != DefaultPageSize {
		t.Fatalf("zero should default to %d, got %d err %v", DefaultPageSize, got, err)
	}
	if got, err := NormalizePageSize(50); err != nil || got != 50 {
		t.Fatalf("50 should pass through, got %d err %v", got, err)
	}
	if _, err := NormalizePageSize(MaxPageSize + 1); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("over-max should be ErrInvalidInput, got %v", err)
	}
	if _, err := NormalizePageSize(-5); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative should be ErrInvalidInput, got %v", err)
	}
}

func TestSubjectKindValid(t *testing.T) {
	valid := []SubjectKind{SubjectKindCase, SubjectKindDocument, SubjectKindThing, SubjectKindActor, SubjectKindUser, SubjectKindOrgUnit}
	for _, k := range valid {
		if !k.Valid() {
			t.Fatalf("%q should be valid", k)
		}
	}
	for _, k := range []SubjectKind{SubjectKindUnspecified, SubjectKind("BOGUS")} {
		if k.Valid() {
			t.Fatalf("%q should be invalid", k)
		}
	}
}
