# Demo geometries (GLD-079)

`things.geojson` places the things of the demo stories (docs/DEMO_SCRIPT.md §4) on the map: one
feature per thing, found by its `key`. The generator reads it, converts it to the Swiss LV95
coordinates the POC stores (EPSG:2056) and checks that every thing of a story is there, with the
geometry type its thing type needs, inside the extent of the map tiles.

**Edit the positions, keep the properties.** Each feature says what to look for (`wanted`): move
or redraw it where such a place is plausible.

- **geojson.io** (simplest): open the file, move the points, redraw the polygons and the line,
  save. Coordinates are WGS84 longitude/latitude, as RFC 7946 GeoJSON requires.
- **QGIS** (most precise, on a swisstopo or cadastral background): open the file, edit, then
  export it again as GeoJSON in **EPSG:4326** (WGS84).

The generator converts WGS84 to LV95 with swisstopo's approximate formulas (about one metre),
enough for a demo. The municipality is fictional: a parcel may follow a real outline, but its
number, EGRID and owner stay invented; avoid recognizable landmarks and public buildings.

| Property | Meaning |
| --- | --- |
| `key` | Name the stories use; do not change it. |
| `thingType` | Thing type of the POC: a `PARCEL` is a polygon, a `BUILDING` a polygon or a point. |
| `hero` | Story of the demo script using it (`background` for the background data). |
| `wanted` | What kind of place to look for. |
