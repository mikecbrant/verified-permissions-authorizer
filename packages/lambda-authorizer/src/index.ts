import type {
  APIGatewayAuthorizerWithContextResult,
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
  AppSyncAuthorizerResult,
} from 'aws-lambda'

import { type AuthorizerEvent,processAuthorization } from './auth/process.js'
import { isApiGatewayRequestAuthorizerEvent, isAppSyncAuthorizerEvent } from './utils/events.js'
import { apiGatewayPolicy, appSyncAuthResult } from './utils/responses.js'

type EmptyCtx = Record<string, never>


type AuthorizerResult =
  | APIGatewayAuthorizerWithContextResult<EmptyCtx>
  | AppSyncAuthorizerResult

const denyFor = (event: AuthorizerEvent): AuthorizerResult => {
  if (isApiGatewayRequestAuthorizerEvent(event)) {
    return apiGatewayPolicy('Deny', event.methodArn, 'anonymous')
  }
  if (isAppSyncAuthorizerEvent(event)) {
    return appSyncAuthResult(false)
  }
  return appSyncAuthResult(false)
}

const handler = async (event: AuthorizerEvent): Promise<AuthorizerResult> => {
  const { POLICY_STORE_ID: storeId } = process.env as Record<string, string | undefined>
  if (!storeId) return denyFor(event)

  return processAuthorization(storeId, event)
    .then((ok) => {
      if (isApiGatewayRequestAuthorizerEvent(event)) {
        // We do not have principalId here; API GW requires a principal. Use anonymous when denied.
        const principal = ok ? 'subject' : 'anonymous'
        return apiGatewayPolicy(ok ? 'Allow' : 'Deny', event.methodArn, principal)
      }
      if (isAppSyncAuthorizerEvent(event)) return appSyncAuthResult(ok)
      return appSyncAuthResult(false)
    })
    .catch((err) => {
      console.error('[authorizer] error while processing auth:', err)
      return denyFor(event)
    })
}

export { handler }
