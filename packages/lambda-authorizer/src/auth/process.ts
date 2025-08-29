import type {
  EntitiesDefinition,
  IsAuthorizedInput,
} from '@aws-sdk/client-verifiedpermissions'
import type {
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
} from 'aws-lambda'

import { isApiGatewayAuthorizerEvent } from '../utils/events.js'
import { getBearerToken } from '../utils/jwt.js'
import { authenticate } from './authenticate.js'
import { authorize } from './authorize.js'

type AuthorizerEvent = APIGatewayRequestAuthorizerEvent | AppSyncAuthorizerEvent

const buildInput = (
  policyStoreId: string,
  event: AuthorizerEvent,
  principalId: string,
): IsAuthorizedInput => {
  const principal = { entityType: 'User', entityId: principalId }
  const action = { actionType: 'Action', actionId: 'invoke' }
  const resourceId = isApiGatewayAuthorizerEvent(event)
    ? event.methodArn
    : `appsync:${event.requestContext.apiId}`
  const resource = { entityType: 'Resource', entityId: resourceId }
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
