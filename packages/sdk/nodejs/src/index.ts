import * as pulumi from '@pulumi/pulumi'

type AuthorizerWithPolicyStoreArgs = {
  description?: pulumi.Input<string>
  enableDynamoDbStream?: pulumi.Input<boolean>
  isEphemeral?: pulumi.Input<boolean>
  lambdaEnvironment?: pulumi.Input<Record<string, pulumi.Input<string>>>
}

class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  public readonly policyStoreId!: pulumi.Output<string>
  public readonly policyStoreArn!: pulumi.Output<string>
  public readonly authorizerFunctionArn!: pulumi.Output<string>
  public readonly roleArn!: pulumi.Output<string>
  public readonly TenantTableArn!: pulumi.Output<string>
  public readonly TenantTableStreamArn!: pulumi.Output<string | undefined>

  constructor(name: string, args: AuthorizerWithPolicyStoreArgs = {}, opts?: pulumi.ComponentResourceOptions) {
    super('verified-permissions-authorizer:index:AuthorizerWithPolicyStore', name, args, opts, true)
    const get = (n: string): pulumi.Output<any> => (this as any).getOutput(n) as pulumi.Output<any>
    this.policyStoreId = get('policyStoreId') as pulumi.Output<string>
    this.policyStoreArn = get('policyStoreArn') as pulumi.Output<string>
    this.authorizerFunctionArn = get('authorizerFunctionArn') as pulumi.Output<string>
    this.roleArn = get('roleArn') as pulumi.Output<string>
    this.TenantTableArn = get('TenantTableArn') as pulumi.Output<string>
    this.TenantTableStreamArn = get('TenantTableStreamArn') as pulumi.Output<string | undefined>
  }
}

export { AuthorizerWithPolicyStore, type AuthorizerWithPolicyStoreArgs }
