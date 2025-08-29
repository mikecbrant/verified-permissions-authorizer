import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import YAML from 'yaml'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)
const repoRoot = join(__dirname, '..')

const yamlPath = join(repoRoot, 'packages/provider/schema.yaml')
const jsonPath = join(repoRoot, 'packages/provider/schema.json')

const text = readFileSync(yamlPath, 'utf8')
const obj = YAML.parse(text)
writeFileSync(jsonPath, JSON.stringify(obj, null, 2))
console.log(`Generated ${jsonPath} from ${yamlPath}`)
