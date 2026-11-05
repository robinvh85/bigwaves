import { describe, expect, it } from 'vitest'

import { amountForPercent, percentOfTotal } from './amount-percent'

describe('amountForPercent', () => {
  it('rounds down to the quantity step', () => {
    expect(amountForPercent('1235', 50, '1')).toBe('617')
    expect(amountForPercent('0.123', 50, '0.001')).toBe('0.061')
    expect(amountForPercent('10', 25, '0.5')).toBe('2.5')
  })

  it("falls back to total's decimals without a step", () => {
    expect(amountForPercent('1235', 50)).toBe('617')
    expect(amountForPercent('0.123', 75)).toBe('0.092')
  })
})

describe('percentOfTotal', () => {
  it('returns the share in percent, clamped to 0–100', () => {
    expect(percentOfTotal('25', '100')).toBe(25)
    expect(percentOfTotal('150', '100')).toBe(100)
    expect(percentOfTotal('', '100')).toBe(0)
    expect(percentOfTotal('1.', '100')).toBe(0)
  })
})
