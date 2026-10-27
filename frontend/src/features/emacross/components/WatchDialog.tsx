import { useQuery } from '@tanstack/react-query'
import { Loader2Icon } from 'lucide-react'
import { useState } from 'react'

import { pairsQuery } from '@/features/pairs/api'
import { Combobox } from '@/shared/components/combobox'
import { Button } from '@/shared/ui/button'
import { Checkbox } from '@/shared/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/shared/ui/dialog'
import { Label } from '@/shared/ui/label'

import { type EmaCrossPair, TIMEFRAMES, useWatchPair, type Watch, watchKey } from '../api'

/**
 * Adds a pair to the watchlist (no `pair`) or changes its timeframes. Unticking every timeframe
 * removes the pair.
 */
export function WatchDialog({
  open,
  onOpenChange,
  pair,
  watched,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  pair?: EmaCrossPair
  /** Pair ids already on the list, left out of the picker. */
  watched: number[]
}) {
  const { data: pairs } = useQuery({ ...pairsQuery({ status: 'ACTIVE' }), enabled: open && !pair })
  const save = useWatchPair()
  const [pairId, setPairId] = useState('')
  const [flags, setFlags] = useState<Watch>({ h1: false, h4: true, d1: true })
  const [prev, setPrev] = useState<{ open: boolean; pair?: EmaCrossPair }>({ open: false })

  // Reset the form each time the dialog opens.
  if (prev.open !== open || prev.pair !== pair) {
    setPrev({ open, pair })
    if (open) {
      setPairId(pair ? String(pair.pairId) : '')
      setFlags(pair ? { h1: pair.h1, h4: pair.h4, d1: pair.d1 } : { h1: false, h4: true, d1: true })
    }
  }

  const options = (pairs ?? [])
    .filter((p) => !watched.includes(p.id))
    .map((p) => ({ value: String(p.id), label: p.name }))
  const none = !flags.h1 && !flags.h4 && !flags.d1

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{pair ? pair.pairName : 'Watch a pair'}</DialogTitle>
          <DialogDescription>
            An alert goes to Discord when EMA14 of the close crosses Smooth10 on a ticked timeframe.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {!pair && (
            <div className="grid gap-2">
              <Label htmlFor="emacross-pair">Pair</Label>
              <Combobox
                id="emacross-pair"
                options={options}
                value={pairId}
                onChange={setPairId}
                placeholder="Select a pair"
                searchPlaceholder="Search pairs…"
              />
            </div>
          )}
          <div className="grid gap-2">
            <Label>Timeframes</Label>
            <div className="flex gap-6">
              {TIMEFRAMES.map((tf) => (
                <label key={tf} className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={flags[watchKey(tf)]}
                    onCheckedChange={(v) => setFlags({ ...flags, [watchKey(tf)]: v === true })}
                  />
                  {tf}
                </label>
              ))}
            </div>
            {pair && none && (
              <p className="text-sm text-warning">No timeframe ticked: the pair leaves the watchlist.</p>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant={pair && none ? 'destructive' : 'default'}
            disabled={!pairId || (!pair && none) || save.isPending}
            onClick={() =>
              save.mutate({ pairId: Number(pairId), body: flags }, { onSuccess: () => onOpenChange(false) })
            }
          >
            {save.isPending && <Loader2Icon className="animate-spin" />}
            {pair && none ? 'Remove' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
