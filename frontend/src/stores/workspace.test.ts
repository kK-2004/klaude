import { beforeEach, describe, expect, it } from 'vitest'
import { useWorkspaceStore } from './workspace'

describe('workspace store', () => {
  beforeEach(() => useWorkspaceStore.setState({ messages: [], project: { id: 'p', name: 'klaude', rootPath: '~/Documents/ChatGPT/klaude', createdAt: '', updatedAt: '' } }))

  it('appends conversation messages in order', () => {
    useWorkspaceStore.getState().addMessage({ role: 'user', content: 'Inspect the project' })
    useWorkspaceStore.getState().addMessage({ role: 'assistant', content: 'I am ready.' })
    expect(useWorkspaceStore.getState().messages.map(({ role, content }) => ({ role, content }))).toEqual([
      { role: 'user', content: 'Inspect the project' },
      { role: 'assistant', content: 'I am ready.' },
    ])
  })
})
