import { createFileRoute, redirect } from '@tanstack/react-router'

/** Campaigns moved to /ema-settings. */
export const Route = createFileRoute('/_app/ema-cross/campaigns/')({
  beforeLoad: () => {
    throw redirect({ to: '/ema-settings', replace: true })
  },
})
