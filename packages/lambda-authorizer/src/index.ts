import type {
  APIGatewayAuthorizerWithContextResult,
  APIGatewayRequestAuthorizerEvent,
  PolicyDocument,
} from 'aws-lambda';
import {
  VerifiedPermissionsClient,
  GetPolicyStoreCommand,
} from '@aws-sdk/client-verifiedpermissions';

type AuthorizerEvent = APIGatewayRequestAuthorizerEvent;

const { POLICY_STORE_ID: policyStoreIdFromEnv } = process.env as Record<string, string | undefined>;

function buildPolicy(
  effect: 'Allow' | 'Deny',
  resourceArn: string,
  principalId: string,
): APIGatewayAuthorizerWithContextResult<Record<string, never>> {
  const policyDocument: PolicyDocument = {
    Version: '2012-10-17',
    Statement: [
      {
        Action: 'execute-api:Invoke',
        Effect: effect,
        Resource: resourceArn,
      },
    ],
  };
  return {
    principalId,
    policyDocument,
    context: {},
  };
}

export async function handler(
  event: AuthorizerEvent,
): Promise<APIGatewayAuthorizerWithContextResult<Record<string, never>>> {
  const token = event.headers?.authorization ?? event.headers?.Authorization;
  const methodArn = event.methodArn;
  const principalId = token ?? 'anonymous';

  const storeId = policyStoreIdFromEnv;
  if (!storeId) {
    // Fail closed: deny when not configured
    return buildPolicy('Deny', methodArn, principalId);
  }

  // Lightweight interaction with the Policy Store to verify it exists and is accessible.
  const vp = new VerifiedPermissionsClient({});
  try {
    await vp.send(new GetPolicyStoreCommand({ policyStoreId: storeId }));
    // For initial scaffold we allow when the store is reachable. Replace with IsAuthorized checks later.
    return buildPolicy('Allow', methodArn, principalId);
  } catch {
    return buildPolicy('Deny', methodArn, principalId);
  }
}

export type { AuthorizerEvent };
