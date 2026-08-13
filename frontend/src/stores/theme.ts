import { create } from 'zustand'

export type Theme = 'light' | 'dark'

const storageKey = 'klaude.theme'

function prefersDark() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches
}

function readTheme(): Theme {
  if (typeof window === 'undefined') return 'light'
  const saved = window.localStorage.getItem(storageKey)
  if (saved === 'light' || saved === 'dark') return saved
  return prefersDark() ? 'dark' : 'light'
}

function applyTheme(theme: Theme) {
  if (typeof document === 'undefined') return
  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
}

applyTheme(readTheme())

export const useThemeStore = create<{ theme: Theme; setTheme: (theme: Theme) => void; toggleTheme: () => void }>((set, get) => ({
  theme: readTheme(),
  setTheme: (theme) => {
    window.localStorage.setItem(storageKey, theme)
    applyTheme(theme)
    set({ theme })
  },
  toggleTheme: () => get().setTheme(get().theme === 'dark' ? 'light' : 'dark'),
}))
