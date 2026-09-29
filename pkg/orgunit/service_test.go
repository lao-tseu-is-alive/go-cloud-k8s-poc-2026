package orgunit

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

func TestNormalizeToken(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"  SOI ", "SOI", false},
		{"Parcs & domaines", "Parcs & domaines", false},
		{"", "", false},
		{"   ", "", false},
		{strings.Repeat("A", MaxAbbreviationLength+1), "", true},
		{"SO\tI", "", true},
	}
	for _, tt := range tests {
		got, err := normalizeToken("abbreviation", tt.in, MaxAbbreviationLength)
		if tt.wantErr != (err != nil) || got != tt.want {
			t.Errorf("normalizeToken(%q) = %q, %v", tt.in, got, err)
		}
		if tt.wantErr && !errors.Is(err, core.ErrInvalidInput) {
			t.Errorf("normalizeToken(%q): want ErrInvalidInput, got %v", tt.in, err)
		}
	}
}

func TestNormalizeInput(t *testing.T) {
	nilParent := uuid.Nil
	in, err := normalizeInput(Input{TypeCode: " SERVICE ", Abbreviation: " SOI ", Label: " Service ", Email: " Info@Lausanne.CH ", ParentID: &nilParent, Reason: " r "})
	if err != nil {
		t.Fatalf("normalizeInput: %v", err)
	}
	if in.TypeCode != "SERVICE" || in.Abbreviation != "SOI" || in.Label != "Service" || in.Email != "Info@lausanne.ch" || in.ParentID != nil || in.Reason != "r" {
		t.Fatalf("unexpected normalization %+v", in)
	}
	for name, bad := range map[string]Input{
		"missing type":      {Label: "x"},
		"missing label":     {TypeCode: "UNIT"},
		"label too long":    {TypeCode: "UNIT", Label: strings.Repeat("é", MaxLabelLength+1)},
		"description long":  {TypeCode: "UNIT", Label: "x", Description: strings.Repeat("a", MaxDescriptionLength+1)},
		"bad email":         {TypeCode: "UNIT", Label: "x", Email: "Service <info@lausanne.ch>"},
		"long abbreviation": {TypeCode: "UNIT", Label: "x", Abbreviation: strings.Repeat("A", MaxAbbreviationLength+1)},
		"email no domain":   {TypeCode: "UNIT", Label: "x", Email: "info@localhost"},
	} {
		if _, err := normalizeInput(bad); !errors.Is(err, core.ErrInvalidInput) {
			t.Errorf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
}

func TestDisplayLabel(t *testing.T) {
	if got := DisplayLabel("Secrétariat", "SOI"); got != "Secrétariat (SOI)" {
		t.Fatalf("DisplayLabel = %q", got)
	}
	if got := DisplayLabel("Ville", ""); got != "Ville" {
		t.Fatalf("DisplayLabel without abbreviation = %q", got)
	}
}
