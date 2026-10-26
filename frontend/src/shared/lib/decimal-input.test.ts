import { describe, expect, it } from 'vitest'

import { sanitizeDecimal } from './decimal-input'

describe('sanitizeDecimal', () => {
  it.each([
    ['123.45', '123.45'],
    ['12a3b', '123'],
    ['1,5', '1.5'],
    ['1.2.3', '1.23'],
    ['-5', '5'],
    [' 1 000 ', '1000'],
    ['1e5', '15'],
  ])('%s → %s', (raw, want) => {
    expect(sanitizeDecimal(raw)).toBe(want)
  })

  it('keeps one leading minus when allowed', () => {
    expect(sanitizeDecimal('-1-2', true)).toBe('-12')
    expect(sanitizeDecimal('1-2', true)).toBe('12')
  })
})
