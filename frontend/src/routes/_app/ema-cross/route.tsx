import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_app/ema-cross')({
  staticData: { title: 'EMA Cross' },
  component: Outlet,
})
