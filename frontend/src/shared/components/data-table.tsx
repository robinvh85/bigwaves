import {
  type ColumnDef,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  type Row,
  type RowData,
  type SortingState,
  useReactTable,
} from '@tanstack/react-table'
import { ArrowDownIcon, ArrowUpDownIcon, ArrowUpIcon, ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'

import { compareCellValues } from '@/shared/lib/compare'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { Skeleton } from '@/shared/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/ui/table'
import { useIsMobile } from '@/shared/hooks/use-mobile'
import { EmptyState, ErrorState } from '@/shared/components/states'

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    /** Right-align numbers. */
    align?: 'left' | 'right'
    /**
     * How the column shows up in the mobile card: `title` (card heading), `badge` (next to the
     * title), `primary` (big value on the right), `field` (label + value grid, the default) or
     * `hidden`.
     */
    mobile?: 'title' | 'badge' | 'primary' | 'field' | 'hidden'
    /** Label used in the mobile card when the header is not plain text. */
    label?: string
    className?: string
  }
}

export type DataTableProps<T> = {
  // Columns of one table have different value types; TanStack's own docs type the array with any.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  columns: ColumnDef<T, any>[]
  data: T[] | undefined
  isLoading?: boolean
  error?: unknown
  onRetry?: () => void
  getRowId?: (row: T) => string
  onRowClick?: (row: T) => void
  /**
   * With `onSortingChange` the caller owns the sort order (e.g. kept in the URL); without it,
   * `sorting` is only the initial order and the table sorts on header clicks by itself.
   */
  sorting?: SortingState
  onSortingChange?: (s: SortingState) => void
  globalFilter?: string
  /** Rows per page; lists shorter than this show no pagination. */
  pageSize?: number
  empty?: ReactNode
  rowClassName?: (row: T) => string | undefined
}

const NO_SORTING: SortingState = []

function sameSorting(a: SortingState, b: SortingState) {
  return (
    a.length === b.length &&
    a.every((s, i) => {
      const t = b[i]
      return t !== undefined && s.id === t.id && s.desc === t.desc
    })
  )
}

/**
 * Callers pass `sorting` as a literal (`sorting={[{ id: 'pnl', desc: true }]}`), a new array on
 * every render. TanStack Table recomputes the sorted rows on a new array and, with
 * autoResetPageIndex, resets the page — another render, another array: the tab freezes. Keep the
 * previous array while the value is the same.
 */
function useStableSorting(sorting: SortingState | undefined): SortingState {
  const next = sorting ?? NO_SORTING
  const [stable, setStable] = useState(next)
  if (stable !== next && !sameSorting(stable, next)) {
    setStable(next)
    return next
  }
  return stable
}

/**
 * Money and prices come from the API as decimal strings; TanStack's default would sort them as
 * text ("-5" after "3", "0.0012" equal to "0.012"). Compare them as numbers.
 */
function sortByValue<T>(a: Row<T>, b: Row<T>, id: string) {
  return compareCellValues(a.getValue(id), b.getValue(id))
}

