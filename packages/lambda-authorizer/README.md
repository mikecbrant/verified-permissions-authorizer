# verified-permissions-lambda-authorizer

TypeScript Lambda Authorizer packaged for AWS API Gateway that interacts with AWS Verified Permissions.

Build produces a single bundled CommonJS file suitable for deployment by the provider package or independently.

Exports a `handler` compatible with `APIGatewayRequestAuthorizer`.

Runtime
- Compatible with AWS Lambda `nodejs22.x` (the provider enforces this runtime). No runtime configuration knob is exposed.

Environment variables:

- `POLICY_STORE_ID` – the Verified Permissions Policy Store ID. When not set, the authorizer denies requests.

Bundling and releases
- This package is built by CI and embedded into the Go provider via `go:embed`. Provider releases are therefore triggered on any change to this package. Changesets automatically bumps the provider’s Node SDK when this package changes, which in turn triggers the provider plugin + Pulumi Registry release.
