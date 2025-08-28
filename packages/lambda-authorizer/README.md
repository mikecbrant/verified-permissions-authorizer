# verified-permissions-lambda-authorizer

TypeScript Lambda Authorizer packaged for AWS API Gateway that interacts with AWS Verified Permissions.

Build produces a single bundled CommonJS file suitable for deployment by the provider package or independently.

Exports a `handler` compatible with `APIGatewayRequestAuthorizer`.

Runtime
- Compatible with AWS Lambda `nodejs22.x` (the provider enforces this runtime). No runtime configuration knob is exposed.

Environment variables:

- `POLICY_STORE_ID` – the Verified Permissions Policy Store ID. When not set, the authorizer denies requests.
