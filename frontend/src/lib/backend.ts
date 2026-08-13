import type { AgentTurn, Capability, ConversationSnapshot, FileChange, FileEntry, GitBranchSnapshot, Project, Session, ApprovalResolution } from '../types/backend'

type ServiceBinding = Record<string, (...args: unknown[]) => Promise<unknown>>
const service = (): ServiceBinding | undefined => window.go?.app?.RPCService
const call = async <T>(name: string, ...args: unknown[]): Promise<T> => {
  const method = service()?.[name]
  if (!method) throw new Error(`Backend method ${name} is unavailable. Start Klaude through Wails.`)
  return method(...args) as Promise<T>
}

export const backend = {
  health: () => call<{ ready: boolean; product: string; version: string }>('Health'),
  listProjects: () => call<Project[]>('ListProjects'),
  openProject: (path: string) => call<Project>('OpenProject', path),
  createSession: (projectId: string, title: string, provider: string, model: string) => call<Session>('CreateSession', projectId, title, provider, model),
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
}
