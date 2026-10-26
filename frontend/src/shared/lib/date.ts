import dayjs from 'dayjs'
import 'dayjs/locale/vi'
import relativeTime from 'dayjs/plugin/relativeTime'
import timezone from 'dayjs/plugin/timezone'
import utc from 'dayjs/plugin/utc'

dayjs.extend(utc)
dayjs.extend(timezone)
dayjs.extend(relativeTime)
dayjs.locale('vi')

/** Strategy times (candle opens, daily checks) are read in Vietnam time. */
export const APP_TZ = 'Asia/Ho_Chi_Minh'

type DateInput = string | Date | null | undefined

export function formatDateTime(v: DateInput): string {
  if (!v) return '—'
  return dayjs(v).tz(APP_TZ).format('DD/MM/YYYY HH:mm')
}

export function formatDate(v: DateInput): string {
  if (!v) return '—'
  return dayjs(v).tz(APP_TZ).format('DD/MM/YYYY')
}

/** "3 phút trước", "trong 5 phút". */
export function fromNow(v: DateInput): string {
  if (!v) return '—'
  return dayjs(v).fromNow()
}

export { dayjs }
