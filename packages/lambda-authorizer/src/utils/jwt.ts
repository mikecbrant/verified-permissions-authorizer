import type {
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
} from 'aws-lambda'
import jwt from 'jsonwebtoken'

import {
  isApiGatewayAuthorizerEvent,
  isAppSyncAuthorizerEvent,
} from './events.js'

type JwtPayload = Record<string, unknown> & {
  exp?: number
  nbf?: number
  iat?: number
  sub?: string
}

const parseJwtPayload = (
  token: string,
  key: jwt.Secret | jwt.GetPublicKeyOrSecret,
  options?: jwt.VerifyOptions,
): JwtPayload | undefined => {
  try {
    const base: jwt.VerifyOptions = {
      // Default to enforcing exp and nbf; caller can override via options
      clockTimestamp: Math.floor(Date.now() / 1000),
      ...options,
    }
    // Enforce an explicit algorithms allowlist if caller didn't provide one.
    if (!base.algorithms) base.algorithms = ['HS256']
    const verified = jwt.verify(token, key, base)
    return typeof verified === 'object' ? (verified as JwtPayload) : undefined
  } catch {
    return undefined
  }
}

const getBearerToken = (
  event: APIGatewayRequestAuthorizerEvent | AppSyncAuthorizerEvent,
): string | undefined => {
  if (isApiGatewayAuthorizerEvent(event)) {
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
