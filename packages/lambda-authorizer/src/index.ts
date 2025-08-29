import type {
  APIGatewayAuthorizerWithContextResult,
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
  AppSyncAuthorizerResult,
  PolicyDocument,
} from 'aws-lambda'

import { type AuthorizerEvent, processAuthorization } from './auth/process.js'
import { isApiGatewayAuthorizerEvent, isAppSyncAuthorizerEvent } from './utils/events.js'

type EmptyCtx = Record<string, never>

const apiGatewayPolicy = (
  effect: 'Allow' | 'Deny',
  resourceArn: string,
  principalId: string,
): APIGatewayAuthorizerWithContextResult<EmptyCtx> => {
  const policyDocument: PolicyDocument = {
    Version: '2012-10-17',
    Statement: [
      {
        Action: 'execute-api:Invoke',
        Effect: effect,
        Resource: resourceArn,
      },
    ],
  }
  return { principalId, policyDocument, context: {} }
}

const appSyncAuthResult = (isAuthorized: boolean): AppSyncAuthorizerResult => ({
  isAuthorized,
})
type AuthorizerResult =
  | APIGatewayAuthorizerWithContextResult<EmptyCtx>
  | AppSyncAuthorizerResult

const denyFor = (event: AuthorizerEvent): AuthorizerResult => {
  if (isApiGatewayAuthorizerEvent(event)) {
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
      if (isApiGatewayAuthorizerEvent(event)) {
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
