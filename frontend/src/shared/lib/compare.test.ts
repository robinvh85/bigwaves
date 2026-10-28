import { compareCellValues } from './compare'

describe('compareCellValues', () => {
  it('compares decimal strings as numbers', () => {
    expect(['3', '-5', '0.0012', '0.012', '10', '12%'].sort(compareCellValues)).toEqual([
      '-5',
      '0.0012',
      '0.012',
      '3',
      '10',
      '12%',
    ])
  })

  it('puts empty values first and compares text', () => {
    expect(['b', '', 'A', null].sort(compareCellValues)).toEqual(['', null, 'A', 'b'])
  })
})
