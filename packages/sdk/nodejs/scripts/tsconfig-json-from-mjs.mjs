import { writeFileSync } from 'node:fs'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { dirname, join } from 'node:path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)
const root = join(__dirname, '..')

const cfgUrl = pathToFileURL(join(root, 'tsconfig.mjs')).href
const mod = await import(cfgUrl)
const config = mod.default ?? mod

writeFileSync(join(root, 'tsconfig.generated.json'), JSON.stringify(config, null, 2))
console.log('wrote tsconfig.generated.json')
