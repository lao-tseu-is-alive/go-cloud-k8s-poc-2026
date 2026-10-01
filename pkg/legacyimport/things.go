package legacyimport

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const (
	keyThingType = "THING_TYPE"
	// legacyParcelType and legacyBuildingType are the legacy types with a specialization.
	legacyParcelType   = 3
	legacyBuildingType = 5
	// centimetresPerMetre: the legacy positions are LV95 coordinates in centimetres.
	centimetresPerMetre = 100.0
)

// seededThingTypes maps legacy thing types to the seeded POC types; the others
// become generic types coded LEG_<IdTypeThing>.
var seededThingTypes = map[int64]string{
	legacyParcelType:   "PARCEL",
	legacyBuildingType: "BUILDING",
	74:                 "TREE",
	46:                 "ADVERTISEMENT",
}

// sourceThingType is a legacy thing type.
type sourceThingType struct {
	// ID is the source column idtypething.
	ID int64 `db:"id"`
	// Name is the source column name.
	Name string `db:"name"`
	// Description is the source column description.
	Description string `db:"description"`
	// Active is the source column isactive.
	Active bool `db:"active"`
}

var thingTypeColumns = []string{"id", "code", "label", "description", "specialization", "is_active"}

// importThingTypes maps the legacy thing types to the seeded ones or creates
// generic LEG_<id> types.
func (imp *Importer) importThingTypes(ctx context.Context, c *StageCounts) error {
	types, err := querySource[sourceThingType](ctx, imp, thingTypesSQL)
	if err != nil {
		return err
	}
	c.Read = len(types)
	seeded, err := imp.codeIDs(ctx, `SELECT code, id FROM thing_type`)
	if err != nil {
		return err
	}
	var rows [][]any
	for _, t := range types {
		if code, ok := seededThingTypes[t.ID]; ok {
			imp.thingTypes[t.ID] = seeded[code]
			c.adjust("mapped to a seeded type")
			continue
		}
		id := ID(keyThingType, t.ID)
		label := t.Name
		if label == "" {
			label = fmt.Sprintf("Type d'objet %d", t.ID)
		}
		rows = append(rows, []any{id, "LEG_" + strconv.FormatInt(t.ID, 10), label, t.Description, int16(0), t.Active})
		imp.thingTypes[t.ID] = id
	}
	if err := imp.copyRows(ctx, "thing_type", thingTypeColumns, rows); err != nil {
		return err
	}
	c.Written = len(rows)
	return nil
}

// sourceThing is a legacy thing with its position and specializations.
type sourceThing struct {
	// ID is the source column idthing.
	ID int64 `db:"id"`
	// TypeID is the source column idtypething.
	TypeID int64 `db:"type_id"`
	// Name is the source column name.
	Name string `db:"name"`
	// Description is the source column description.
	Description string `db:"description"`
	// CreatedAt is the source column datecreated.
	CreatedAt *time.Time `db:"created_at"`
	// CreatorID is the source column idcreator.
	CreatorID *int64 `db:"creator_id"`
	// MinE, MaxE, MinN and MaxN are the thing_position extent in centimetres.
	MinE *int64 `db:"min_e"`
	// MaxE is the source column maxeo.
	MaxE *int64 `db:"max_e"`
	// MinN is the source column minsn.
	MinN *int64 `db:"min_n"`
	// MaxN is the source column maxsn.
	MaxN *int64 `db:"max_n"`
	// Commune is the parcel's OFS commune number (parcelle.idcommune).
	Commune *int32 `db:"commune"`
	// ParcelNumber is the source column numparcelle.
	ParcelNumber string `db:"parcel_number"`
	// EGRID is the source column egrid.
	EGRID string `db:"egrid"`
	// Surface is the source column surface.
	Surface *int64 `db:"surface"`
	// EGID is the building's federal id (thi_building_egid.egid).
	EGID *int32 `db:"egid"`
}

var (
	thingColumns         = []string{"id", "thing_type_id", "name", "description", "external_ref", "geom", "metadata", "created_at", "created_by"}
	thingParcelColumns   = []string{"thing_id", "commune_ofs", "parcel_number", "egrid", "surface_m2"}
	thingBuildingColumns = []string{"thing_id", "egid"}
)

// thingLoad collects the rows of the thing stage.
type thingLoad struct {
	subjects             []subject
	things, parcels, egs [][]any
	egids                map[int32]bool
	egrids               map[string]bool
}

