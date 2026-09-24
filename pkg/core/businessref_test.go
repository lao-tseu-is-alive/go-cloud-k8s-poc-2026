package core

import (
	"errors"
	"strings"
	"testing"
)

func TestBusinessRefRequestNormalized(t *testing.T) {
	tests := []struct {
		name    string
		in      BusinessRefRequest
		want    BusinessRefRequest
		wantErr bool
	}{
		{name: "explicit value without namespace", in: BusinessRefRequest{Value: " LEG-42 "}, want: BusinessRefRequest{Value: "LEG-42"}},
		{name: "explicit value in namespace", in: BusinessRefRequest{Namespace: " OPC ", Value: "2026-000001"}, want: BusinessRefRequest{Namespace: "OPC", Value: "2026-000001"}},
		{name: "allocation in namespace", in: BusinessRefRequest{Namespace: "PERMIS_2", Allocate: true}, want: BusinessRefRequest{Namespace: "PERMIS_2", Allocate: true}},
		{name: "both value and allocate", in: BusinessRefRequest{Namespace: "OPC", Value: "x", Allocate: true}, wantErr: true},
		{name: "neither value nor allocate", in: BusinessRefRequest{Namespace: "OPC"}, wantErr: true},
		{name: "allocate without namespace", in: BusinessRefRequest{Allocate: true}, wantErr: true},
		{name: "lower-case namespace", in: BusinessRefRequest{Namespace: "opc", Value: "x"}, wantErr: true},
		{name: "namespace starting with a digit", in: BusinessRefRequest{Namespace: "1OPC", Value: "x"}, wantErr: true},
		{name: "value too long", in: BusinessRefRequest{Value: strings.Repeat("é", MaxBusinessRefLength+1)}, wantErr: true},
		{name: "blank value", in: BusinessRefRequest{Value: "   "}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Normalized()
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBusinessRefRequestIsZero(t *testing.T) {
	if !(BusinessRefRequest{}).IsZero() {
		t.Fatal("zero value must report IsZero")
	}
	if (BusinessRefRequest{Namespace: "OPC"}).IsZero() {
		t.Fatal("a namespace alone is a (invalid) request, not zero")
	}
}

func TestFormatAllocatedBusinessRef(t *testing.T) {
	tests := map[string]struct {
		period   string
		sequence int64
		want     string
	}{
		"padded":           {period: "2026", sequence: 1245, want: "2026-001245"},
		"first":            {period: "2026", sequence: 1, want: "2026-000001"},
		"wider than width": {period: "2026", sequence: 1234567, want: "2026-1234567"},
	}
	for name, tt := range tests {
		if got := FormatAllocatedBusinessRef(tt.period, tt.sequence); got != tt.want {
			t.Errorf("%s: got %q, want %q", name, got, tt.want)
		}
	}
}
