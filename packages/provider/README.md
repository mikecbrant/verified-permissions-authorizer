# verified-permissions-authorizer (Go bridged provider)

This is a multi-language Pulumi Component Provider implemented in Go. It provisions:

- An AWS Verified Permissions Policy Store
- An AWS Lambda Request Authorizer whose code is bundled from the sibling TypeScript package at `packages/lambda-authorizer`
- Optionally, an AWS Cognito User Pool and configures it as the Verified Permissions identity source

Interface (stable)
- Resource token: `verified-permissions-authorizer:index:AuthorizerWithPolicyStore`
- Inputs:
  - `description?`
  - `retainOnDelete?` (boolean, default `false`) — when `true`, resources are retained on delete and protected where supported (e.g., Cognito User Pool deletion protection). When `false`, resources are fully destroyable.
  - `lambda?` — settings for the bundled Lambda authorizer
    - `memorySize?` (MB; default `128`)
    - `reservedConcurrency?` (default `1`)
    - `provisionedConcurrency?` (units; default `0` to disable)
  - `dynamo?` — DynamoDB-related options for the provider-managed auth table
    - `enableDynamoDbStream?` (boolean, default `false`)
  - `cognito?` — provision a Cognito User Pool and set it as the Verified Permissions identity source
    - `signInAliases?` — array of allowed values: `email`, `phone`, `preferredUsername` (default: `["email"]`). `username` is intentionally not supported.
    - `sesConfig?` — when provided, the User Pool sends email via Amazon SES (Cognito `DEVELOPER` mode) and the provider grants the User Pool permission to use the specified SES identity.
      - `sourceArn` (string, required) — SES identity ARN: `arn:aws:ses:<region>:<account-id>:identity/<email-or-domain>`
      - `from` (string, required) — From address
      - `replyToEmail` (string, optional)
      - `configurationSet` (string, optional)
  - `avpAssets?` — ingest AVP schema and Cedar policies from your repo and validate them
    - `dir` (string, required) — directory containing a schema file and a `policies/` subfolder. Paths resolve relative to the Pulumi project root (where you run `pulumi up`).
    - `schemaFile?` (string) — optional file name relative to `dir`. Defaults to `schema.yaml`/`schema.yml`/`schema.json`.
    - `policiesGlob?` (string) — glob for policy files relative to `dir` (supports `**`). Default: `policies/**/*.cedar`.
    - `actionGroupEnforcement?` ("off" | "warn" | "error"; default `"warn"`) — validates that schema action names map to canonical action groups (`batchCreate`, `create`, `batchDelete`, `delete`, `find`, `get`, `batchUpdate`, `update`).
    - `requireGuardrails?` (boolean; default `true`) — fails if required guardrail deny policies are missing.
    - `postDeployCanary?` (boolean; default `false`) — when `true`, runs basic `IsAuthorized` checks from `canaryFile` after policies are created.
    - `canaryFile?` (string) — YAML file relative to `dir` describing canary checks. Default: `canaries.yaml`.
- Outputs:
  - Top-level:
    - `policyStoreId`, `policyStoreArn`, `parameters?`
  - Grouped (mirrors inputs):
    - `lambda`: `{ authorizerFunctionArn, roleArn }`
    - `dynamo`: `{ AuthTableArn, AuthTableStreamArn? }`
    - `cognito` (when configured): `{ userPoolId?, userPoolArn?, userPoolClientIds?[] }`

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

## AVP schema and policy assets

- Place your schema and policies under a single directory (`avpAssets.dir`).
- The provider accepts schema in Cedar JSON Schema format expressed as YAML (`schema.yaml`/`schema.yml`) or raw JSON (`schema.json`).
- AVP requires a single namespace per schema. This provider enforces that and fails when multiple namespaces are present.
- Required entities (must exist):
  - Principals: `Tenant`, `User`, `Group`, `Role`, `GlobalRole`, `TenantGrant`
  - Resources: `Event`, `Files`, `Grant`, `GlobalGrant`, `Ticket`
- Hierarchy expectations:
  - `Tenant` and `Group` should include themselves in `memberOfTypes` to enable nested principals.
  - `User` and `Role` have no hierarchy; a user can be in many roles and groups.
- Action-group convention:
  - Keep actions per-entity (for example, `createTicket`, `deleteTicket`, `getFile`).
  - The leading verb maps to a canonical action group: `batchCreate`, `create`, `batchDelete`, `delete`, `find`, `get`, `batchUpdate`, `update`.
  - Set `avpAssets.actionGroupEnforcement` to `"error"` to fail deploys when action names don’t follow this convention.

### Guardrail policies (required)

Place the following files under `policies/` (exact file names required when `requireGuardrails=true`):

- `01-deny-tenant-mismatch.cedar` — explicit deny when `principal.tenantId != resource.tenantId`.
- `02-deny-tenant-role-global-admin.cedar` — explicit deny when a tenant-scoped role attempts a globally-scoped admin action.

### Examples

An example set is included at `packages/provider/examples/avp`:

- Schema: `packages/provider/examples/avp/schema.yaml` (namespace key shows a suggested pattern using `{project}-{stack}` for uniqueness)
- Policies: `packages/provider/examples/avp/policies/*.cedar`
- Canaries: `packages/provider/examples/avp/canaries.yaml`

### Local validation (no AWS)

We ship a tiny validator script to catch common issues before `pulumi up`:

```
pnpm validate:avp --dir packages/provider/examples/avp
```

This checks:

- Single namespace and required entity presence
- Action group naming
- Required guardrail policy files present

### CI validation

This repo includes a GitHub Actions workflow `.github/workflows/avp-validate.yml` that runs the same local validation on every PR. You can extend it to also deploy an ephemeral stack and run canaries.

### Using from a Pulumi program

```ts
import * as pulumi from "@pulumi/pulumi";
import { AuthorizerWithPolicyStore } from "pulumi-verified-permissions-authorizer";

const authz = new AuthorizerWithPolicyStore("authz", {
  description: "Ticketing policy store",
  avpAssets: {
    dir: "./packages/provider/examples/avp",
    actionGroupEnforcement: "warn",
    requireGuardrails: true,
    // postDeployCanary: true,
  },
});

export const policyStoreId = authz.policyStoreId;
```

### Namespace uniqueness

We recommend incorporating `{project}-{stack}` in your schema’s namespace to avoid collisions across stacks in a Region. For example: `ticketing-{project}-{stack}`. The provider does not currently rewrite namespaces; it validates and applies the schema as provided.

