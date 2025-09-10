import type { APIGatewayRequestAuthorizerEvent, AppSyncAuthorizerEvent } from 'aws-lambda'
import { describe, expect, it } from 'vitest'

import { extractFromApiGateway, extractFromAppSync } from './mapping.js'

describe('extractFromAppSync', () => {
  it('reads props and action via paths', () => {
    const ev = {
      authorizationToken: 't',
      requestContext: { apiId: 'x' },
      identity: { sub: 'u1' },
      arguments: { tenantId: 't1' },
      info: { fieldName: 'GetTicket' },
    } as unknown as AppSyncAuthorizerEvent
    const { props, action } = extractFromAppSync(ev, {
      properties: {
        tenantId: { appsync: { path: 'arguments.tenantId' } },
        userId: { appsync: { path: 'identity.sub' } },
      },
      actions: { appsync: { path: 'info.fieldName' } },
    })
    expect(props.tenantId).toBe('t1')
    expect(props.userId).toBe('u1')
    expect(action).toBe('GetTicket')
  })

  it('handles cfg undefined (no mappings)', () => {
    const ev = { authorizationToken: 't', requestContext: { apiId: 'x' } } as unknown as AppSyncAuthorizerEvent
    const out = extractFromAppSync(ev, undefined)
    expect(out.props).toEqual({})
    expect(out.action).toBeUndefined()
  })

  it('treats empty appsync property path as undefined', () => {
    const ev = { authorizationToken: 't', requestContext: { apiId: 'x' } } as unknown as AppSyncAuthorizerEvent
    const out = extractFromAppSync(ev, { properties: { tenantId: { appsync: { path: '' as any } } } as any })
    expect(out.props.tenantId).toBeUndefined()
  })
})

describe('extractFromApiGateway', () => {
  it('reads from path/query and best-effort body path', () => {
    const ev = {
      type: 'REQUEST',
      methodArn: 'arn',
      headers: {},
      pathParameters: { tenantId: 't1' },
      queryStringParameters: { action: 'GET' },
      body: JSON.stringify({ nested: { userId: 'u1' } }),
      requestContext: { httpMethod: 'GET' },
    } as unknown as APIGatewayRequestAuthorizerEvent
    const { props, action } = extractFromApiGateway(ev, {
      properties: {
        tenantId: { apiGateway: { source: 'path', name: 'tenantId' } },
        userId: { apiGateway: { source: 'body', path: 'nested.userId' } },
      },
      actions: { apiGateway: { path: 'requestContext.httpMethod' } },
    })
    expect(props.tenantId).toBe('t1')
    expect(props.userId).toBe('u1')
    expect(action).toBe('GET')
  })

  it('handles cfg undefined and query source', () => {
    const ev = {
      type: 'REQUEST', methodArn: 'arn', headers: {},
      pathParameters: {}, queryStringParameters: { tenantId: 't2' },
    } as unknown as APIGatewayRequestAuthorizerEvent
    const res1 = extractFromApiGateway(ev, undefined)
    expect(res1.props).toEqual({})
    const { props } = extractFromApiGateway(ev, { properties: { tenantId: { apiGateway: { source: 'query' } } } })
    expect(props.tenantId).toBe('t2')
  })

  it('body mapping tolerates invalid JSON', () => {
    const ev = { type: 'REQUEST', methodArn: 'arn', headers: {}, body: '{' } as unknown as APIGatewayRequestAuthorizerEvent
    const { props } = extractFromApiGateway(ev, { properties: { userId: { apiGateway: { source: 'body', path: 'nested.userId' } } } })
    expect(props.userId).toBeUndefined()
  })

  it('skips properties without apiGateway mapping', () => {
    const ev = { type: 'REQUEST', methodArn: 'arn', headers: {} } as unknown as APIGatewayRequestAuthorizerEvent
    const { props, action } = extractFromApiGateway(ev, { properties: { foo: {} as any } })
    expect(props).toEqual({})
    expect(action).toBeUndefined()
  })

  it('returns undefined action when API Gateway action path omitted', () => {
    const ev = { type: 'REQUEST', methodArn: 'arn', headers: {} } as unknown as APIGatewayRequestAuthorizerEvent
    const out = extractFromApiGateway(ev, { properties: {} })
    expect(out.action).toBeUndefined()
  })
})
