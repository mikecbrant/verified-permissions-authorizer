import type { APIGatewayRequestAuthorizerEvent, AppSyncAuthorizerEvent } from 'aws-lambda'

import type { MappingConfig } from './merge.js'

const getByPath = (obj: any, path: string): unknown => {
  if (!obj || !path) return undefined
  return path.split('.').reduce((acc: any, k) => (acc && typeof acc === 'object' ? acc[k] : undefined), obj)
}

export const extractFromAppSync = (event: AppSyncAuthorizerEvent, cfg: MappingConfig | undefined): { props: Record<string, unknown>, action?: string } => {
  const props: Record<string, unknown> = {}
  if (cfg?.properties) {
    for (const [k, v] of Object.entries(cfg.properties)) {
      if (v.appsync?.path) props[k] = getByPath(event, v.appsync.path)
    }
  }
  const action = cfg?.actions?.appsync?.path ? String(getByPath(event, cfg.actions.appsync.path)) : undefined
  return { props, action }
}

export const extractFromApiGateway = (event: APIGatewayRequestAuthorizerEvent, cfg: MappingConfig | undefined): { props: Record<string, unknown>, action?: string } => {
  const props: Record<string, unknown> = {}
  if (cfg?.properties) {
    for (const [k, v] of Object.entries(cfg.properties)) {
      const ag = v.apiGateway
      if (!ag) continue
      switch (ag.source) {
        case 'path':
          props[k] = event.pathParameters?.[String(ag.name ?? k)]
          break
        case 'query':
          props[k] = event.queryStringParameters?.[String(ag.name ?? k)]
          break
        case 'body': {
          let body: any
          try { body = typeof (event as any).body === 'string' ? JSON.parse((event as any).body) : (event as any).body } catch { body = undefined }
          props[k] = ag.path ? getByPath(body, ag.path) : undefined
          break
        }
      }
    }
  }
  const action = cfg?.actions?.apiGateway?.path ? String(getByPath(event, cfg.actions.apiGateway.path)) : undefined
  return { props, action }
}
