# verified-permissions-authorizer (Go bridged provider)

This is a multi-language Pulumi Component Provider implemented in Go. It provisions:

- An AWS Verified Permissions Policy Store
- An AWS Lambda Request Authorizer whose code is bundled from the sibling TypeScript package at `packages/lambda-authorizer`

Interface (stable)
- Resource token: `verified-permissions-authorizer:index:AuthorizerWithPolicyStore`
- Inputs: `description?`, `lambdaEnvironment?` (map<string,string>), `enableDynamoDbStream?` (boolean, default `false`), `isEphemeral?` (boolean, default `false`)
- Outputs: `policyStoreId`, `policyStoreArn`, `authorizerFunctionArn`, `roleArn`, `TenantTableArn`, `TenantTableStreamArn?`

## DynamoDB tenant table
- Keys/attributes: `PK` (hash), `SK` (range), `GSI1PK` (hash), `GSI1SK` (range), `GSI2PK` (hash), `GSI2SK` (range)
- GSIs: `GSI1` and `GSI2` (ProjectionType `ALL`)
- Billing mode: `PAY_PER_REQUEST`
- Stream behavior (controlled by `enableDynamoDbStream`):
  - `true`: stream enabled with `NEW_AND_OLD_IMAGES`; `TenantTableStreamArn` is set
  - `false`: stream disabled; `TenantTableStreamArn` is not set
- Retention semantics (controlled by `isEphemeral`):
  - Non‑ephemeral (`false` or unset): table is retained on stack deletion
  - Ephemeral (`true`): table is not retained on deletion

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
- It is maintained alongside the Go provider; no YAML conversion step is required.

Publishing
- The provider schema (`packages/provider/schema.json`) is published to the Pulumi Registry.
- Provider plugin binaries (`pulumi-resource-verified-permissions-authorizer`) are built for common platforms and uploaded to the corresponding GitHub Release tag.
- The Node SDK is published from `packages/sdk/nodejs` to npm as `pulumi-verified-permissions-authorizer`.

See the root README for release automation details.
