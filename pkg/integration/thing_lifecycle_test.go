package integration

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/thing"
)

// square returns a GeoJSON LV95 square of side metres with its south-west corner at (e, n).
func square(e, n, side float64) string {
	return fmt.Sprintf(`{"type":"Polygon","coordinates":[[[%[1]g,%[2]g],[%[3]g,%[2]g],[%[3]g,%[4]g],[%[1]g,%[4]g],[%[1]g,%[2]g]]]}`, e, n, e+side, n+side)
}

// TestThingLifecycle covers the spec scenario (parcel, building on it, case
// concerning the parcel, owner) plus geometry, identifier and search rules.
func TestThingLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	// Unique parcel number, EGID and LV95 spot per run, so reruns never collide.
	token := uniqueToken()
	number := "P-" + token
	e0 := 2_530_000 + float64(rand.IntN(20_000))*10
	n0 := 1_150_000 + float64(rand.IntN(1_000))*10

	parcel, ev, err := env.thingSvc.Create(ctx, thing.CreateInput{
		TypeCode: "PARCEL", GeometryGeoJSON: square(e0, n0, 100), OperatorID: testOperator,
		Parcel: &thing.Parcel{CommuneOFS: 5586, ParcelNumber: number},
	})
	if err != nil {
		t.Fatalf("create parcel: %v", err)
	}
	if parcel.Name != "Parcelle "+number || ev.EventType != "THING_CREATED" || parcel.Parcel == nil {
		t.Fatalf("parcel not created as expected: %+v %+v", parcel, ev)
	}
	if parcel.AreaM2 == nil || math.Abs(*parcel.AreaM2-10_000) > 0.01 || parcel.AnchorE == nil {
		t.Fatalf("area/anchor not computed: %+v", parcel)
	}

	egid := int32(100_000_000 + rand.IntN(800_000_000))
	building, _, err := env.thingSvc.Create(ctx, thing.CreateInput{
		TypeCode: "BUILDING", GeometryGeoJSON: fmt.Sprintf(`{"type":"Point","coordinates":[%g,%g]}`, e0+50, n0+50),
		Building: &thing.Building{EGID: &egid, Status: 4}, OperatorID: testOperator,
	})
	if err != nil {
		t.Fatalf("create building: %v", err)
	}
	link(t, env, parcel.ID, core.LinkInput{TargetSubjectID: building.ID, RelationshipTypeCode: "THING_CONTAINS_THING"})
	c := openCase(t, env, "Permis "+token)
	link(t, env, c.ID, core.LinkInput{TargetSubjectID: parcel.ID, RelationshipTypeCode: "CASE_CONCERNS_THING"})
	link(t, env, parcel.ID, core.LinkInput{TargetSubjectID: newActor(t, env).ID, RelationshipTypeCode: "THING_HAS_ACTOR_OWNER"})
	if rels, err := env.thingSvc.Relationships(ctx, parcel.ID); err != nil || len(rels) != 3 {
		t.Fatalf("parcel relationships (building, owner, case): %d (%v)", len(rels), err)
	}

	assertThingSearch(t, env, parcel, e0, n0)
	assertGeometryRules(t, env, e0, n0)

	// Identifiers are unique: same commune + number, and same EGID.
	if _, _, err := env.thingSvc.Create(ctx, thing.CreateInput{TypeCode: "PARCEL", OperatorID: testOperator, Parcel: &thing.Parcel{CommuneOFS: 5586, ParcelNumber: number}}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate parcel number: want ErrConflict, got %v", err)
	}
	if _, _, err := env.thingSvc.Create(ctx, thing.CreateInput{TypeCode: "BUILDING", Name: "Twin", OperatorID: testOperator, Building: &thing.Building{EGID: &egid}}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate EGID: want ErrConflict, got %v", err)
	}

	updated, uev, err := env.thingSvc.Update(ctx, parcel.ID, thing.UpdateInput{Name: "Parcelle agrandie", GeometryGeoJSON: square(e0, n0, 120), OperatorID: testOperator})
	if err != nil || math.Abs(*updated.AreaM2-14_400) > 0.01 || uev.AfterState["geometry_changed"] != true || updated.Parcel == nil {
		t.Fatalf("update geometry: %+v %+v (%v)", updated, uev, err)
	}
	if _, err := env.thingSvc.SoftDelete(ctx, building.ID, testOperator, "démoli"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
}

// assertThingSearch finds the parcel by number, text and LV95 extent.
func assertThingSearch(t *testing.T, env *testEnv, parcel *thing.Thing, e0, n0 float64) {
	t.Helper()
	for name, filter := range map[string]thing.SearchFilter{
		"parcel number": {Query: parcel.Parcel.ParcelNumber},
		"bbox":          {TypeCode: "PARCEL", BBox: &thing.BBox{EMin: e0 + 10, NMin: n0 + 10, EMax: e0 + 20, NMax: n0 + 20}},
	} {
		res, err := env.thingSvc.Search(env.ctx, filter)
		if err != nil || !containsThing(res.Things, parcel) {
			t.Fatalf("search by %s must find the parcel: %v", name, err)
		}
	}
	far, err := env.thingSvc.Search(env.ctx, thing.SearchFilter{BBox: &thing.BBox{EMin: e0 + 500, NMin: n0 + 500, EMax: e0 + 600, NMax: n0 + 600}})
	if err != nil || containsThing(far.Things, parcel) {
		t.Fatalf("a distant extent must not find the parcel: %v", err)
	}
}

// assertGeometryRules checks the refused geometries.
func assertGeometryRules(t *testing.T, env *testEnv, e0, n0 float64) {
	t.Helper()
	bowtie := fmt.Sprintf(`{"type":"Polygon","coordinates":[[[%[1]g,%[2]g],[%[3]g,%[4]g],[%[3]g,%[2]g],[%[1]g,%[4]g],[%[1]g,%[2]g]]]}`, e0, n0, e0+10, n0+10)
	for name, in := range map[string]thing.CreateInput{
		"WGS84 degrees":       {TypeCode: "TREE", Name: "Arbre", GeometryGeoJSON: `{"type":"Point","coordinates":[6.6323,46.5197]}`},
		"point for a parcel":  {TypeCode: "PARCEL", GeometryGeoJSON: fmt.Sprintf(`{"type":"Point","coordinates":[%g,%g]}`, e0, n0), Parcel: &thing.Parcel{CommuneOFS: 5586, ParcelNumber: "X" + uniqueToken()}},
		"self-intersecting":   {TypeCode: "TREE", Name: "Zone", GeometryGeoJSON: bowtie},
		"malformed GeoJSON":   {TypeCode: "TREE", Name: "Arbre", GeometryGeoJSON: `{"type":"Point","coordinates":"x"}`},
		"parcel without data": {TypeCode: "PARCEL", Name: "Sans numéro"},
	} {
		in.OperatorID = testOperator
		if _, _, err := env.thingSvc.Create(env.ctx, in); !errors.Is(err, core.ErrInvalidInput) {
			t.Fatalf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
}

func containsThing(things []*thing.Thing, target *thing.Thing) bool {
	for _, th := range things {
		if th.ID == target.ID {
			return true
		}
	}
	return false
}