// importThings writes the things with an approximate location (the legacy
// keeps an extent: a point is exact, a wider extent gives its centre, a parcel
// gets none since its shape is unknown), and the parcel and building details.
func (imp *Importer) importThings(ctx context.Context, c *StageCounts) error {
	things, err := querySource[sourceThing](ctx, imp, thingsSQL)
	if err != nil {
		return err
	}
	c.Read = len(things)
	load := &thingLoad{egids: map[int32]bool{}, egrids: map[string]bool{}}
	for _, t := range things {
		imp.addThing(t, load, c)
	}
	if err := imp.writeSubjects(ctx, load.subjects); err != nil {
		return err
	}
	if err := imp.copyThings(ctx, load.things); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "thing_parcel", thingParcelColumns, load.parcels); err != nil {
		return err
	}
	if err := imp.copyRows(ctx, "thing_building", thingBuildingColumns, load.egs); err != nil {
		return err
	}
	c.Written = len(load.things)
	return nil
}

// addThing adds one legacy thing to the load.
func (imp *Importer) addThing(t *sourceThing, load *thingLoad, c *StageCounts) {
	typeID, ok := imp.thingTypes[t.TypeID]
	if !ok {
		c.skip("unknown thing type")
		return
	}
	name := t.Name
	if name == "" {
		name = fmt.Sprintf("Objet %d", t.ID)
	}
	s := newSubject(core.SubjectKindThing, "thing", t.ID, name)
	s.createdAt, s.createdBy = t.CreatedAt, imp.operator(t.CreatorID)
	geom, metadata := imp.thingLocation(t, c)
	load.subjects = append(load.subjects, s)
	load.things = append(load.things, []any{s.id, typeID, name, t.Description, "goeland:" + strconv.FormatInt(t.ID, 10), geom, metadata,
		firstTime(t.CreatedAt, &imp.now), s.createdBy})
	switch t.TypeID {
	case legacyParcelType:
		load.addParcel(s.id, t, c)
	case legacyBuildingType:
		load.addBuilding(s.id, t, c)
	}
}

// addParcel adds the parcel details of a thing; an EGRID already taken is left empty.
func (load *thingLoad) addParcel(id uuid.UUID, t *sourceThing, c *StageCounts) {
	if t.Commune == nil || t.ParcelNumber == "" {
		c.adjust("parcel without a commune or number, no parcel details")
		return
	}
	var surface any
	if t.Surface != nil && *t.Surface > 0 {
		surface = *t.Surface
	}
	egrid := t.EGRID
	if egrid != "" && load.egrids[egrid] {
		egrid = ""
		c.adjust("EGRID already used by another parcel, left empty")
	}
	load.egrids[egrid] = egrid != ""
	load.parcels = append(load.parcels, []any{id, *t.Commune, t.ParcelNumber, egrid, surface})
}

// addBuilding adds the building details of a thing; an EGID already taken is left empty.
func (load *thingLoad) addBuilding(id uuid.UUID, t *sourceThing, c *StageCounts) {
	var egid any
	if t.EGID != nil && !load.egids[*t.EGID] {
		load.egids[*t.EGID] = true
		egid = *t.EGID
	} else if t.EGID != nil {
		c.adjust("EGID already used by another building, left empty")
	}
	load.egs = append(load.egs, []any{id, egid})
}

// thingLocation is the geometry (EWKT, LV95) and metadata of a legacy thing.
func (imp *Importer) thingLocation(t *sourceThing, c *StageCounts) (any, map[string]any) {
	metadata := map[string]any{}
	if t.MinE == nil || t.MaxE == nil || t.MinN == nil || t.MaxN == nil {
		return nil, metadata
	}
	if t.TypeID == legacyParcelType {
		c.adjust("parcel extent not imported (shape unknown)")
		return nil, metadata
	}
	e := float64(*t.MinE+*t.MaxE) / 2 / centimetresPerMetre
	n := float64(*t.MinN+*t.MaxN) / 2 / centimetresPerMetre
	if *t.MinE != *t.MaxE || *t.MinN != *t.MaxN {
		metadata["legacy_location"] = "centre of the legacy extent"
		c.adjust("located at the centre of its extent")
	}
	return fmt.Sprintf("SRID=2056;POINT(%.2f %.2f)", e, n), metadata
}

// copyThings loads the thing rows through a staging table: COPY cannot encode
// a geometry, so the location travels as EWKT text and is parsed by PostGIS.
func (imp *Importer) copyThings(ctx context.Context, rows [][]any) error {
	if _, err := imp.tx.Exec(ctx, createThingStagingSQL); err != nil {
		return fmt.Errorf("create thing staging: %w", err)
	}
	if err := imp.copyRows(ctx, "import_thing", thingColumns, rows); err != nil {
		return err
	}
	if _, err := imp.tx.Exec(ctx, insertThingsFromStagingSQL); err != nil {
		return fmt.Errorf("insert things: %w", err)
	}
	return nil
}
