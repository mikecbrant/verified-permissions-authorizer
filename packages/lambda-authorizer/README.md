# verified-permissions-lambda-authorizer

TypeScript Lambda Authorizer for AWS Verified Permissions. It is not limited to API Gateway; it can be used wherever a Lambda authorizer is supported, typically with API Gateway and AWS AppSync.

Build and module format
- Produces a single bundled ECMA Module file with a `.mjs` extension. CommonJS is not used in this repository.
- Exports a `handler` compatible with API Gateway Request Authorizer and an AWS AppSync Lambda Authorizer.

Runtime
- Compatible with AWS Lambda `nodejs22.x` (the provider enforces this runtime). No runtime configuration is exposed.

Environment variables
- `POLICY_STORE_ID` – the Verified Permissions Policy Store ID. When not set, the authorizer denies requests.

Bundling and releases
- This package is built by CI and embedded into the Go provider via `go:embed`. Provider releases are triggered when this package changes. Changesets also bumps the provider’s Node SDK when this package changes, which in turn triggers the provider plugin + Pulumi Registry release.
