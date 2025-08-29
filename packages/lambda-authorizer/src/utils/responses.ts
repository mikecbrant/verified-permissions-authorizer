import type {
  APIGatewayAuthorizerWithContextResult,
  AppSyncAuthorizerResult,
  PolicyDocument,
} from 'aws-lambda'

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

export { apiGatewayPolicy, appSyncAuthResult }
