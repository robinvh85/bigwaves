import { useEffect } from 'react'
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemePreference = 'dark' | 'light' | 'system'

type ThemeState = {
  theme: ThemePreference
  setTheme: (theme: ThemePreference) => void
}

// Dark is the default: this is a trading dashboard that is often open all day.
export const useThemeStore = create<ThemeState>()(
  persist((set) => ({ theme: 'dark', setTheme: (theme) => set({ theme }) }), {
    name: 'bw-theme',
  }),
)

export function resolveTheme(theme: ThemePreference): 'dark' | 'light' {
  if (theme !== 'system') return theme
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** Keeps the `dark` class on <html> in sync with the stored preference and the OS setting. */
export function useApplyTheme() {
  const theme = useThemeStore((s) => s.theme)
  useEffect(() => {
    const apply = () => {
      document.documentElement.classList.toggle('dark', resolveTheme(theme) === 'dark')
    }
    apply()
    if (theme !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    mq.addEventListener('change', apply)
    return () => {
      mq.removeEventListener('change', apply)
    }
  }, [theme])
}
