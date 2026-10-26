import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_app/admin')({
  staticData: { title: 'Admin' },
  component: Outlet,
})
