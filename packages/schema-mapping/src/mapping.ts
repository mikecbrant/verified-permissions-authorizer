import type { APIGatewayRequestAuthorizerEvent, AppSyncAuthorizerEvent } from 'aws-lambda'

import type { MappingConfig } from './merge.js'

type Json = Record<string, any>

const getByPath = (obj: any, path: string): unknown => {
  if (!obj || !path) return undefined
  return path.split('.').reduce((acc: any, k) => (acc && typeof acc === 'object' ? acc[k] : undefined), obj)
}

const getFromJsonBody = (body: any, spec: string | undefined): unknown => {
  if (!spec) return undefined
  if (spec.startsWith('$.')) {
    // simple JSONPath subset: $.a.b.c
    const p = spec.slice(2)
    return getByPath(body, p)
  }
  return body?.[spec]
}

const parseUrlTemplate = (template: string, path: string): Record<string, string> | undefined => {
  // Minimal express-style ":var" segment extraction
  if (!template || !path) return undefined
  const tplSegs = template.split('/').filter(Boolean)
  const pathSegs = path.split('/').filter(Boolean)
  if (tplSegs.length !== pathSegs.length) return undefined
  const out: Record<string, string> = {}
  for (let i = 0; i < tplSegs.length; i++) {
    const t = tplSegs[i]
    const s = pathSegs[i]
    if (t.startsWith(':')) {
      out[t.slice(1)] = decodeURIComponent(s)
    } else if (t !== s) {
      return undefined
    }
  }
  return out
}

type SupersetDoc = {
  [namespace: string]: {
    entityTypes?: Record<string, any>
    actions?: Record<string, any>
    mappings?: MappingConfig
  }
}

const firstNs = (doc: SupersetDoc): { ns: string; body: any } => {
  const k = Object.keys(doc)[0]
  return { ns: k, body: (doc as any)[k] }
}

const substitute = (template: string, vars: Record<string, any>): string => {
  return String(template).replace(/\$([a-zA-Z0-9_]+)/g, (_m, g1) => (vars[g1] ?? '').toString())
}

type ExtractResult = {
  action?: string
  resource?: { entityType: string; entityId: string }
  // variables extracted for the action (debugging/validation aid)
  vars: Record<string, any>
}

const extractFromAppSync = (
  event: AppSyncAuthorizerEvent,
  superset: SupersetDoc,
): ExtractResult => {
  const { body } = firstNs(superset)
  const actionPath = body?.mappings?.actions?.appsync?.path || 'info.fieldName'
  const action = String(getByPath(event, actionPath) ?? '') || undefined
  const vars: Record<string, any> = {}
  if (!action) return { action: undefined, resource: undefined, vars }
  const actionDef = body?.actions?.[action]
  if (!actionDef) return { action, resource: undefined, vars }
  const appsync = actionDef.input?.appsync
  const bodyMap: Record<string, string> = appsync?.body ?? {}
  for (const [name, spec] of Object.entries(bodyMap)) {
    vars[name] = getFromJsonBody((event as any).arguments ?? {}, spec)
  }
  // Build resource from first resourceType via entityMap → template
  const rtypes: string[] = actionDef?.appliesTo?.resourceTypes ?? []
  const first = rtypes[0]
  const tplName: string | undefined = actionDef?.entityMap?.[first]
  if (!first || !tplName) return { action, resource: undefined, vars }
  const tpl = body?.entityTypes?.[first]?.resourceEntities?.[tplName]
  if (!tpl) return { action, resource: undefined, vars }
  const entityType = tpl.type ?? first
  const entityId = substitute(tpl.id ?? '', vars)
  return { action, resource: { entityType, entityId }, vars }
}

const extractFromApiGateway = (
  event: APIGatewayRequestAuthorizerEvent,
  superset: SupersetDoc,
): ExtractResult => {
  const { body } = firstNs(superset)
  const actionPath = body?.mappings?.actions?.apiGateway?.path || 'requestContext.httpMethod'
  const action = String(getByPath(event, actionPath) ?? '') || undefined
  const vars: Record<string, any> = {}
  if (!action) return { action: undefined, resource: undefined, vars }
  const actionDef = body?.actions?.[action]
  if (!actionDef) return { action, resource: undefined, vars }
  const rest = actionDef.input?.rest ?? {}
  // URL params via express-style template
  const path = (event as any).rawPath || (event as any).path || ''
  if (typeof rest.url === 'string' && rest.url) {
    const uvars = parseUrlTemplate(rest.url, path)
    if (uvars) Object.assign(vars, uvars)
  }
  // Query mapping: either string name or object map
  const q = (event.queryStringParameters ?? {}) as Record<string, string>
  if (rest.query) {
    if (typeof rest.query === 'string') {
      vars[rest.query] = q?.[rest.query]
    } else {
      for (const [name, key] of Object.entries(rest.query as Record<string, string>)) {
        vars[name] = q?.[key]
      }
    }
  }
  // Body mapping
  let bodyJson: any
  try {
    bodyJson = typeof (event as any).body === 'string' ? JSON.parse((event as any).body) : (event as any).body
  } catch {
    bodyJson = undefined
  }
  const bodyMap: Record<string, string> = rest.body ?? {}
  for (const [name, spec] of Object.entries(bodyMap)) {
    vars[name] = getFromJsonBody(bodyJson, spec)
  }
  // Build resource
  const rtypes: string[] = actionDef?.appliesTo?.resourceTypes ?? []
  const first = rtypes[0]
  const tplName: string | undefined = actionDef?.entityMap?.[first]
  if (!first || !tplName) return { action, resource: undefined, vars }
  const tpl = body?.entityTypes?.[first]?.resourceEntities?.[tplName]
  if (!tpl) return { action, resource: undefined, vars }
  const entityType = tpl.type ?? first
  const entityId = substitute(tpl.id ?? '', vars)
  return { action, resource: { entityType, entityId }, vars }
}

export { extractFromApiGateway, extractFromAppSync }
export type { SupersetDoc }
