import { sortFromParam, sortToParam } from './sort-param'

describe('sort URL param', () => {
  it('round-trips', () => {
    const s = sortFromParam('-price,name')
    expect(s).toEqual([
      { id: 'price', desc: true },
      { id: 'name', desc: false },
    ])
    expect(sortToParam(s)).toBe('-price,name')
  })
  it('handles empty', () => {
    expect(sortFromParam(undefined)).toEqual([])
    expect(sortToParam([])).toBeUndefined()
  })
})
