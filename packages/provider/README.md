# verified-permissions-authorizer (Go bridged provider)

This is a multi-language Pulumi Component Provider implemented in Go. It provisions:

- An AWS Verified Permissions Policy Store
- An AWS Lambda Request Authorizer whose code is bundled from the sibling TypeScript package at `packages/lambda-authorizer`
- Optionally, an AWS Cognito User Pool (domain and Identity Pool) and configures it as the Verified Permissions identity source

Interface (stable)
- Resource token: `verified-permissions-authorizer:index:AuthorizerWithPolicyStore`
- Inputs:
  - `description?`
  - `retainOnDelete?` (boolean, default `false`) — when `true`, resources are retained on delete and protected where supported (e.g., Cognito User Pool deletion protection). When `false`, resources are fully destroyable.
  - `lambda?` — settings for the bundled Lambda authorizer
    - `memorySize?` (MB; default `128`)
    - `reservedConcurrency?` (default `1`)
    - `provisionedConcurrency?` (units; default `0` to disable)
  - `dynamo?` — DynamoDB-related options for the provider-managed tenant table
    - `enableDynamoDbStream?` (boolean, default `false`)
  - `cognito?` — provision a Cognito User Pool and set it as the Verified Permissions identity source
    - `signInAliases?` — array of allowed values: `email`, `phone`, `preferredUsername` (default: `["email"]`). `username` is intentionally not supported.
    - `sesConfig?` — when provided, the User Pool sends email via Amazon SES (Cognito `DEVELOPER` mode) and the provider grants the User Pool permission to use the specified SES identity.
      - `sourceArn` (string, required) — SES identity ARN: `arn:aws:ses:<region>:<account-id>:identity/<email-or-domain>`
      - `from` (string, required) — From address
      - `replyToEmail` (string, optional)
      - `configurationSet` (string, optional)
- Outputs:
  - `policyStoreId`, `policyStoreArn`, `authorizerFunctionArn`, `roleArn`, `TenantTableArn`, `TenantTableStreamArn?`
  - When Cognito is provisioned: `userPoolId`, `userPoolArn`, `userPoolDomain`, `identityPoolId?`, `authRoleArn?`, `unauthRoleArn?`, `userPoolClientIds[]`, `parameters` (includes `USER_POOL_ID`)

Lambda contract (fixed)
- Runtime: `nodejs22.x` (not configurable)
- Handler: `index.handler`
- Environment: includes `POLICY_STORE_ID` and `JWT_SECRET` (used to verify incoming JWTs; default algorithms allowlist is `HS256`).

IAM permissions
- The Lambda execution role is granted `verifiedpermissions:GetPolicyStore` and `verifiedpermissions:IsAuthorized`, scoped to the created Policy Store ARN (no wildcard resource).

Tight coupling to the Lambda package
- The provider embeds the compiled authorizer (`packages/lambda-authorizer/dist/index.mjs`) at build time via `go:embed`.
- CI ensures that any change to the Lambda package triggers a provider release by bumping the Node SDK package and rebuilding the provider plugin with the new embedded artifact.

Schema
- The provider schema is `packages/provider/schema.json`.

Verified Permissions identity source
- When `cognito` is supplied, an `aws.verifiedpermissions.IdentitySource` is created pointing at the provisioned User Pool and its client IDs.
- When `cognito.sesConfig` is supplied, the User Pool `EmailConfiguration` is set to use SES with the provided values.

Email via Amazon SES
- Behavior when omitted: If `cognito.sesConfig` is not provided, the User Pool uses Cognito-managed default email sending (no SES configuration is applied).
- Behavior when provided: The provider sets `EmailConfiguration.EmailSendingAccount=DEVELOPER`, `SourceArn`, `From`, and optionally `ReplyToEmailAddress` and `ConfigurationSet`.
- IAM: The provider attaches a scoped SES identity resource policy (aws.sesv2.EmailIdentityPolicy) that authorizes the Amazon Cognito service principal to use the identity. The policy is restricted to the created User Pool ARN via the `aws:SourceArn` condition and to your account via `aws:SourceAccount`. When possible, the policy also restricts `ses:FromAddress` to the specified `from` address.

Validation rules
- `sourceArn` must be an SES identity ARN of the form `…:ses:<region>:<account>:identity/<email-or-domain>`.
- If the identity is a domain identity, `from` must be an email address within that domain (subdomains allowed).
- If the identity is an email identity, `from` must match that exact email address.
- Region constraints: the SES identity region must be compatible with the Cognito region. In general, identities in the same Region are always valid. For “backwards compatible” Regions, identities in `us-east-1`, `us-west-2`, or `eu-west-1` are also accepted. Certain Regions require an alternate SES Region (for example, `ap-east-1` must use `ap-southeast-1`). Attempts to use an invalid Region pairing are rejected with a clear error.

Note: This replaces the previously discussed `emailSendingAccount` input per PR #3 review. Existing users without `sesConfig` are unaffected.

Retention / deletion semantics
- Controlled by `retainOnDelete`: when `true`, resources use retain-on-delete and the User Pool has deletion protection enabled; when `false`, resources are fully destroyable and deletion protection is disabled.

Publishing
- The provider schema (`packages/provider/schema.json`) is published to the Pulumi Registry.
- Provider plugin binaries (`pulumi-resource-verified-permissions-authorizer`) are built for common platforms and uploaded to the corresponding GitHub Release tag.
- The Node SDK is published from `packages/sdk/nodejs` to npm as `pulumi-verified-permissions-authorizer`.

See the root README for release automation details.
