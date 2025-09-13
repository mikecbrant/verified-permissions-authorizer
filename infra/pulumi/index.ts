// Example Pulumi program that provisions the AVP policy store + Lambda authorizer
import * as pulumi from "@pulumi/pulumi"
import { AuthorizerWithPolicyStore } from "pulumi-verified-permissions-authorizer"

const authz = new AuthorizerWithPolicyStore("authz", {
  description: "Example AVP stack",
  verifiedPermissions: {
    // Defaults search for assets under ../authorizer
    canaryFile: "../authorizer/canaries.yaml",
  },
})

export const policyStoreId = authz.policyStoreId
