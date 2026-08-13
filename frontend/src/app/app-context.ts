import { createContext } from 'react'
import type { AppController } from './types'

export const AppContext = createContext<AppController | null>(null)
