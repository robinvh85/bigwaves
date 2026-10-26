import { useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCwIcon, WalletIcon } from 'lucide-react'
import { useState } from 'react'

import { api, unwrap } from '@/shared/api/client'
import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { Money } from '@/shared/components/numbers'
import { ErrorState } from '@/shared/components/states'
import { fromNow } from '@/shared/lib/date'
import { formatMoney } from '@/shared/lib/format'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { NumberInput } from '@/shared/components/number-input'
import { Label } from '@/shared/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/ui/popover'
import { Separator } from '@/shared/ui/separator'
import { Skeleton } from '@/shared/ui/skeleton'

import { type AccountAction, accountKeys, balancesQuery, useAccountAction } from './api'
import { marginLevelClass } from './margin'

const ACTIONS: Record<AccountAction, { label: string; verb: string }> = {
  borrow: { label: 'Borrow USDT', verb: 'Borrow' },
  repay: { label: 'Repay USDT', verb: 'Repay' },
  spotToCross: { label: 'Spot → Cross', verb: 'Transfer' },
  crossToSpot: { label: 'Cross → Spot', verb: 'Transfer' },
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{children}</span>
    </div>
  )
}

export function AccountPopover() {
  const [open, setOpen] = useState(false)
  const qc = useQueryClient()
  const { data, error, isLoading, isFetching, refetch } = useQuery({ ...balancesQuery, enabled: open })
  const [action, setAction] = useState<AccountAction | null>(null)
  const [amount, setAmount] = useState('')
  const mutation = useAccountAction()
  const amountValid = /^\d+(\.\d+)?$/.test(amount) && Number(amount) > 0

  const refresh = async () => {
    const fresh = await unwrap(api.GET('/account/balances', { params: { query: { refresh: true } } }))
    qc.setQueryData(accountKeys.balances(), fresh)
  }

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button variant="ghost" size="sm" className="gap-2" aria-label="Account balances">
            <WalletIcon />
            <span className="hidden sm:inline">Account</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="w-80">
          <div className="mb-3 flex items-center justify-between">
            <p className="font-medium">Binance</p>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              onClick={() => void refresh()}
              aria-label="Refresh balances"
              disabled={isFetching}
            >
              <RefreshCwIcon className={cn(isFetching && 'animate-spin')} />
            </Button>
          </div>
          {error ? (
            <ErrorState error={error} onRetry={() => void refetch()} />
          ) : isLoading || !data ? (
            <div className="space-y-2">
              {Array.from({ length: 6 }, (_, i) => (
                <Skeleton key={i} className="h-5 w-full" />
              ))}
            </div>
          ) : (
            <div className="space-y-2">
              <Row label="Total net assets">
                <Money value={data.netTotalUsdt} />
              </Row>
              <Row label="Margin level">
                <span className={cn('num', marginLevelClass(data.marginLevel))}>
                  {formatMoney(data.marginLevel)}
                </span>
              </Row>
              <Separator />
              <Row label="Spot USDT available">
                <Money value={data.spotUsdtFree} />
              </Row>
              <Row label="Cross USDT available">
                <Money value={data.crossUsdtFree} />
              </Row>
              <Row label="USDT debt (principal + interest)">
                <Money value={data.crossUsdtDebt} />
              </Row>
              <Row label="Total debt in USDT">
                <Money value={data.crossDebtUsdt} />
              </Row>
              <Separator />
              <div className="grid grid-cols-2 gap-2">
                {(Object.keys(ACTIONS) as AccountAction[]).map((a) => (
                  <Button
                    key={a}
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      setAmount('')
                      setAction(a)
                    }}
                  >
                    {ACTIONS[a].label}
                  </Button>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">Updated {fromNow(data.fetchedAt)}</p>
            </div>
          )}
        </PopoverContent>
      </Popover>

      {action && (
        <ConfirmDialog
          open
          onOpenChange={(o) => {
            if (!o) setAction(null)
          }}
          title={ACTIONS[action].label}
          description="This is sent straight to Binance and cannot be undone."
          confirmLabel={`${ACTIONS[action].verb} ${amountValid ? `${formatMoney(amount)} USDT` : ''}`}
          confirmDisabled={!amountValid}
          onConfirm={async () => {
            if (!amountValid) throw new Error('invalid amount')
            await mutation.mutateAsync({ action, amount })
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="account-amount">Amount (USDT)</Label>
            <NumberInput
              id="account-amount"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              aria-invalid={amount !== '' && !amountValid}
            />
          </div>
        </ConfirmDialog>
      )}
    </>
  )
}
