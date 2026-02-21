import type {
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
} from "aws-lambda";
import { describe, expect, it } from "vitest";

import { extractFromApiGateway, extractFromAppSync } from "./mapping.js";

const superset = {
  ns: {
    entityTypes: {
      TenantGrant: {
        resourceEntities: {
          byTenantIdAndUserId: {
            id: "$tenantId:$userId",
            type: "TenantGrant",
            attributes: { tenantId: "$tenantId", userId: "$userId" },
            parents: [],
          },
        },
      },
    },
    actions: {
      getTenantGrant: {
        appliesTo: { resourceTypes: ["TenantGrant"] },
        entityMap: { TenantGrant: "byTenantIdAndUserId" },
        input: {
          appsync: { body: { tenantId: "tenantId", userId: "userId" } },
          rest: { url: "/tenant-grant/:tenantId/:userId" },
        },
      },
    },
    mappings: {
      actions: {
        appsync: { path: "info.fieldName" },
        apiGateway: { path: "requestContext.httpMethod" },
      },
    },
  },
};

describe("extractFromAppSync (per-action)", () => {
  it("extracts action and resolves resource entity via entityMap/template", () => {
    const ev = {
      authorizationToken: "t",
      requestContext: { apiId: "x" },
      arguments: { tenantId: "t1", userId: "u1" },
      info: { fieldName: "getTenantGrant" },
    } as unknown as AppSyncAuthorizerEvent;
    const { action, resource, vars } = extractFromAppSync(ev, superset as any);
    expect(action).toBe("getTenantGrant");
    expect(resource).toEqual({ entityType: "TenantGrant", entityId: "t1:u1" });
    expect(vars).toMatchObject({ tenantId: "t1", userId: "u1" });
  });
});

describe("extractFromApiGateway (per-action)", () => {
  it("extracts vars from URL template and builds resource", () => {
    const ev = {
      type: "REQUEST",
      methodArn: "arn",
      headers: {},
      rawPath: "/tenant-grant/acme/alice",
      requestContext: { httpMethod: "getTenantGrant" },
    } as unknown as APIGatewayRequestAuthorizerEvent;
    const { action, resource, vars } = extractFromApiGateway(
      ev,
      superset as any,
    );
    expect(action).toBe("getTenantGrant");
    expect(vars).toMatchObject({ tenantId: "acme", userId: "alice" });
    expect(resource).toEqual({
      entityType: "TenantGrant",
      entityId: "acme:alice",
    });
  });
});
