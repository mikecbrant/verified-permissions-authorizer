import { describe, expect, it } from 'vitest'

import { extractFromAppSync } from './appsync.js'

const doc = {
  ns: {
    entityTypes: {
      TenantGrant: {
        resourceEntities: {
          byTenantIdAndUserId: { id: '$tenantId:$userId', type: 'TenantGrant' },
        },
      },
    },
    actions: {
      getTenantGrant: {
        appliesTo: { resourceTypes: ['TenantGrant'] },
        entityMap: { TenantGrant: 'byTenantIdAndUserId' },
        input: {
          appsync: { body: { tenantId: 'tenantId', userId: 'userId' } },
        },
      },
    },
    mappings: { actions: { appsync: { path: 'info.fieldName' } } },
  },
}

describe('extractFromAppSync', () => {
  it('extracts action and resource from arguments', () => {
    const ev = {
      requestContext: { apiId: 'x' },
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const out = extractFromAppSync(ev as any, doc as any)
    expect(out.action).toBe('getTenantGrant')
    expect(out.resource).toEqual({
      entityType: 'TenantGrant',
      entityId: 't1:u1',
    })
  })

  it('returns undefineds when action not found', () => {
    const ev = { info: { fieldName: 'Nope' }, arguments: {} }
    const out = extractFromAppSync(ev as any, doc as any)
    expect(out.action).toBe('Nope')
    expect(out.resource).toBeUndefined()
  })

  it('returns undefined resource when template is missing', () => {
    const bad = JSON.parse(JSON.stringify(doc))
    delete bad.ns.entityTypes.TenantGrant.resourceEntities.byTenantIdAndUserId
    const ev = {
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const out = extractFromAppSync(ev as any, bad as any)
    expect(out.action).toBe('getTenantGrant')
    expect(out.resource).toBeUndefined()
  })

  it('returns undefined resource when actionDef is missing', () => {
    const bad = JSON.parse(JSON.stringify(doc))
    delete bad.ns.actions.getTenantGrant
    const ev = {
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const out = extractFromAppSync(ev as any, bad as any)
    expect(out.action).toBe('getTenantGrant')
    expect(out.resource).toBeUndefined()
  })

  it('falls back to default action path when mappings missing', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.mappings
    const ev = {
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const out = extractFromAppSync(ev as any, alt as any)
    expect(out.action).toBe('getTenantGrant')
  })

  it('treats empty action as undefined and returns no resource', () => {
    const ev = { info: {}, arguments: { tenantId: 't1', userId: 'u1' } }
    const out = extractFromAppSync(ev as any, doc as any)
    expect(out.action).toBeUndefined()
    expect(out.resource).toBeUndefined()
  })

  it('uses resource type fallback when template.type is absent', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.entityTypes.TenantGrant.resourceEntities.byTenantIdAndUserId
      .type
    const ev = {
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const out = extractFromAppSync(ev as any, alt as any)
    expect(out.resource).toEqual({
      entityType: 'TenantGrant',
      entityId: 't1:u1',
    })
  })

  it('returns undefined when appliesTo is missing (no resourceTypes)', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.actions.getTenantGrant.appliesTo
    const ev = {
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const out = extractFromAppSync(ev as any, alt as any)
    expect(out.resource).toBeUndefined()
  })

  it('handles missing input.appsync by treating it as empty object', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.actions.getTenantGrant.input
    const ev = {
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const out = extractFromAppSync(ev as any, alt as any)
    expect(out.action).toBe('getTenantGrant')
  })

  it('handles missing arguments by treating them as empty object', () => {
    const ev = { info: { fieldName: 'getTenantGrant' } }
    const out = extractFromAppSync(ev as any, doc as any)
    expect(out.action).toBe('getTenantGrant')
  })

  it('falls back to empty template.id when id is absent', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.entityTypes.TenantGrant.resourceEntities.byTenantIdAndUserId
      .id
    const ev = {
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const out = extractFromAppSync(ev as any, alt as any)
    expect(out.resource).toEqual({ entityType: 'TenantGrant', entityId: '' })
  })
})
