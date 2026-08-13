export type Project = { id: string; name: string; rootPath: string; gitRoot?: string; pinned?: boolean; createdAt: string; updatedAt: string }
export type SessionStatus = 'idle' | 'running' | 'waiting_approval' | 'cancelled' | 'failed' | 'completed'
export type Session = { id: string; projectId: string; title: string; provider: string; model: string; status: SessionStatus; createdAt: string; updatedAt: string }
export type TurnStatus = 'queued' | 'running' | 'waiting_approval' | 'completed' | 'cancelled' | 'failed' | 'interrupted'
export type AgentTurn = { id: string; sessionId: string; status: TurnStatus; startedAt: string; finishedAt?: string; errorCode?: string; errorText?: string }
export type FileEntry = { name: string; path: string; dir: boolean; size: number; externalSymlink: boolean }
export type Capability = { name: string; available: boolean; detail: string }
export type AgentEvent<T = unknown> = { version: number; eventId: string; sequence: number; occurredAt: string; projectId?: string; sessionId: string; turnId: string; type: string; payload?: T }
export type ApprovalResolution = { approvalId: string; status: 'approved' | 'rejected' | 'cancelled' | 'expired'; requestHash: string }
export type Message = { id: string; sessionId: string; turnId?: string; role: 'system' | 'user' | 'assistant' | 'tool'; content: string; createdAt: string }
export type ConversationSnapshot = { session: Session; messages: Message[]; turns: AgentTurn[] }
export type FileChange = { id: string; turnId: string; toolCallId: string; path: string; status: string; beforeHash: string; afterHash: string; diff: string; addedLines: number; deletedLines: number; createdAt: string }
export type GitBranch = { name: string; remote: boolean; current: boolean }
export type GitBranchSnapshot = { current: string; branches: GitBranch[]; worktreeBase: string }

declare global {
  interface Window {
    go?: { app?: { RPCService?: Record<string, (...args: unknown[]) => Promise<unknown>> } }
    runtime?: { EventsOn?: (name: string, callback: (event: AgentEvent) => void) => () => void }
  }
}
