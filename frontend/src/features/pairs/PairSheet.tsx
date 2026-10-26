import { useQuery } from '@tanstack/react-query'
import { Trash2Icon } from 'lucide-react'
import { useState } from 'react'

import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { Price, Qty } from '@/shared/components/numbers'
import { ErrorState, SkeletonRows } from '@/shared/components/states'
import { formatDateTime, fromNow } from '@/shared/lib/date'
import { formatPct } from '@/shared/lib/format'
import { Button } from '@/shared/ui/button'
import { Separator } from '@/shared/ui/separator'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/shared/ui/sheet'
import { Label } from '@/shared/ui/label'
import { Switch } from '@/shared/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

import {
  ACCOUNT_HINT,
  ACCOUNT_OPTIONS,
  type AccountType,
  accountFlags,
  accountLabel,
  accountOf,
} from './account'
import { pairQuery, useDeletePair, useUpdatePair } from './api'
import { CoinDescription } from './CoinDescription'
import { AccountTypeLabel, PairStatusBadge } from './PairStatusBadge'

function Info({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium">{children}</span>
    </div>
  )
}

export function PairSheet({ pairId, onClose }: { pairId: number | undefined; onClose: () => void }) {
  return (
    <Sheet
      open={pairId !== undefined}
      onOpenChange={(o) => {
        if (!o) onClose()
      }}
    >
      <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
        {pairId !== undefined && <PairDetail id={pairId} onDeleted={onClose} />}
      </SheetContent>
    </Sheet>
  )
}

function PairDetail({ id, onDeleted }: { id: number; onDeleted: () => void }) {
  const { data: pair, error, refetch } = useQuery(pairQuery(id))
  const update = useUpdatePair(id)
  const del = useDeletePair()
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [nextAccount, setNextAccount] = useState<AccountType | null>(null)

  if (error) {
    return (
      <div className="p-4">
        <ErrorState error={error} onRetry={() => void refetch()} />
      </div>
    )
  }
  if (!pair) {
    return (
      <div className="p-4">
        <SkeletonRows rows={8} />
      </div>
    )
  }

  return (
    <>
      <SheetHeader>
        <SheetTitle className="flex items-center gap-2 font-mono">
          {pair.name} <PairStatusBadge status={pair.status} />
        </SheetTitle>
        <SheetDescription>
          {pair.coin.name} / {pair.quoteAsset} ·{' '}
          <AccountTypeLabel isSpot={pair.isSpot} isIsolated={pair.isIsolated} />
        </SheetDescription>
      </SheetHeader>

      <div className="space-y-4 px-4 pb-6">
        <div className="rounded-lg border p-3">
          <Info label="Current price">
            <Price value={pair.currentPrice} />
          </Info>
          <Info label="Price updated">
            <span title={formatDateTime(pair.priceUpdatedAt)}>{fromNow(pair.priceUpdatedAt)}</span>
          </Info>
          <Info label="Interest / year">
            {pair.interestRate ? formatPct(pair.interestRate, { sign: false }) : '—'}
          </Info>
          <Info label="Step size">
            <Qty value={pair.stepSize} />
          </Info>
          <Info label="Min notional">{pair.minNotional ? `${pair.minNotional} USDT` : '—'}</Info>
        </div>

        <div className="flex items-center justify-between rounded-lg border p-3">
          <div>
            <Label htmlFor="pair-active">Active</Label>
            <p className="text-xs text-muted-foreground">
              A disabled pair still gets price updates, but strategies open no new trades on it.
            </p>
          </div>
          <Switch
            id="pair-active"
            checked={pair.status === 'ACTIVE'}
            disabled={update.isPending}
            onCheckedChange={(on) => update.mutate({ status: on ? 'ACTIVE' : 'DISABLED' })}
          />
        </div>

        <div className="space-y-2 rounded-lg border p-3">
          <Label id="pair-account">Trade on</Label>
          <ToggleGroup
            type="single"
            variant="outline"
            aria-labelledby="pair-account"
            value={accountOf(pair)}
            disabled={update.isPending}
            onValueChange={(v) => {
              if (v && v !== accountOf(pair)) setNextAccount(v as AccountType)
            }}
          >
            {ACCOUNT_OPTIONS.map((o) => (
              <ToggleGroupItem key={o.value} value={o.value}>
                {o.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <p className="text-xs text-muted-foreground">
            {ACCOUNT_HINT} Can only change while no strategy holds coins on this pair.
          </p>
        </div>

        <Separator />
        <CoinDescription key={pair.coin.id} pair={pair} />
        <Separator />

        <Button variant="outline" className="w-full text-destructive" onClick={() => setConfirmDelete(true)}>
          <Trash2Icon />
          Delete pair
        </Button>
      </div>

      {nextAccount && (
        <ConfirmDialog
          open
          onOpenChange={(o) => {
            if (!o) setNextAccount(null)
          }}
          title={`Trade ${pair.name} on ${accountLabel(nextAccount)}?`}
          description={`From now on, every order of every strategy on ${pair.name} goes to the ${accountLabel(nextAccount)} account. Refused while an EMA1D position or a bought DCA rung still holds coins here.${nextAccount !== 'CROSS' ? ' EMA1D will not open new positions on this pair.' : ''}`}
          confirmLabel="Change account"
          onConfirm={() => update.mutateAsync(accountFlags(nextAccount))}
        />
      )}

      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        destructive
        title={`Delete ${pair.name}?`}
        description="The pair is hidden and takes no new trades. Trade history is kept. Adding the same symbol again restores it."
        typeToConfirm={pair.name}
        confirmLabel="Delete"
        onConfirm={async () => {
          await del.mutateAsync(pair.id)
          onDeleted()
        }}
      />
    </>
  )
}
