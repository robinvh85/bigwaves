import { Link } from '@tanstack/react-router'
import { AlertTriangleIcon, InfoIcon, ShieldOffIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'

import type { Ema1dStatus } from '../api'

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
  const failed = status.todos.filter((t) => t.status === 'FAILED')
  const ml = status.marginLevel.value ? Number(status.marginLevel.value) : undefined
  const items: ReactNode[] = []

  if (ml !== undefined && ml < Number(status.marginLevel.alert)) {
    items.push(
      <Item
        key="ml-alert"
        tone="danger"
        title={`Margin level ${ml.toFixed(2)} below the alert level ${status.marginLevel.alert}`}
      >
        Short squeeze / liquidation risk. Consider reducing SHORT positions or adding USDT.
      </Item>,
    )
  } else if (ml !== undefined && ml < Number(status.marginLevel.warn)) {
    items.push(
      <Item
        key="ml-warn"
        tone="warn"
        title={`Margin level ${ml.toFixed(2)} below the warning level ${status.marginLevel.warn}`}
      />,
    )
  }
  if (failed.length > 0) {
    items.push(
      <Item key="failed" tone="danger" title={`${failed.length} failed order(s) need attention`}>
        {failed.map((t) => `${t.action} ${t.side} ${t.coin}`).join(', ')} —{' '}
        <Link to="/ema1d/todos" className="underline underline-offset-2">
          open the queue
        </Link>
      </Item>,
    )
  }
  if (!status.tradingEnabled) {
    items.push(
      <Item key="trading" tone="warn" title="TRADING_ENABLED is off">
        Read-only mode: signals and todos are still computed, but no order reaches Binance.
      </Item>,
    )
  } else if (!status.enabled) {
    items.push(
      <Item key="enabled" tone="warn" title="Order execution is off">
        Todos are created but not run.
      </Item>,
    )
  }
  if (status.syncMissingToday) {
    items.push(
      <Item key="sync" tone="warn" title="No sync for today yet">
        It is past 12:00 ICT with no sync. Check the daily check under Equity &amp; reports.
      </Item>,
    )
  }
  if (items.length === 0) return null
  return <div className="mb-4 grid gap-2 lg:grid-cols-2">{items}</div>
}
