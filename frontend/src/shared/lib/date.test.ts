import { formatDateTime } from './date'

describe('formatDateTime', () => {
  it('renders in Vietnam time', () => {
    expect(formatDateTime('2026-10-26T00:03:00Z')).toBe('26 Oct 2026 07:03')
  })
  it('renders a dash for empty', () => {
    expect(formatDateTime(null)).toBe('—')
  })
})
