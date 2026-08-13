import type { AgentTurn, Capability, ConversationSnapshot, FileChange, FileEntry, GitBranchSnapshot, Project, Session, ApprovalResolution } from '../types/backend'

type ServiceBinding = Record<string, (...args: unknown[]) => Promise<unknown>>
const service = (): ServiceBinding | undefined => window.go?.app?.RPCService
const call = async <T>(name: string, ...args: unknown[]): Promise<T> => {
  const method = service()?.[name]
  if (!method) throw new Error(`Backend method ${name} is unavailable. Start Klaude through Wails.`)
  return method(...args) as Promise<T>
}

export const backend = {
  health: () => call<{ ready: boolean; product: string; version: string; platform?: Platform }>('Health'),
  listProjects: () => call<Project[]>('ListProjects'),
  openProject: (path: string) => call<Project>('OpenProject', path),
  renameProject: (projectId: string, name: string) => call<Project>('RenameProject', projectId, name),
  setProjectPinned: (projectId: string, pinned: boolean) => call<Project>('SetProjectPinned', projectId, pinned),
  deleteProject: (projectId: string) => call<void>('DeleteProject', projectId),
  revealProject: (projectId: string) => call<void>('RevealProject', projectId),
  createSession: (projectId: string, title: string, provider: string, model: string) => call<Session>('CreateSession', projectId, title, provider, model),
  moveSession: (sessionId: string, projectId: string) => call<Session>('MoveSession', sessionId, projectId),
  listSessions: (projectId: string) => call<Session[]>('ListSessions', projectId),
  renameSession: (sessionId: string, title: string) => call<void>('RenameSession', sessionId, title),
  loadConversation: (sessionId: string) => call<ConversationSnapshot>('LoadConversation', sessionId),
  sendMessage: (sessionId: string, content: string, provider: string, model: string) => call<AgentTurn>('SendMessage', sessionId, content, provider, model),
  cancelAgent: (sessionId: string) => call<boolean>('CancelAgent', sessionId),
  browseProject: (root: string, target: string) => call<FileEntry[]>('BrowseProject', root, target),
  gitBranches: (root: string) => call<GitBranchSnapshot>('GitBranches', root),
  checkoutGitBranch: (root: string, name: string, remote: boolean) => call<void>('CheckoutGitBranch', root, name, remote),
  deleteGitBranch: (root: string, name: string, remote: boolean) => call<void>('DeleteGitBranch', root, name, remote),
  createGitWorktree: (root: string, startRef: string, branchName: string, targetPath: string) => call<string>('CreateGitWorktree', root, startRef, branchName, targetPath),
  selectDirectory: (defaultDirectory: string) => call<string>('SelectDirectory', defaultDirectory),
  capabilities: () => call<Capability[]>('Capabilities'),
  turnChanges: (turnId: string) => call<FileChange[]>('GetTurnChanges', turnId),
  undoTurn: (turnId: string) => call<void>('UndoTurn', turnId),
  resolveApproval: (resolution: ApprovalResolution) => call<void>('ResolveApproval', resolution),
  settings: () => call<BackendSettings>('Settings'),
  updateSettings: (update: SettingsUpdate) => call<BackendSettings>('UpdateSettings', update),
  modelProfiles: () => call<ModelCatalog>('ModelProfiles'),
  saveModelProfile: (input: ModelProfileInput) => call<ModelCatalog>('SaveModelProfile', input),
  selectModelProfile: (profileId: string) => call<ModelCatalog>('SelectModelProfile', profileId),
  testModelConnection: (input: ModelProfileInput) => call<ModelConnectionResult>('TestModelConnection', input),
}

export type Platform = 'darwin' | 'windows' | 'linux'
export type ApprovalMode = 'ask' | 'manual' | 'full'
export type ProviderSpec = 'openai' | 'anthropic'
export type ModelAPIMode = 'chat_completions' | 'responses' | 'messages'
export type ModelProfile = {
  id: string
  name: string
  providerSpec: ProviderSpec
  apiMode: ModelAPIMode
  baseUrl: string
  model: string
  contextWindow: number
  maxOutputTokens: number
  temperature: number
  hasApiKey: boolean
}
export type ModelProfileInput = Omit<ModelProfile, 'hasApiKey'> & { apiKey: string }
export type ModelCatalog = { activeId: string; profiles: ModelProfile[] }
export type ModelConnectionResult = { success: boolean; latencyMs: number; message: string }
export type BackendSettings = {
  UI?: { Theme?: string }
  Agent?: { ParallelTools?: boolean; LLMSchedule?: boolean; MaxTurns?: number; ContextBudgetChars?: number; ToolResultChars?: number }
  Provider?: { Name?: string; Protocol?: ProviderSpec; APIMode?: ModelAPIMode; Endpoint?: string; Model?: string; CredentialEnv?: string; CredentialKey?: string }
  Permissions?: { Read?: string; Write?: string; Shell?: string; Network?: string }
}
export type SettingsUpdate = {
  theme: 'light' | 'dark' | 'system'
  endpoint: string
  model: string
  credentialEnv: string
  contextBudgetChars: number
  maxTurns: number
  parallelTools: boolean
  llmSchedule: boolean
  approvalMode: ApprovalMode
}
