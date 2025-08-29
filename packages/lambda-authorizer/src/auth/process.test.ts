import jwt from 'jsonwebtoken'
import { beforeEach,describe, expect, it, vi } from 'vitest'

import { processAuthorization } from './process.js'

vi.mock('../aws/verified-permissions.js', () => ({
  isAuthorized: vi.fn().mockResolvedValue('ALLOW'),
}))

const secret = 's'
const makeJwt = (): string =>
  jwt.sign({ sub: 'user-1', exp: Math.floor(Date.now() / 1000) + 60 }, secret)

beforeEach(() => {
  process.env.JWT_SECRET = secret
})

describe('processAuthorization', () => {
  it('returns false when no token provided', async () => {
    const ev = { type: 'REQUEST', methodArn: 'arn', headers: {} }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(false)
  })

  it('authorizes with Verified Permissions when token present', async () => {
    const ev = {
      type: 'REQUEST',
      methodArn: 'arn:aws:execute-api:us-east-1:123:rest/GET/foo',
      headers: { authorization: `Bearer ${makeJwt()}` },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(true)
  })

  it('returns false when JWT_SECRET is not set', async () => {
    delete process.env.JWT_SECRET
    const ev = {
      type: 'REQUEST',
      methodArn: 'arn:aws:execute-api:us-east-1:123:rest/GET/foo',
      headers: { authorization: `Bearer ${makeJwt()}` },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(false)
  })

  it('authorizes with AppSync event shape', async () => {
    const ev = {
      authorizationToken: `Bearer ${makeJwt()}`,
      requestContext: { apiId: 'abc' },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(true)
  })

  it('falls back to default subject when sub claim missing', async () => {
    const token = jwt.sign({ exp: Math.floor(Date.now() / 1000) + 60 }, secret)
    const ev = { type: 'REQUEST', methodArn: 'arn', headers: { authorization: `Bearer ${token}` } }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(true)
  })
})
