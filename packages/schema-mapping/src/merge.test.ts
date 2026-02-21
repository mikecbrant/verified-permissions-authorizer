import { describe, expect, it } from "vitest";

import { mergeCedarSchemas, validateSuperset } from "./merge.js";

const baseYaml = `ns:
  entityTypes:
    Tenant: { shape: { type: Record, attributes: {} } }
    User: { shape: { type: Record, attributes: {} } }
    Role: { shape: { type: Record, attributes: {} } }
    GlobalRole: { shape: { type: Record, attributes: {} } }
    TenantGrant:
      memberOfTypes: [Role, Tenant, User]
      shape: { type: Record, attributes: { tenantId: { type: String }, userId: { type: String } } }
  actions:
    Get: { appliesTo: { principalTypes: [User, GlobalRole, Role, Tenant, TenantGrant] } }
`;

describe("mergeCedarSchemas (per-action, resourceEntities)", () => {
  it("augments existing entity with resourceEntities and adds per-action input/entityMap", () => {
    const partial = `ns:
  entityTypes:
    TenantGrant:
      resourceEntities:
        anyByTenantId:
          id: '*'
          type: TenantGrant
          attributes: { tenantId: $tenantId, userId: '*' }
          parents: []
        byTenantIdAndUserId:
          id: $tenantId:$userId
          type: TenantGrant
          attributes: { tenantId: $tenantId, userId: $userId }
          parents: []
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
      appsync: { path: info.fieldName }
      apiGateway: { path: requestContext.httpMethod }
`;
    const { supersetJson, cedarJson, namespace } = mergeCedarSchemas(
      baseYaml,
      partial,
    );
    expect(namespace).toBe("ns");
    const superset = JSON.parse(supersetJson);
    expect(
      superset.ns.entityTypes.TenantGrant.resourceEntities.byTenantIdAndUserId,
    ).toBeTruthy();
    expect(superset.ns.actions.getTenantGrant.entityMap.TenantGrant).toBe(
      "byTenantIdAndUserId",
    );
    // Pruned Cedar should not contain superset-only keys
    const cedar = JSON.parse(cedarJson);
    expect(cedar.ns.entityTypes.TenantGrant.resourceEntities).toBeUndefined();
    expect(cedar.ns.actions.getTenantGrant.entityMap).toBeUndefined();
    expect(cedar.ns.actions.getTenantGrant.input).toBeUndefined();
    // Validate cross-references
    const errs = validateSuperset(superset);
    expect(errs).toEqual([]);
  });

  it("rejects overriding base Cedar fields on entity and action", () => {
    const badEntity = `ns:\n  entityTypes:\n    Tenant: { shape: { type: Record, attributes: { x: { type: String } } } }\n`;
    expect(() => mergeCedarSchemas(baseYaml, badEntity)).toThrow(
      /cannot override base entityType Tenant\.shape/,
    );
    const badAction = `ns:\n  actions:\n    Get: { appliesTo: { resourceTypes: [User] } }\n`;
    expect(() => mergeCedarSchemas(baseYaml, badAction)).toThrow(
      /cannot override base action Get\.appliesTo/,
    );
  });

  it("rejects namespace mismatch and multiple namespaces", () => {
    expect(() => mergeCedarSchemas(baseYaml, "other: {}")).toThrow(
      /namespace mismatch/,
    );
    expect(() => mergeCedarSchemas(baseYaml, "a: {}\nb: {}")).toThrow(
      /exactly one namespace/,
    );
  });
});
