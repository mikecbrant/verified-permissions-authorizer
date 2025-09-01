import * as pulumi from '@pulumi/pulumi'

type CognitoSignInAlias = 'email' | 'phone' | 'preferredUsername'

type CognitoConfig = {
  signInAliases?: pulumi.Input<pulumi.Input<CognitoSignInAlias>[]>
}

type AuthorizerLambdaConfig = {
  memorySize?: pulumi.Input<number>
  reservedConcurrency?: pulumi.Input<number>
  provisionedConcurrency?: pulumi.Input<number>
}

type AuthorizerWithPolicyStoreArgs = {
  description?: pulumi.Input<string>
  enableDynamoDbStream?: pulumi.Input<boolean>
  isEphemeral?: pulumi.Input<boolean>
  authorizerLambda?: pulumi.Input<AuthorizerLambdaConfig>
  cognito?: pulumi.Input<CognitoConfig>
}

class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  public readonly policyStoreId!: pulumi.Output<string>
  public readonly policyStoreArn!: pulumi.Output<string>
  public readonly authorizerFunctionArn!: pulumi.Output<string>
  public readonly roleArn!: pulumi.Output<string>
  public readonly TenantTableArn!: pulumi.Output<string>
  public readonly TenantTableStreamArn!: pulumi.Output<string | undefined>
  // Optional Cognito outputs
  public readonly userPoolId!: pulumi.Output<string | undefined>
  public readonly userPoolArn!: pulumi.Output<string | undefined>
  public readonly userPoolDomain!: pulumi.Output<string | undefined>
  public readonly identityPoolId!: pulumi.Output<string | undefined>
  public readonly authRoleArn!: pulumi.Output<string | undefined>
  public readonly unauthRoleArn!: pulumi.Output<string | undefined>
  public readonly userPoolClientIds!: pulumi.Output<string[] | undefined>
  public readonly parameters!: pulumi.Output<Record<string, string> | undefined>

  constructor(name: string, args: AuthorizerWithPolicyStoreArgs = {}, opts?: pulumi.ComponentResourceOptions) {
    super('verified-permissions-authorizer:index:AuthorizerWithPolicyStore', name, args, opts, true)
    const get = (n: string): pulumi.Output<any> => (this as any).getOutput(n) as pulumi.Output<any>
    this.policyStoreId = get('policyStoreId') as pulumi.Output<string>
    this.policyStoreArn = get('policyStoreArn') as pulumi.Output<string>
    this.authorizerFunctionArn = get('authorizerFunctionArn') as pulumi.Output<string>
    this.roleArn = get('roleArn') as pulumi.Output<string>
    this.TenantTableArn = get('TenantTableArn') as pulumi.Output<string>
    this.TenantTableStreamArn = get('TenantTableStreamArn') as pulumi.Output<string | undefined>
    this.userPoolId = get('userPoolId') as pulumi.Output<string | undefined>
    this.userPoolArn = get('userPoolArn') as pulumi.Output<string | undefined>
    this.userPoolDomain = get('userPoolDomain') as pulumi.Output<string | undefined>
    this.identityPoolId = get('identityPoolId') as pulumi.Output<string | undefined>
    this.authRoleArn = get('authRoleArn') as pulumi.Output<string | undefined>
    this.unauthRoleArn = get('unauthRoleArn') as pulumi.Output<string | undefined>
    this.userPoolClientIds = get('userPoolClientIds') as pulumi.Output<string[] | undefined>
    this.parameters = get('parameters') as pulumi.Output<Record<string, string> | undefined>
  }
}

export {
  type AuthorizerLambdaConfig,
  AuthorizerWithPolicyStore,
  type AuthorizerWithPolicyStoreArgs,
  type CognitoConfig,
  type CognitoSignInAlias,
}
