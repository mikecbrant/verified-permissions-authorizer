import type { AppSyncAuthorizerEvent } from 'aws-lambda'

import type { ExtractResult, SupersetDoc } from './common.js'
import {
  getByPath,
  getFromJsonBody,
  getNamespaceBody,
  substitute,
} from './common.js'

const extractFromAppSync = (
  event: AppSyncAuthorizerEvent,
  doc: SupersetDoc,
): ExtractResult => {
  const body = getNamespaceBody(doc)
  const actionPath = body?.mappings?.actions?.appsync?.path || 'info.fieldName'
  const action = String(getByPath(event, actionPath) ?? '') || undefined
  const vars: Record<string, any> = {}
  if (!action) return { action: undefined, resource: undefined, vars }
  const actionDef = body?.actions?.[action]
  if (!actionDef) return { action, resource: undefined, vars }
  const appsync = actionDef.input?.appsync
  const bodyMap: Record<string, string> = appsync?.body ?? {}
  for (const [name, spec] of Object.entries(bodyMap)) {
    vars[name] = getFromJsonBody((event as any).arguments ?? {}, String(spec))
  }
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

export { extractFromAppSync }
