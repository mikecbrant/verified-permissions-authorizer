import type * as pulumi from "@pulumi/pulumi";

// Alphabetized properties in all types

export type CognitoSignInAlias = "email" | "phone" | "preferredUsername";

export type CognitoConfig = {
  sesConfig?: pulumi.Input<{
    configurationSet?: pulumi.Input<string>;
    from: pulumi.Input<string>;
    replyToEmail?: pulumi.Input<string>;
    sourceArn: pulumi.Input<string>;
  }>;
  signInAliases?: pulumi.Input<pulumi.Input<CognitoSignInAlias>[]>;
};

export type DynamoConfig = {
  enableDynamoDbStream?: pulumi.Input<boolean>;
};

export type LambdaConfig = {
  memorySize?: pulumi.Input<number>;
  provisionedConcurrency?: pulumi.Input<number>;
  reservedConcurrency?: pulumi.Input<number>;
};

export type VerifiedPermissionsConfig = {
  actionGroupEnforcement?: pulumi.Input<"off" | "warn" | "error">;
  canaryFile?: pulumi.Input<string>;
  disableGuardrails?: pulumi.Input<boolean>; // default false; warn when true
  policyDir?: pulumi.Input<string>;
  schemaFile?: pulumi.Input<string>;
};

export type AuthorizerWithPolicyStoreArgs = {
  cognito?: pulumi.Input<CognitoConfig>;
  description?: pulumi.Input<string>;
  dynamo?: pulumi.Input<DynamoConfig>;
  lambda?: pulumi.Input<LambdaConfig>;
  retainOnDelete?: pulumi.Input<boolean>;
  verifiedPermissions?: pulumi.Input<VerifiedPermissionsConfig>;
};

// Output group shapes
export type AuthorizerLambdaOutputs = {
  authorizerFunctionArn: pulumi.Output<string>;
  roleArn: pulumi.Output<string>;
};

export type AuthorizerDynamoOutputs = {
  authTableArn: pulumi.Output<string>;
  authTableStreamArn: pulumi.Output<string | undefined>;
};

export type AuthorizerCognitoOutputs = {
  userPoolArn: pulumi.Output<string | undefined>;
  userPoolClientIds: pulumi.Output<string[] | undefined>;
  userPoolId: pulumi.Output<string | undefined>;
};
