import type { Message } from '../types/backend'
import type { Capability, FileChange, FileEntry, Project, Session } from '../types/backend'

export type AppPage = 'home' | 'pull-requests' | 'sites' | 'scheduled' | 'plugins' | 'settings'
export type AppState = 'loading' | 'ready' | 'diagnostic' | 'fatal'
export type PendingApproval = {
  id: string
  summary: string
  cwd: string
  hash: string
  risk?: string
  timeout?: string
}

export type AppController = {
  appState: AppState
  diagnostic: string
  dismissDiagnostic: () => void
  retryInit: () => void
  page: AppPage
  setPage: (page: AppPage) => void
  sidebarOpen: boolean
  setSidebarOpen: (open: boolean) => void
  composer: string
  setComposer: (value: string) => void
  running: boolean
  approval?: PendingApproval
  changes: FileChange[]
  selectedChange?: FileChange
  setSelectedChange: (change?: FileChange) => void
  model: string
  setModel: (value: string) => void
  endpoint: string
  setEndpoint: (value: string) => void
  credentialEnv: string
  setCredentialEnv: (value: string) => void
  contextLimit: string
  setContextLimit: (value: string) => void
  turnLimit: string
  setTurnLimit: (value: string) => void
  turnStatus: string
  usage: { input?: number; output?: number }
  project?: Project
  projects: Project[]
  sessions: Session[]
  session?: Session
  files: FileEntry[]
  capabilities: Capability[]
  messages: Message[]
  groupedChanges: Record<string, FileChange[]>
  setupDone: number
  setupTotal: number
  openProject: () => Promise<void>
  selectProject: (project: Project) => Promise<void>
  selectSession: (session: Session) => Promise<void>
  createSession: (title?: string) => Promise<void>
  renameCurrentSession: () => Promise<void>
  startNewChat: () => Promise<void>
  browse: (path: string) => Promise<void>
  submitMessage: () => Promise<void>
  stopAgent: () => Promise<void>
  resolveApproval: (status: 'approved' | 'rejected') => Promise<void>
  undoSelectedTurn: () => Promise<void>
}
