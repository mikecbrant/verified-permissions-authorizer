import type { APIGatewayRequestAuthorizerEvent } from 'aws-lambda'

import type { ExtractResult, SupersetDoc } from './common.js'
import {
  getByPath,
  getFromJsonBody,
  getNamespaceBody,
  parseUrlTemplate,
  substitute,
} from './common.js'

const extractFromApiGateway = (
  event: APIGatewayRequestAuthorizerEvent,
  doc: SupersetDoc,
): ExtractResult => {
  const body = getNamespaceBody(doc)
  const actionPath =
    body?.mappings?.actions?.apiGateway?.path || 'requestContext.httpMethod'
  const action = String(getByPath(event, actionPath) ?? '') || undefined
  const vars: Record<string, any> = {}
  if (!action) return { action: undefined, resource: undefined, vars }
  const actionDef = body?.actions?.[action]
  if (!actionDef) return { action, resource: undefined, vars }

  const rest = actionDef.input?.rest ?? {}
  const rawPath = (event as any).rawPath || (event as any).path || ''
  if (typeof rest.url === 'string' && rest.url) {
    const uvars = parseUrlTemplate(rest.url, rawPath)
    if (uvars) Object.assign(vars, uvars)
  }
  const q = (event.queryStringParameters ?? {}) as Record<string, string>
  if (rest.query) {
    if (typeof rest.query === 'string') vars[rest.query] = q?.[rest.query]
    else
      for (const [name, key] of Object.entries(
        rest.query as Record<string, string>,
      ))
        vars[name] = q?.[key]
  }
  let bodyJson: any
  try {
    bodyJson =
      typeof (event as any).body === 'string'
        ? JSON.parse((event as any).body)
        : (event as any).body
  } catch {
    bodyJson = undefined
  }
  for (const [name, spec] of Object.entries(rest.body ?? {}))
    vars[name] = getFromJsonBody(bodyJson, String(spec))

  const rtypes: string[] = actionDef?.appliesTo?.resourceTypes ?? []
  const firstType = rtypes[0]
  const templateName: string | undefined = actionDef?.entityMap?.[firstType]
  if (!firstType || !templateName) return { action, resource: undefined, vars }
  const template =
    body?.entityTypes?.[firstType]?.resourceEntities?.[templateName]
  if (!template) return { action, resource: undefined, vars }
  const entityType = template.type ?? firstType
  const entityId = substitute(template.id ?? '', vars)
  return { action, resource: { entityType, entityId }, vars }
}

export { extractFromApiGateway }
