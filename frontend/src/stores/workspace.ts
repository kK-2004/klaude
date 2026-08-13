import { create } from 'zustand'
import type { Capability, FileEntry, Message, Project, Session } from '../types/backend'

type WorkspaceState = {
  project?: Project
  projects: Project[]
  sessions: Session[]
  session?: Session
  files: FileEntry[]
  capabilities: Capability[]
  messages: Message[]
  panelLeft: number
  panelRight: number
  setProject: (project?: Project) => void
  setProjects: (projects: Project[]) => void
  setSessions: (sessions: Session[]) => void
  setSession: (session?: Session) => void
  setFiles: (files: FileEntry[]) => void
  setCapabilities: (capabilities: Capability[]) => void
  setMessages: (messages: Message[]) => void
  addMessage: (message: Message | Pick<Message, 'role' | 'content'>) => void
  appendAssistantDelta: (delta: string) => void
  setPanelWidth: (side: 'left' | 'right', width: number) => void
}

const initialProject: Project = {
  id: 'demo-project',
  name: 'klaude',
  rootPath: '~/Documents/ChatGPT/klaude',
  gitRoot: '~/Documents/ChatGPT/klaude',
  createdAt: new Date(0).toISOString(),
  updatedAt: new Date(0).toISOString(),
}

const readWidth = (key: string, fallback: number) => {
  if (typeof window === 'undefined') return fallback
  const value = Number(window.localStorage.getItem(key))
  return Number.isFinite(value) && value >= 180 && value <= 420 ? value : fallback
}

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  project: initialProject,
  projects: [initialProject],
  sessions: [],
  files: [],
  capabilities: [],
  messages: [],
  panelLeft: readWidth('klaude.panel.left', 250),
  panelRight: readWidth('klaude.panel.right', 278),
  setProject: (project) => set({ project }),
  setProjects: (projects) => set({ projects }),
  setSessions: (sessions) => set({ sessions }),
  setSession: (session) => set({ session }),
  setFiles: (files) => set({ files }),
  setCapabilities: (capabilities) => set({ capabilities }),
  setMessages: (messages) => set({ messages }),
  addMessage: (message) => set((state) => ({
    messages: [...state.messages, 'id' in message ? message : { ...message, id: `local-${crypto.randomUUID()}`, sessionId: state.session?.id ?? 'local', createdAt: new Date().toISOString() }],
  })),
  appendAssistantDelta: (delta) => set((state) => {
    const last = state.messages[state.messages.length - 1]
    if (last?.role === 'assistant') return { messages: [...state.messages.slice(0, -1), { ...last, content: last.content + delta }] }
    return { messages: [...state.messages, { id: `local-${crypto.randomUUID()}`, sessionId: state.session?.id ?? 'local', role: 'assistant', content: delta, createdAt: new Date().toISOString() }] }
  }),
  setPanelWidth: (side, width) => {
    const bounded = Math.max(180, Math.min(420, width))
    if (typeof window !== 'undefined') window.localStorage.setItem(`klaude.panel.${side}`, String(bounded))
    set(side === 'left' ? { panelLeft: bounded } : { panelRight: bounded })
  },
}))
