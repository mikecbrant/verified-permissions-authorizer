import { describe, expect, it, vi } from 'vitest'
import { processAuthorization } from './process.js'

vi.mock('../aws/verified-permissions.js', () => ({
  isAuthorized: vi.fn().mockResolvedValue('ALLOW'),
}))

const makeJwt = () =>
  // minimal valid JWT string with future exp; signature not verified by our utils
  [
    Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url'),
    Buffer.from(
      JSON.stringify({ sub: 'user-1', exp: Math.floor(Date.now() / 1000) + 60 }),
    ).toString('base64url'),
    'sig',
  ].join('.')

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
})
