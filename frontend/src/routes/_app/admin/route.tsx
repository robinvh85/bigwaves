import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_app/admin')({
  staticData: { title: 'Quản trị' },
  component: Outlet,
})
