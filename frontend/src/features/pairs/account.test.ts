import { describe, expect, it } from 'vitest'

import { accountFlags, accountOf } from './account'

describe('pair account', () => {
  it.each(['CROSS', 'ISOLATED', 'SPOT'] as const)('%s round-trips through the flags', (a) => {
    expect(accountOf(accountFlags(a))).toBe(a)
  })
})
