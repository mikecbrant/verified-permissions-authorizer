// This is an empty Pulumi program in TypeScript.
// You can add AWS resources here as needed.

import * as pulumi from "@pulumi/pulumi";
import { AuthorizerWithPolicyStore } from "pulumi-verified-permissions-authorizer";

// Minimal example stack wiring the provider with example assets for validation/testing.
// You can replace the asset directory with your own repository path.
const authz = new AuthorizerWithPolicyStore("authz", {
  description: "Example AVP stack",
  verifiedPermissions: {
    // Using provider defaults: ./authorizer/schema.yaml and ./authorizer/policies
    // actionGroupEnforcement defaults to 'error'
    // Uncomment to run canaries as part of deploy (defaults to ./authorize/canaries.yaml when present)
    // canaryFile: "./authorizer/canaries.yaml",
  },
});

export const policyStoreId = authz.policyStoreId;
