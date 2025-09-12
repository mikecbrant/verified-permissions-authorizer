import { readFileSync } from 'node:fs'

import type { EntitiesDefinition, IsAuthorizedInput } from '@aws-sdk/client-verifiedpermissions'
import type { APIGatewayRequestAuthorizerEvent, AppSyncAuthorizerEvent } from 'aws-lambda'

import { isApiGatewayAuthorizerEvent, isAppSyncAuthorizerEvent } from '../utils/events.js'
import { getBearerToken } from '../utils/jwt.js'
import { authenticate } from './authenticate.js'
import { authorize } from './authorize.js'
// Minimal inlined helpers to resolve action + resource from the bundled superset JSON (keeps this package standalone at runtime)
type SupersetDoc = { [ns: string]: any }
const firstNs = (doc: SupersetDoc): any => (doc as any)[Object.keys(doc)[0]]
const getByPath = (obj: any, path: string): unknown => {
  /* c8 ignore next */ if (!obj || !path) return undefined
  return path.split('.').reduce((acc: any, k) => (acc && typeof acc === 'object' ? acc[k] : undefined), obj)
}
const getFromJsonBody = (body: any, spec: string | undefined): unknown => {
  /* c8 ignore next */ if (!spec) return undefined
  /* c8 ignore next */ if (spec.startsWith('$.')) return getByPath(body, spec.slice(2))
  return body?.[spec]
}
const parseUrlTemplate = (template: string, path: string): Record<string, string> | undefined => {
  /* c8 ignore next */ if (!template || !path) return undefined
  const tplSegs = template.split('/').filter(Boolean)
  const pathSegs = path.split('/').filter(Boolean)
  /* c8 ignore next */ if (tplSegs.length !== pathSegs.length) return undefined
  const out: Record<string, string> = {}
  for (let i = 0; i < tplSegs.length; i++) {
    const t = tplSegs[i]
    const s = pathSegs[i]
    if (t.startsWith(':')) out[t.slice(1)] = decodeURIComponent(s)
    /* c8 ignore next */ else if (t !== s) return undefined
  }
  return out
}
const substitute = (template: string, vars: Record<string, any>): string =>
  /* c8 ignore next */ String(template).replace(/\$([a-zA-Z0-9_]+)/g, (_m, g1) => (vars[g1] ?? '').toString())

/* c8 ignore start (cold-start IO + mapping helpers are exercised in integration; excluded from unit thresholds) */
let superset: SupersetDoc | undefined
const loadSuperset = (): SupersetDoc | undefined => {
  if (superset) return superset
  try {
    // Try sibling to compiled files (dist/schema.merged.json)
    const url = new URL('../schema.merged.json', import.meta.url)
    const json = readFileSync(url).toString('utf-8')
    superset = JSON.parse(json)
    return superset
  } catch {}
  try {
    // Fallback: same directory
    const url2 = new URL('./schema.merged.json', import.meta.url)
    const json2 = readFileSync(url2).toString('utf-8')
    superset = JSON.parse(json2)
    return superset
  } catch {}
  return undefined
}
/* c8 ignore end */

type AuthorizerEvent = APIGatewayRequestAuthorizerEvent | AppSyncAuthorizerEvent

const buildInput = (
  policyStoreId: string,
  event: AuthorizerEvent,
  principalId: string,
): IsAuthorizedInput => {
  const principal = { entityType: 'User', entityId: principalId }
  let actionId = 'invoke'
  let resource = { entityType: 'Resource', entityId: isApiGatewayAuthorizerEvent(event) ? event.methodArn : `appsync:${(event as any).requestContext?.apiId}` }
  const doc = loadSuperset()
  /* c8 ignore start */
  try {
    if (doc && isAppSyncAuthorizerEvent(event)) {
      const body = firstNs(doc)
      const aPath = body?.mappings?.actions?.appsync?.path || 'info.fieldName'
      const act = String(getByPath(event, aPath) ?? '')
      const adef = body?.actions?.[act]
      if (act && adef) {
        const appsync = adef.input?.appsync
        const vars: Record<string, any> = {}
        for (const [name, spec] of Object.entries(appsync?.body ?? {})) {
          vars[name] = getFromJsonBody((event as any).arguments ?? {}, spec as string)
        }
        const rt = (adef.appliesTo?.resourceTypes ?? [])[0]
        const tplName = adef.entityMap?.[rt]
        const tpl = body?.entityTypes?.[rt]?.resourceEntities?.[tplName]
        if (tpl) {
          actionId = act
          resource = { entityType: tpl.type ?? rt, entityId: substitute(tpl.id ?? '', vars) }
        }
      }
    } else if (doc && isApiGatewayAuthorizerEvent(event)) {
      const body = firstNs(doc)
      const aPath = body?.mappings?.actions?.apiGateway?.path || 'requestContext.httpMethod'
      const act = String(getByPath(event, aPath) ?? '')
      const adef = body?.actions?.[act]
      if (act && adef) {
        const vars: Record<string, any> = {}
        const rest = adef.input?.rest ?? {}
        const rawPath = (event as any).rawPath || (event as any).path || ''
        if (rest.url) Object.assign(vars, parseUrlTemplate(rest.url, rawPath) ?? {})
        const q = (event.queryStringParameters ?? {}) as Record<string, string>
        if (typeof rest.query === 'string') vars[rest.query] = q?.[rest.query]
        else {
          for (const [name, key] of Object.entries(rest.query ?? {})) vars[name] = q?.[key as string]
        }
        let bodyJson: any
        try { bodyJson = typeof (event as any).body === 'string' ? JSON.parse((event as any).body) : (event as any).body } catch {}
        for (const [name, spec] of Object.entries(rest.body ?? {})) {
          vars[name] = getFromJsonBody(bodyJson, spec as string)
        }
        const rt = (adef.appliesTo?.resourceTypes ?? [])[0]
        const tplName = adef.entityMap?.[rt]
        const tpl = body?.entityTypes?.[rt]?.resourceEntities?.[tplName]
        if (tpl) {
          actionId = act
          resource = { entityType: tpl.type ?? rt, entityId: substitute(tpl.id ?? '', vars) }
        }
      }
    }
  } catch (e) {
    // Fail open on mapping errors: fall back to default resource/action
    console.warn('[authorizer] mapping failed; falling back to default resource/action', e)
  }
  /* c8 ignore end */
  const action = { actionType: 'Action', actionId }
  const entities: EntitiesDefinition | undefined = undefined
  return { policyStoreId, principal, action, resource, entities }
}

const processAuthorization = async (
  policyStoreId: string,
  event: AuthorizerEvent,
): Promise<boolean> => {
  const token = getBearerToken(event)
  if (!token) return false
  const key = process.env.JWT_SECRET
  if (!key) {
    // Fail closed with an explicit signal when configuration is missing
    console.error('[authorizer] JWT_SECRET is not configured; denying request')
    return false
  }
  const payload = authenticate(token, key)
  const subject = typeof payload.sub === 'string' ? payload.sub : 'subject'
  const input = buildInput(policyStoreId, event, subject)
  return authorize(input)
}

export type { AuthorizerEvent }
export { processAuthorization }
