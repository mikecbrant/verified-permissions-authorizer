import { describe, expect, it } from 'vitest'

import { authenticate } from './authenticate.js'

describe('authenticate()', () => {
  it('throws on invalid token', () => {
    expect(() => authenticate('not-a-jwt', 's')).toThrow('Invalid or expired JWT')
  })
})
