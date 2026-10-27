package core

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestParseDateRange(t *testing.T) {
	tests := []struct {
		name, from, to   string
		wantFrom, wantTo any
		wantErr          bool
	}{
		{name: "open", wantFrom: nil, wantTo: nil},
		{name: "both", from: "2026-01-01", to: "2026-03-31", wantFrom: "2026-01-01", wantTo: "2026-03-31"},
		{name: "one day", from: "2026-02-28", to: "2026-02-28", wantFrom: "2026-02-28", wantTo: "2026-02-28"},
		{name: "from only", from: "2020-05-01", wantFrom: "2020-05-01", wantTo: nil},
		{name: "not a day", from: "2026-02-30", wantErr: true},
		{name: "not a date", to: "31.03.2026", wantErr: true},
		{name: "reversed", from: "2026-04-01", to: "2026-03-31", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ParseDateRange(tt.from, tt.to)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("want ErrInvalidInput, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			args := r.AddTo(pgx.NamedArgs{}, "opened")
			if args["opened_from"] != tt.wantFrom || args["opened_to"] != tt.wantTo {
				t.Fatalf("args = %v, want from %v to %v", args, tt.wantFrom, tt.wantTo)
			}
		})
	}
}

func TestRecordFilterAddTo(t *testing.T) {
	f, err := ParseRecordFilter("", "2026-10-27", "u-1")
	if err != nil {
		t.Fatal(err)
	}
	args := f.AddTo(pgx.NamedArgs{})
	if args["created_by"] != "u-1" || args["created_from"] != nil || args["created_to"] != "2026-10-27" {
		t.Fatalf("args = %v", args)
	}
}
