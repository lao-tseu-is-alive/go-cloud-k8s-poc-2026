package thing

// SQL fragments for the thing repository. Column projections are the single
// source of truth for pgx named scanning; they are alias-prefixed, so INSERT
// statements alias their target (AS t). The geometry is read as GeoJSON with
// its computed area and a point on its surface (for map links).

const thingColumns = `
t.id, t.thing_type_id, t.name, t.description, t.external_ref,
ST_AsGeoJSON(t.geom, 3) AS geometry_geojson,
CASE WHEN ST_Dimension(t.geom) = 2 THEN ST_Area(t.geom) END AS area_m2,
ST_X(ST_PointOnSurface(t.geom)) AS anchor_e,
ST_Y(ST_PointOnSurface(t.geom)) AS anchor_n,
t.metadata, t.created_at, t.created_by, t.updated_at`

// geomFromGeoJSON turns the @geojson parameter into an EPSG:2056 geometry;
// an empty string means no geometry.
const geomFromGeoJSON = `CASE WHEN @geojson::text = '' THEN NULL ELSE ST_SetSRID(ST_GeomFromGeoJSON(@geojson::text), 2056) END`

// checkGeometrySQL reports validity and envelope of a candidate geometry.
const checkGeometrySQL = `
SELECT ST_IsValid(g) AS valid, coalesce(ST_IsValidReason(g), '') AS reason,
       ST_XMin(g) AS xmin, ST_YMin(g) AS ymin, ST_XMax(g) AS xmax, ST_YMax(g) AS ymax
FROM (SELECT ST_SetSRID(ST_GeomFromGeoJSON(@geojson::text), 2056) AS g) s;`

const insertThingSQL = `
INSERT INTO thing AS t (id, thing_type_id, name, description, external_ref, geom, metadata, created_by)
VALUES (@id, @thing_type_id, @name, @description, @external_ref, ` + geomFromGeoJSON + `, @metadata, @created_by)
RETURNING ` + thingColumns + `;`

const getThingSQL = `
SELECT ` + thingColumns + `
FROM thing t
WHERE t.id = @id;`

const getThingForUpdateSQL = `
SELECT ` + thingColumns + `
FROM thing t
WHERE t.id = @id
FOR UPDATE;`

const updateThingSQL = `
UPDATE thing t
SET name = @name, description = @description, external_ref = @external_ref,
    geom = ` + geomFromGeoJSON + `, metadata = @metadata
WHERE t.id = @id
RETURNING ` + thingColumns + `;`

// --- specializations ---------------------------------------------------------------

const upsertParcelSQL = `
INSERT INTO thing_parcel (thing_id, commune_ofs, parcel_number, egrid, surface_m2)
VALUES (@thing_id, @commune_ofs, @parcel_number, @egrid, @surface_m2)
ON CONFLICT (thing_id) DO UPDATE
SET commune_ofs = EXCLUDED.commune_ofs, parcel_number = EXCLUDED.parcel_number,
    egrid = EXCLUDED.egrid, surface_m2 = EXCLUDED.surface_m2;`

const getParcelSQL = `
SELECT commune_ofs, parcel_number, egrid, surface_m2::float8 AS surface_m2
FROM thing_parcel
WHERE thing_id = @thing_id;`

const upsertBuildingSQL = `
INSERT INTO thing_building (thing_id, egid, eca_number, construction_year, building_status)
VALUES (@thing_id, @egid, @eca_number, @construction_year, @building_status)
ON CONFLICT (thing_id) DO UPDATE
SET egid = EXCLUDED.egid, eca_number = EXCLUDED.eca_number,
    construction_year = EXCLUDED.construction_year, building_status = EXCLUDED.building_status;`

const getBuildingSQL = `
SELECT egid, eca_number, construction_year, building_status
FROM thing_building
WHERE thing_id = @thing_id;`

// --- thing_type ----------------------------------------------------------------------

const thingTypeColumns = `id, code, label, description, specialization, is_active`

const getThingTypeByCodeSQL = `
SELECT ` + thingTypeColumns + `
FROM thing_type
WHERE code = @code;`

const getThingTypeByIDSQL = `
SELECT ` + thingTypeColumns + `
FROM thing_type
WHERE id = @id;`

const listThingTypesSQL = `
SELECT ` + thingTypeColumns + `
FROM thing_type
WHERE (NOT @only_active OR is_active)
ORDER BY code;`

const insertThingTypeSQL = `
INSERT INTO thing_type (code, label, description)
VALUES (@code, @label, @description)
RETURNING ` + thingTypeColumns + `;`

const getThingTypeForUpdateSQL = `
SELECT ` + thingTypeColumns + `
FROM thing_type
WHERE code = @code
FOR UPDATE;`

// updateThingTypeSQL replaces the fields given; code and specialization are immutable.
const updateThingTypeSQL = `
UPDATE thing_type
SET label = coalesce(@label::text, label),
    description = coalesce(@description::text, description),
    is_active = coalesce(@is_active::boolean, is_active)
WHERE code = @code
RETURNING ` + thingTypeColumns + `;`

// --- search -----------------------------------------------------------------------

// searchThingsSQL matches the accent-folded search_vector or an exact parcel
// number, EGRID or EGID, plus type, LV95 extent (index-backed &&, then exact
// ST_Intersects) and deletion filters.
const searchThingsSQL = `
SELECT ` + thingColumns + `,
COUNT(*) OVER() AS total_count
FROM thing t
JOIN record_metadata rm ON rm.subject_id = t.id
LEFT JOIN thing_parcel p ON p.thing_id = t.id
LEFT JOIN thing_building b ON b.thing_id = t.id
WHERE (@query = ''
       OR t.search_vector @@ plainto_tsquery('simple', immutable_unaccent(@query))
       OR p.parcel_number = @query OR p.egrid = upper(@query) OR b.egid::text = @query)
  AND (@thing_type_code = '' OR t.thing_type_id = (SELECT id FROM thing_type WHERE code = @thing_type_code))
  AND (NOT @has_bbox OR (t.geom && ST_MakeEnvelope(@e_min::float8, @n_min::float8, @e_max::float8, @n_max::float8, 2056)
       AND ST_Intersects(t.geom, ST_MakeEnvelope(@e_min::float8, @n_min::float8, @e_max::float8, @n_max::float8, 2056))))
  AND (@include_deleted OR rm.deleted_at IS NULL)
ORDER BY t.created_at DESC
LIMIT @limit OFFSET @offset;`