export function DataTable<T>({
  columns,
  data,
  isLoading,
  error,
  onRetry,
  getRowId,
  onRowClick,
  sorting,
  onSortingChange,
  globalFilter,
  pageSize = 50,
  empty,
  rowClassName,
}: DataTableProps<T>) {
  const isMobile = useIsMobile()
  const stableSorting = useStableSorting(sorting)
  const [ownSorting, setOwnSorting] = useState(stableSorting)
  const controlled = onSortingChange !== undefined
  // One object for the table's lifetime: a new defaultColumn rebuilds the columns and the rows.
  const [defaultColumn] = useState(() => ({ sortingFn: sortByValue<T> }))
  const currentSorting = controlled ? stableSorting : ownSorting
  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Table is not compiler-safe; this component is not memoized by the compiler.
  const table = useReactTable({
    data: data ?? [],
    columns,
    getRowId,
    defaultColumn,
    state: { sorting: currentSorting, globalFilter },
    onSortingChange: (updater) => {
      const next = typeof updater === 'function' ? updater(currentSorting) : updater
      if (onSortingChange) onSortingChange(next)
      else setOwnSorting(next)
    },
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize } },
    autoResetPageIndex: true,
  })

  if (error) return <ErrorState error={error} onRetry={onRetry} />

  if (isLoading && !data) {
    return (
      <div className="space-y-2" aria-busy="true" aria-label="Loading">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} className="h-10 w-full" />
        ))}
      </div>
    )
  }

  const rows = table.getRowModel().rows
  if (rows.length === 0) {
    return <>{empty ?? <EmptyState title={globalFilter ? 'No matching results' : 'No data yet'} />}</>
  }

  const pager = table.getPageCount() > 1 && <Pager table={table} />

  if (isMobile) {
    return (
      <div className="space-y-2">
        {rows.map((row) => (
          <MobileCard key={row.id} row={row} onClick={onRowClick} className={rowClassName?.(row.original)} />
        ))}
        {pager}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <div className="overflow-x-auto rounded-lg border">
        <Table>
          <TableHeader className="sticky top-0 z-[1] bg-muted/50 backdrop-blur">
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id} className="hover:bg-transparent">
                {hg.headers.map((header) => {
                  const meta = header.column.columnDef.meta
                  const canSort = header.column.getCanSort()
                  const dir = header.column.getIsSorted()
                  const label = header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())
                  return (
                    <TableHead
                      key={header.id}
                      className={cn('h-9 text-xs', meta?.align === 'right' && 'text-right', meta?.className)}
                      aria-sort={dir === 'asc' ? 'ascending' : dir === 'desc' ? 'descending' : undefined}
                    >
                      {canSort ? (
                        <button
                          type="button"
                          className={cn(
                            'inline-flex items-center gap-1 hover:text-foreground',
                            meta?.align === 'right' && 'flex-row-reverse',
                          )}
                          onClick={header.column.getToggleSortingHandler()}
                        >
                          {label}
                          {dir === 'asc' ? (
                            <ArrowUpIcon className="size-3" />
                          ) : dir === 'desc' ? (
                            <ArrowDownIcon className="size-3" />
                          ) : (
                            <ArrowUpDownIcon className="size-3 opacity-40" />
                          )}
                        </button>
                      ) : (
                        label
                      )}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={row.id}
                className={cn(onRowClick && 'cursor-pointer', rowClassName?.(row.original))}
                onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                onKeyDown={
                  onRowClick
                    ? (e) => {
                        if (e.key === 'Enter') onRowClick(row.original)
                      }
                    : undefined
                }
                tabIndex={onRowClick ? 0 : undefined}
              >
                {row.getVisibleCells().map((cell) => {
                  const meta = cell.column.columnDef.meta
                  return (
                    <TableCell
                      key={cell.id}
                      className={cn('py-2', meta?.align === 'right' && 'text-right', meta?.className)}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  )
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {pager}
    </div>
  )
}

function MobileCard<T>({
  row,
  onClick,
  className,
}: {
  row: Row<T>
  onClick?: (row: T) => void
  className?: string
}) {
  const cells = row.getVisibleCells()
  const pick = (kind: string) => cells.filter((c) => (c.column.columnDef.meta?.mobile ?? 'field') === kind)
  const render = (c: (typeof cells)[number]) => flexRender(c.column.columnDef.cell, c.getContext())
  const labelOf = (c: (typeof cells)[number]) => {
    const meta = c.column.columnDef.meta
    const h = c.column.columnDef.header
    return meta?.label ?? (typeof h === 'string' ? h : c.column.id)
  }
  const Comp = onClick ? 'button' : 'div'
  return (
    <Comp
      type={onClick ? 'button' : undefined}
      onClick={onClick ? () => onClick(row.original) : undefined}
      className={cn(
        'block w-full rounded-lg border bg-card p-3 text-left',
        onClick && 'active:bg-muted',
        className,
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2 font-medium">
          {pick('title').map((c) => (
            <span key={c.id}>{render(c)}</span>
          ))}
          {pick('badge').map((c) => (
            <span key={c.id}>{render(c)}</span>
          ))}
        </div>
        <div className="shrink-0 text-right font-medium">
          {pick('primary').map((c) => (
            <div key={c.id}>{render(c)}</div>
          ))}
        </div>
      </div>
      {pick('field').length > 0 && (
        <dl className="mt-2 grid grid-cols-3 gap-x-3 gap-y-2 text-sm">
          {pick('field').map((c) => (
            <div key={c.id} className="min-w-0">
              <dt className="truncate text-xs text-muted-foreground">{labelOf(c)}</dt>
              <dd className="truncate">{render(c)}</dd>
            </div>
          ))}
        </dl>
      )}
    </Comp>
  )
}

function Pager<T>({ table }: { table: ReturnType<typeof useReactTable<T>> }) {
  const { pageIndex, pageSize } = table.getState().pagination
  const total = table.getFilteredRowModel().rows.length
  const from = pageIndex * pageSize + 1
  const to = Math.min(total, from + pageSize - 1)
  return (
    <div className="flex items-center justify-end gap-2 text-sm text-muted-foreground">
      <span className="num">
        {from}–{to} / {total}
      </span>
      <Button
        variant="outline"
        size="icon"
        className="size-8"
        onClick={() => table.previousPage()}
        disabled={!table.getCanPreviousPage()}
        aria-label="Previous page"
      >
        <ChevronLeftIcon />
      </Button>
      <Button
        variant="outline"
        size="icon"
        className="size-8"
        onClick={() => table.nextPage()}
        disabled={!table.getCanNextPage()}
        aria-label="Next page"
      >
        <ChevronRightIcon />
      </Button>
    </div>
  )
}
