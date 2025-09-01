import { describe, expect, it } from "vitest";

import {
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
