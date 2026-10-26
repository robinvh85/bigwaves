import { Link, useRouterState } from '@tanstack/react-router'
import { WavesIcon } from 'lucide-react'

import { NAV } from '@/app/nav'
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from '@/shared/ui/sidebar'

function isActive(pathname: string, to: string, exact?: boolean) {
  if (exact) return pathname === to
  return pathname === to || pathname.startsWith(`${to}/`)
}

export function AppSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const { isMobile, setOpenMobile } = useSidebar()

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <Link to="/">
                <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                  <WavesIcon className="size-4" />
                </div>
                <span className="font-semibold">bigwaves</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        {NAV.map((group, i) => (
          <SidebarGroup key={group.title ?? i}>
            {group.title && <SidebarGroupLabel>{group.title}</SidebarGroupLabel>}
            <SidebarGroupContent>
              <SidebarMenu>
                {group.items.map((item) => (
                  <SidebarMenuItem key={item.to}>
                    <SidebarMenuButton
                      asChild
                      tooltip={group.title ? `${group.title} · ${item.title}` : item.title}
                      isActive={isActive(pathname, item.to, item.exact)}
                    >
                      <Link
                        to={item.to}
                        onClick={() => {
                          if (isMobile) setOpenMobile(false)
                        }}
                      >
                        <item.icon />
                        <span>{item.title}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>
      <SidebarRail />
    </Sidebar>
  )
}
