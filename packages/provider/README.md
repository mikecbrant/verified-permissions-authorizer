# verified-permissions-authorizer (Go bridged provider)

This is a multi-language Pulumi Component Provider implemented in Go. It provisions:

- An AWS Verified Permissions Policy Store
- An AWS Lambda Request Authorizer whose code is bundled from the sibling TypeScript package at `packages/lambda-authorizer`
- Optionally, an AWS Cognito User Pool (domain and Identity Pool) and configures it as the Verified Permissions identity source

Interface (stable)
- Resource token: `verified-permissions-authorizer:index:AuthorizerWithPolicyStore`
- Inputs:
  - `description?`
  - `isEphemeral?` (boolean, default `false`) — when `false`, the DynamoDB table is retained-on-delete and Cognito deletion protection is enabled
  - `lambda?` — settings for the bundled Lambda authorizer
    - `memorySize?` (MB; default `128`)
    - `reservedConcurrency?` (default `1`)
    - `provisionedConcurrency?` (units; default `0` to disable)
  - `dynamo?` — DynamoDB-related options for the provider-managed tenant table
    - `enableDynamoDbStream?` (boolean, default `false`)
  - `cognito?` — provision a Cognito User Pool and set it as the Verified Permissions identity source
    - `signInAliases?` — array of allowed values: `email`, `phone`, `preferredUsername` (default: `["email"]`). `username` is intentionally not supported.
- Outputs:
  - `policyStoreId`, `policyStoreArn`, `authorizerFunctionArn`, `roleArn`, `TenantTableArn`, `TenantTableStreamArn?`
  - When Cognito is provisioned: `userPoolId`, `userPoolArn`, `userPoolDomain`, `identityPoolId?`, `authRoleArn?`, `unauthRoleArn?`, `userPoolClientIds[]`, `parameters` (includes `USER_POOL_ID`)

Lambda contract (fixed)
- Runtime: `nodejs22.x` (not configurable)
- Handler: `index.handler`
- Environment: includes `POLICY_STORE_ID` and `JWT_SECRET` (used to verify incoming JWTs; default algorithms allowlist is `HS256`).

IAM permissions
- The Lambda execution role is granted `verifiedpermissions:GetPolicyStore` and `verifiedpermissions:IsAuthorized`.

Tight coupling to the Lambda package
- The provider embeds the compiled authorizer (`packages/lambda-authorizer/dist/index.mjs`) at build time via `go:embed`.
- CI ensures that any change to the Lambda package triggers a provider release by bumping the Node SDK package and rebuilding the provider plugin with the new embedded artifact.

Schema
- The provider schema is `packages/provider/schema.json`.

Verified Permissions identity source
- When `cognito` is supplied, an `aws.verifiedpermissions.IdentitySource` is created pointing at the provisioned User Pool and its client IDs.

Retention / deletion semantics
- Controlled by `isEphemeral`: if `false`, resources use retain-on-delete and the User Pool has deletion protection enabled; if `true`, resources are fully destroyable and deletion protection is disabled.

Publishing
- The provider schema (`packages/provider/schema.json`) is published to the Pulumi Registry.
- Provider plugin binaries (`pulumi-resource-verified-permissions-authorizer`) are built for common platforms and uploaded to the corresponding GitHub Release tag.
- The Node SDK is published from `packages/sdk/nodejs` to npm as `pulumi-verified-permissions-authorizer`.

See the root README for release automation details.
