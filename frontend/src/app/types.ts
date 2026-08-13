import type { Message } from '../types/backend'
import type { Capability, FileChange, FileEntry, Project, Session } from '../types/backend'
import type { ApprovalMode, ModelCatalog, ModelConnectionResult, ModelProfileInput } from '../lib/backend'

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
  modelCatalog: ModelCatalog
  selectModelProfile: (profileId: string) => Promise<boolean>
  saveModelProfile: (input: ModelProfileInput) => Promise<ModelCatalog | undefined>
  testModelConnection: (input: ModelProfileInput) => Promise<ModelConnectionResult>
  openModelSettings: () => void
  contextLimit: string
  setContextLimit: (value: string) => void
  turnLimit: string
  setTurnLimit: (value: string) => void
  parallelTools: boolean
  setParallelTools: (value: boolean) => void
  llmSchedule: boolean
  setLLMSchedule: (value: boolean) => void
  approvalMode: ApprovalMode
  setApprovalMode: (value: ApprovalMode) => void
  applyApprovalMode: (value: ApprovalMode) => Promise<boolean>
  saveSettings: () => Promise<void>
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
