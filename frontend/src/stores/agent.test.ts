import { beforeEach, describe, expect, it } from 'vitest'
import { useAgentStore } from './agent'

describe('agent event reducer', () => {
  beforeEach(() => useAgentStore.setState({ lastSequence: {}, seenEventIds: {}, needsSnapshot: {}, events: [] }))

  it('deduplicates events and flags sequence gaps', () => {
    const apply = useAgentStore.getState().applyEvent
    apply({ version: 1, eventId: 'one', sequence: 1, occurredAt: '', sessionId: 's', turnId: 't', type: 'agent.started' })
    apply({ version: 1, eventId: 'one', sequence: 1, occurredAt: '', sessionId: 's', turnId: 't', type: 'agent.started' })
    apply({ version: 1, eventId: 'three', sequence: 3, occurredAt: '', sessionId: 's', turnId: 't', type: 'agent.finished' })
    const state = useAgentStore.getState()
    expect(state.events).toHaveLength(2)
    expect(state.needsSnapshot.t).toBe(true)
  })
})
