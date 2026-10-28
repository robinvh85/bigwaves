import { formatDate, formatDateTime } from './date'

describe('formatDateTime', () => {
  it('renders in Vietnam time', () => {
    expect(formatDateTime('2026-10-26T00:03:00Z')).toBe('2026/10/26 07:03')
  })
  it('renders a dash for empty', () => {
    expect(formatDateTime(null)).toBe('—')
  })
})

describe('formatDate', () => {
  it('renders YYYY/MM/DD in Vietnam time', () => {
    expect(formatDate('2026-10-20T18:00:00Z')).toBe('2026/10/21')
  })
})
