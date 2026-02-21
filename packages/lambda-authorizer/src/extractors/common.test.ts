import { describe, expect, it } from 'vitest'

import {
  getByPath,
  getFromJsonBody,
  parseUrlTemplate,
  substitute,
} from './common.js'

describe('common extractor helpers', () => {
  it('getByPath and getFromJsonBody cover direct and $.a.b forms', () => {
    const obj = { a: { b: { c: 1 } }, x: 2 }
    expect(getByPath(obj, 'a.b.c')).toBe(1)
    expect(getFromJsonBody(obj, '$.a.b.c')).toBe(1)
    expect(getFromJsonBody(obj, 'x')).toBe(2)
    expect(getByPath(undefined as any, 'a')).toBeUndefined()
    expect(getFromJsonBody(obj, undefined as any)).toBeUndefined()
  })

  it('parseUrlTemplate extracts and rejects on mismatch; substitute replaces vars', () => {
    expect(parseUrlTemplate('/foo/:id', '/foo/123')).toEqual({ id: '123' })
    expect(parseUrlTemplate('/foo/:id', '/bar/123')).toBeUndefined()
    expect(parseUrlTemplate('/foo/bar', '/foo/bar')).toEqual({})
    expect(substitute('id=$id', { id: 5 })).toBe('id=5')
    expect(parseUrlTemplate('', '/x')).toBeUndefined()
    expect(parseUrlTemplate('/a/:b', '/a')).toBeUndefined()
  })
})
