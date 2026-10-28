import { useQuery } from '@tanstack/react-query'
import { Link, useMatches } from '@tanstack/react-router'
import { LogOutIcon, MonitorIcon, MoonIcon, SunIcon } from 'lucide-react'
import { Fragment } from 'react'

import { StatusIndicators } from '@/app/layout/StatusIndicators'
import { type ThemePreference, useThemeStore } from '@/app/theme'
import { AccountPopover } from '@/features/account/AccountPopover'
import { logout, meQuery } from '@/features/auth/api'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/shared/ui/breadcrumb'
import { Button } from '@/shared/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { Separator } from '@/shared/ui/separator'
import { SidebarTrigger } from '@/shared/ui/sidebar'

function Crumbs() {
  const crumbs = useMatches({
    select: (matches) =>
      matches.flatMap((m) => {
        const { title, parent } = m.staticData
        if (!title) return []
        const crumb = { title, path: m.pathname }
        return parent ? [{ title: parent.title, path: parent.to }, crumb] : [crumb]
      }),
  })
  return (
    <Breadcrumb>
      <BreadcrumbList>
        {crumbs.map((c, i) => (
          <Fragment key={`${c.path}:${c.title}`}>
            {i > 0 && <BreadcrumbSeparator />}
            <BreadcrumbItem>
              {i === crumbs.length - 1 ? (
                <BreadcrumbPage>{c.title}</BreadcrumbPage>
              ) : (
                <BreadcrumbLink asChild>
                  <Link to={c.path}>{c.title}</Link>
                </BreadcrumbLink>
              )}
            </BreadcrumbItem>
          </Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  )
}

const THEME_ICON = { dark: MoonIcon, light: SunIcon, system: MonitorIcon } as const

function UserMenu() {
  const { data: me } = useQuery(meQuery)
  const { theme, setTheme } = useThemeStore()
  const Icon = THEME_ICON[theme]
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Account and appearance">
          <Icon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        {me && <DropdownMenuLabel className="truncate font-normal">{me.email}</DropdownMenuLabel>}
        <DropdownMenuSeparator />
        <DropdownMenuLabel className="text-xs text-muted-foreground">Theme</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={theme} onValueChange={(v) => setTheme(v as ThemePreference)}>
          <DropdownMenuRadioItem value="dark">Dark</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="light">Light</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="system">System</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onSelect={() => {
            void logout()
          }}
        >
          <LogOutIcon />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function TopBar() {
  return (
    <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center gap-2 border-b bg-background/95 px-4 backdrop-blur">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-2 data-[orientation=vertical]:h-4" />
      <div className="min-w-0 flex-1">
        <Crumbs />
      </div>
      <StatusIndicators />
      <AccountPopover />
      <UserMenu />
    </header>
  )
}
