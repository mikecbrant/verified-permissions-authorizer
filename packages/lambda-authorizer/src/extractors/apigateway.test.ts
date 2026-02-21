import { describe, expect, it } from 'vitest'

import { extractFromApiGateway } from './apigateway.js'

const doc = {
  ns: {
    entityTypes: {
      Ticket: {
        resourceEntities: {
          byTenantAndTicket: { id: '$tenantId:$ticketId', type: 'Ticket' },
        },
      },
    },
    actions: {
      GET: {
        appliesTo: { resourceTypes: ['Ticket'] },
        entityMap: { Ticket: 'byTenantAndTicket' },
        input: {
          rest: {
            url: '/tenants/:tenantId/tickets/:ticketId',
            query: { q: 'q' },
            body: { x: 'x' },
          },
        },
      },
    },
    mappings: {
      actions: { apiGateway: { path: 'requestContext.httpMethod' } },
    },
  },
}

describe('extractFromApiGateway', () => {
  it('extracts action and resource from path/query/body', () => {
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/tenants/t1/tickets/abc',
      queryStringParameters: { q: 'ok' },
      body: JSON.stringify({ x: 'y' }),
    }
    const out = extractFromApiGateway(ev as any, doc as any)
    expect(out.action).toBe('GET')
    expect(out.resource).toEqual({ entityType: 'Ticket', entityId: 't1:abc' })
    expect(out.vars).toMatchObject({
      tenantId: 't1',
      ticketId: 'abc',
      q: 'ok',
      x: 'y',
    })
  })

  it('returns undefineds when action not in schema', () => {
    const ev = { requestContext: { httpMethod: 'POST' }, rawPath: '/' }
    const out = extractFromApiGateway(ev as any, doc as any)
    expect(out.action).toBe('POST')
    expect(out.resource).toBeUndefined()
  })

  it('handles invalid JSON body gracefully', () => {
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/tenants/t1/tickets/abc',
      body: '{ not json }',
    }
    const out = extractFromApiGateway(ev as any, doc as any)
    expect(out.action).toBe('GET')
    expect(out.vars).toHaveProperty('tenantId', 't1')
  })

  it('returns undefined resource when entityMap is missing', () => {
    const bad = JSON.parse(JSON.stringify(doc))
    delete bad.ns.actions.GET.entityMap
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/tenants/t1/tickets/abc',
    }
    const out = extractFromApiGateway(ev as any, bad as any)
    expect(out.action).toBe('GET')
    expect(out.resource).toBeUndefined()
  })

  it('supports rest.query as a string name and handles missing rest.url', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    alt.ns.actions.GET.input.rest = { query: 'tenantId' } // no url/body
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/',
      queryStringParameters: { tenantId: 't1' },
    }
    const out = extractFromApiGateway(ev as any, alt as any)
    expect(out.vars).toHaveProperty('tenantId', 't1')
  })

  it('falls back to default action path when mappings missing', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.mappings
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/tenants/t1/tickets/abc',
    }
    const out = extractFromApiGateway(ev as any, alt as any)
    expect(out.action).toBe('GET')
  })

  it('uses legacy path property when rawPath is absent', () => {
    const ev = {
      requestContext: { httpMethod: 'GET' },
      path: '/tenants/t1/tickets/abc',
    }
    const out = extractFromApiGateway(ev as any, doc as any)
    expect(out.resource).toEqual({ entityType: 'Ticket', entityId: 't1:abc' })
  })

  it('returns undefined when appliesTo is missing', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.actions.GET.appliesTo
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/tenants/t1/tickets/abc',
    }
    const out = extractFromApiGateway(ev as any, alt as any)
    expect(out.resource).toBeUndefined()
  })

  it('uses resource type fallback when template.type is absent', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.entityTypes.Ticket.resourceEntities.byTenantAndTicket.type
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/tenants/t1/tickets/abc',
    }
    const out = extractFromApiGateway(ev as any, alt as any)
    expect(out.resource).toEqual({ entityType: 'Ticket', entityId: 't1:abc' })
  })

  it('handles missing input.rest by treating it as empty object', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.actions.GET.input
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/tenants/t1/tickets/abc',
    }
    const out = extractFromApiGateway(ev as any, alt as any)
    expect(out.action).toBe('GET')
  })

  it('handles empty path when both rawPath and path are absent', () => {
    const ev = { requestContext: { httpMethod: 'GET' } }
    const out = extractFromApiGateway(ev as any, doc as any)
    expect(out.action).toBe('GET')
  })

  it('falls back to empty template.id when id is absent', () => {
    const alt = JSON.parse(JSON.stringify(doc))
    delete alt.ns.entityTypes.Ticket.resourceEntities.byTenantAndTicket.id
    const ev = {
      requestContext: { httpMethod: 'GET' },
      rawPath: '/tenants/t1/tickets/abc',
    }
    const out = extractFromApiGateway(ev as any, alt as any)
    expect(out.resource).toEqual({ entityType: 'Ticket', entityId: '' })
  })
})
