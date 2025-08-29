# verified-permissions-authorizer (Go bridged provider)

This is a multi-language Pulumi Component Provider implemented in Go. It provisions:

- An AWS Verified Permissions Policy Store
- An AWS Lambda Request Authorizer whose code is bundled from the sibling TypeScript package at `packages/lambda-authorizer`

Interface (stable)
- Resource token: `verified-permissions-authorizer:index:AuthorizerWithPolicyStore`
- Inputs: `description?`, `validationMode?` ("STRICT" | "OFF"), `lambdaEnvironment?` (map<string,string>)
- Outputs: `policyStoreId`, `policyStoreArn`, `functionArn`, `roleArn`

Lambda contract (fixed)
- Runtime: `nodejs22.x` (not configurable)
- Handler: `index.handler`
- Environment: includes `POLICY_STORE_ID`

IAM permissions
- The Lambda execution role is granted `verifiedpermissions:GetPolicyStore` and `verifiedpermissions:IsAuthorized`.

Tight coupling to the Lambda package
- The provider embeds the compiled authorizer (`packages/lambda-authorizer/dist/index.mjs`) at build time via `go:embed`.
- CI ensures that any change to the Lambda package triggers a provider release by bumping the Node SDK package and rebuilding the provider plugin with the new embedded artifact.

Schema authoring
- Author the provider schema in YAML at `packages/provider/schema.yaml`.
- The JSON artifact `packages/provider/schema.json` is generated from the YAML for codegen/publishing.

Publishing
- The provider schema (`packages/provider/schema.json`) is published to the Pulumi Registry.
- Provider plugin binaries (`pulumi-resource-verified-permissions-authorizer`) are built for common platforms and uploaded to the corresponding GitHub Release tag.
- The Node SDK is published from `packages/sdk/nodejs` to npm as `pulumi-verified-permissions-authorizer`.

See the root README for release automation details.
