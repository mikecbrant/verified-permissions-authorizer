# ADR 0004: Cedar schema superset, per‑action input mappings, and packaging

Status: Accepted

Date: 2025-09-12

Context
Consumers want to extend the base Cedar schema and drive authorization from AppSync/API Gateway events without hard‑coding mappers in the Lambda. We adopt a declarative superset that keeps Cedar valid while adding the minimum extra structure we need at runtime.

Decisions
- Partial superset merge
  - Accept a partial YAML with exactly one namespace that only adds:
    - New resource entity types, actions, and action groups
    - Superset fields on existing defs: `entityTypes.<Type>.resourceEntities`, `actions.<Action>.entityMap`, and `actions.<Action>.input`
  - Do not allow modifying base principal types (Tenant, User, Role, GlobalRole, TenantGrant) or overriding Cedar fields like `shape`, `memberOfTypes`, or `appliesTo`.
- Per‑action mapping model with a minimal top‑level action identifier
  - Each action declares its inputs under `actions.<Action>.input` by integration:
    - AppSync: `body` maps variables from `event.arguments` using simple keys or `$.a.b` JSON paths.
    - REST/API Gateway: `url` is an express‑style route template (e.g., `/tenant-grant/:tenantId/:userId`), plus optional `body`/`query` variable maps.
  - Each action declares how to build a resource entity via `actions.<Action>.entityMap` (resource type → `resourceEntities` template name).
  - A minimal root mapping `mappings.actions.{appsync|apiGateway}.path` remains to extract the action identifier (e.g., `info.fieldName` for AppSync). All other input extraction is per‑action.
- Packaging and pruning
  - The full merged superset JSON is bundled alongside the Lambda (`schema.merged.json`) so the authorizer can resolve resource entities at runtime.
  - Before uploading the schema to AWS Verified Permissions, we prune all superset keys, producing Cedar‑only JSON. This avoids AVP schema rejections on unknown keys.
  - Namespace input: the authorizer accepts a `namespace` input (same as the single namespace in user‑authored YAML). The provider enforces single‑namespace equality and writes this namespace as the single top‑level key in the final merged JSON. Because we don’t expose partial JSON, namespace is always included automatically in the bundled `schema.merged.json`.

YAML shape (excerpt)
```yaml
<namespace>:
  entityTypes:
    TenantGrant:
      memberOfTypes: [Role, Tenant, User]
      resourceEntities:
        byTenantIdAndUserId:
          id: $tenantId:$userId
          type: TenantGrant
          attributes: { tenantId: $tenantId, userId: $userId }
          parents: []
      shape:
        type: Record
        attributes: { tenantId: { type: String }, userId: { type: String } }

  actions:
    getTenantGrant:
      memberOf: [Get]
      appliesTo: { resourceTypes: [TenantGrant] }
      entityMap: { TenantGrant: byTenantIdAndUserId }
      input:
        appsync:
          body: { tenantId: tenantId, userId: userId }
        rest:
          url: '/tenant-grant/:tenantId/:userId'

  mappings:
    actions:
      appsync:   { path: info.fieldName }
      apiGateway:{ path: requestContext.httpMethod }
```

Merge/validation rules
- Single namespace; equality with base required.
- No overrides of base entity/action Cedar fields; only the superset keys above may be added to existing defs.
- actions.<Action>.appliesTo.resourceTypes must be non‑empty, and every listed type must appear in `actions.<Action>.entityMap` pointing to an existing `entityTypes.<Type>.resourceEntities.<Template>`.
- Templates may reference only variables exposed by the per‑action inputs for that integration (`appsync.body`, `rest.url`/`rest.body`/`rest.query`), or wildcards (`*`).

Packaging expectations
- The provider includes `schema.merged.json` in the Lambda code archive. Any change to the schema causes a new code hash → Lambda redeploy.
- The provider prunes superset fields before calling `PutSchema` (Cedar JSON only).

Rationale / discrepancy resolution
- Earlier drafts had a top‑level, property‑centric mapping model. We keep only the top‑level action identifier extraction and move all variable extraction and resource construction to per‑action definitions. This keeps schemas readable and keeps action/resource coupling where it belongs.

Consequences
- Declarative, testable mappings with strong validation guardrails.
- Clean separation between runtime needs (superset JSON in the Lambda) and AVP needs (Cedar‑only JSON for the Policy Store).
