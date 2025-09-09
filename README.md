# Verified Permissions Authorizer (Pulumi provider)

Policy-based, multi-tenant authorization for small teams—powered by AWS Verified Permissions (Cedar) and Lambda

- Provision an AWS Verified Permissions policy store and a reusable Lambda authorizer compatible with both API Gateway and AppSync.
- Author your Cedar schema and policies as declarative files in your repo; the stack loads and validates them.
- Adopt a multi-tenant-first model with clear separation between global (admin) and tenant-scoped actions.

Read the ADR for the full technical approach, assumptions, and integration guidance: [docs/adr-0001-authorization-approach.md](docs/adr-0001-authorization-approach.md).

## What you get

- A bridged Pulumi Component Provider (Go) that creates a Verified Permissions policy store and deploys a reusable Lambda authorizer.
- Optional Cognito user pool configured as the Verified Permissions identity source.


## Notes
- The Lambda runtime is fixed to `nodejs22.x`. Ensure your target AWS regions support `nodejs22.x` (see the [AWS Lambda runtime support matrix](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtimes.html)).
- AWS Verified Permissions is not available in all Regions; ensure your target Region supports it before deploying. See the [AWS Regional Services list](https://aws.amazon.com/about-aws/global-infrastructure/regional-product-services/).
- AWS region/credentials are inherited from the standard Pulumi AWS provider.
- The provider is tightly coupled to the Lambda: changes to `packages/lambda-authorizer` cause a provider release.

See `packages/provider/README.md`, `packages/lambda-authorizer/README.md`, and `packages/sdk/nodejs/` for package‑specific details.

### Compatibility
- API Gateway (REST): Request authorizer mode only.
- AppSync (GraphQL): Lambda authorizer supported.
- API Gateway (HTTP APIs): not supported yet.

## Monorepo packages

- `packages/provider`: the Go, bridged Pulumi Component Provider.
- `packages/lambda-authorizer`: the TypeScript Lambda authorizer implementation used by the provider.
- `packages/sdk/nodejs`: the generated Node.js SDK published as `pulumi-verified-permissions-authorizer`.

For local pre‑PR checks (Go build/vet/test and workspace lint/type/tests), see [.charlie/preflight.md](.charlie/preflight.md).

## CLI: avp-validate

The Node SDK ships a small CLI that validates AVP assets locally (no AWS calls):

```
npx avp-validate --schema ./infra/authorizer/schema.yaml --policyDir ./infra/authorizer/policies --mode error
# optional canaries
npx avp-validate --schema ./infra/authorizer/schema.yaml --policyDir ./infra/authorizer/policies --canary ./infra/authorizer/canaries.yaml
```

- `--schema` (required): path to `schema.yaml`/`schema.yml` or `schema.json`
- `--policyDir` (required): directory containing `.cedar` files (recursively discovered)
- `--canary` (optional): YAML file with canary cases `{ principal, action, resource, expect }`
- `--mode` (optional): `off|warn|error` (default `error`)

Action group enforcement is exact and case-sensitive against the canonical groups (including `Global*` variants).

## Deployment considerations and ephemeral environments

- If you plan to deploy this provider and/or spin up short-lived ephemeral stacks, see [docs/vp-14-ephemeral-vp-stacks-plan.md](docs/vp-14-ephemeral-vp-stacks-plan.md).

## Additional compatibility note

- Cognito + SES email is supported when `cognito.sesConfig` is provided via the provider inputs (see `packages/provider/README.md`).
