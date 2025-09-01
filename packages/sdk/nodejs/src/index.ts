import * as pulumi from '@pulumi/pulumi'

type CognitoTriggerConfig = {
  enabled?: pulumi.Input<boolean>
  environment?: pulumi.Input<Record<string, pulumi.Input<string>>>
  permissions?: pulumi.Input<pulumi.Input<string>[]>
}

type CognitoConfig = {
  identityPoolFederation?: pulumi.Input<boolean>
  signInAliases?: pulumi.Input<{
    username?: pulumi.Input<boolean>
    email?: pulumi.Input<boolean>
    phone?: pulumi.Input<boolean>
    preferredUsername?: pulumi.Input<boolean>
  }>
  emailSendingAccount?: pulumi.Input<'COGNITO_DEFAULT' | 'DEVELOPER'>
  mfa?: pulumi.Input<'OFF' | 'ON' | 'OPTIONAL'>
  mfaMessage?: pulumi.Input<string>
  accountRecovery?: pulumi.Input<string>
  autoVerify?: pulumi.Input<{ email?: pulumi.Input<boolean>; phone?: pulumi.Input<boolean> }>
  advancedSecurityMode?: pulumi.Input<'OFF' | 'AUDIT' | 'ENFORCED'>
  userInvitation?: pulumi.Input<{ emailSubject?: pulumi.Input<string>; emailBody?: pulumi.Input<string>; smsMessage?: pulumi.Input<string> }>
  userVerification?: pulumi.Input<{ emailSubject?: pulumi.Input<string>; emailBody?: pulumi.Input<string>; smsMessage?: pulumi.Input<string> }>
  customAttributes?: pulumi.Input<{ globalRoles?: pulumi.Input<boolean>; tenantId?: pulumi.Input<boolean>; tenantName?: pulumi.Input<boolean>; userId?: pulumi.Input<boolean> }>
  domain?: pulumi.Input<{ domainName?: pulumi.Input<string>; certificateArn?: pulumi.Input<string> }>
  triggers?: pulumi.Input<Record<string, pulumi.Input<CognitoTriggerConfig>>>
  clients?: pulumi.Input<pulumi.Input<string>[]>
}

type AuthorizerWithPolicyStoreArgs = {
  description?: pulumi.Input<string>
  enableDynamoDbStream?: pulumi.Input<boolean>
  isEphemeral?: pulumi.Input<boolean>
  lambdaEnvironment?: pulumi.Input<Record<string, pulumi.Input<string>>>
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

export { AuthorizerWithPolicyStore, type AuthorizerWithPolicyStoreArgs, type CognitoConfig, type CognitoTriggerConfig }
