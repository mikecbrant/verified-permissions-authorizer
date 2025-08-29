import { describe, expect, it } from 'vitest'
import {
  isApiGatewayRequestAuthorizerEvent,
  isAppSyncAuthorizerEvent,
} from './events.js'

describe('event type guards', () => {
  it('detects API Gateway Request Authorizer events', () => {
    const ev = {
      type: 'REQUEST',
      methodArn: 'arn:aws:execute-api:us-east-1:123:abc/GET/foo',
      headers: { authorization: 'Bearer x' },
    }
    expect(isApiGatewayRequestAuthorizerEvent(ev)).toBe(true)
    expect(isAppSyncAuthorizerEvent(ev)).toBe(false)
  })

  it('detects AppSync Authorizer events', () => {
    const ev = {
      authorizationToken: 'Bearer y',
      requestContext: { apiId: 'abc123' },
    }
    expect(isAppSyncAuthorizerEvent(ev)).toBe(true)
    expect(isApiGatewayRequestAuthorizerEvent(ev)).toBe(false)
  })

  it('returns false for non-object inputs', () => {
    expect(isApiGatewayRequestAuthorizerEvent(null as any)).toBe(false)
    expect(isAppSyncAuthorizerEvent(42 as any)).toBe(false)
  })
})
