import { describe, expect, it } from 'vitest'
import { documentIcon, downloadable } from '../documents'

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
    expect(downloadable('alfresco://x')).toBe(false)
    expect(downloadable()).toBe(false)
  })
})
