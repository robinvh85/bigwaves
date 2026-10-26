import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_app/dca')({
  staticData: { title: 'DCA' },
  component: Outlet,
})
