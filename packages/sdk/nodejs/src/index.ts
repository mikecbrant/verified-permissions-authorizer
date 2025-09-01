/**
 * This file is a small, hand-authored Node.js SDK wrapper for the Go component provider.
 * It is not code-generated. Keep the args/types and output names in sync with the provider schema.
 */
import * as pulumi from "@pulumi/pulumi";

type CognitoSignInAlias = "email" | "phone" | "preferredUsername";

type CognitoConfig = {
  signInAliases?: pulumi.Input<pulumi.Input<CognitoSignInAlias>[]>;
  sesConfig?: pulumi.Input<{
    sourceArn: pulumi.Input<string>;
    from: pulumi.Input<string>;
    replyToEmail?: pulumi.Input<string>;
    configurationSet?: pulumi.Input<string>;
  }>;
};

type LambdaConfig = {
  memorySize?: pulumi.Input<number>;
  reservedConcurrency?: pulumi.Input<number>;
  provisionedConcurrency?: pulumi.Input<number>;
};

type DynamoConfig = {
  enableDynamoDbStream?: pulumi.Input<boolean>;
};

type AuthorizerWithPolicyStoreArgs = {
  description?: pulumi.Input<string>;
  retainOnDelete?: pulumi.Input<boolean>;
  lambda?: pulumi.Input<LambdaConfig>;
  dynamo?: pulumi.Input<DynamoConfig>;
  cognito?: pulumi.Input<CognitoConfig>;
};

// Output group shapes
type AuthorizerLambdaOutputs = {
  authorizerFunctionArn: pulumi.Output<string>;
  roleArn: pulumi.Output<string>;
};
type AuthorizerDynamoOutputs = {
  AuthTableArn: pulumi.Output<string>;
  AuthTableStreamArn: pulumi.Output<string | undefined>;
};
type AuthorizerCognitoOutputs = {
  userPoolId: pulumi.Output<string | undefined>;
  userPoolArn: pulumi.Output<string | undefined>;
  userPoolClientIds: pulumi.Output<string[] | undefined>;
};

class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  public readonly policyStoreId!: pulumi.Output<string>;
  public readonly policyStoreArn!: pulumi.Output<string>;
  // Grouped outputs
  public readonly lambda!: AuthorizerLambdaOutputs;
  public readonly dynamo!: AuthorizerDynamoOutputs;
  public readonly cognito!: AuthorizerCognitoOutputs;
  public readonly parameters!: pulumi.Output<
    Record<string, string> | undefined
  >;

  constructor(
    name: string,
    args: AuthorizerWithPolicyStoreArgs = {},
    opts?: pulumi.ComponentResourceOptions,
  ) {
    super(
      "verified-permissions-authorizer:index:AuthorizerWithPolicyStore",
      name,
      args,
      opts,
      true,
    );
    const get = (n: string): pulumi.Output<any> =>
      (this as any).getOutput(n) as pulumi.Output<any>;
    this.policyStoreId = get("policyStoreId") as pulumi.Output<string>;
    this.policyStoreArn = get("policyStoreArn") as pulumi.Output<string>;

    const g = (n: string): pulumi.Output<any> => get(n) as pulumi.Output<any>;

    // lambda group
    const lambda = g("lambda");
    this.lambda = {
      authorizerFunctionArn: lambda.apply((o) => {
        if (!o?.authorizerFunctionArn) {
          throw new Error(
            "Required output not set: lambda.authorizerFunctionArn",
          );
        }
        return o.authorizerFunctionArn as string;
      }),
      roleArn: lambda.apply((o) => {
        if (!o?.roleArn) {
          throw new Error("Required output not set: lambda.roleArn");
        }
        return o.roleArn as string;
      }),
    };

    // dynamo group
    const dynamo = g("dynamo");
    this.dynamo = {
      AuthTableArn: dynamo.apply((o) => {
        if (!o?.AuthTableArn) {
          throw new Error("Required output not set: dynamo.AuthTableArn");
        }
        return o.AuthTableArn as string;
      }),
      AuthTableStreamArn: dynamo.apply(
        (o) => (o?.AuthTableStreamArn as string | undefined) ?? undefined,
      ),
    };

    // cognito group (optional fields)
    const cognito = g("cognito");
    this.cognito = {
      userPoolId: cognito.apply((o) => o?.userPoolId as string | undefined),
      userPoolArn: cognito.apply((o) => o?.userPoolArn as string | undefined),
      userPoolClientIds: cognito.apply(
        (o) => o?.userPoolClientIds as string[] | undefined,
      ),
    };

    this.parameters = get("parameters") as pulumi.Output<
      Record<string, string> | undefined
    >;
  }
}

export {
  // Output group shapes (exported for convenience in TS projects)
  type AuthorizerCognitoOutputs,
  type AuthorizerDynamoOutputs,
  type AuthorizerLambdaOutputs,
  AuthorizerWithPolicyStore,
  type AuthorizerWithPolicyStoreArgs,
  type CognitoConfig,
  type CognitoSignInAlias,
  type DynamoConfig,
  type LambdaConfig,
};
