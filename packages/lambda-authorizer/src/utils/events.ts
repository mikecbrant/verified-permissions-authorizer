import type {
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
} from 'aws-lambda';

type AnyAuthorizerEvent = APIGatewayRequestAuthorizerEvent | AppSyncAuthorizerEvent | Record<string, unknown>;

const isApiGatewayRequestAuthorizerEvent = (
  event: AnyAuthorizerEvent,
): event is APIGatewayRequestAuthorizerEvent => {
  return (
    typeof (event as APIGatewayRequestAuthorizerEvent).methodArn === 'string' &&
    typeof (event as APIGatewayRequestAuthorizerEvent).type === 'string' &&
    // APIGW Request Authorizer events have headers and resource/methodArn
    'headers' in (event as object)
  );
};

const isAppSyncAuthorizerEvent = (event: AnyAuthorizerEvent): event is AppSyncAuthorizerEvent => {
  // AppSync Authorizer events include an `authorizationToken` and `requestContext` with `apiId`.
  const maybe = event as Partial<AppSyncAuthorizerEvent>;
  return (
    typeof maybe?.authorizationToken === 'string' &&
    typeof maybe?.requestContext === 'object' &&
    maybe?.requestContext !== null &&
    typeof (maybe.requestContext as any).apiId === 'string'
  );
};

export type { AnyAuthorizerEvent };
export { isApiGatewayRequestAuthorizerEvent, isAppSyncAuthorizerEvent };
