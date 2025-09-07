// This is an empty Pulumi program in TypeScript.
// You can add AWS resources here as needed.

import * as pulumi from "@pulumi/pulumi";
import { AuthorizerWithPolicyStore } from "pulumi-verified-permissions-authorizer";

// Minimal example stack wiring the provider with example assets for validation/testing.
// You can replace the asset directory with your own repository path.
const authz = new AuthorizerWithPolicyStore("authz", {
  description: "Example AVP stack",
  verifiedPermissions: {
    schemaFile: "../packages/provider/examples/avp/schema.yaml",
    policyDir: "../packages/provider/examples/avp/policies",
    actionGroupEnforcement: "error",
    // canaryFile: "../packages/provider/examples/avp/canaries.yaml",
  },
});

export const policyStoreId = authz.policyStoreId;
