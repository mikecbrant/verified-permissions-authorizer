import * as pulumi from '@pulumi/pulumi'

type ValidationMode = 'STRICT' | 'OFF'

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
  validationMode?: pulumi.Input<ValidationMode>
  lambdaEnvironment?: pulumi.Input<Record<string, pulumi.Input<string>>>
  isEphemeral?: pulumi.Input<boolean>
  cognito?: pulumi.Input<CognitoConfig>
}

class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  constructor(name: string, args: AuthorizerWithPolicyStoreArgs, opts?: pulumi.ComponentResourceOptions) {
    super('verified-permissions-authorizer:index:AuthorizerWithPolicyStore', name, {}, opts)
  }
}

export { AuthorizerWithPolicyStore, type AuthorizerWithPolicyStoreArgs, type CognitoConfig, type CognitoTriggerConfig, type ValidationMode }
