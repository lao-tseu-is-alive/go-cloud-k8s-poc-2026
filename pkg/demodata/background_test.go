package demodata

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// TestBackgroundTexts checks that every language gives each background case
// type its subjects (one address each), each of its documents a closing line
// and each of its things a name.
func TestBackgroundTexts(t *testing.T) {
	for _, lang := range Languages() {
		texts, err := textsFor(lang)
		if err != nil {
			t.Fatal(err)
		}
		bg := texts.Background
		for _, format := range []string{bg.Description, bg.Lines.Case, bg.Lines.Reference, bg.Lines.Party, bg.Lines.Date, bg.Things["PARCEL"]} {
			if strings.Count(format, "%s") != 1 {
				t.Errorf("%s: %q needs one %%s", lang, format)
			}
		}
		for _, bc := range backgroundCases {
			caseTypeByCode(bc.caseType) // panics on a code the catalogue lacks
			if len(bg.Subjects[bc.caseType]) == 0 {
				t.Errorf("%s: no subject for %s", lang, bc.caseType)
			}
			for _, subject := range bg.Subjects[bc.caseType] {
				if strings.Count(subject, "%s") != 1 {
					t.Errorf("%s: subject %q needs one %%s", lang, subject)
				}
			}
			for _, docType := range slices.Concat(bc.filed, bc.answer) {
				if bg.Bodies[docType] == "" || texts.DocumentTypes[docType].Label == "" {
					t.Errorf("%s: document type %s has no closing line or label", lang, docType)
				}
			}
			if name := bg.Things[bc.thingType]; bc.thingType != "" && strings.Count(name, "%s") != 1 {
				t.Errorf("%s: thing type %s needs a name with one %%s", lang, bc.thingType)
			}
		}
	}
}

// TestBackgroundPlan checks the planned background: the actors' names are
// unique, every thing lies inside the map tiles with a geometry its type
// takes, and the reference numbers start above the stories'.
func TestBackgroundPlan(t *testing.T) {
	b := newBackdrop(&run{})
	names := map[string]bool{}
	for _, p := range b.parties {
		if names[p.name] {
			t.Errorf("actor %q planned twice", p.name)
		}
		names[p.name] = true
	}
	if len(b.parties) != backgroundPersons+backgroundCompanies {
		t.Errorf("%d actors planned", len(b.parties))
	}
	counts := map[string]int{}
	for thingType, spots := range b.spots {
		for _, s := range spots {
			counts[thingType]++
			checkGeometry(t, thingType, s.geometry)
			if s.parcel != nil {
				counts["PARCEL"]++
				checkGeometry(t, "PARCEL", s.parcel.geometry)
			}
		}
	}
	for thingType, n := range backgroundThings {
		if counts[thingType] != n {
			t.Errorf("%s: %d planned, want %d", thingType, counts[thingType], n)
		}
	}
	if first, second := b.number("OPC", 2025), b.number("OPC", 2025); first != "2025-000151" || second != "2025-000152" {
		t.Errorf("reference numbers %s, %s", first, second)
	}
}

// checkGeometry fails unless geometry is a GeoJSON of a type thingType takes
// whose positions all lie inside the map tiles.
func checkGeometry(t *testing.T, thingType, geometry string) {
	t.Helper()
	var g struct {
		Type        string `json:"type"`
		Coordinates any    `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(geometry), &g); err != nil {
		t.Fatalf("%s: %v", thingType, err)
	}
	if allowed, ok := geometryTypes[thingType]; ok && !slices.Contains(allowed, g.Type) {
		t.Errorf("%s: a %s", thingType, g.Type)
	}
	var walk func(any)
	walk = func(c any) {
		list, _ := c.([]any)
		if east, ok := list[0].(float64); ok {
			north, _ := list[1].(float64)
			if east < mapExtent[0] || north < mapExtent[1] || east > mapExtent[2] || north > mapExtent[3] {
				t.Errorf("%s: %.1f, %.1f lies outside the map tiles", thingType, east, north)
			}
			return
		}
		for _, item := range list {
			walk(item)
		}
	}
	walk(g.Coordinates)
}
