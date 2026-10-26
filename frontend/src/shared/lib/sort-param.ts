import type { SortingState } from '@tanstack/react-table'

/** "-name" ⇄ [{ id: 'name', desc: true }] for keeping sort order in the URL. */
export function sortFromParam(param: string | undefined): SortingState {
  if (!param) return []
  return param
    .split(',')
    .filter(Boolean)
    .map((p) => (p.startsWith('-') ? { id: p.slice(1), desc: true } : { id: p, desc: false }))
}

export function sortToParam(s: SortingState): string | undefined {
  if (s.length === 0) return undefined
  return s.map((x) => (x.desc ? `-${x.id}` : x.id)).join(',')
}
