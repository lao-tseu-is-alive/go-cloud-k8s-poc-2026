package demodata

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

// thingsGeoJSON places the things of the stories (geodata/README.md): WGS84
// GeoJSON, edited with geojson.io or QGIS.
//
//go:embed geodata/things.geojson
var thingsGeoJSON []byte

// assets holds the demo photographs (assets/README.md), AI-generated.
//
//go:embed assets/*.jpg
var assets embed.FS

// mapExtent is the extent of the City of Lausanne's map tiles, in LV95 metres
// (as utils/lausanneMap.ts): every demo thing lies inside it.
var mapExtent = [4]float64{2_532_500, 1_149_000, 2_545_625, 1_161_000}

// geometryTypes lists the geometry types a thing type accepts (pkg/thing:
// a polygonal parcel, a point or polygonal building); other types take any.
var geometryTypes = map[string][]string{
	"PARCEL":   {"Polygon", "MultiPolygon"},
	"BUILDING": {"Point", "Polygon", "MultiPolygon"},
}

// place is a thing of the stories: its thing type and its LV95 GeoJSON.
type place struct {
	thingType string
	geometry  string
}

// geoFeature is a feature of things.geojson.
type geoFeature struct {
	// Properties name the thing (key) and its thing type.
	Properties struct {
		Key       string `json:"key"`
		ThingType string `json:"thingType"`
	} `json:"properties"`
	// Geometry is the WGS84 GeoJSON geometry.
	Geometry struct {
		Type        string `json:"type"`
		Coordinates any    `json:"coordinates"`
	} `json:"geometry"`
}

// loadPlaces reads things.geojson, converts every geometry to LV95 and checks
// its type and extent.
func loadPlaces() (map[string]place, error) {
	var collection struct {
		Features []geoFeature `json:"features"`
	}
	if err := json.Unmarshal(thingsGeoJSON, &collection); err != nil {
		return nil, fmt.Errorf("geodata/things.geojson: %w", err)
	}
	places := map[string]place{}
	for _, f := range collection.Features {
		key := f.Properties.Key
		if key == "" || places[key].geometry != "" {
			return nil, fmt.Errorf("geodata/things.geojson: missing or repeated key %q", key)
		}
		if allowed, ok := geometryTypes[f.Properties.ThingType]; ok && !slices.Contains(allowed, f.Geometry.Type) {
			return nil, fmt.Errorf("geodata/things.geojson: %s is a %s, a %s needs %v", key, f.Geometry.Type, f.Properties.ThingType, allowed)
		}
		coordinates, err := toLV95(f.Geometry.Coordinates)
		if err != nil {
			return nil, fmt.Errorf("geodata/things.geojson: %s: %w", key, err)
		}
		geometry, err := json.Marshal(map[string]any{"type": f.Geometry.Type, "coordinates": coordinates})
		if err != nil {
			return nil, err
		}
		places[key] = place{thingType: f.Properties.ThingType, geometry: string(geometry)}
	}
	return places, nil
}

// toLV95 converts nested GeoJSON coordinates from WGS84 to LV95, checking
// that every position lies inside the map extent.
func toLV95(coordinates any) (any, error) {
	list, ok := coordinates.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("invalid coordinates")
	}
	if lon, isNumber := list[0].(float64); isNumber {
		if len(list) < 2 {
			return nil, fmt.Errorf("invalid position %v", list)
		}
		lat, ok := list[1].(float64)
		if !ok {
			return nil, fmt.Errorf("invalid position %v", list)
		}
		east, north := wgs84ToLV95(lon, lat)
		if east < mapExtent[0] || north < mapExtent[1] || east > mapExtent[2] || north > mapExtent[3] {
			return nil, fmt.Errorf("position %.6f,%.6f lies outside the map tiles", lon, lat)
		}
		return []float64{east, north}, nil
	}
	converted := make([]any, len(list))
	for i, item := range list {
		c, err := toLV95(item)
		if err != nil {
			return nil, err
		}
		converted[i] = c
	}
	return converted, nil
}

// wgs84ToLV95 converts WGS84 degrees to LV95 metres with swisstopo's
// approximate formulas (about one metre), rounded to 10 cm.
func wgs84ToLV95(lon, lat float64) (east, north float64) {
	phi := (lat*3600 - 169028.66) / 10000
	lambda := (lon*3600 - 26782.5) / 10000
	east = 2600072.37 + 211455.93*lambda - 10938.51*lambda*phi - 0.36*lambda*phi*phi - 44.54*lambda*lambda*lambda
	north = 1200147.07 + 308807.95*phi + 3745.25*lambda*lambda + 76.63*phi*phi - 194.56*lambda*lambda*phi + 119.79*phi*phi*phi
	return math.Round(east*10) / 10, math.Round(north*10) / 10
}

// placeOf returns the LV95 GeoJSON of the thing key, checking its thing type.
func (r *run) placeOf(key, thingType string) string {
	p, ok := r.places[key]
	if r.err == nil && (!ok || p.thingType != thingType) {
		r.err = fmt.Errorf("geodata/things.geojson has no %s %q", thingType, key)
	}
	return p.geometry
}

// asset returns the bytes of a demo photograph.
func (r *run) asset(name string) []byte {
	if r.err != nil {
		return nil
	}
	content, err := assets.ReadFile("assets/" + name)
	if err != nil {
		r.err = fmt.Errorf("demo photograph %s: %w", name, err)
	}
	return content
}
