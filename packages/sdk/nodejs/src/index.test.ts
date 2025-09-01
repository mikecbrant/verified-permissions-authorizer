import { describe, expect, it } from "vitest";

import {
  type AuthorizerCognitoOutputs,
  type AuthorizerDynamoOutputs,
  type AuthorizerLambdaOutputs,
  AuthorizerWithPolicyStore,
  type AuthorizerWithPolicyStoreArgs,
} from "./index.js";

describe("SDK exports", () => {
  it("exports AuthorizerWithPolicyStore", () => {
    expect(typeof AuthorizerWithPolicyStore).toBe("function");
  });
});

describe("SDK args typing", () => {
  it("accepts nested cognito.sesConfig", () => {
    const args: AuthorizerWithPolicyStoreArgs = {
      cognito: {
        sesConfig: {
          sourceArn: "arn:aws:ses:us-east-1:123456789012:identity/example.com",
          from: "no-reply@example.com",
          replyToEmail: "support@example.com",
          configurationSet: "prod",
        },
      },
    };
    expect(!!args).toBe(true);
  });
});

describe("SDK grouped outputs (types)", () => {
  it("exposes grouped output types", () => {
    // Type-only checks; these do not instantiate a Pulumi resource
    const _cog: AuthorizerCognitoOutputs | undefined = undefined;
    const _ddb: AuthorizerDynamoOutputs | undefined = undefined;
    const _lam: AuthorizerLambdaOutputs | undefined = undefined;
    expect([_cog, _ddb, _lam]).toBeTruthy();
  });
});
