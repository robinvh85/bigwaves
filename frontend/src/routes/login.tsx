import { createFileRoute } from '@tanstack/react-router'
import { WavesIcon } from 'lucide-react'
import { z } from 'zod'

import { login } from '@/features/auth/api'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'

export const Route = createFileRoute('/login')({
  validateSearch: z.object({ redirect: z.string().optional() }),
  component: LoginPage,
})

function LoginPage() {
  const { redirect } = Route.useSearch()
  return (
    <main className="flex min-h-svh items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="items-center text-center">
          <div className="mx-auto mb-2 flex size-10 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <WavesIcon className="size-5" />
          </div>
          <CardTitle>bigwaves</CardTitle>
          <CardDescription>Sign in to continue</CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            className="w-full"
            onClick={() => {
              login(redirect)
            }}
          >
            Sign in with Auth0
          </Button>
        </CardContent>
      </Card>
    </main>
  )
}
