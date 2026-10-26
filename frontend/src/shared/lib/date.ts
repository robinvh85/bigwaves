import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import timezone from 'dayjs/plugin/timezone'
import utc from 'dayjs/plugin/utc'

dayjs.extend(utc)
dayjs.extend(timezone)
dayjs.extend(relativeTime)

/** Strategy times (candle opens, daily checks) are read in Vietnam time (ICT). */
export const APP_TZ = 'Asia/Ho_Chi_Minh'

type DateInput = string | Date | null | undefined

export function formatDateTime(v: DateInput): string {
  if (!v) return '—'
  return dayjs(v).tz(APP_TZ).format('DD MMM YYYY HH:mm')
}

export function formatDate(v: DateInput): string {
  if (!v) return '—'
  return dayjs(v).tz(APP_TZ).format('DD MMM YYYY')
}

/** "3 minutes ago", "in 5 minutes". */
export function fromNow(v: DateInput): string {
  if (!v) return '—'
  return dayjs(v).fromNow()
}

export { dayjs }
