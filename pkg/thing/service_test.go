package thing

import (
	"errors"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

func TestCheckGeometryType(t *testing.T) {
	polygon := `{"type":"Polygon","coordinates":[[[2538000,1152000],[2538100,1152000],[2538100,1152100],[2538000,1152000]]]}`
	point := `{"type":"Point","coordinates":[2538050,1152050]}`
	line := `{"type":"LineString","coordinates":[[2538000,1152000],[2538100,1152100]]}`
	ok := []struct {
		geo  string
		spec Specialization
	}{{polygon, SpecializationParcel}, {point, SpecializationBuilding}, {polygon, SpecializationBuilding}, {line, SpecializationGeneric}}
	for _, c := range ok {
		if err := checkGeometryType(c.geo, c.spec); err != nil {
			t.Fatalf("%s for %d: %v", c.geo[:20], c.spec, err)
		}
	}
	bad := []struct {
		geo  string
		spec Specialization
	}{{point, SpecializationParcel}, {line, SpecializationBuilding}, {"not json", SpecializationGeneric}, {`{"coordinates":[1,2]}`, SpecializationGeneric}}
	for _, c := range bad {
		if err := checkGeometryType(c.geo, c.spec); !errors.Is(err, core.ErrInvalidInput) {
			t.Fatalf("%q for %d: want ErrInvalidInput, got %v", c.geo, c.spec, err)
		}
	}
}

func TestParseBBox(t *testing.T) {
	box, err := ParseBBox(" 2537000, 1151000,2539000,1153000 ")
	if err != nil || box.EMin != 2537000 || box.NMax != 1153000 {
		t.Fatalf("parse: %+v, %v", box, err)
	}
	if box, err := ParseBBox(""); box != nil || err != nil {
		t.Fatalf("empty bbox must mean anywhere: %+v, %v", box, err)
	}
	for _, bad := range []string{"1,2,3", "a,b,c,d", "2539000,1151000,2537000,1153000"} {
		if _, err := ParseBBox(bad); !errors.Is(err, core.ErrInvalidInput) {
			t.Fatalf("%q: want ErrInvalidInput, got %v", bad, err)
		}
	}
}

func TestSwissExtentRejectsWGS84(t *testing.T) {
	if swissExtent.contains(geometryCheck{XMin: 6.63, YMin: 46.52, XMax: 6.64, YMax: 46.53}) {
		t.Fatal("WGS84 degrees must be outside the LV95 extent")
	}
	if !swissExtent.contains(geometryCheck{XMin: 2538000, YMin: 1152000, XMax: 2538100, YMax: 1152100}) {
		t.Fatal("Lausanne LV95 coordinates must be inside")
	}
}

func TestDetailsAndDerivedNames(t *testing.T) {
	egid := int32(190_123_456)
	parcel := &Parcel{CommuneOFS: 5586, ParcelNumber: " 1234 ", EGRID: "ch123456789012"}
	in := CreateInput{Parcel: parcel}
	if err := prepareCreate(&in, SpecializationParcel); err != nil || in.Name != "Parcelle 1234" || parcel.EGRID != "CH123456789012" {
		t.Fatalf("parcel: %+v %+v %v", in, parcel, err)
	}
	b := CreateInput{Building: &Building{EGID: &egid}}
	if err := prepareCreate(&b, SpecializationBuilding); err != nil || b.Name != "Bâtiment EGID 190123456" {
		t.Fatalf("building: %q %v", b.Name, err)
	}
	year := int16(900)
	zero := 0.0
	cases := map[string]error{
		"parcel block on a building": checkDetails(SpecializationBuilding, &Parcel{CommuneOFS: 1, ParcelNumber: "1"}, nil, true),
		"missing parcel block":       checkDetails(SpecializationParcel, nil, nil, true),
		"bad commune":                checkParcel(&Parcel{CommuneOFS: 0, ParcelNumber: "1"}),
		"bad egrid":                  checkParcel(&Parcel{CommuneOFS: 5586, ParcelNumber: "1", EGRID: "CH12"}),
		"zero surface":               checkParcel(&Parcel{CommuneOFS: 5586, ParcelNumber: "1", SurfaceM2: &zero}),
		"old building":               checkBuilding(&Building{ConstructionYear: &year}),
		"bad status":                 checkBuilding(&Building{Status: 9}),
		"generic without name":       prepareCreate(&CreateInput{}, SpecializationGeneric),
	}
	for name, err := range cases {
		if !errors.Is(err, core.ErrInvalidInput) {
			t.Fatalf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
	if err := checkDetails(SpecializationBuilding, nil, nil, true); err != nil {
		t.Fatalf("a building block is optional: %v", err)
	}
}
