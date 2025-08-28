# Verified Permissions Authorizer Monorepo

This repository is a pnpm + Changesets monorepo containing:

- `packages/provider`: a Pulumi component package that deploys an AWS Verified Permissions Policy Store and a companion AWS Lambda Authorizer function.
- `packages/lambda-authorizer`: an npm‑publishable TypeScript package that bundles a Lambda Authorizer handler designed to interact with the Policy Store.

Notes
- The Lambda runtime is fixed to `nodejs22.x` and is not configurable.
- AWS region/credentials are inherited from the standard Pulumi AWS provider.

See `packages/provider/README.md` and `packages/lambda-authorizer/README.md` for package‑specific details.
