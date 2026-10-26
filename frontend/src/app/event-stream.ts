import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { create } from 'zustand'

type StreamState = { connected: boolean; lastEventAt?: number }

export const useStreamStore = create<StreamState>(() => ({ connected: false }))

/**
 * Which query keys each server event invalidates. Features add their own entries as they are
 * built; keys are prefixes, so ['pairs'] refreshes every pairs query.
 */
const INVALIDATES: Record<string, readonly (readonly unknown[])[]> = {
  'job.finished': [['jobs']],
  'prices.updated': [['pairs'], ['account']],
}

export function registerEventInvalidation(type: string, keys: readonly (readonly unknown[])[]) {
  INVALIDATES[type] = [...(INVALIDATES[type] ?? []), ...keys]
}

/** Listens to /api/v1/events and refreshes the queries an event touches. Reconnects on its own. */
export function useEventStream() {
  const queryClient = useQueryClient()
  useEffect(() => {
    const es = new EventSource('/api/v1/events')
    es.onopen = () => {
      useStreamStore.setState({ connected: true })
    }
    es.onerror = () => {
      useStreamStore.setState({ connected: false })
    }
    es.onmessage = (msg: MessageEvent<string>) => {
      let type: string
      try {
        type = (JSON.parse(msg.data) as { type: string }).type
      } catch {
        return
      }
      useStreamStore.setState({ connected: true, lastEventAt: Date.now() })
      // Looked up on every event: features register their keys when their module loads.
      for (const key of INVALIDATES[type] ?? []) void queryClient.invalidateQueries({ queryKey: key })
    }
    return () => {
      es.close()
    }
  }, [queryClient])
}
