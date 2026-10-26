import { Loader2Icon, PencilIcon } from 'lucide-react'
import { useState } from 'react'

import { Markdown } from '@/shared/components/markdown'
import { Button } from '@/shared/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/shared/ui/tabs'
import { Textarea } from '@/shared/ui/textarea'

import { type Pair, useUpdateCoinDescription } from './api'

export function CoinDescription({ pair }: { pair: Pair }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(pair.coin.description)
  const save = useUpdateCoinDescription(pair.id)

  if (!editing) {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium">Ghi chú về {pair.coin.name}</h3>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setDraft(pair.coin.description)
              setEditing(true)
            }}
          >
            <PencilIcon />
            Sửa
          </Button>
        </div>
        {pair.coin.description ? (
          <Markdown>{pair.coin.description}</Markdown>
        ) : (
          <p className="text-sm text-muted-foreground">Chưa có ghi chú. Hỗ trợ markdown.</p>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium">Ghi chú về {pair.coin.name}</h3>
      <Tabs defaultValue="write">
        <TabsList>
          <TabsTrigger value="write">Viết</TabsTrigger>
          <TabsTrigger value="preview">Xem trước</TabsTrigger>
        </TabsList>
        <TabsContent value="write">
          <Textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            rows={14}
            className="font-mono text-sm"
            aria-label="Ghi chú markdown"
          />
        </TabsContent>
        <TabsContent value="preview" className="min-h-40 rounded-md border p-3">
          {draft ? <Markdown>{draft}</Markdown> : <p className="text-sm text-muted-foreground">Trống</p>}
        </TabsContent>
      </Tabs>
      <div className="flex justify-end gap-2">
        <Button variant="outline" onClick={() => setEditing(false)} disabled={save.isPending}>
          Huỷ
        </Button>
        <Button
          disabled={save.isPending || draft === pair.coin.description}
          onClick={() =>
            save.mutate({ coinId: pair.coin.id, description: draft }, { onSuccess: () => setEditing(false) })
          }
        >
          {save.isPending && <Loader2Icon className="animate-spin" />}
          Lưu
        </Button>
      </div>
    </div>
  )
}
