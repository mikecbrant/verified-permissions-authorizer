import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('./auth/process.js', () => ({
  processAuthorization: vi.fn(),
}))

import { handler } from './index.js'
import { processAuthorization } from './auth/process.js'

const apiEvent = {
  type: 'REQUEST',
  methodArn: 'arn:aws:execute-api:us-east-1:123:rest/GET/foo',
  headers: { authorization: 'Bearer t' },
}

const appSyncEvent = {
  authorizationToken: 'Bearer t',
  requestContext: { apiId: 'abc' },
}

beforeEach(() => {
  ;(processAuthorization as any).mockReset()
  process.env.POLICY_STORE_ID = 'ps'
})

describe('handler', () => {
  it('denies when POLICY_STORE_ID not set', async () => {
    delete process.env.POLICY_STORE_ID
    ;(processAuthorization as any).mockResolvedValue(true)
    const res = await handler(apiEvent as any)
    expect((res as any).policyDocument.Statement[0].Effect).toBe('Deny')
  })

  it('allows API Gateway when auth true', async () => {
    ;(processAuthorization as any).mockResolvedValue(true)
    const res = await handler(apiEvent as any)
    expect((res as any).policyDocument.Statement[0].Effect).toBe('Allow')
  })

  it('denies on error path', async () => {
    ;(processAuthorization as any).mockRejectedValue(new Error('boom'))
    const res = await handler(apiEvent as any)
    expect((res as any).policyDocument.Statement[0].Effect).toBe('Deny')
  })

  it('returns AppSync authorization result shape', async () => {
    ;(processAuthorization as any).mockResolvedValue(false)
    const res = await handler(appSyncEvent as any)
    expect(res).toEqual({ isAuthorized: false })
  })

  it('returns AppSync authorization success when auth true', async () => {
    ;(processAuthorization as any).mockResolvedValue(true)
    const res = await handler(appSyncEvent as any)
    expect(res).toEqual({ isAuthorized: true })
  })

  it('falls back to AppSync-style deny for unknown event shape', async () => {
    ;(processAuthorization as any).mockResolvedValue(true)
    const res = await handler({ foo: 'bar' } as any)
    expect(res).toEqual({ isAuthorized: false })
  })

  it('denyFor() fallback is used when storeId missing and event is unknown', async () => {
    delete process.env.POLICY_STORE_ID
    const res = await handler({ something: 'else' } as any)
    expect(res).toEqual({ isAuthorized: false })
  })

  it('denyFor() returns AppSync auth result when storeId missing and event is AppSync', async () => {
    delete process.env.POLICY_STORE_ID
    const res = await handler(appSyncEvent as any)
    expect(res).toEqual({ isAuthorized: false })
  })

  it('API Gateway denies when auth false', async () => {
    process.env.POLICY_STORE_ID = 'ps'
    ;(processAuthorization as any).mockResolvedValue(false)
    const res = await handler(apiEvent as any)
    expect((res as any).policyDocument.Statement[0].Effect).toBe('Deny')
  })
})
