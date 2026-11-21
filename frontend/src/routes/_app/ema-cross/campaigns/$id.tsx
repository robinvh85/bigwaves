import { createFileRoute, redirect } from '@tanstack/react-router'

/** Campaigns moved to /ema-settings. */
export const Route = createFileRoute('/_app/ema-cross/campaigns/$id')({
  beforeLoad: ({ params }) => {
    throw redirect({ to: '/ema-settings/$id', params, replace: true })
  },
})
