import * as pulumi from '@pulumi/pulumi'

// @deprecated No longer configurable; provider always uses STRICT
type ValidationMode = 'STRICT' | 'OFF'

type AuthorizerWithPolicyStoreArgs = {
  description?: pulumi.Input<string>
  enableDynamoDbStreams?: pulumi.Input<boolean>
  isEphemeral?: pulumi.Input<boolean>
  lambdaEnvironment?: pulumi.Input<Record<string, pulumi.Input<string>>>
  /** @deprecated Use isEphemeral instead */
  ephemeralStage?: pulumi.Input<boolean>
  /** @deprecated No longer configurable; ignored */
  validationMode?: pulumi.Input<ValidationMode>
}

class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  public readonly policyStoreId!: pulumi.Output<string>
  public readonly policyStoreArn!: pulumi.Output<string>
  public readonly functionArn!: pulumi.Output<string>
  public readonly roleArn!: pulumi.Output<string>
  public readonly TenantTableArn!: pulumi.Output<string>
  public readonly TenantTableStreamArn!: pulumi.Output<string | undefined>

  constructor(name: string, args: AuthorizerWithPolicyStoreArgs = {}, opts?: pulumi.ComponentResourceOptions) {
    super('verified-permissions-authorizer:index:AuthorizerWithPolicyStore', name, args, opts, true)
    const get = (n: string) => (this as any).getOutput(n) as pulumi.Output<any>
    this.policyStoreId = get('policyStoreId') as pulumi.Output<string>
    this.policyStoreArn = get('policyStoreArn') as pulumi.Output<string>
    this.functionArn = get('functionArn') as pulumi.Output<string>
    this.roleArn = get('roleArn') as pulumi.Output<string>
    this.TenantTableArn = get('TenantTableArn') as pulumi.Output<string>
    this.TenantTableStreamArn = get('TenantTableStreamArn') as pulumi.Output<string | undefined>
  }
}

export { AuthorizerWithPolicyStore, type AuthorizerWithPolicyStoreArgs, type ValidationMode }
