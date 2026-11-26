/** Presentation helpers of the document page (GLD-075). */

// The icon of a file family, by MIME type prefix or exact type, first match wins.
const MIME_ICONS: ReadonlyArray<[test: (mime: string) => boolean, icon: string]> = [
  [m => m === 'application/pdf', 'mdi-file-pdf-box'],
  [m => m.startsWith('image/'), 'mdi-file-image-outline'],
  [m => m.includes('wordprocessing') || m === 'application/msword' || m.includes('opendocument.text'), 'mdi-file-word-outline'],
  [m => m.includes('spreadsheet') || m === 'application/vnd.ms-excel' || m === 'text/csv', 'mdi-file-excel-outline'],
  [m => m.includes('presentation') || m === 'application/vnd.ms-powerpoint', 'mdi-file-powerpoint-outline'],
  [m => m.startsWith('text/'), 'mdi-file-document-outline'],
  [m => m === 'application/zip' || m.includes('compressed'), 'mdi-folder-zip-outline'],
]

/** The icon of a document from the MIME type of its current content. */
export function documentIcon (mimeType?: string): string {
  const mime = (mimeType ?? '').toLowerCase()
  if (!mime) {
    return 'mdi-file-outline'
  }
  return MIME_ICONS.find(([test]) => test(mime))?.[1] ?? 'mdi-file-outline'
}

/**
 * Reference schemes of the blob stores Goéland writes to: the local files and
 * the S3 bucket (GLD-020, pkg/blobstore). Keep in step with the server stores.
 */
const HELD_SCHEMES = ['internal://', 's3://']

/** Whether the content is stored by Goéland, so the SPA can download and verify it. */
export function downloadable (storageRef?: string): boolean {
  return !!storageRef && HELD_SCHEMES.some(scheme => storageRef.startsWith(scheme))
}

/** How the document page shows a content inline (GLD-087). */
export type PreviewKind = 'image' | 'pdf' | 'text'

// Images an <img> shows (an SVG in an <img> runs no script).
const PREVIEW_IMAGES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif', 'image/bmp', 'image/svg+xml'])
// Structured texts shown as plain text, besides every text/* type.
const PREVIEW_TEXTS = new Set(['application/json', 'application/xml', 'application/x-yaml', 'application/yaml'])

/** The way a content of this MIME type is previewed, or null when it is only downloaded. */
export function previewKind (mimeType?: string): PreviewKind | null {
  const mime = (mimeType ?? '').toLowerCase().split(';', 1)[0]?.trim() ?? ''
  if (PREVIEW_IMAGES.has(mime)) {
    return 'image'
  }
  if (mime === 'application/pdf') {
    return 'pdf'
  }
  if (mime.startsWith('text/') || PREVIEW_TEXTS.has(mime)) {
    return 'text'
  }
  return null
}

/** Largest content (bytes) previewed without asking, by kind: beyond, a button loads it. */
export const PREVIEW_AUTO_LIMITS: Readonly<Record<PreviewKind, number>> = {
  image: 10 * 1024 * 1024,
  pdf: 20 * 1024 * 1024,
  text: 512 * 1024,
}

/** Characters of a text content shown inline; the rest is left to the download. */
export const PREVIEW_TEXT_CHARS = 200_000

/**
 * Whether the preview loads by itself: never for a confidential document, whose
 * every content read is recorded (DOWNLOAD, GLD-033), so the viewer asks for it;
 * otherwise up to the size limit of its kind (the size may be unknown: int64 as string).
 */
export function previewLoadsByItself (kind: PreviewKind, fileSizeBytes?: string, confidential = false): boolean {
  if (confidential) {
    return false
  }
  const size = Number(fileSizeBytes ?? '')
  return Number.isFinite(size) && size > 0 && size <= PREVIEW_AUTO_LIMITS[kind]
}
