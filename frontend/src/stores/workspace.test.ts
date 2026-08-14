import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useWorkspaceStore } from './workspace'

describe('workspace store', () => {
  beforeEach(() => useWorkspaceStore.setState({ messages: [], project: undefined, projects: [] }))
  afterEach(() => vi.unstubAllGlobals())

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

  it('can create local messages without crypto.randomUUID', () => {
    vi.stubGlobal('crypto', {})

    expect(() => useWorkspaceStore.getState().addMessage({ role: 'user', content: 'Hello' })).not.toThrow()
    expect(useWorkspaceStore.getState().messages[0].id).toMatch(/^local-/)
  })
})
