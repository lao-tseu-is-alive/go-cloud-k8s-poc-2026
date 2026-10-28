import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')
const types = read('../types.ts')
const contract = read('../typesContract.ts')

describe('SPA types contract (GLD-072)', () => {
  it('checks every hand-written type or says why not', () => {
    const exported = [...types.matchAll(/^export (?:interface|type) (\w+)/gm)].map(m => m[1]!)
    const unchecked = (name: string) => new RegExp(`^  ${name}: '`, 'm').test(contract)
    const missing = exported.filter(name => !contract.includes(`T.${name},`) && !contract.includes(`T.${name}>`) && !unchecked(name))
    expect(missing).toEqual([])
  })

  it('generates with the protobuf-es version the SPA type-checks against', () => {
    const plugin = /buf\.build\/bufbuild\/es:v([\d.]+)/.exec(read('../../../../../../buf.gen.yaml'))?.[1]
    const runtime = JSON.parse(read('../../../package.json')).devDependencies['@bufbuild/protobuf']
    expect(plugin).toBeDefined()
    expect(runtime).toBe(plugin)
  })
})
