import { describe, expect, it, vi } from 'vitest'
import { IsAuthorizedCommand } from '@aws-sdk/client-verifiedpermissions'
import { getClient, isAuthorized } from './verified-permissions.js'

describe('verified-permissions utilities', () => {
  it('getClient returns singleton', () => {
    const a = getClient()
    const b = getClient()
    expect(a).toBe(b)
  })

  it('isAuthorized delegates to client with IsAuthorizedCommand', async () => {
    const c = getClient()
    const spy = vi.spyOn(c, 'send').mockResolvedValue({ decision: 'ALLOW' } as any)
    const res = await isAuthorized({
      policyStoreId: 'ps',
      principal: { entityType: 'User', entityId: 'u' },
      action: { actionType: 'Action', actionId: 'invoke' },
      resource: { entityType: 'Resource', entityId: 'arn' },
    })
    expect(res).toBe('ALLOW')
    expect(spy).toHaveBeenCalled()
    expect(spy.mock.calls.at(0)?.[0]).toBeInstanceOf(IsAuthorizedCommand as any)
  })
})
