package actor

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// contactValueCase is one entry of testdata/contact_values.json, the fixture
// shared with the SPA's contactRules.test.ts so both rule sets stay in step.
type contactValueCase struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Value      string `json:"value"`
	Valid      bool   `json:"valid"`
	Normalized string `json:"normalized"`
}

func TestNormalizeContactValueSharedCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/contact_values.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Cases []contactValueCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) == 0 {
		t.Fatalf("decode fixture: %v", err)
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			typ, ok := goelandv1.ContactType_value[tc.Type]
			if !ok {
				t.Fatalf("unknown contact type %q", tc.Type)
			}
			got, err := NormalizeContactValue(ContactType(typ), tc.Value)
			switch {
			case tc.Valid && (err != nil || got != tc.Normalized):
				t.Fatalf("NormalizeContactValue(%s, %q) = %q, %v; want %q", tc.Type, tc.Value, got, err, tc.Normalized)
			case !tc.Valid && !errors.Is(err, core.ErrInvalidInput):
				t.Fatalf("NormalizeContactValue(%s, %q): want ErrInvalidInput, got %q, %v", tc.Type, tc.Value, got, err)
			}
		})
	}
}

func TestNormalizeContactsRequiresLabelForOther(t *testing.T) {
	if _, err := normalizeContacts([]ContactInput{{ContactType: ContactTypeOther, Value: "x"}}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("OTHER without label: want ErrInvalidInput, got %v", err)
	}
	got, err := normalizeContacts([]ContactInput{{ContactType: ContactTypePhone, Value: "021 315 22 22"}})
	if err != nil || got[0].Value != "+41213152222" {
		t.Fatalf("phone not normalized: %+v, %v", got, err)
	}
}
