# Terraform provider specification and Pulumi parity matrix (VP-19)

Status: Accepted

Date: 2025-09-10

This document defines the Terraform provider surface and feature set required to match the existing Pulumi provider in this repository. It intentionally defers final implementation choices (SDK/Framework, test scope) to the ADR and review process.

## Scope and goals

- Provide a single high‑level Terraform resource that provisions:
  - An AWS Verified Permissions Policy Store (STRICT validation)
  - A bundled AWS Lambda Request Authorizer (runtime: `nodejs22.x`)
  - A provider‑managed DynamoDB table for auth/identity/roles data
  - Optional Cognito User Pool and Verified Permissions Identity Source
  - AVP schema/policy ingestion with validation and optional canaries
- Match configuration inputs and outputs 1:1 with the Pulumi provider where feasible.
- Reuse common Go provider logic across the Pulumi and Terraform providers to the greatest extent possible to ensure behavior parity.

Non‑goals
- Expose raw underlying AWS resources directly as separate Terraform resources (the AWS provider already covers those). This provider focuses on a cohesive authorizer component resource that implements opinionated wiring and AVP ingest/validation.

## Provider name and resource shape

- Provider source: `mikecbrant/vpauthorizer`.
- Primary resource type: `vpauthorizer_authorizer`.
- Data sources: none required for parity.

## Transparency and exports

To aid composability, the provider must be transparent about what it creates and expose key identifiers/ARNs as outputs from the high‑level resource. This does not contradict the non‑goal of exposing each raw AWS resource as its own Terraform resource; these are computed attributes on the component resource that enable downstream wiring.

Exports include (non‑exhaustive, aligned with Pulumi outputs):
- Verified Permissions: `policy_store_id`, `policy_store_arn`.
- Lambda authorizer: `authorizer_function_arn`, `role_arn`.
- DynamoDB: `auth_table_arn`, and `auth_table_stream_arn` when streams are enabled.
- Cognito (when enabled): `user_pool_id`, `user_pool_arn`, `user_pool_client_ids`.
- Verified Permissions identity source (when created): identity source id and ARN.
- Additional computed values as needed by consumers (e.g., a `parameters` map mirroring Pulumi) without leaking implementation‑private details.

## Configuration schema (inputs) — 1:1 with Pulumi

The Terraform resource inputs mirror the Pulumi provider’s schema definitions in this repository (see the Pulumi provider under the monorepo providers directory).

Top‑level
- `description` (string, optional)
- `retain_on_delete` (bool, optional; default `false`)
- `lambda` (block, optional)
  - `memory_size` (number, optional; default `128` MB)
  - `reserved_concurrency` (number, optional; default `1`)
  - `provisioned_concurrency` (number, optional; default `0`; validation: when > 0, must be <= `reserved_concurrency`)
- `dynamo` (block, optional)
  - `enable_dynamo_db_stream` (bool, optional; default `false`)
- `cognito` (block, optional)
  - `sign_in_aliases` (list(string), optional; allowed: `email`, `phone`, `preferredUsername`; default: `["email"]`)
  - `ses_config` (block, optional)
    - `source_arn` (string, required) — SES identity ARN `arn:aws:ses:<region>:<account-id>:identity/<email-or-domain>`
    - `from` (string, required) — From address
    - `reply_to_email` (string, optional)
    - `configuration_set` (string, optional)
- `verified_permissions` (block, optional)
  - `schema_file` (string, optional; default `./authorizer/schema.yaml`)
  - `policy_dir` (string, optional; default `./authorizer/policies`)
  - `action_group_enforcement` (string, optional; `off|warn|error`; default `error`)
  - `disable_guardrails` (bool, optional; default `false`)
  - `canary_file` (string, optional; default `./authorizer/canaries.yaml` when file exists)

Validation rules (should match Pulumi provider behavior where possible)
- Verified Permissions schema file must be YAML/JSON; exactly one namespace; required principals: `Tenant`, `User`, `Role`, `GlobalRole`, `TenantGrant`.
- Namespace naming: hard error if the namespace does not meet Verified Permissions namespacing requirements.
- Action group enforcement uses exact, case‑sensitive prefixes against the canonical set: `Create|Delete|Find|Get|Update|Batch*` and their `Global*` equivalents. Modes: `off|warn|error`.
- Schema JSON size limit: error > 100,000 bytes; warn at ≥ 95% of limit.
- `provisioned_concurrency` must be `<= reserved_concurrency` when set.
- Cognito SES validation:
  - `source_arn` must be an SES identity ARN with `identity/<email-or-domain>`.
  - If identity is a domain, `from` must be an email within that domain; if identity is an email, `from` must match exactly.
  - Identity region must be compatible with the Cognito region (same region or the documented backwards‑compatible set); partitions must match.

