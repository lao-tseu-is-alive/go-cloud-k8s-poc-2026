package thing

import (
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Specialization selects the detail block of a thing type; it mirrors the
// ThingSpecialization proto enum and the thing_type.specialization column.
type Specialization int16

const (
	// SpecializationGeneric has no detail block.
	SpecializationGeneric Specialization = 0
	// SpecializationParcel carries a Parcel; the geometry is polygonal.
	SpecializationParcel Specialization = 1
	// SpecializationBuilding carries a Building; the geometry is a point or polygonal.
	SpecializationBuilding Specialization = 2
)

// BuildingStatus is the RegBL/GWR building status; it mirrors the
// BuildingStatus proto enum and the thing_building.building_status column.
type BuildingStatus int16

// BuildingStatusNotRealized is the highest status value (1008, non réalisé).
const BuildingStatusNotRealized BuildingStatus = 7

// Valid reports whether s is a known status (the zero value included).
func (s BuildingStatus) Valid() bool {
	return s >= 0 && s <= BuildingStatusNotRealized
}

// ThingType is a controlled classification of things (seeded, administrable).
type ThingType struct {
	// ID is the catalogue row identity.
	ID uuid.UUID `db:"id"`
	// Code is the unique, stable key used by APIs.
	Code string `db:"code"`
	// Label is the human label.
	Label string `db:"label"`
	// Description documents the business meaning.
	Description string `db:"description"`
	// Specialization selects the detail block; fixed for a type.
	Specialization Specialization `db:"specialization"`
	// IsActive reports whether new things of this type may be created.
	IsActive bool `db:"is_active"`
}

// Parcel are the land-register identifiers of a parcel.
type Parcel struct {
	// CommuneOFS is the federal (OFS) commune number, 1-9999.
	CommuneOFS int32 `db:"commune_ofs"`
	// ParcelNumber is the parcel number, unique per commune.
	ParcelNumber string `db:"parcel_number"`
	// EGRID is the federal parcel identifier ("CH" + 12); empty when unknown.
	EGRID string `db:"egrid"`
	// SurfaceM2 is the registered surface; nil when unknown.
	SurfaceM2 *float64 `db:"surface_m2"`
}

// Building are the register identifiers of a building.
type Building struct {
	// EGID is the federal building identifier; nil when unknown.
	EGID *int32 `db:"egid"`
	// ECANumber is the cantonal insurance number; empty when unknown.
	ECANumber string `db:"eca_number"`
	// ConstructionYear is 1000-2100; nil when unknown.
	ConstructionYear *int16 `db:"construction_year"`
	// Status is the RegBL status; 0 when unknown.
	Status BuildingStatus `db:"building_status"`
}

// Thing is the thing projection (1:1 with a THING subject_ref). The geometry is
// exchanged as GeoJSON in EPSG:2056; area and anchor point are computed by
// PostGIS. The generated search_vector column is never written or scanned.
type Thing struct {
	// ID is the THING subject_ref ID; a composite foreign key pins the kind.
	ID uuid.UUID `db:"id"`
	// TypeID references the ThingType.
	TypeID uuid.UUID `db:"thing_type_id"`
	// Name is the non-blank name, mirrored into the subject label.
	Name string `db:"name"`
	// Description is free text.
	Description string `db:"description"`
	// ExternalRef is the identifier in a source system; empty when none.
	ExternalRef string `db:"external_ref"`
	// GeometryGeoJSON is the geometry as GeoJSON (EPSG:2056); nil when none.
	GeometryGeoJSON *string `db:"geometry_geojson"`
	// AreaM2 is the area of a polygonal geometry; nil otherwise.
	AreaM2 *float64 `db:"area_m2"`
	// AnchorE is the LV95 east coordinate of a point on the geometry; nil without geometry.
	AnchorE *float64 `db:"anchor_e"`
	// AnchorN is the LV95 north coordinate of that point; nil without geometry.
	AnchorN *float64 `db:"anchor_n"`
	// Metadata holds secondary data.
	Metadata map[string]any `db:"metadata"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who created the thing.
	CreatedBy string `db:"created_by"`
	// UpdatedAt is maintained by a trigger on every update.
	UpdatedAt time.Time `db:"updated_at"`

	// Type is the hydrated thing type.
	Type *ThingType `db:"-"`
	// Parcel is the hydrated parcel block of a parcel; nil otherwise.
	Parcel *Parcel `db:"-"`
	// Building is the hydrated building block of a building; nil otherwise.
	Building *Building `db:"-"`
	// Subject is the hydrated canonical identity.
	Subject *core.SubjectRef `db:"-"`
	// RecordMetadata is the hydrated governance record.
	RecordMetadata *core.RecordMetadata `db:"-"`
}

// CreateInput is a new thing.
type CreateInput struct {
	// TypeCode is the required code of an active thing type.
	TypeCode string
	// Name is the name; derived for a parcel or a building with an EGID when blank.
	Name string
	// Description is optional free text.
	Description string
	// ExternalRef is an optional source-system identifier.
	ExternalRef string
	// GeometryGeoJSON is an optional GeoJSON geometry in EPSG:2056 (see checkGeometry).
	GeometryGeoJSON string
	// Parcel is the detail block of a parcel type (required there).
	Parcel *Parcel
	// Building is the optional detail block of a building type.
	Building *Building
	// Metadata holds secondary data.
	Metadata map[string]any
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Governance is the initial governance, completed by the service.
	Governance core.CreateSubjectInput
}

// UpdateInput replaces the editable fields of a thing (the type is fixed).
type UpdateInput struct {
	// Name is the new, non-blank name.
	Name string
	// Description replaces the description.
	Description string
	// ExternalRef replaces the source-system identifier.
	ExternalRef string
	// GeometryGeoJSON replaces the geometry; empty removes it.
	GeometryGeoJSON string
	// Parcel replaces the parcel block when non-nil.
	Parcel *Parcel
	// Building replaces the building block when non-nil.
	Building *Building
	// Metadata replaces the secondary data.
	Metadata map[string]any
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// BBox is an LV95 (EPSG:2056) extent.
type BBox struct {
	// EMin is the west bound.
	EMin float64
	// NMin is the south bound.
	NMin float64
	// EMax is the east bound.
	EMax float64
	// NMax is the north bound.
	NMax float64
}

// SearchFilter selects things.
type SearchFilter struct {
	// Query matches name, description and external reference accent-insensitively,
	// or exactly a parcel number, EGRID or EGID; empty matches every thing.
	Query string
	// TypeCode restricts results to one type; empty means any.
	TypeCode string
	// BBox restricts results to geometries intersecting the extent; nil means anywhere.
	BBox *BBox
	// IncludeDeleted also returns soft-deleted things.
	IncludeDeleted bool
	// Limit is the page size, normalized by the service.
	Limit int
	// Offset is the zero-based number of rows to skip.
	Offset int
}

// SearchResult is one page of things.
type SearchResult struct {
	// Things is the requested page, newest first.
	Things []*Thing
	// TotalSize is the number of matches across all pages.
	TotalSize int32
}
