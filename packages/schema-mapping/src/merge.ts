import yaml from 'yaml'

type CedarSchema = Record<string, any>

const singleNs = (schema: CedarSchema): { ns: string; body: any } => {
  const keys = Object.keys(schema)
  if (keys.length !== 1) throw new Error(`schema must contain exactly one namespace, found ${keys.length}`)
  const ns = keys[0]
  return { ns, body: (schema as any)[ns] }
}

const deepClone = <T>(v: T): T => JSON.parse(JSON.stringify(v))

const principals = new Set(['Tenant', 'User', 'Role', 'GlobalRole', 'TenantGrant'])

export const mergeCedarSchemas = (baseYaml: string, partialYaml: string): MergeResult => {
  const base = yaml.parse(baseYaml) as CedarSchema
  const partial = yaml.parse(partialYaml) as CedarSchema
  const { ns: bns, body: bbody } = singleNs(base)
  const { ns: pns, body: pbody } = singleNs(partial)
  if (bns !== pns) throw new Error(`namespace mismatch: base=${bns} partial=${pns}`)

  const out: CedarSchema = { [bns]: deepClone(bbody) }
  const obody = out[bns]

  if (pbody?.entityTypes) {
    obody.entityTypes ??= {}
    for (const [name, def] of Object.entries<any>(pbody.entityTypes)) {
      if (obody.entityTypes[name]) throw new Error(`cannot override base entityType ${name}`)
      if (principals.has(name)) throw new Error(`cannot add or modify principal type ${name}`)
      obody.entityTypes[name] = deepClone(def)
    }
  }
  if (pbody?.actions) {
    obody.actions ??= {}
    for (const [name, def] of Object.entries<any>(pbody.actions)) {
      if (obody.actions[name]) throw new Error(`cannot override base action ${name}`)
      obody.actions[name] = deepClone(def)
    }
  }
  const mappings: MappingConfig | undefined = pbody?.mappings ? deepClone(pbody.mappings) : undefined

  const cedarJson = JSON.stringify(out)
  return { cedarJson, namespace: bns, mappings }
}

export type MappingConfig = {
  properties?: Record<string, {
    appsync?: { path: string }
    apiGateway?: { source: 'path' | 'query' | 'body'; name?: string; path?: string }
  }>
  actions?: {
    appsync?: { path: string }
    apiGateway?: { path: string }
  }
}

export type MergeResult = {
  cedarJson: string
  namespace: string
  mappings?: MappingConfig
}
