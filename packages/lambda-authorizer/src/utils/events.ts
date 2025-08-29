import type {
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
} from 'aws-lambda'

type AnyAuthorizerEvent = APIGatewayRequestAuthorizerEvent | AppSyncAuthorizerEvent

const isRecord = (v: unknown): v is Record<string, unknown> =>
  typeof v === 'object' && v !== null

const isApiGatewayRequestAuthorizerEvent = (
  event: unknown,
): event is APIGatewayRequestAuthorizerEvent => {
  if (!isRecord(event)) return false
  const methodArn = event['methodArn']
  const headers = event['headers']
  const typ = event['type']
  return (
    typeof methodArn === 'string' &&
    typeof typ === 'string' &&
    typeof headers === 'object' && headers !== null
  )
}

const isAppSyncAuthorizerEvent = (event: unknown): event is AppSyncAuthorizerEvent => {
  if (!isRecord(event)) return false
  const authorizationToken = event['authorizationToken']
  const requestContext = event['requestContext']
  const apiId = isRecord(requestContext) ? requestContext['apiId'] : undefined
  return (
    typeof authorizationToken === 'string' &&
    typeof apiId === 'string'
  )
}

export type { AnyAuthorizerEvent }
export { isApiGatewayRequestAuthorizerEvent, isAppSyncAuthorizerEvent }
