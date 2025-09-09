# ADR 0001: Authorization approach for the Verified Permissions Authorizer provider

Status: Proposed

Date: 2025-09-09

Decision drivers
- Enable small software teams to adopt multi-tenant, policy-based authorization that is secure, scalable, and low cost.
- Provide a single, reusable Lambda authorizer that works with both API Gateway and AppSync, so applications don’t each implement bespoke auth logic.
- Keep authorization definitions (schema + policies) in the consumer’s repository as declarative IaC—no UI-driven configuration.
- Make multi-tenant the default model to avoid painful migrations later.

## Summary
This repository ships a Pulumi provider that provisions an AWS Verified Permissions policy store and a bundled Lambda authorizer. Consumers bring a Cedar entity schema and policies as files in their own repo; the stack loads those into the policy store. Requests entering API Gateway or AppSync are routed to the authorizer, which extracts identity and request attributes and calls `verifiedpermissions:IsAuthorized` against the policy store. The outcome determines allow/deny.

## Services and technologies in scope
- AWS Verified Permissions (Cedar policy language)
- AWS Lambda (authorizer function)
- AWS DynamoDB (provider-managed table intended for auth/tenant-related data)
- AWS AppSync (supported authorizer target)
- AWS Cognito (optional; can be configured as the Verified Permissions identity source)

## Infrastructure-as-code posture
- Consumers own Cedar schema and policy files in their repositories. These are treated as code and versioned alongside application and infra code. No console-driven policy edits.
- The provider/resource stack is responsible for creating the Verified Permissions policy store and loading/validating the supplied schema and policies.
- This approach makes authorization intent explicit, reviewable, and testable in PRs.

## Multi-tenant first model
- The default mental model is multi-tenant: entities include at least a `Tenant`, `User`, and application-specific `Resource` types.
- Relationships (e.g., user membership in a tenant; a resource belonging to a tenant) should be encoded in the schema and referenced in policies.
- Starting with multi-tenant semantics avoids disruptive, retrofitted model and policy changes when you add your second customer.

## Scope disambiguation: global vs tenant
- Global (admin) scope: Actions that cross tenant boundaries (e.g., read all tenants, manage billing) are modeled as global actions. Policies for these actions should require principals with global roles/claims and must not rely on tenant context.
- Tenant scope: Actions within a single tenant (e.g., read project, invite member) are modeled so that authorization checks include the tenant relationship (subject’s tenant must match the resource’s tenant, or the subject must have an appropriate role in that tenant).
- Consumers should reflect this in their Cedar model via distinct actions or action groups (e.g., `Global*` vs tenant-scoped actions) and resource relations.

## Request-shape requirement
All information required to make an authorization decision must be derivable from the incoming request. The authorizer does not fetch additional context from application backends to “fill in” missing attributes. The request is expected to carry, at a minimum:
- Identity: a bearer token whose claims identify the subject (e.g., a `sub` claim for the principal ID). Additional claims (email, roles, tenantId, etc.) may be used by your policies.
- Tenant context: an identifier that ties the request to a tenant (for tenant-scoped operations). This can come from a claim (e.g., `tenantId`) and/or from request metadata like the hostname, URL path, headers, or variables.
- Resource identifiers: material that lets policies determine which resource is being accessed. For API Gateway this could be the method ARN or a path-derived ID; for AppSync the API ID and field can be used to map to a resource.
- Action: the operation the caller intends to perform. Your Cedar policies should define actions/action groups that map to your API surface. The bundled authorizer passes a single action (`invoke`) with the request’s resource identifier; consumers typically discriminate on resource and/or enrich policies to reflect their action taxonomy.

Implication: design your APIs so that identity, tenant, resource, and action are present (or derivable deterministically) at the time the authorizer runs.

## Compatibility targets
- API Gateway: compatible with Lambda Request Authorizers. The function expects a standard `Authorization: Bearer <token>` header and returns an allow/deny decision for the request.
- AppSync: compatible with AppSync Lambda authorizers. The function reads the AppSync `authorizationToken` field, evaluates, and returns the `{ isAuthorized: boolean }` result shape expected by AppSync.
- Constraints/assumptions:
  - The authorizer evaluates based solely on the incoming event (URL/path, headers, query string, and, where applicable, body variables). If required attributes are missing from the request, evaluation will deny.
  - Integrations should ensure the token is present in the expected location (API Gateway header or AppSync token field) and that any service-specific limits (e.g., event size) are respected.

## Consumer responsibilities
To use this provider effectively, consumers must:
1. Define a Cedar entity schema that models tenants, users, and application resources, including relationships used in authorization decisions.
2. Author Cedar policies that reference request attributes and entity types consistent with the schema.
3. Structure requests so the authorizer can extract identity, tenant context, resource identifiers, and the intended action.
4. Provide a verifiable bearer token format for identity. The reference authorizer verifies JWTs using a symmetric key (HS256) by default; other options are possible (see Cognito note below).
5. Provision and operate any application data that your policies rely on (for example, maintaining tenant/user membership or role assignment data in your systems). The provider includes a DynamoDB table that teams may use for auth-related data modeling if helpful.

## Optional Cognito usage
- Cognito is optional. When configured via the provider, a Cognito User Pool can be created and registered as the Verified Permissions identity source for the policy store. This enables policies to reference Cognito-based principals.
- Bearer tokens and identity:
  - With Cognito: you’ll typically use Cognito-issued JWTs (RS256). The reference authorizer included in this repo verifies JWTs using a symmetric secret by default (HS256). If you adopt Cognito, plan your integration so the authorizer receives a token it can verify (for example, by adding asymmetric/JWKS verification support or by fronting the authorizer with an upstream JWT validation step). This ADR stays conceptual and does not prescribe a specific verification mechanism.
  - Without Cognito: any identity provider that can issue a bearer token with a stable subject identifier is acceptable, provided the authorizer can verify it (default HS256 shared secret) and your policies align with the token’s claims.

## Data and state (DynamoDB)
- The provider creates a DynamoDB table intended for authorization-related data (e.g., tenant membership, role assignments, resource-to-tenant mappings) and grants the authorizer read access.
- Policies should be written so entity relationships are explicit in your Cedar model. The initial authorizer flow evaluates policies based on the request shape and the policy store; it does not depend on runtime database lookups for basic decisions.
- Teams may use the table to centralize auth data as their needs evolve; doing so does not change the core request-driven evaluation requirement described above.

## References
- Example of schema-level approach (open PR): https://github.com/mikecbrant/verified-permissions-authorizer/pull/9
- Background on Lambda custom authorizers: https://www.alexdebrie.com/posts/lambda-custom-authorizers/

## Consequences
- Authorization becomes portable across services (API Gateway and AppSync) and applications by centralizing policy in a Verified Permissions store and a single authorizer.
- Consumers get a clear, testable change process for auth (code review on schema/policies), but must ensure their request shapes carry the attributes policies expect.
- Adopting a multi-tenant-first model constrains early design choices in exchange for much easier growth later.
