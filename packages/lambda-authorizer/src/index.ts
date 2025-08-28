import type {
  APIGatewayAuthorizerWithContextResult,
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
  AppSyncAuthorizerResult,
} from 'aws-lambda';
import { GetPolicyStoreCommand } from '@aws-sdk/client-verifiedpermissions';
import { getVerifiedPermissionsClient } from './aws/vpClient.js';
import { buildApiGatewayPolicy, buildAppSyncAuthResult } from './policy.js';
import { getBearerToken, parseJwtPayload } from './utils/jwt.js';
import { isApiGatewayRequestAuthorizerEvent, isAppSyncAuthorizerEvent } from './utils/events.js';

type EmptyCtx = Record<string, never>;

const { POLICY_STORE_ID: policyStoreIdFromEnv } = process.env as Record<string, string | undefined>;

type AuthorizerEvent = APIGatewayRequestAuthorizerEvent | AppSyncAuthorizerEvent;
type AuthorizerResult =
  | APIGatewayAuthorizerWithContextResult<EmptyCtx>
  | AppSyncAuthorizerResult;

const denyFor = (event: AuthorizerEvent): AuthorizerResult => {
  if (isApiGatewayRequestAuthorizerEvent(event)) {
    return buildApiGatewayPolicy('Deny', event.methodArn, 'anonymous');
  }
  if (isAppSyncAuthorizerEvent(event)) {
    return buildAppSyncAuthResult(false);
  }
  // Default safe deny
  return buildAppSyncAuthResult(false);
};

const handler = async (event: AuthorizerEvent): Promise<AuthorizerResult> => {
  const token = getBearerToken(event);
  if (!token) return denyFor(event);

  // Minimal local JWT validation (structure/exp/nbf)
  const payload = parseJwtPayload(token);
  if (!payload) return denyFor(event);

  // Ensure a policy store is configured
  const storeId = policyStoreIdFromEnv;
  if (!storeId) return denyFor(event);

  const client = getVerifiedPermissionsClient();
  const cmd = new GetPolicyStoreCommand({ policyStoreId: storeId });

  return client
    .send(cmd)
    .then(() => {
      if (isApiGatewayRequestAuthorizerEvent(event)) {
        const principalId = typeof payload.sub === 'string' ? payload.sub : 'subject';
        return buildApiGatewayPolicy('Allow', event.methodArn, principalId);
      }
      if (isAppSyncAuthorizerEvent(event)) {
        return buildAppSyncAuthResult(true);
      }
      return buildAppSyncAuthResult(false);
    })
    .catch(() => denyFor(event));
};

export { handler };
