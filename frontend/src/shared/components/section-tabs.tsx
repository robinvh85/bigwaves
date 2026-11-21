import { Link } from '@tanstack/react-router'

export type SectionTab = { title: string; to: string; exact?: boolean }

/** Link tabs between the screens of one section (a strategy), shown above its pages. */
export function SectionTabs({ tabs }: { tabs: SectionTab[] }) {
  return (
    <nav aria-label="Section" className="mb-4 overflow-x-auto border-b">
      <div className="flex gap-5">
        {tabs.map((t) => (
          <Link
            key={t.to}
            to={t.to}
            activeOptions={{ exact: t.exact, includeSearch: false }}
            className="-mb-px border-b-2 border-transparent py-2 text-sm font-medium whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground data-[status=active]:border-foreground data-[status=active]:text-foreground"
          >
            {t.title}
          </Link>
        ))}
      </div>
    </nav>
  )
}
