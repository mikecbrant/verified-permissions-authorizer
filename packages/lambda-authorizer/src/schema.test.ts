import { promises as fs } from 'node:fs'

import { afterEach, describe, expect, it, vi } from 'vitest'

const fixture = new URL('./testdata/schema.merged.json', import.meta.url)
  .pathname
const badFixture = new URL('./testdata/invalid.txt', import.meta.url).pathname

afterEach(() => {
  delete process.env.VP_MERGED_SCHEMA_PATH
})

describe('schema loader', () => {
  it('loads the merged schema once at cold start', async () => {
    process.env.VP_MERGED_SCHEMA_PATH = fixture
    vi.resetModules()
    const m = await import('./schema.js')
    expect(Object.keys(m.extendedSchema ?? {})).toEqual(['ns'])
  })

  it('throws on missing file', async () => {
    process.env.VP_MERGED_SCHEMA_PATH = '/no/such/file.json'
    vi.resetModules()
    await expect(import('./schema.js')).rejects.toThrow(
      /failed to load merged schema/,
    )
  })

  it('throws on invalid JSON', async () => {
    process.env.VP_MERGED_SCHEMA_PATH = badFixture
    vi.resetModules()
    await expect(import('./schema.js')).rejects.toThrow(
      /failed to load merged schema/,
    )
  })

  it('uses default path when VP_MERGED_SCHEMA_PATH is not set', async () => {
    // Write a temp default-path schema next to cwd and import
    const payload = { ns: { entityTypes: {}, actions: {} } }
    await fs.writeFile('schema.merged.json', JSON.stringify(payload), 'utf8')
    delete process.env.VP_MERGED_SCHEMA_PATH
    vi.resetModules()
    const mod = await import('./schema.js')
    expect(Object.keys(mod.extendedSchema ?? {})).toEqual(['ns'])
    await fs.rm('schema.merged.json')
  })

  it('throws when multiple namespaces are present', async () => {
    process.env.VP_MERGED_SCHEMA_PATH = new URL(
      './testdata/two-ns.json',
      import.meta.url,
    ).pathname
    vi.resetModules()
    await expect(import('./schema.js')).rejects.toThrow(
      /failed to load merged schema/,
    )
  })
})
