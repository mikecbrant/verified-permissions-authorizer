# Verified Permissions Authorizer Monorepo

This repository is a pnpm + Changesets monorepo containing:

- `packages/provider`: a Go, bridged Pulumi Component Provider that deploys an AWS Verified Permissions Policy Store and a bundled AWS Lambda Request Authorizer function
- `packages/lambda-authorizer`: an npm‑publishable TypeScript package that bundles a Lambda Authorizer handler designed to interact with the Policy Store
- `packages/sdk/nodejs`: the generated Node.js SDK published as `pulumi-verified-permissions-authorizer`

Notes
- The Lambda runtime is fixed to `nodejs22.x` and is not configurable.
- AWS region/credentials are inherited from the standard Pulumi AWS provider.
- The Lambda execution role includes `verifiedpermissions:GetPolicyStore` and `verifiedpermissions:IsAuthorized`.
- The provider is tightly coupled to the Lambda: changes to `packages/lambda-authorizer` cause a provider release.

See `packages/provider/README.md`, `packages/sdk/nodejs/`, and `packages/lambda-authorizer/README.md` for package‑specific details.

## CLI: avp-validate

The Node SDK ships a small CLI that validates AVP assets locally (no AWS calls):

```
npx avp-validate --schema ./infra/authorizer/schema.yaml --policyDir ./infra/authorizer/policies --mode error
# optional canaries
npx avp-validate --schema ./infra/authorizer/schema.yaml --policyDir ./infra/authorizer/policies --canary ./infra/authorize/canaries.yaml
```

- `--schema` (required): path to `schema.yaml`/`schema.yml` or `schema.json`
- `--policyDir` (required): directory containing `.cedar` files (recursively discovered)
- `--canary` (optional): YAML file with canary cases `{ principal, action, resource, expect }`
- `--mode` (optional): `off|warn|error` (default `error`)

Action group enforcement is exact and case-sensitive against the canonical groups (including `Global*` variants).

## Deployment considerations and ephemeral environments

- If you plan to deploy this provider and/or spin up short‑lived ephemeral stacks, see [docs/vp-14-ephemeral-vp-stacks-plan.md](docs/vp-14-ephemeral-vp-stacks-plan.md) for a more detailed exploration of possible options.

## Cognito + SES

When you opt into Cognito by supplying the top-level `cognito` input on the provider, you can also configure SES-backed email sending via `cognito.sesConfig`. See `packages/provider/README.md` for the full field list and validation rules.
