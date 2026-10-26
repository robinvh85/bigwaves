import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Loader2Icon, PlusIcon } from 'lucide-react'
import { useState } from 'react'

import { pairsQuery } from '@/features/pairs/api'
import { Button } from '@/shared/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/shared/ui/dialog'
import { Label } from '@/shared/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Textarea } from '@/shared/ui/textarea'

import { useCreateSetting } from '../api'

export function CreateLadderDialog() {
  const [open, setOpen] = useState(false)
  const [pairId, setPairId] = useState<string>('')
  const [note, setNote] = useState('')
  const { data: pairs } = useQuery(pairsQuery({ status: 'ACTIVE' }))
  const create = useCreateSetting()
  const navigate = useNavigate()

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <PlusIcon />
          New ladder
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create DCA ladder</DialogTitle>
          <DialogDescription>
            Pick a pair, then add rungs: the buy price, the sell price and how many coins.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid gap-2">
            <Label>Pair</Label>
            <Select value={pairId} onValueChange={setPairId}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select a pair" />
              </SelectTrigger>
              <SelectContent>
                {pairs?.map((p) => (
                  <SelectItem key={p.id} value={String(p.id)}>
                    {p.name} · {p.isSpot ? 'Spot' : p.isIsolated ? 'Isolated' : 'Cross'}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="dca-note">Note</Label>
            <Textarea id="dca-note" rows={3} value={note} onChange={(e) => setNote(e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            Cancel
          </Button>
          <Button
            disabled={!pairId || create.isPending}
            onClick={() =>
              create.mutate(
                { pairId: Number(pairId), note },
                {
                  onSuccess: (s) => {
                    setOpen(false)
                    void navigate({ to: '/dca/$id', params: { id: String(s.id) } })
                  },
                },
              )
            }
          >
            {create.isPending && <Loader2Icon className="animate-spin" />}
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