## Configuration interface
- Configuration is supported only via Terraform resource arguments. No provider‑specific environment variable configuration is supported. Standard AWS SDK credential/region discovery remains unchanged and outside this provider.

## Outputs — 1:1 with Pulumi

Top‑level
- `policy_store_id` (string)
- `policy_store_arn` (string)
- `parameters` (map(string)) — e.g., includes `USER_POOL_ID` when Cognito is provisioned

Grouped
- `lambda` — `{ authorizer_function_arn, role_arn }`
- `dynamo` — `{ auth_table_arn, auth_table_stream_arn? }`
- `cognito` — `{ user_pool_id?, user_pool_arn?, user_pool_client_ids?[] }`

## Feature parity matrix

Pulumi capability → Terraform resource behavior

- Verified Permissions
  - Create Policy Store with STRICT validation → same
  - Apply schema only when changed → same (diff against `GetSchema`, then `PutSchema`)
  - Ingest static `.cedar` policies under `policy_dir` (deterministic order) → same (create/update/delete policies to match on apply)
  - Provider‑managed guardrail deny policies (toggle via `disable_guardrails`) → same
  - Canary checks (provider base + consumer file) after apply; fail on mismatch → same (run during Create/Update; no‑op during Read)
- DynamoDB
  - Single‑table with PK/SK and two GSIs; PAY_PER_REQUEST; optional streams → same
  - When `retain_on_delete` true: deletion protection + PITR → same
- IAM
  - Lambda role; AWSLambdaBasicExecutionRole; AVP GetPolicyStore/IsAuthorized scoped to policy store; DynamoDB read‑only → same
- Lambda
  - Bundled authorizer code (embedded) with env `POLICY_STORE_ID`; runtime `nodejs22.x`, handler `index.handler`, arch `arm64`; memory/RC/PC knobs → same
  - CloudWatch log group with 14‑day retention → same
- Cognito (optional)
  - User Pool + client(s); deletion protection toggled by `retain_on_delete` → same
  - SES email configuration + SES identity policy with `aws:SourceArn`, `aws:SourceAccount`, optional `ses:FromAddress` condition → same
  - AVP Identity Source referencing the user pool and client IDs → same

Known deviations/notes
- The Terraform provider will directly call AWS APIs (Go SDK v2) to manage AWS resources within this single high‑level resource. Consumers should not manage the same underlying resources with the AWS provider to avoid drift. This mirrors the Pulumi component’s single‑owner model in spirit, but via Terraform’s provider resource model.
- We will not expose additional knobs beyond the Pulumi surface in the initial release.

## Intentional differences vs Pulumi

- No environment variable fallback for Terraform resource arguments. Rationale: follow Terraform idioms and keep configuration declarative in HCL; AWS SDK environment discovery remains unchanged.
- Namespace validation is a hard error when not compliant with Verified Permissions naming requirements (Pulumi may warn today). Rationale: fail early to prevent misconfigured policy stores.

## Documentation requirements (to publish)
- Provider overview page: purpose, architecture diagram/flow, prerequisites.
- Resource docs for `vpauthorizer_authorizer`: full argument and attribute reference; clear warnings about single‑owner model.
- Examples: minimal, Cognito+SES example, and schema/policy ingestion with canaries.

## Test strategy (scope — decisions captured in ADR)
- Unit tests for validation logic (schema parsing, action‑group enforcement, SES config checks).
- Acceptance tests gated behind `TF_ACC` using a real AWS account (opt‑in for CI).
- Contract tests for idempotency: apply with no changes is a no‑op; policy drift detection works.

Open questions (tied to ADR)
1. Final provider name and namespace (affects examples and docs).
2. Terraform Plugin Framework vs legacy SDK v2. Default proposal: Plugin Framework (protocol v6).
3. Minimum supported Terraform CLI version (to be documented; proposal: 1.6+).
4. Packaging location: separate repo vs this monorepo (and how we share the built Lambda bundle during provider build).

## References to Pulumi provider (parity source of truth)
- Inputs/outputs/types: `internal/pulumi/provider.go`
- AVP ingestion & validations: `internal/pulumi/schema.go`
- Guardrails & canary assets: `internal/pulumi/policies.go` (guardrails) and `internal/common/canaries.go`
- SES validation: `internal/pulumi/ses_helpers.go`

