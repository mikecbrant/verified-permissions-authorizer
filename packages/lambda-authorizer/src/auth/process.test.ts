import jwt from 'jsonwebtoken'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../aws/verified-permissions.js', () => ({
  isAuthorized: vi.fn().mockResolvedValue('ALLOW'),
}))

const secret = 's'
const makeJwt = (): string =>
  jwt.sign({ sub: 'user-1', exp: Math.floor(Date.now() / 1000) + 60 }, secret)

beforeEach(() => {
  process.env.JWT_SECRET = secret
  // Point schema loader to test fixture and reset module cache so TLA re-runs per test suite
  process.env.VP_MERGED_SCHEMA_PATH = new URL(
    '../testdata/schema.merged.json',
    import.meta.url,
  ).pathname
  vi.resetModules()
})

describe('processAuthorization', () => {
  it('returns false when no token provided', async () => {
    const { processAuthorization } = await import('./process.js')
    const ev = { type: 'REQUEST', methodArn: 'arn', headers: {} }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(false)
  })

  it('authorizes with Verified Permissions when token present', async () => {
    const { processAuthorization } = await import('./process.js')
    const ev = {
      type: 'REQUEST',
      methodArn:
        'arn:aws:execute-api:us-east-1:123:rest/GET/tenant-grant/acme/alice',
      headers: { authorization: `Bearer ${makeJwt()}` },
      rawPath: '/tenant-grant/acme/alice',
      requestContext: { httpMethod: 'getTenantGrant' },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(true)
  })

  it('returns false when JWT_SECRET is not set', async () => {
    const { processAuthorization } = await import('./process.js')
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
    const { processAuthorization } = await import('./process.js')
    const ev = {
      authorizationToken: `Bearer ${makeJwt()}`,
      requestContext: { apiId: 'abc' },
      // Provide action + args so per-action mapping is exercised
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(true)
  })

  it('falls back to default subject when sub claim missing', async () => {
    const { processAuthorization } = await import('./process.js')
    const token = jwt.sign({ exp: Math.floor(Date.now() / 1000) + 60 }, secret)
    const ev = {
      type: 'REQUEST',
      methodArn:
        'arn:aws:execute-api:us-east-1:123:rest/GET/tenant-grant/acme/alice',
      headers: { authorization: `Bearer ${token}` },
      rawPath: '/tenant-grant/acme/alice',
      requestContext: { httpMethod: 'getTenantGrant' },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(true)
  })

  it('authorizes with API Gateway event using REST URL mapping', async () => {
    const { processAuthorization } = await import('./process.js')
    const ev = {
      type: 'REQUEST',
      methodArn: 'arn:aws:execute-api:us-east-1:123:rest/GET/foo',
      headers: { authorization: `Bearer ${makeJwt()}` },
      rawPath: '/tenant-grant/acme/alice',
      requestContext: { httpMethod: 'getTenantGrant' },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(true)
  })

  it('denies when mapping template is missing for action', async () => {
    // Use a schema with missing template mapping
    process.env.VP_MERGED_SCHEMA_PATH = new URL(
      '../testdata/schema.missing-template.json',
      import.meta.url,
    ).pathname
    const { processAuthorization } = await import('./process.js')
    const ev = {
      type: 'REQUEST',
      methodArn: 'arn:aws:execute-api:us-east-1:123:rest/GET/foo',
      headers: { authorization: `Bearer ${makeJwt()}` },
      rawPath: '/tenant-grant/acme/alice',
      requestContext: { httpMethod: 'getTenantGrant' },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(false)
  })

  it('denies when AppSync action identifier is missing', async () => {
    const { processAuthorization } = await import('./process.js')
    const ev = {
      authorizationToken: `Bearer ${makeJwt()}`,
      requestContext: { apiId: 'abc' },
      // info.fieldName absent so actionId cannot be resolved
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(false)
  })

  it('denies when API Gateway action identifier is missing', async () => {
    const { processAuthorization } = await import('./process.js')
    const ev = {
      type: 'REQUEST',
      methodArn:
        'arn:aws:execute-api:us-east-1:123:rest/GET/tenant-grant/acme/alice',
      headers: { authorization: `Bearer ${makeJwt()}` },
      rawPath: '/tenant-grant/acme/alice',
      // requestContext.httpMethod absent
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(false)
  })

  it('denies when AppSync template is missing for action', async () => {
    // Switch to a schema where the referenced template is missing
    process.env.VP_MERGED_SCHEMA_PATH = new URL(
      '../testdata/schema.missing-template.json',
      import.meta.url,
    ).pathname
    const { processAuthorization } = await import('./process.js')
    const ev = {
      authorizationToken: `Bearer ${makeJwt()}`,
      requestContext: { apiId: 'abc' },
      info: { fieldName: 'getTenantGrant' },
      arguments: { tenantId: 't1', userId: 'u1' },
    }
    const ok = await processAuthorization('ps', ev as any)
    expect(ok).toBe(false)
  })
})
