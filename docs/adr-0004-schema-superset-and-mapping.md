# ADR 0004: Cedar schema superset extension and request mappings

Status: Proposed

Date: 2025-09-10

Context
Consumers want to extend the base Cedar schema with additional resource entity types, actions, and action groups without overriding any base definitions. They also need a static, declarative way to extract top-level properties (e.g., `tenantId`, `userId`) and action identifiers from AppSync and API Gateway authorizer events.

## Decisions
- Accept a partial YAML file that contains exactly one namespace and only adds:
   - New resource entity types
   - New actions and action groups
- Do not allow modifying or redefining base principal types (Tenant, User, Role, GlobalRole, TenantGrant) or changing any existing base entity/action definitions.
- Support a superset `mappings` section with static instructions for extracting top-level properties used in policies and an action identifier from authorizer events.
- Keep functions short with verbose comments and 100% unit test coverage.

## YAML shape (superset)
```yaml
<namespace>:
   entityTypes:   # optional; only new resource entities are allowed here
     FileAttachment: { ... }
   actions:       # optional; may include new actions or action groups
     CreateFileAttachment: { memberOf: [Create], appliesTo: { resourceTypes: [FileAttachment] } }
   mappings:      # optional; static request-mapping config
     properties:
       tenantId:
         appsync:   { path: "arguments.tenantId" }
         apiGateway:{ source: "path", name: "tenantId" }
       userId:
         appsync:   { path: "identity.sub" }
         apiGateway:{ source: "body", path: "user.id" }
     actions:
       appsync:   { path: "info.fieldName" }
       apiGateway:{ path: "requestContext.http.method" }
```

## Merge rules
- Validate both base and partial define a single, identical namespace.
- `entityTypes`: allow only keys that do not exist in base; reject attempts to redefine base keys (especially principals).
- `actions`: allow adding new action keys; reject redefinitions of existing action keys.
- `mappings`: extracted as-is (not part of the Cedar schema JSON sent to AVP).

## Request mappings
- AppSync: values are read using dot-paths relative to the Lambda authorizer event shape (e.g., `identity.sub`, `arguments.tenantId`).
- API Gateway: values can come from URL path parameters (`source: path`), query string (`source: query`), or body path (`source: body`, dot-path under the parsed JSON body when present).
- Action mapping: optional path expressions that produce a string identifier used by policies (e.g., a GraphQL field name or HTTP method + route).

## Consequences
- Consumers can safely extend vocabulary without risking breaking base invariants.
- Mapping behavior is explicit, versionable, and testable in code review.
