import { beforeEach, describe, expect, it } from 'vitest'
import { useWorkspaceStore } from './workspace'

describe('workspace store', () => {
  beforeEach(() => useWorkspaceStore.setState({ messages: [], project: undefined, projects: [] }))

  it('starts without a fabricated project', () => {
    expect(useWorkspaceStore.getInitialState().project).toBeUndefined()
    expect(useWorkspaceStore.getInitialState().projects).toEqual([])
  })

  it('appends conversation messages in order', () => {
    useWorkspaceStore.getState().addMessage({ role: 'user', content: 'Inspect the project' })
    useWorkspaceStore.getState().addMessage({ role: 'assistant', content: 'I am ready.' })
    expect(useWorkspaceStore.getState().messages.map(({ role, content }) => ({ role, content }))).toEqual([
      { role: 'user', content: 'Inspect the project' },
      { role: 'assistant', content: 'I am ready.' },
    ])
  })
})
