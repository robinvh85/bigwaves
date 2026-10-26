import ReactMarkdown from 'react-markdown'

import { cn } from '@/shared/lib/utils'

/** Renders user-written markdown. Raw HTML is not rendered (react-markdown's default). */
export function Markdown({ children, className }: { children: string; className?: string }) {
  return (
    <div
      className={cn(
        'prose prose-sm max-w-none dark:prose-invert prose-headings:mt-4 prose-a:text-primary',
        className,
      )}
    >
      <ReactMarkdown
        components={{
          a: ({ node, children, ...props }) => (
            <a {...props} target="_blank" rel="noreferrer noopener">
              {children}
            </a>
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  )
}
