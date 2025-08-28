import type {
  APIGatewayAuthorizerWithContextResult,
  PolicyDocument,
  AppSyncAuthorizerResult,
} from 'aws-lambda';

type EmptyCtx = Record<string, never>;

const buildApiGatewayPolicy = (
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
  };
  return {
    principalId,
    policyDocument,
    context: {},
  };
};

const buildAppSyncAuthResult = (isAuthorized: boolean): AppSyncAuthorizerResult => ({
  isAuthorized,
});

export { buildApiGatewayPolicy, buildAppSyncAuthResult };
