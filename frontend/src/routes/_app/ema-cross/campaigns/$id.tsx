import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'
import { PauseIcon, PencilIcon, PlayIcon, PlusIcon, SquareIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'

import {
  CHECK_TYPE_LABEL,
  CLOSE_REASON_LABEL,
  type EmaSettingTrade,
  emaSettingDetailQuery,
  TRADE_TYPE_LABEL,
  type TradeAction,
  useSaveSetting,
  useSettingAction,
  useTradeAction,
  useUpdateTrade,
} from '@/features/emasettings/api'
import { SettingStatusBadge, TradeStatusBadge } from '@/features/emasettings/components/badges'
import { SettingDialog } from '@/features/emasettings/components/SettingDialog'
import { TradeDialog } from '@/features/emasettings/components/TradeDialog'
import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { DataTable } from '@/shared/components/data-table'
import { Markdown } from '@/shared/components/markdown'
import { NumberInput } from '@/shared/components/number-input'
import { Money, Pct, Pnl, Price, Qty, Side } from '@/shared/components/numbers'
import { EmptyState, ErrorState, PageHeader, SkeletonRows } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'
import { formatDateTime } from '@/shared/lib/date'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/shared/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { Label } from '@/shared/ui/label'

export const Route = createFileRoute('/_app/ema-cross/campaigns/$id')({
  staticData: { title: 'Campaign', parent: { title: 'Campaigns', to: '/ema-cross/campaigns' } },
  component: CampaignDetail,
})

type Pending = { action: TradeAction | 'mark-closed'; trade: EmaSettingTrade }
const col = createColumnHelper<EmaSettingTrade>()
const isNum = (v: string) => /^\d+(\.\d+)?$/.test(v)

const ACTION_TITLE: Record<Pending['action'], string> = {
  'force-open': 'Open now at market',
  'force-close': 'Close now at market',
  'partial-close': 'Close part at market',
  split: 'Split into two trades',
  delete: 'Delete waiting trade',
  'mark-closed': 'Record as closed by hand',
}

function CampaignDetail() {
  const id = Number(Route.useParams().id)
  const navigate = useNavigate()
  const { data: s, error, refetch } = useQuery(emaSettingDetailQuery(id))
  const saveSetting = useSaveSetting()
  const settingAction = useSettingAction()
  const tradeAction = useTradeAction()
  const updateTrade = useUpdateTrade()
  const [editing, setEditing] = useState(false)
  const [tradeEdit, setTradeEdit] = useState<{ trade?: EmaSettingTrade } | null>(null)
  const [pending, setPending] = useState<Pending | null>(null)
  const [amount, setAmount] = useState('')
  const [confirm, setConfirm] = useState<'close' | 'delete' | null>(null)

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (!s) return <SkeletonRows rows={8} />
  const coin = s.pairName.replace(/USDT$/, '')
  const trades = s.trades ?? []
  const active = trades.filter((t) => t.status !== 'CLOSED')

  const ask = (action: Pending['action'], trade: EmaSettingTrade) => {
    setAmount('')
    setPending({ action, trade })
  }

  const columns = [
    col.accessor('id', {
      header: '#',
      cell: (c) => (
        <span className="num text-muted-foreground">
          {c.getValue()}
          {c.row.original.legacy && <span className="ml-1 text-[10px] uppercase">rails</span>}
        </span>
      ),
      meta: { mobile: 'title' },
    }),
    col.accessor('status', {
      header: 'Status',
      cell: (c) => <TradeStatusBadge status={c.getValue()} />,
      meta: { mobile: 'badge' },
    }),
    col.accessor('side', { header: 'Side', cell: (c) => <Side side={c.getValue()} /> }),
    col.accessor((t) => Number(t.amount), {
      id: 'amount',
      header: `Amount (${coin})`,
      cell: (c) => <Qty value={c.row.original.amount} />,
      meta: { align: 'right' },
    }),
    col.display({
      id: 'prices',
      header: 'Entry / target / SL',
      cell: (c) => {
        const t = c.row.original
        return (
          <span className="text-xs">
            {t.status === 'OPENING' ? (
              t.entryPrice ? (
                <Price value={t.entryPrice} />
              ) : (
                'on cross'
              )
            ) : (
              <Price value={t.openPrice} />
            )}
            {' / '}
            {t.targetPrice ? <Price value={t.targetPrice} /> : '—'}
            {' / '}
            {t.slPrice ? <Price value={t.slPrice} /> : '—'}
          </span>
        )
      },
      meta: { mobile: 'hidden' },
    }),
    col.accessor('closePrice', {
      header: 'Exit fill',
      cell: (c) => <Price value={c.getValue()} />,
      meta: { align: 'right', mobile: 'hidden' },
    }),
    col.accessor((t) => Number(t.status === 'OPEN' ? t.unrealizedProfit : t.realizedProfit), {
      id: 'pnl',
      header: 'PnL',
      cell: (c) => {
        const t = c.row.original
        if (t.status === 'OPENING') return <span className="text-muted-foreground">—</span>
        return <Pnl value={t.status === 'OPEN' ? t.unrealizedProfit : t.realizedProfit} />
      },
      meta: { align: 'right', mobile: 'primary' },
    }),
    col.display({
      id: 'info',
      header: 'Note',
      cell: (c) => {
        const t = c.row.original
        if (t.lastError)
          return <span className="line-clamp-2 max-w-60 text-xs text-destructive">{t.lastError}</span>
        if (t.status === 'CLOSED')
          return (
            <span className="text-xs text-muted-foreground">
              {CLOSE_REASON_LABEL[t.closeReason] ?? t.closeReason} {formatDateTime(t.closeAt)}
            </span>
          )
        return (
          <span className="text-xs text-muted-foreground">
            {t.isAuto ? 'auto' : 'by hand'} {t.openAt && `· ${formatDateTime(t.openAt)}`}
          </span>
        )
      },
      meta: { mobile: 'hidden' },
    }),
    col.display({
      id: 'actions',
      header: '',
      meta: { align: 'right', label: 'Actions' },
      cell: (c) => {
        const t = c.row.original
        if (t.status === 'CLOSED') return null
        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" variant="ghost">
                Actions
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setTradeEdit({ trade: t })}>
                <PencilIcon />
                Edit
              </DropdownMenuItem>
              {t.status === 'OPENING' ? (
                <>
                  <DropdownMenuItem onSelect={() => ask('force-open', t)}>Open now (market)</DropdownMenuItem>
                  <DropdownMenuItem className="text-destructive" onSelect={() => ask('delete', t)}>
                    <Trash2Icon />
                    Delete
                  </DropdownMenuItem>
                </>
              ) : (
                <>
                  <DropdownMenuItem onSelect={() => ask('force-close', t)}>
                    Close now (market)
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => ask('partial-close', t)}>
                    Close part (market)…
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => ask('split', t)}>Split…</DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onSelect={() => ask('mark-closed', t)}>
                    Closed by hand on Binance…
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    }),
  ]

  const needsAmount = !!pending && ['partial-close', 'split', 'mark-closed'].includes(pending.action)
  const amountOk =
    !pending ||
    !needsAmount ||
    (isNum(amount) &&
      Number(amount) > 0 &&
      (pending.action === 'mark-closed' || Number(amount) < Number(pending.trade.amount)))

  return (
    <>
      <PageHeader
        title={s.pairName}
        description={
          <span className="inline-flex flex-wrap items-center gap-2">
            <SettingStatusBadge status={s.status} />
            <Badge variant="outline">{s.period}</Badge>
            {TRADE_TYPE_LABEL[s.tradeType]} · {CHECK_TYPE_LABEL[s.checkType]} · {s.account} · price{' '}
            <Price value={s.currentPrice} updatedAt={s.priceUpdatedAt} />
          </span>
        }
        actions={
          s.status !== 'DONE' && (
            <>
              {s.status === 'ACTIVE' ? (
                <Button
                  variant="outline"
                  onClick={() => saveSetting.mutate({ id, body: { status: 'INACTIVE' } })}
                >
                  <PauseIcon />
                  Pause
                </Button>
              ) : (
                <Button
                  variant="outline"
                  onClick={() => saveSetting.mutate({ id, body: { status: 'ACTIVE' } })}
                >
                  <PlayIcon />
                  Resume
                </Button>
              )}
              <Button variant="outline" onClick={() => setEditing(true)}>
                <PencilIcon />
                Edit
              </Button>
              <Button variant="outline" className="text-destructive" onClick={() => setConfirm('close')}>
                <SquareIcon />
                End campaign
              </Button>
              <Button onClick={() => setTradeEdit({})}>
                <PlusIcon />
                Add trade
              </Button>
            </>
          )
        }
      />
      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard label="Budget per trade" value={<Money value={s.budget} />} />
        <StatCard
          label="Total PnL"
          value={<Pnl value={s.totalProfit} />}
          hint={<Pct value={s.totalPercent} />}
        />
        <StatCard label="Realized" value={<Pnl value={s.realizedProfit} />} hint={`${s.tradeCount} trades`} />
        <StatCard
          label="Unrealized"
          value={<Pnl value={s.unrealizedProfit} />}
          hint={`${active.length} active`}
        />
      </div>
      <div className="grid gap-6 2xl:grid-cols-4">
        <div className="2xl:col-span-3">
          <DataTable
            columns={columns}
            data={trades}
            getRowId={(t) => String(t.id)}
            rowClassName={(t) => (t.status === 'CLOSED' ? 'opacity-70' : undefined)}
            empty={
              <EmptyState
                title="No trade yet"
                description="The scan opens one on the next cross, or add one by hand."
              />
            }
          />
        </div>
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Campaign limits</CardTitle>
            </CardHeader>
            <CardContent className="space-y-1 text-sm">
              <p>Take profit: {Number(s.tpPercent) > 0 ? `+${s.tpPercent}% of the budget` : 'off'}</p>
              <p>Stop loss: {Number(s.slPercent) > 0 ? `−${s.slPercent}% of the budget` : 'off'}</p>
              <p>
                {s.nextTradeType === 'NEW_TRADE'
                  ? 'New trade on the next cross after a close'
                  : 'No new trade after a close'}
              </p>
            </CardContent>
          </Card>
          {s.note && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Note</CardTitle>
              </CardHeader>
              <CardContent>
                <Markdown>{s.note}</Markdown>
              </CardContent>
            </Card>
          )}
          {s.status === 'DONE' && trades.every((t) => t.status === 'CLOSED') && (
            <Button
              variant="outline"
              className="w-full text-destructive"
              onClick={() => setConfirm('delete')}
            >
              <Trash2Icon />
              Delete campaign
            </Button>
          )}
        </div>
      </div>

      <SettingDialog open={editing} onOpenChange={setEditing} setting={s} />
      {tradeEdit && (
        <TradeDialog
          open
          setting={s}
          trade={tradeEdit.trade}
          onOpenChange={(o) => !o && setTradeEdit(null)}
        />
      )}
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setPending(null)}
          destructive={pending.action !== 'force-open' && pending.action !== 'split'}
          title={`${ACTION_TITLE[pending.action]} · #${pending.trade.id} ${pending.trade.side} ${pending.trade.amount} ${coin}`}
          description={
            pending.action === 'force-open' ||
            pending.action === 'force-close' ||
            pending.action === 'partial-close'
              ? `A market order goes to Binance now at about ${s.currentPrice ?? '?'}.`
              : pending.action === 'split'
                ? 'The amount moves into a new trade with the same prices. No order.'
                : pending.action === 'mark-closed'
                  ? 'For a position already closed on Binance by hand: books the PnL at the price you give. No order.'
                  : 'The waiting trade is deleted. No order.'
          }
          confirmDisabled={!amountOk}
          confirmLabel={pending.action === 'delete' ? 'Delete' : 'Confirm'}
          onConfirm={() =>
            pending.action === 'mark-closed'
              ? updateTrade.mutateAsync({
                  id: pending.trade.id,
                  body: { status: 'CLOSED', closePrice: amount },
                })
              : tradeAction.mutateAsync({ id: pending.trade.id, action: pending.action, amount })
          }
        >
          {needsAmount && (
            <div className="grid gap-2">
              <Label htmlFor="es-action-amount">
                {pending.action === 'mark-closed'
                  ? 'Exit price'
                  : `Amount (${coin}, below ${pending.trade.amount})`}
              </Label>
              <NumberInput id="es-action-amount" value={amount} onChange={(e) => setAmount(e.target.value)} />
            </div>
          )}
        </ConfirmDialog>
      )}
      <ConfirmDialog
        open={confirm === 'close'}
        onOpenChange={(o) => !o && setConfirm(null)}
        destructive
        title={`End campaign ${s.pairName}?`}
        description={
          active.length > 0
            ? `Waiting trades are cancelled and ${active.filter((t) => t.status === 'OPEN').length} held trade(s) close at market now. The campaign becomes Done.`
            : 'Nothing is held. The campaign becomes Done.'
        }
        confirmLabel="End campaign"
        onConfirm={() => settingAction.mutateAsync({ id, action: 'close' })}
      />
      <ConfirmDialog
        open={confirm === 'delete'}
        onOpenChange={(o) => !o && setConfirm(null)}
        destructive
        title={`Delete campaign ${s.pairName}?`}
        description="The campaign and its trade history are deleted."
        typeToConfirm={s.pairName}
        confirmLabel="Delete"
        onConfirm={async () => {
          await settingAction.mutateAsync({ id, action: 'delete' })
          void navigate({ to: '/ema-cross/campaigns' })
        }}
      />
    </>
  )
}
