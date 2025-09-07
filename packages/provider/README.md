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
  - `verifiedPermissions?` — ingest AVP schema and Cedar policies and validate them
    - `schemaFile` (string, required) — path to schema file (`.yaml`/`.yml` or `.json`). YAML is always converted to canonical JSON before validation and upload.
    - `policyDir` (string, required) — directory containing `.cedar` policy files (recursively discovered).
    - `actionGroupEnforcement?` ("off" | "warn" | "error"; default `"error"`) — enforces canonical PascalCase action groups (Create/Delete/Find/Get/Update and Batch* variants) and their Global* equivalents.
    - `disableGuardrails?` (boolean; default `false`) — when `true`, the provider will not install deny guardrail policies. A warning is emitted as this posture is not recommended.
    - `canaryFile?` (string) — optional YAML file with canary authorization cases to execute post-deploy.
- Outputs:
  - Top-level:
    - `policyStoreId`, `policyStoreArn`, `parameters?`
  - Grouped (mirrors inputs):
    - `lambda`: `{ authorizerFunctionArn, roleArn }`
    - `dynamo`: `{ authTableArn, authTableStreamArn? }`
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

- AVP requires a single namespace per schema; this provider enforces that and fails when multiple namespaces are present.
- Required principals: `Tenant`, `User`, `Role`, `GlobalRole`, `TenantGrant`. Example resources (e.g., `Ticket`, `File`) are intentionally not enforced at the provider level.
- Hierarchy expectations:
  - `Tenant` must be a homogeneous tree (its `memberOfTypes` should include only `Tenant`).
  - `User` and `Role` have no hierarchy; a user can be in many roles.
- Action-group convention:
  - Define actions per-entity (for example, `createTicket`, `deleteTicket`, `getFile`). The leading verb maps to a canonical PascalCase action group: `Create`, `Delete`, `Find`, `Get`, `Update` and their `Batch*` variants.
  - A globally-scoped set exists with the `Global*` prefix (for cross-tenant access): `GlobalCreate`, `GlobalDelete`, `GlobalFind`, `GlobalGet`, `GlobalUpdate` (plus `GlobalBatch*` variants).
  - Enforcement occurs when `actionGroupEnforcement` is enabled (default is `error`).

- Guardrails: When guardrails are enabled (default), the provider installs a consolidated deny policy that:
  - Denies `Global*` actions when the principal has a `tenantId`.
  - Denies tenant-scoped actions on resources missing `tenantId`.
  - Denies actions that are not in the approved action-group set.

> Cedar patterns primer: see https://docs.cedarpolicy.com/overview/patterns.html

### Examples

An example set is included at `packages/provider/examples/avp`:

- Schema: `packages/provider/examples/avp/schema.yaml` (namespace key shows a suggested pattern using `{project}-{stack}` for uniqueness)
- Policies: `packages/provider/examples/avp/policies/*.cedar`
- Canaries: `packages/provider/examples/avp/canaries.yaml`

### Local validation (no AWS)

The SDK ships a CLI validator `avp-validate` that mirrors the provider’s validations (schema checks, action-group enforcement, policy syntax scan, and canary structure):

```
npx avp-validate --schema ./packages/provider/examples/avp/schema.yaml --policyDir ./packages/provider/examples/avp/policies --mode error
```

### CI validation

This repo includes a GitHub Actions workflow `.github/workflows/avp-validate.yml` that runs the same local validation on every PR. You can extend it to also deploy an ephemeral stack and run canaries.

### Using from a Pulumi program

```ts
import * as pulumi from "@pulumi/pulumi";
import { AuthorizerWithPolicyStore } from "pulumi-verified-permissions-authorizer";

const authz = new AuthorizerWithPolicyStore("authz", {
  description: "Ticketing policy store",
  verifiedPermissions: {
    schemaFile: "./packages/provider/examples/avp/schema.yaml",
    policyDir: "./packages/provider/examples/avp/policies",
    actionGroupEnforcement: "error",
    // canaryFile: "./packages/provider/examples/avp/canaries.yaml",
  },
});

export const policyStoreId = authz.policyStoreId;
```

### Namespace uniqueness

We recommend incorporating `{project}-{stack}` in your schema’s namespace to avoid collisions across stacks in a Region. For example: `ticketing-{project}-{stack}`. The provider does not currently rewrite namespaces; it validates and applies the schema as provided.

