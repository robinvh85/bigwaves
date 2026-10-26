import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { AlertTriangleIcon, InfoIcon, ShieldOffIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'

import { type Ema1dStatus, pairsQuery } from '../api'

function Item({
  tone,
  title,
  children,
}: {
  tone: 'danger' | 'warn' | 'info'
  title: string
  children?: ReactNode
}) {
  const Icon = tone === 'info' ? InfoIcon : tone === 'warn' ? ShieldOffIcon : AlertTriangleIcon
  const cls =
    tone === 'danger'
      ? 'border-destructive/50 text-destructive [&>svg]:text-destructive'
      : tone === 'warn'
        ? 'border-warning/50 [&>svg]:text-warning'
        : ''
  return (
    <Alert className={cls}>
      <Icon />
      <AlertTitle>{title}</AlertTitle>
      {children && <AlertDescription>{children}</AlertDescription>}
    </Alert>
  )
}

/** What needs a look, most serious first. Renders nothing when all is well. */
export function Ema1dAlerts({ status }: { status: Ema1dStatus }) {
  const { data: pairs } = useQuery(pairsQuery)
  const failed = status.todos.filter((t) => t.status === 'FAILED')
  const mismatched = pairs?.filter((p) => p.mismatch) ?? []
  const ml = status.marginLevel.value ? Number(status.marginLevel.value) : undefined
  const items: ReactNode[] = []

  if (ml !== undefined && ml < Number(status.marginLevel.alert)) {
    items.push(
      <Item
        key="ml-alert"
        tone="danger"
        title={`Margin level ${ml.toFixed(2)} dưới ngưỡng báo động ${status.marginLevel.alert}`}
      >
        Rủi ro short squeeze / thanh lý. Cân nhắc giảm vị thế SHORT hoặc nạp thêm USDT.
      </Item>,
    )
  } else if (ml !== undefined && ml < Number(status.marginLevel.warn)) {
    items.push(
      <Item
        key="ml-warn"
        tone="warn"
        title={`Margin level ${ml.toFixed(2)} dưới ngưỡng cảnh báo ${status.marginLevel.warn}`}
      />,
    )
  }
  if (failed.length > 0) {
    items.push(
      <Item key="failed" tone="danger" title={`${failed.length} lệnh lỗi cần xử lý`}>
        {failed.map((t) => `${t.action} ${t.side} ${t.coin}`).join(', ')} —{' '}
        <Link to="/ema1d/todos" className="underline underline-offset-2">
          mở hàng đợi
        </Link>
      </Item>,
    )
  }
  if (!status.tradingEnabled) {
    items.push(
      <Item key="trading" tone="warn" title="TRADING_ENABLED đang tắt">
        Hệ thống chạy chế độ chỉ đọc: tín hiệu và todo vẫn được tính, nhưng không lệnh nào lên Binance.
      </Item>,
    )
  } else if (!status.enabled) {
    items.push(
      <Item key="enabled" tone="warn" title="Thực thi lệnh đang tắt">
        Todo được tạo nhưng không được chạy.
      </Item>,
    )
  }
  if (status.syncMissingToday) {
    items.push(
      <Item key="sync" tone="warn" title="Chưa có sync cho hôm nay">
        Đã quá 12:00 VN mà chưa có sync. Kiểm tra daily check ở tab Equity &amp; báo cáo.
      </Item>,
    )
  }
  if (mismatched.length > 0) {
    items.push(
      <Item key="mismatch" tone="info" title={`${mismatched.length} pair lệch với chiến lược`}>
        {mismatched.map((p) => p.coin).join(', ')} —{' '}
        <Link to="/ema1d/pairs" className="underline underline-offset-2">
          xem rổ coin
        </Link>
      </Item>,
    )
  }
  if (items.length === 0) return null
  return <div className="mb-4 grid gap-2 lg:grid-cols-2">{items}</div>
}
