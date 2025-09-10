import { describe, expect, it } from 'vitest'

import { mergeCedarSchemas } from './merge.js'

const baseYaml = `ns:
  entityTypes:
    Tenant: { shape: { type: Record, attributes: {} } }
    User: { shape: { type: Record, attributes: {} } }
    Role: { shape: { type: Record, attributes: {} } }
    GlobalRole: { shape: { type: Record, attributes: {} } }
    TenantGrant: { shape: { type: Record, attributes: {} } }
  actions:
    Get: { appliesTo: { resourceTypes: [Tenant] } }
`

describe('mergeCedarSchemas', () => {
  it('merges new resource entity and action and returns mappings', () => {
    const partial = `ns:
  entityTypes:
    File: { shape: { type: Record, attributes: { path: { type: String } } } }
  actions:
    GetFile: { memberOf: [Get], appliesTo: { resourceTypes: [File] } }
  mappings:
    properties:
      tenantId:
        appsync: { path: "arguments.tenantId" }
        apiGateway: { source: path, name: tenantId }
`
    const { cedarJson, namespace, mappings } = mergeCedarSchemas(baseYaml, partial)
    expect(namespace).toBe('ns')
    const obj = JSON.parse(cedarJson)
    expect(obj.ns.entityTypes.File).toBeTruthy()
    expect(obj.ns.actions.GetFile).toBeTruthy()
    expect(mappings?.properties?.tenantId?.appsync?.path).toBe('arguments.tenantId')
  })

  it('rejects overriding base entity or action', () => {
    const partial = `ns:
  entityTypes:
    Tenant: { shape: { type: Record, attributes: {} } }
`
    expect(() => mergeCedarSchemas(baseYaml, partial)).toThrow(/cannot override base entityType Tenant/)
  })

  it('rejects principal type additions', () => {
    const partial = `ns:
  entityTypes:
    GlobalRole: { shape: { type: Record, attributes: {} } }
`
    expect(() => mergeCedarSchemas(baseYaml, partial)).toThrow(/cannot (override base entityType|add or modify principal type) GlobalRole/)
  })

  it('rejects namespace mismatch', () => {
    const partial = `other: {}`
    expect(() => mergeCedarSchemas(baseYaml, partial)).toThrow(/namespace mismatch/)
  })

  it('rejects multiple namespaces in partial', () => {
    const partial = `a: {}\nb: {}`
    expect(() => mergeCedarSchemas(baseYaml, partial)).toThrow(/exactly one namespace/)
  })

  it('rejects overriding existing action', () => {
    const partial = `ns:\n  actions:\n    Get: { appliesTo: { resourceTypes: [User] } }\n`
    expect(() => mergeCedarSchemas(baseYaml, partial)).toThrow(/cannot override base action Get/)
  })

  it('accepts partial with only mappings (no entityTypes/actions)', () => {
    const partial = `ns:\n  mappings:\n    properties: { tenantId: { appsync: { path: arguments.tenantId } } }\n`
    const res = mergeCedarSchemas(baseYaml, partial)
    expect(JSON.parse(res.cedarJson).ns.entityTypes.Tenant).toBeTruthy()
    expect(res.mappings?.properties?.tenantId).toBeTruthy()
  })
})
