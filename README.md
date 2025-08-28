# Verified Permissions Authorizer Monorepo

This repository is a pnpm + Changesets monorepo containing:

- `packages/provider`: a Pulumi TypeScript component provider that deploys an AWS Verified Permissions Policy Store and a companion AWS Lambda Authorizer function.
- `packages/lambda-authorizer`: an npm‑publishable TypeScript package that bundles a Lambda Authorizer handler designed to interact with the Policy Store.

The provider package is structured to be Pulumi Registry–ready (schema, metadata, examples), but automation for publishing to the Registry is intentionally out of scope for this initial commit.

See `packages/provider/README.md` and `packages/lambda-authorizer/README.md` for package‑specific details.
