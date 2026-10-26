import { describe, expect, it } from 'vitest'

import { engineTerm, engineText } from './engine-text'

describe('engine text', () => {
  it('maps fixed terms', () => {
    expect(engineTerm('ĐỨNG NGOÀI')).toBe('FLAT')
    expect(engineTerm('RỔ CŨ')).toBe('OLD BASKET')
    expect(engineTerm('LONG')).toBe('LONG')
  })

  it.each([
    ['ĐÓNG LONG (XO) +3.7% · VÀO SHORT @ 1.23', 'CLOSE LONG (XO) +3.7% · OPEN SHORT @ 1.23'],
    ['2026-10-01 → thoát 2026-10-09', '2026-10-01 → exit 2026-10-09'],
    ['lệnh cuối -2.1%', 'last trade -2.1%'],
    [
      'thêm vào rổ 2026-10-20, chỉ vào ở tín hiệu mới — chưa có tín hiệu mới (lệnh chiến lược LONG từ 2026-10-02 là trước ngày thêm, không theo)',
      'added to basket 2026-10-20, enters on new signals only — no new signal yet (strategy trade LONG from 2026-10-02 predates the add date, not followed)',
    ],
    [
      'loại khỏi rổ 2026-10-01; đã thoát xong 2026-10-05, không vào lại (dọn ở lần tái chọn sau)',
      'removed from basket 2026-10-01; fully exited 2026-10-05, no re-entry (cleaned up at the next reselection)',
    ],
  ])('%s', (vi, en) => {
    expect(engineText(vi)).toBe(en)
  })
})
