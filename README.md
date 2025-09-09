# Verified Permissions Authorizer (Pulumi provider)

Policy-based, multi-tenant authorization for small teams—powered by AWS Verified Permissions (Cedar) and delivered as a low-cost Lambda authorizer that works with API Gateway and AppSync.

- Provision an AWS Verified Permissions policy store and a reusable Lambda authorizer (API Gateway REST: Lambda Request Authorizer; AppSync: Lambda authorizer).
- Author your Cedar schema and policies as code in your repo; the stack loads and validates them.
- Adopt a multi-tenant-first model with clear separation between global (admin) and tenant-scoped actions.

Read the ADR for the full technical approach, assumptions, and integration guidance: [docs/adr-0001-authorization-approach.md](docs/adr-0001-authorization-approach.md).

## What you get

- A bridged Pulumi Component Provider (Go) that creates a Verified Permissions policy store and deploys a bundled Lambda authorizer with least-privilege IAM: `AWSLambdaBasicExecutionRole` for logs, plus `verifiedpermissions:GetPolicyStore` and `verifiedpermissions:IsAuthorized` scoped to the created policy store.
- Optional Cognito user pool integration as the Verified Permissions identity source. When enabled and `cognito.sesConfig` is provided, SES is used by Cognito for email delivery (e.g., verification and password reset). Ensure you have a verified SES identity (domain or email) in the sending Region, that SES is out of the sandbox or you are sending to verified recipients, and that the SES Region configured for your user pool is compatible with Cognito. See configuring user‑pool email in the [Cognito developer guide](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-email.html).


## Notes
- The Lambda runtime is fixed to `nodejs22.x` and is not configurable. Ensure your target AWS regions support `nodejs22.x` (see the [AWS Lambda runtime support matrix](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtimes.html)).
- AWS Verified Permissions is not available in all Regions; ensure your target Region supports it before deploying. See the [AWS Regional Services list](https://aws.amazon.com/about-aws/global-infrastructure/regional-product-services/).
- AWS region/credentials are inherited from the standard Pulumi AWS provider.
- The provider is tightly coupled to the Lambda: changes to `packages/lambda-authorizer` cause a provider release.

### Compatibility
- Amazon API Gateway REST APIs: [Lambda Request Authorizer](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-use-lambda-authorizer.html) — supported.
- Amazon API Gateway REST APIs: Lambda TOKEN Authorizer — not supported.
- AWS AppSync GraphQL APIs: [Lambda authorizer](https://docs.aws.amazon.com/appsync/latest/devguide/security-authz.html#aws-lambda-authorization) — supported.
- Amazon API Gateway HTTP APIs: not supported yet (event/handler shape differs).

## Monorepo packages

- `packages/provider`: the Go, bridged Pulumi Component Provider.
- `packages/lambda-authorizer`: the TypeScript Lambda authorizer implementation used by the provider.
- `packages/sdk/nodejs`: the generated Node.js SDK published as `pulumi-verified-permissions-authorizer`.

See `packages/provider/README.md`, `packages/lambda-authorizer/README.md`, and `packages/sdk/nodejs/` for package-specific details.

## Deployment considerations and ephemeral environments

- If you plan to deploy this provider and/or spin up short-lived ephemeral stacks, see [docs/vp-14-ephemeral-vp-stacks-plan.md](docs/vp-14-ephemeral-vp-stacks-plan.md).

## Cognito + SES

When you opt into Cognito by supplying the top-level `cognito` input on the provider, you can also configure SES-backed email sending via `cognito.sesConfig`. See `packages/provider/README.md` for the full field list and validation rules.
