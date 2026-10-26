import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_app/ema1d')({
  staticData: { title: 'EMA1D' },
  component: Outlet,
})
