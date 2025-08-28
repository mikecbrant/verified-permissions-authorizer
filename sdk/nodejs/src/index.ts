import * as pulumi from '@pulumi/pulumi';

type ValidationMode = 'STRICT' | 'OFF';

type AuthorizerWithPolicyStoreArgs = {
  description?: pulumi.Input<string>;
  validationMode?: pulumi.Input<ValidationMode>;
  lambdaEnvironment?: pulumi.Input<Record<string, pulumi.Input<string>>>;
}

class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
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

export { AuthorizerWithPolicyStore, type AuthorizerWithPolicyStoreArgs, type ValidationMode };
