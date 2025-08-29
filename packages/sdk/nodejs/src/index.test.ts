import { describe, expect, it } from 'vitest'

import { AuthorizerWithPolicyStore } from './index.js'

describe('SDK exports', () => {
  it('exports AuthorizerWithPolicyStore', () => {
    expect(typeof AuthorizerWithPolicyStore).toBe('function')
  })
})
