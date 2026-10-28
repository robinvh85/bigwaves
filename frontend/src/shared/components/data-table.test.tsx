import { createColumnHelper } from '@tanstack/react-table'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { DataTable } from './data-table'

type R = { name: string; price: string }
const col = createColumnHelper<R>()
const columns = [col.accessor('name', { header: 'Name' }), col.accessor('price', { header: 'Price' })]
const data: R[] = [
  { name: 'b', price: '3' },
  { name: 'a', price: '-5' },
  { name: 'c', price: '0.0012' },
]

const firstColumn = () =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map((r) => within(r).getAllByRole('cell')[0]?.textContent)

describe('DataTable sorting', () => {
  beforeAll(() => {
    window.matchMedia = (query: string) =>
      ({
        matches: false,
        media: query,
        addEventListener() {},
        removeEventListener() {},
      }) as unknown as MediaQueryList
  })

  it('sorts on header click without a sorting handler', async () => {
    render(<DataTable columns={columns} data={data} />)
    expect(firstColumn()).toEqual(['b', 'a', 'c'])
    await userEvent.click(screen.getByRole('button', { name: 'Name' }))
    expect(firstColumn()).toEqual(['a', 'b', 'c'])
    await userEvent.click(screen.getByRole('button', { name: 'Name' }))
    expect(firstColumn()).toEqual(['c', 'b', 'a'])
  })

  it('starts from the given sorting and lets the user change it', async () => {
    render(<DataTable columns={columns} data={data} sorting={[{ id: 'name', desc: true }]} />)
    expect(firstColumn()).toEqual(['c', 'b', 'a'])
    await userEvent.click(screen.getByRole('button', { name: 'Price' }))
    expect(firstColumn()).toEqual(['a', 'c', 'b'])
  })

  it('leaves a controlled sort order to the caller', async () => {
    const onSortingChange = vi.fn()
    render(<DataTable columns={columns} data={data} sorting={[]} onSortingChange={onSortingChange} />)
    await userEvent.click(screen.getByRole('button', { name: 'Name' }))
    expect(onSortingChange).toHaveBeenCalledWith([{ id: 'name', desc: false }])
    expect(firstColumn()).toEqual(['b', 'a', 'c'])
  })
})
