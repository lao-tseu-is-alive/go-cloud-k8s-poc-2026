import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { ICON_PATHS } from '../icons'

const SRC = fileURLToPath(new URL('../..', import.meta.url))
const ICON_NAME = /['"`](mdi-[a-z0-9-]+)['"`]/g

// Every `mdi-<name>` literal of the sources (tests and the registry excluded).
function usedIconNames (): Set<string> {
  const names = new Set<string>()
  for (const entry of readdirSync(SRC, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !/\.(?:vue|ts)$/.test(entry.name)) {
      continue
    }
    const path = join(entry.parentPath, entry.name)
    if (path.includes('__tests__') || path.endsWith(join('plugins', 'icons.ts'))) {
      continue
    }
    for (const match of readFileSync(path, 'utf8').matchAll(ICON_NAME)) {
      names.add(match[1]!)
    }
  }
  return names
}

describe('icon registry', () => {
  const used = usedIconNames()
  const registered = Object.keys(ICON_PATHS)

  it('registers every icon the sources name', () => {
    expect([...used].filter(name => !(name in ICON_PATHS))).toEqual([])
  })

  it('bundles no icon the sources do not name', () => {
    expect(registered.filter(name => !used.has(name))).toEqual([])
  })

  it('maps each name to an SVG path', () => {
    for (const name of registered) {
      expect(ICON_PATHS[name]).toMatch(/^M/)
    }
  })
})
