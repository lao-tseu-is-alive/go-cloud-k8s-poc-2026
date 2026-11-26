import { describe, expect, it } from 'vitest'
import { documentIcon, downloadable, PREVIEW_AUTO_LIMITS, previewKind, previewLoadsByItself } from '../documents'

describe('documents', () => {
  it('picks an icon by file family', () => {
    expect(documentIcon('application/pdf')).toBe('mdi-file-pdf-box')
    expect(documentIcon('IMAGE/PNG')).toBe('mdi-file-image-outline')
    expect(documentIcon('application/vnd.openxmlformats-officedocument.wordprocessingml.document')).toBe('mdi-file-word-outline')
    expect(documentIcon('application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')).toBe('mdi-file-excel-outline')
    expect(documentIcon('text/plain')).toBe('mdi-file-document-outline')
    expect(documentIcon('application/octet-stream')).toBe('mdi-file-outline')
    expect(documentIcon()).toBe('mdi-file-outline')
  })

  it('downloads only the contents Goéland stores', () => {
    expect(downloadable('internal://ab/cd')).toBe(true)
    expect(downloadable('s3://goeland/documents/ab.pdf')).toBe(true)
    expect(downloadable('')).toBe(false)
    expect(downloadable('alfresco://x')).toBe(false)
    expect(downloadable()).toBe(false)
  })

  it('previews images, PDFs and texts only', () => {
    expect(previewKind('image/jpeg')).toBe('image')
    expect(previewKind('IMAGE/SVG+XML')).toBe('image')
    expect(previewKind('application/pdf')).toBe('pdf')
    expect(previewKind('text/plain; charset=utf-8')).toBe('text')
    expect(previewKind('text/html')).toBe('text')
    expect(previewKind('application/json')).toBe('text')
    expect(previewKind('image/tiff')).toBeNull()
    expect(previewKind('application/vnd.openxmlformats-officedocument.wordprocessingml.document')).toBeNull()
    expect(previewKind()).toBeNull()
  })

  it('loads a preview by itself only when small and not audited', () => {
    expect(previewLoadsByItself('image', '120000')).toBe(true)
    expect(previewLoadsByItself('image', String(PREVIEW_AUTO_LIMITS.image + 1))).toBe(false)
    expect(previewLoadsByItself('text', '600000')).toBe(false)
    expect(previewLoadsByItself('pdf', '120000', true)).toBe(false)
    expect(previewLoadsByItself('pdf')).toBe(false)
    expect(previewLoadsByItself('pdf', 'x')).toBe(false)
  })
})
