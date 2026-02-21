import type {
  EntitiesDefinition,
  IsAuthorizedInput,
} from '@aws-sdk/client-verifiedpermissions'
import type {
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
} from 'aws-lambda'

import { extractFromApiGateway } from '../extractors/apigateway.js'
import { extractFromAppSync } from '../extractors/appsync.js'
import { extendedSchema } from '../schema.js'
import {
  isApiGatewayAuthorizerEvent,
  isAppSyncAuthorizerEvent,
} from '../utils/events.js'
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
  let actionId = 'invoke'
  let resource = {
    entityType: 'Resource',
    entityId: isApiGatewayAuthorizerEvent(event)
      ? (event as APIGatewayRequestAuthorizerEvent).methodArn
      : `appsync:${(event as any).requestContext?.apiId}`,
  }
  if (extendedSchema) {
    if (isAppSyncAuthorizerEvent(event)) {
      const out = extractFromAppSync(event, extendedSchema)
      if (!out.action)
        throw new Error('mapping: missing action identifier for AppSync event')
      if (!out.resource)
        throw new Error(
          `mapping: missing resource template for action ${out.action}`,
        )
      actionId = out.action
      resource = out.resource
    } else if (isApiGatewayAuthorizerEvent(event)) {
      const out = extractFromApiGateway(event, extendedSchema)
      if (!out.action)
        throw new Error(
          'mapping: missing action identifier for API Gateway event',
        )
      if (!out.resource)
        throw new Error(
          `mapping: missing resource template for action ${out.action}`,
        )
      actionId = out.action
      resource = out.resource
    }
  }
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
  try {
    const input = buildInput(policyStoreId, event, subject)
    return await authorize(input)
  } catch (err) {
    console.error(
      '[authorizer] refusing request due to mapping/schema error',
      err,
    )
    return false
  }
}

export type { AuthorizerEvent }
export { processAuthorization }
