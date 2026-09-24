package thing

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// swissExtent is a generous LV95 envelope of Switzerland: a geometry outside it
// is refused, which also catches WGS84 degrees sent instead of LV95 metres.
var swissExtent = BBox{EMin: 2_480_000, NMin: 1_070_000, EMax: 2_840_000, NMax: 1_300_000}

// allowedGeometryTypes lists the GeoJSON types accepted per specialization;
// a generic type accepts any.
var allowedGeometryTypes = map[Specialization][]string{
	SpecializationParcel:   {"Polygon", "MultiPolygon"},
	SpecializationBuilding: {"Point", "Polygon", "MultiPolygon"},
}

// geometryCheck is what PostGIS reports about a candidate geometry.
type geometryCheck struct {
	// Valid is ST_IsValid of the geometry.
	Valid bool `db:"valid"`
	// Reason is ST_IsValidReason when the geometry is invalid.
	Reason string `db:"reason"`
	// XMin is the west bound of the envelope (LV95 east).
	XMin float64 `db:"xmin"`
	// YMin is the south bound of the envelope (LV95 north).
	YMin float64 `db:"ymin"`
	// XMax is the east bound of the envelope.
	XMax float64 `db:"xmax"`
	// YMax is the north bound of the envelope.
	YMax float64 `db:"ymax"`
}

// checkGeometryType rejects a GeoJSON geometry whose type does not fit the
// specialization (before any database round trip).
func checkGeometryType(geojson string, spec Specialization) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(geojson), &head); err != nil || head.Type == "" {
		return fmt.Errorf("%w: geometry_geojson must be a GeoJSON geometry object", core.ErrInvalidInput)
	}
	if allowed, ok := allowedGeometryTypes[spec]; ok && !slices.Contains(allowed, head.Type) {
		return fmt.Errorf("%w: a %s geometry is not allowed for this type (allowed: %s)",
			core.ErrInvalidInput, head.Type, strings.Join(allowed, ", "))
	}
	return nil
}

// checkGeometry validates a GeoJSON geometry for a thing type: parseable,
// of an allowed type, valid (no self-intersection) and inside Switzerland in
// LV95 coordinates. An empty string (no geometry) is accepted.
func checkGeometry(ctx context.Context, q core.Querier, geojson string, spec Specialization) error {
	if geojson == "" {
		return nil
	}
	if err := checkGeometryType(geojson, spec); err != nil {
		return err
	}
	rows, err := q.Query(ctx, checkGeometrySQL, pgx.NamedArgs{"geojson": geojson})
	if err != nil {
		return fmt.Errorf("%w: geometry_geojson is not a valid GeoJSON geometry", core.ErrInvalidInput)
	}
	check, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[geometryCheck])
	if err != nil {
		return fmt.Errorf("%w: geometry_geojson is not a valid GeoJSON geometry", core.ErrInvalidInput)
	}
	switch {
	case !check.Valid:
		return fmt.Errorf("%w: invalid geometry: %s", core.ErrInvalidInput, check.Reason)
	case !swissExtent.contains(check):
		return fmt.Errorf("%w: the geometry lies outside Switzerland; coordinates must be LV95 metres (EPSG:2056), e.g. [2538000, 1152000]", core.ErrInvalidInput)
	}
	return nil
}

// contains reports whether the checked envelope lies inside b.
func (b BBox) contains(c geometryCheck) bool {
	return c.XMin >= b.EMin && c.XMax <= b.EMax && c.YMin >= b.NMin && c.YMax <= b.NMax
}

// ParseBBox parses "e_min,n_min,e_max,n_max" (LV95); empty yields nil.
func ParseBBox(raw string) (*BBox, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("%w: bbox must be e_min,n_min,e_max,n_max", core.ErrInvalidInput)
	}
	var v [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("%w: bbox must hold four numbers", core.ErrInvalidInput)
		}
		v[i] = f
	}
	if v[0] >= v[2] || v[1] >= v[3] {
		return nil, fmt.Errorf("%w: bbox minimums must be below maximums", core.ErrInvalidInput)
	}
	return &BBox{EMin: v[0], NMin: v[1], EMax: v[2], NMax: v[3]}, nil
}
