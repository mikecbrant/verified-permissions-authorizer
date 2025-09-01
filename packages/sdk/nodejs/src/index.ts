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

class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  public readonly policyStoreId!: pulumi.Output<string>;
  public readonly policyStoreArn!: pulumi.Output<string>;
  public readonly authorizerFunctionArn!: pulumi.Output<string>;
  public readonly roleArn!: pulumi.Output<string>;
  // Renamed outputs: prefer `AuthTable*`; still resolve values if provider uses legacy `TenantTable*` keys.
  public readonly AuthTableArn!: pulumi.Output<string>;
  public readonly AuthTableStreamArn!: pulumi.Output<string | undefined>;
  /** @deprecated Use AuthTableArn */
  public readonly TenantTableArn!: pulumi.Output<string>;
  /** @deprecated Use AuthTableStreamArn */
  public readonly TenantTableStreamArn!: pulumi.Output<string | undefined>;
  // Optional Cognito outputs
  public readonly userPoolId!: pulumi.Output<string | undefined>;
  public readonly userPoolArn!: pulumi.Output<string | undefined>;
  public readonly userPoolDomain!: pulumi.Output<string | undefined>;
  public readonly identityPoolId!: pulumi.Output<string | undefined>;
  public readonly authRoleArn!: pulumi.Output<string | undefined>;
  public readonly unauthRoleArn!: pulumi.Output<string | undefined>;
  public readonly userPoolClientIds!: pulumi.Output<string[] | undefined>;
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
    const getFirstOrThrow = <T>(...names: string[]): pulumi.Output<T> =>
      pulumi
        .all(names.map((n) => get(n)))
        .apply((vals) => {
          for (const v of vals) {
            if (v !== undefined && v !== null) return v as T;
          }
          throw new Error(
            `None of the expected outputs were set: ${names.join(", ")}`,
          );
        }) as pulumi.Output<T>;
    const getFirstOptional = <T>(
      ...names: string[]
    ): pulumi.Output<T | undefined> =>
      pulumi
        .all(names.map((n) => get(n)))
        .apply((vals) => {
          for (const v of vals) {
            if (v !== undefined && v !== null) return v as T;
          }
          return undefined;
        }) as pulumi.Output<T | undefined>;
    this.policyStoreId = get("policyStoreId") as pulumi.Output<string>;
    this.policyStoreArn = get("policyStoreArn") as pulumi.Output<string>;
    this.authorizerFunctionArn = get(
      "authorizerFunctionArn",
    ) as pulumi.Output<string>;
    this.roleArn = get("roleArn") as pulumi.Output<string>;
    this.AuthTableArn = getFirstOrThrow<string>(
      "AuthTableArn",
      "TenantTableArn",
    );
    this.AuthTableStreamArn = getFirstOptional<string>(
      "AuthTableStreamArn",
      "TenantTableStreamArn",
    );
    // Deprecated aliases for backward compatibility
    this.TenantTableArn = this.AuthTableArn;
    this.TenantTableStreamArn = this.AuthTableStreamArn;
    this.userPoolId = get("userPoolId") as pulumi.Output<string | undefined>;
    this.userPoolArn = get("userPoolArn") as pulumi.Output<string | undefined>;
    this.userPoolDomain = get("userPoolDomain") as pulumi.Output<
      string | undefined
    >;
    this.identityPoolId = get("identityPoolId") as pulumi.Output<
      string | undefined
    >;
    this.authRoleArn = get("authRoleArn") as pulumi.Output<string | undefined>;
    this.unauthRoleArn = get("unauthRoleArn") as pulumi.Output<
      string | undefined
    >;
    this.userPoolClientIds = get("userPoolClientIds") as pulumi.Output<
      string[] | undefined
    >;
    this.parameters = get("parameters") as pulumi.Output<
      Record<string, string> | undefined
    >;
  }
}

export {
  AuthorizerWithPolicyStore,
  type AuthorizerWithPolicyStoreArgs,
  type CognitoConfig,
  type CognitoSignInAlias,
  type DynamoConfig,
  type LambdaConfig,
};
