# Verified Permissions Authorizer Monorepo

This repository is a pnpm + Changesets monorepo containing:

- `provider`: a Go, bridged Pulumi Component Provider that deploys an AWS Verified Permissions Policy Store and a bundled AWS Lambda Request Authorizer function
- `packages/lambda-authorizer`: an npm‑publishable TypeScript package that bundles a Lambda Authorizer handler designed to interact with the Policy Store
- `sdk/nodejs`: the generated Node.js SDK published as `pulumi-verified-permissions-authorizer`

Notes
- The Lambda runtime is fixed to `nodejs22.x` and is not configurable.
- AWS region/credentials are inherited from the standard Pulumi AWS provider.
- The Lambda execution role includes `verifiedpermissions:GetPolicyStore` and `verifiedpermissions:IsAuthorized`.
- The provider is tightly coupled to the Lambda: changes to `packages/lambda-authorizer` cause a provider release.

See `provider/README.md`, `sdk/nodejs/`, and `packages/lambda-authorizer/README.md` for package‑specific details.
