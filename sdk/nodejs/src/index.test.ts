import { describe, expect, it, vi } from 'vitest'

vi.mock('@pulumi/pulumi', () => ({
  // minimal mock to allow class construction in tests
  ComponentResource: class ComponentResource {
    constructor() {}
  },
}))

import { AuthorizerWithPolicyStore } from './index.js'

describe('sdk exports', () => {
  it('exports AuthorizerWithPolicyStore class', () => {
    const c = new AuthorizerWithPolicyStore('name')
    expect(c).toBeInstanceOf(AuthorizerWithPolicyStore)
  })
})
