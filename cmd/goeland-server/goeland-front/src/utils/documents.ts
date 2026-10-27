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

/** Whether the content is stored by Goéland, so the SPA can download it. */
export function downloadable (storageRef?: string): boolean {
  return !!storageRef?.startsWith('internal://')
}
