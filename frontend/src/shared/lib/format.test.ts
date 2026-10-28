import { formatMoney, formatPct, formatPrice, formatQty, percentOf, signOf } from './format'

describe('formatMoney', () => {
  it('groups thousands and keeps 2 decimals', () => {
    expect(formatMoney('1234.5')).toBe('1,234.50')
    expect(formatMoney('-1234567.891')).toBe('-1,234,567.89')
  })
  it('renders a dash for empty values', () => {
    expect(formatMoney(null)).toBe('—')
    expect(formatMoney('')).toBe('—')
    expect(formatMoney('abc')).toBe('—')
  })
  it('does not lose precision on large values', () => {
    expect(formatMoney('12345678901234567.12')).toBe('12,345,678,901,234,567.12')
  })
})

describe('formatPrice', () => {
  it('adapts decimals to the magnitude', () => {
    expect(formatPrice('64123.456')).toBe('64,123.46')
    expect(formatPrice('1.23456')).toBe('1.2346')
    expect(formatPrice('0.00001234')).toBe('0.00001234')
    expect(formatPrice('0')).toBe('0')
  })
})

describe('formatQty', () => {
  it('rounds down to the step size', () => {
    expect(formatQty('1.23456', '0.01')).toBe('1.23')
    expect(formatQty('1234.9', '1')).toBe('1,234')
  })
})

describe('formatPct', () => {
  it('adds a sign for gains', () => {
    expect(formatPct('5.123')).toBe('+5.12%')
    expect(formatPct('-0.5')).toBe('-0.50%')
    expect(formatPct('0.0512', { ratio: true })).toBe('+5.12%')
  })
})

describe('signOf', () => {
  it('returns the sign', () => {
    expect(signOf('-0.1')).toBe(-1)
    expect(signOf('0')).toBe(0)
    expect(signOf('3')).toBe(1)
  })
})

describe('percentOf', () => {
  it('divides with decimals, not floats', () => {
    expect(percentOf('1.5', '30')).toBe('5')
    expect(percentOf('-0.3', '10')).toBe('-3')
  })
  it('has no percent without a base', () => {
    expect(percentOf('1', '0')).toBeNull()
    expect(percentOf('1', undefined)).toBeNull()
  })
})
