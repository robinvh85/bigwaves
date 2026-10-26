import { ChevronsUpDownIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'

import { Button } from '@/shared/ui/button'
import { Command, CommandEmpty, CommandInput, CommandItem, CommandList } from '@/shared/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/ui/popover'

export type ComboboxOption = {
  value: string
  label: string
  /** Extra text shown after the label, e.g. a position. Not searched. */
  hint?: ReactNode
}

/** A select with a search box, for long option lists such as coins. */
export function Combobox({
  id,
  options,
  value,
  onChange,
  placeholder = 'Select…',
  searchPlaceholder = 'Search…',
  emptyText = 'No match',
  invalid,
}: {
  id?: string
  options: ComboboxOption[]
  value: string
  onChange: (value: string) => void
  placeholder?: string
  searchPlaceholder?: string
  emptyText?: string
  invalid?: boolean
}) {
  const [open, setOpen] = useState(false)
  const selected = options.find((o) => o.value === value)

  return (
    <Popover open={open} onOpenChange={setOpen} modal>
      <PopoverTrigger asChild>
        <Button
          id={id}
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          aria-invalid={invalid || undefined}
          className="w-full justify-between font-normal"
        >
          {selected ? (
            <span className="truncate">
              {selected.label}
              {selected.hint && <span className="ml-1 text-muted-foreground">{selected.hint}</span>}
            </span>
          ) : (
            <span className="text-muted-foreground">{placeholder}</span>
          )}
          <ChevronsUpDownIcon className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) p-0" align="start">
        <Command>
          <CommandInput placeholder={searchPlaceholder} />
          <CommandList>
            <CommandEmpty>{emptyText}</CommandEmpty>
            {options.map((o) => (
              <CommandItem
                key={o.value}
                value={o.value}
                keywords={[o.label]}
                data-checked={o.value === value}
                onSelect={() => {
                  onChange(o.value)
                  setOpen(false)
                }}
              >
                {o.label}
                {o.hint && <span className="ml-auto text-xs text-muted-foreground">{o.hint}</span>}
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
