import jwt from 'jsonwebtoken'
import { describe, expect, it } from 'vitest'

import { getBearerToken, parseJwtPayload } from './jwt.js'

describe('jwt utils', () => {
  it('extracts bearer token from API Gateway and AppSync', () => {
    const apiEv = {
      type: 'REQUEST',
      methodArn: 'arn',
      headers: { Authorization: 'Bearer abc' },
    }
    const appSyncEv = {
      authorizationToken: 'Bearer def',
      requestContext: { apiId: 'id' },
    }
    expect(getBearerToken(apiEv as any)).toBe('abc')
    expect(getBearerToken(appSyncEv as any)).toBe('def')
  })

  it('returns undefined when API Gateway header is missing', () => {
    const apiEv = { type: 'REQUEST', methodArn: 'arn', headers: {} }
    expect(getBearerToken(apiEv as any)).toBeUndefined()
  })

  it('supports lower-case authorization header', () => {
    const apiEv = { type: 'REQUEST', methodArn: 'arn', headers: { authorization: 'Bearer z' } }
    expect(getBearerToken(apiEv as any)).toBe('z')
  })

  it('verifies payload and enforces exp/nbf', async () => {
    const secret = 's'
    const good = jwt.sign({ sub: 'u', exp: Math.floor(Date.now() / 1000) + 60 }, secret)
    const bad = jwt.sign({ sub: 'u', exp: Math.floor(Date.now() / 1000) - 1 }, secret)
    expect(parseJwtPayload(good, secret)?.sub).toBe('u')
    expect(parseJwtPayload(bad, secret)).toBeUndefined()
  })

  it('rejects tokens when nbf is in the future', () => {
    const now = Math.floor(Date.now() / 1000)
    const secret = 's'
    const t = jwt.sign({ sub: 'u', nbf: now + 60, exp: now + 120 }, secret)
    expect(parseJwtPayload(t, secret)).toBeUndefined()
  })

  it('returns undefined for unknown event shapes', () => {
    expect(getBearerToken({} as any)).toBeUndefined()
  })

  it('handles non-Bearer AppSync tokens by returning raw value', () => {
    const ev = { authorizationToken: 'rawtoken', requestContext: { apiId: 'id' } }
    expect(getBearerToken(ev as any)).toBe('rawtoken')
  })

  it('returns undefined when AppSync auth token missing', () => {
    const ev = { requestContext: { apiId: 'id' } }
    expect(getBearerToken(ev as any)).toBeUndefined()
  })

  it('returns undefined when AppSync auth token is empty string', () => {
    const ev = { authorizationToken: '   ', requestContext: { apiId: 'id' } }
    expect(getBearerToken(ev as any)).toBeUndefined()
  })

  it('returns undefined for completely invalid JWT strings', () => {
    expect(parseJwtPayload('not-a-jwt', 's')).toBeUndefined()
  })

  it('returns undefined when verified payload is a string', () => {
    const secret = 's'
    const t = jwt.sign('just-a-string', secret)
    expect(parseJwtPayload(t, secret)).toBeUndefined()
  })
})
