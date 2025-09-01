# verified-permissions-authorizer (Go bridged provider)

This is a multi-language Pulumi Component Provider implemented in Go. It provisions:

- An AWS Verified Permissions Policy Store
- An AWS Lambda Request Authorizer whose code is bundled from the sibling TypeScript package at `packages/lambda-authorizer`
- Optionally, an AWS Cognito User Pool (plus triggers, domain, and Identity Pool) and configures it as the Verified Permissions identity source

Interface (stable)
- Resource token: `verified-permissions-authorizer:index:AuthorizerWithPolicyStore`
- Inputs:
  - `description?`
  - `lambdaEnvironment?` (map<string,string>)
  - `enableDynamoDbStream?` (boolean, default `false`)
  - `isEphemeral?` (boolean, default `false`) — when `false`, the DynamoDB table is retained-on-delete and Cognito deletion protection is enabled
  - `cognito?` — provision a Cognito User Pool and set it as the Verified Permissions identity source
    - `identityPoolFederation?` (boolean) — create a Cognito Identity Pool + default roles
    - `signInAliases?` (username, email, phone, preferredUsername)
    - `emailSendingAccount?` ("COGNITO_DEFAULT" | "DEVELOPER")
    - `mfa?` ("OFF" | "ON" | "OPTIONAL"), `mfaMessage?`
    - `accountRecovery?`, `autoVerify?` (email, phone)
    - `advancedSecurityMode?` ("OFF" | "AUDIT" | "ENFORCED")
    - `userInvitation?` and `userVerification?` templates
    - `customAttributes?` (booleans to include: globalRoles, tenantId, tenantName, userId)
    - `domain?` ({ domainName, certificateArn }) — when `isEphemeral` is false a custom domain is created using the certificate; when true, a hosted domain with a `"<stack>-<name>-tenant"` prefix is created
    - `triggers?` (map<string, { enabled?, environment?, permissions[]? }>) — optional Cognito lifecycle Lambda triggers; each trigger deploys a minimal Node.js function by default
    - `clients?` — names of User Pool clients to create (if omitted, a single `default` client is created)
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
- It is maintained alongside the Go provider; no YAML conversion step is required.

Verified Permissions identity source
- When `cognito` is supplied, an `aws.verifiedpermissions.IdentitySource` is created pointing at the provisioned User Pool and its client IDs.

Retention / deletion semantics
- Controlled by `isEphemeral` (mirrors VP-6): if `false`, resources use retain-on-delete and the User Pool has deletion protection enabled; if `true`, resources are fully destroyable and deletion protection is disabled.

Publishing
- The provider schema (`packages/provider/schema.json`) is published to the Pulumi Registry.
- Provider plugin binaries (`pulumi-resource-verified-permissions-authorizer`) are built for common platforms and uploaded to the corresponding GitHub Release tag.
- The Node SDK is published from `packages/sdk/nodejs` to npm as `pulumi-verified-permissions-authorizer`.

See the root README for release automation details.
