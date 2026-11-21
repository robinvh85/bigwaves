import { useQuery } from '@tanstack/react-query'
import { Loader2Icon } from 'lucide-react'
import { useState } from 'react'

import { pairsQuery } from '@/features/pairs/api'
import { Combobox } from '@/shared/components/combobox'
import { Button } from '@/shared/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/shared/ui/dialog'
import { Label } from '@/shared/ui/label'

import { type EmaCrossPair, useWatchPair } from '../api'

/** Adds a pair to the watchlist (no `pair`) or removes a watched one. */
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
  const [prev, setPrev] = useState<{ open: boolean; pair?: EmaCrossPair }>({ open: false })

  // Reset the form each time the dialog opens.
  if (prev.open !== open || prev.pair !== pair) {
    setPrev({ open, pair })
    if (open) setPairId(pair ? String(pair.pairId) : '')
  }

  const options = (pairs ?? [])
    .filter((p) => !watched.includes(p.id))
    .map((p) => ({ value: String(p.id), label: p.name }))

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{pair ? pair.pairName : 'Watch a pair'}</DialogTitle>
          <DialogDescription>
            {pair
              ? 'Remove this pair from the watchlist? Its D1 cross alerts stop.'
              : 'An alert goes to Discord when EMA14 of the close crosses Smooth10 on D1.'}
          </DialogDescription>
        </DialogHeader>
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
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant={pair ? 'destructive' : 'default'}
            disabled={!pairId || save.isPending}
            onClick={() =>
              save.mutate({ pairId: Number(pairId), watched: !pair }, { onSuccess: () => onOpenChange(false) })
            }
          >
            {save.isPending && <Loader2Icon className="animate-spin" />}
            {pair ? 'Remove' : 'Watch'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
