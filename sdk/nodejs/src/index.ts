import * as pulumi from '@pulumi/pulumi';

export type ValidationMode = 'STRICT' | 'OFF';

export interface AuthorizerWithPolicyStoreArgs {
  description?: pulumi.Input<string>;
  validationMode?: pulumi.Input<ValidationMode>;
  lambdaEnvironment?: pulumi.Input<Record<string, pulumi.Input<string>>>;
}

export class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  public readonly policyStoreId!: pulumi.Output<string>;
  public readonly policyStoreArn!: pulumi.Output<string>;
  public readonly functionArn!: pulumi.Output<string>;
  public readonly roleArn!: pulumi.Output<string>;

  constructor(
    name: string,
    args: AuthorizerWithPolicyStoreArgs = {},
    opts?: pulumi.ComponentResourceOptions,
  ) {
    super('verified-permissions-authorizer:index:AuthorizerWithPolicyStore', name, args, opts);

    // Outputs are populated by the provider plugin during registration.
  }
}

export default AuthorizerWithPolicyStore;
