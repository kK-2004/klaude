import type { ReactNode } from 'react'
import { AppContext } from './app-context'
import type { AppController } from './types'

export function AppProvider({ value, children }: { value: AppController; children: ReactNode }) {
  return <AppContext.Provider value={value}>{children}</AppContext.Provider>
}
