/**
 * Maps of the City of Lausanne (GLD-069), adapted from the City's carte_mn95
 * application (ol-lausanne-map-mn95.js, OlMapLausanne.js): the Swiss LV95
 * projection (EPSG:2056, proj4), the City's WMTS base layers read from their
 * capabilities (fetched once per page load) and the extent the tiles cover.
 * The page itself (components/thing/LausanneMap.vue) adds the object on top.
 */
import OlFormatWMTSCapabilities from 'ol/format/WMTSCapabilities'
import OlLayerTile from 'ol/layer/Tile'
import { register } from 'ol/proj/proj4'
import OlProjection from 'ol/proj/Projection'
import OlSourceWMTS, { optionsFromCapabilities } from 'ol/source/WMTS'
import proj4 from 'proj4'

/** The City's WMTS capabilities (LV95 tiles); its origin is allowed by the server's CSP. */
export const LAUSANNE_WMTS_CAPABILITIES = 'https://tilesmn95.lausanne.ch/tiles/1.0.0/LausanneWMTS.xml'

/** The extent the City's tiles cover, in LV95 metres. */
export const LAUSANNE_EXTENT: [number, number, number, number] = [2_532_500, 1_149_000, 2_545_625, 1_161_000]

/** The City's map application, opened on a box (the legacy iframe page). */
export const LAUSANNE_MAP_APP = 'https://golux.lausanne.ch/carte_mn95/'

/** A base layer offered on the map. */
export interface BaseLayerChoice {
  /** The WMTS layer identifier in the City's capabilities. */
  id: string
  /** The i18n key of its name (things.map.layers.*). */
  labelKey: string
}

/** The base layers, the first one shown by default. */
export const BASE_LAYERS: BaseLayerChoice[] = [
  { id: 'fonds_geo_osm_bdcad_couleur', labelKey: 'planVille' },
  { id: 'fonds_geo_osm_bdcad_gris', labelKey: 'cadastral' },
  { id: 'orthophotos_ortho_lidar_2024', labelKey: 'ortho2024' },
  { id: 'orthophotos_ortho_lidar_2016', labelKey: 'ortho2016' },
]

proj4.defs('EPSG:2056', '+proj=somerc +lat_0=46.95240555555556 +lon_0=7.439583333333333 +k_0=1 +x_0=2600000 +y_0=1200000 +ellps=bessel +towgs84=674.374,15.056,405.346,0,0,0,0 +units=m +no_defs')
register(proj4)

/** The LV95 projection of the City's maps. */
export const swissProjection = new OlProjection({ code: 'EPSG:2056', extent: LAUSANNE_EXTENT, units: 'm' })

let capabilities: Promise<unknown> | undefined

/** The parsed capabilities of the City's WMTS, fetched once (a failure is retried next time). */
function lausanneCapabilities (): Promise<unknown> {
  capabilities ??= fetch(LAUSANNE_WMTS_CAPABILITIES)
    .then(async res => {
      if (!res.ok) {
        throw new Error(`WMTS capabilities: HTTP ${res.status}`)
      }
      return new OlFormatWMTSCapabilities().read(await res.text())
    })
    .catch(error => {
      capabilities = undefined
      throw error
    })
  return capabilities
}

/** The City's base layers as tile layers, the default one visible; each knows its id. */
export async function lausanneBaseLayers (): Promise<OlLayerTile<OlSourceWMTS>[]> {
  const parsed = await lausanneCapabilities()
  const layers: OlLayerTile<OlSourceWMTS>[] = []
  for (const [i, choice] of BASE_LAYERS.entries()) {
    const options = optionsFromCapabilities(parsed, {
      layer: choice.id, matrixSet: 'EPSG2056', format: 'image/png', style: 'default', crossOrigin: 'anonymous',
    })
    if (!options) {
      continue
    } // a layer the City no longer publishes
    const layer = new OlLayerTile({ source: new OlSourceWMTS(options), visible: i === 0 })
    layer.set('baseLayerId', choice.id)
    layers.push(layer)
  }
  return layers
}

/** Whether a box (LV95) lies inside the City's tiles. */
export function insideLausanne (extent: number[]): boolean {
  const [minX, minY, maxX, maxY] = LAUSANNE_EXTENT
  return extent[0]! >= minX && extent[1]! >= minY && extent[2]! <= maxX && extent[3]! <= maxY
}

/** The City's map application centred on a box, a margin around it. */
export function lausanneMapAppUrl (extent: number[], baseLayer = BASE_LAYERS[0]!.id): string {
  const margin = 20
  const params = new URLSearchParams({
    baselayer: baseLayer,
    lvisibles: '41',
    infodisplay: 'true',
    bb: 'true',
    minx: String(Math.round(extent[0]! - margin)),
    miny: String(Math.round(extent[1]! - margin)),
    maxx: String(Math.round(extent[2]! + margin)),
    maxy: String(Math.round(extent[3]! + margin)),
  })
  return `${LAUSANNE_MAP_APP}?${params.toString()}`
}
