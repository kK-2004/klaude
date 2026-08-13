import type { FileEntry, Session } from '../types/backend'

export const demoFiles: FileEntry[] = [
  { name: 'internal', path: 'internal', dir: true, size: 0, externalSymlink: false },
  { name: 'frontend', path: 'frontend', dir: true, size: 0, externalSymlink: false },
  { name: 'go.mod', path: 'go.mod', dir: false, size: 810, externalSymlink: false },
  { name: 'ARCHITECTURE.md', path: 'ARCHITECTURE.md', dir: false, size: 2400, externalSymlink: false },
]

export const demoSession: Session = {
  id: 'demo-session',
  projectId: 'demo-project',
  title: '新对话',
  provider: 'openai-compatible',
  model: 'not configured',
  status: 'idle',
  createdAt: new Date(0).toISOString(),
  updatedAt: new Date(0).toISOString(),
}
