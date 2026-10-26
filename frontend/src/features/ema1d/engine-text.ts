/**
 * The strategy engine writes its status rows in Vietnamese: they must match the original Python
 * script byte for byte (golden test) and the legacy /api/ema1d/status consumers. The UI shows them
 * in English, so the fixed phrases are translated here, at display time only.
 */
const TERMS: Record<string, string> = {
  'ĐỨNG NGOÀI': 'FLAT',
  'ĐÃ THOÁT XONG': 'EXITED',
  TĂNG: 'UP',
  GIẢM: 'DOWN',
  LÊN: 'UP',
  XUỐNG: 'DOWN',
  RỔ: 'BASKET',
  MỚI: 'NEW',
  'RỔ CŨ': 'OLD BASKET',
}

/** A one-word engine value (position, EMA direction, group). Unknown values pass through. */
export function engineTerm(v: string): string {
  return TERMS[v] ?? v
}

// Longest phrases first: several share a prefix.
const PHRASES: [RegExp, string][] = [
  [/ là trước ngày thêm, không theo/g, ' predates the add date, not followed'],
  [/lệnh chiến lược /g, 'strategy trade '],
  [/thêm vào rổ /g, 'added to basket '],
  [/, chỉ vào ở tín hiệu mới/g, ', enters on new signals only'],
  [/ — chưa có tín hiệu mới/g, ' — no new signal yet'],
  [/loại khỏi rổ /g, 'removed from basket '],
  [/; không tìm thấy lệnh đang theo — không vào lại/g, '; no followed trade found — no re-entry'],
  [/; chỉ chờ tín hiệu THOÁT \(/g, '; waiting for the EXIT signal only ('],
  [/\), không vào lệnh mới/g, '), no new entries'],
  [/ — rổ cũ, không vào lại/g, ' — old basket, no re-entry'],
  [/; đã thoát xong /g, '; fully exited '],
  [/, không vào lại \(dọn ở lần tái chọn sau\)/g, ', no re-entry (cleaned up at the next reselection)'],
  [/ → thoát /g, ' → exit '],
  [/lệnh cuối /g, 'last trade '],
  [/ĐÓNG /g, 'CLOSE '],
  [/VÀO /g, 'OPEN '],
  [/ từ /g, ' from '],
  [/nhiều ngày nến trong status: /g, 'several candle dates in the status: '],
  [
    /ngày nến (\S+) khác hôm nay UTC (\S+) — dữ liệu chưa cập nhật\?/g,
    'candle date $1 is not today UTC $2 — data not updated?',
  ],
  [/: không có giá open ngày vào /g, ': no open price on entry date '],
  [/: không có giá open ngày /g, ': no open price on '],
]

/** Free engine text: actions, notes, warnings. */
export function engineText(v: string): string {
  return PHRASES.reduce((s, [re, to]) => s.replace(re, to), v)
}
