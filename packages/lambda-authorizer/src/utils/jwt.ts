import type {
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
} from 'aws-lambda'
import jwt from 'jsonwebtoken'

import { isApiGatewayRequestAuthorizerEvent, isAppSyncAuthorizerEvent } from './events.js'

type JwtPayload = Record<string, unknown> & {
  exp?: number
  nbf?: number
  iat?: number
  sub?: string
}

const parseJwtPayload = (token: string): JwtPayload | undefined => {
  const decoded = jwt.decode(token)
  if (!decoded || typeof decoded !== 'object') return undefined
  const obj = decoded as JwtPayload
  const now = Math.floor(Date.now() / 1000)
  if (typeof obj.nbf === 'number' && now < obj.nbf) return undefined
  if (typeof obj.exp === 'number' && now >= obj.exp) return undefined
  return obj
}

const getBearerToken = (
  event: APIGatewayRequestAuthorizerEvent | AppSyncAuthorizerEvent,
): string | undefined => {
  if (isApiGatewayRequestAuthorizerEvent(event)) {
    const header = event.headers?.authorization ?? event.headers?.Authorization
    if (!header) return undefined
    const m = /^Bearer\s+(.+)$/i.exec(header.trim())
    return m?.[1]
  }
  if (isAppSyncAuthorizerEvent(event)) {
    const raw = event.authorizationToken?.trim()
    if (!raw) return undefined
    const m = /^Bearer\s+(.+)$/i.exec(raw) || [undefined, raw]
    return m?.[1]
  }
  return undefined
}

export type { JwtPayload }
export { getBearerToken, parseJwtPayload }
