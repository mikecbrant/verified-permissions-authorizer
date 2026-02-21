import { promises as fs } from 'node:fs'

type Json = Record<string, any>

// Single known path with optional override for tests/local runs.
// Default relies on Lambda's CWD being the function code root where the provider bundles schema.merged.json
const SCHEMA_PATH = process.env.VP_MERGED_SCHEMA_PATH ?? 'schema.merged.json'

let cached: Json | undefined

const load = async (): Promise<Json> => {
  const data = await fs.readFile(SCHEMA_PATH, 'utf8')
  return JSON.parse(data) as Json
}

// Load once during cold start using TLA. Fail loudly if missing/invalid.
const extendedSchema: Json | undefined = await (async (): Promise<
  Json | undefined
> => {
  try {
    const doc = await load()
    // Quick sanity check: exactly one namespace key
    const ns = Object.keys(doc)
    if (ns.length !== 1)
      throw new Error(`expected exactly one namespace, found ${ns.length}`)
    cached = doc
    return cached
  } catch (err) {
    // Fail loudly on cold start if the schema cannot be read or parsed.
    // Tests can point VP_MERGED_SCHEMA_PATH to a fixture.
    const e = err as any
    const reason = e?.code ? ` (${e.code})` : ''
    throw new Error(`failed to load merged schema at ${SCHEMA_PATH}${reason}`)
  }
})()

export { extendedSchema }
