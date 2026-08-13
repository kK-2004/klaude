import { create } from 'zustand'
import type { AgentEvent } from '../types/backend'

type AgentState = {
  lastSequence: Record<string, number>
  seenEventIds: Record<string, true>
  needsSnapshot: Record<string, boolean>
  events: AgentEvent[]
  applyEvent: (event: AgentEvent) => void
  reconcile: (turnId: string) => void
}

export const useAgentStore = create<AgentState>((set) => ({
  lastSequence: {},
  seenEventIds: {},
  needsSnapshot: {},
  events: [],
  applyEvent: (event) => set((state) => {
    if (state.seenEventIds[event.eventId]) return state
    const previous = state.lastSequence[event.turnId] ?? 0
    const gap = event.sequence > previous + 1
    return {
      lastSequence: { ...state.lastSequence, [event.turnId]: Math.max(previous, event.sequence) },
      seenEventIds: { ...state.seenEventIds, [event.eventId]: true },
      needsSnapshot: gap ? { ...state.needsSnapshot, [event.turnId]: true } : state.needsSnapshot,
      events: [...state.events, event],
    }
  }),
  reconcile: (turnId) => set((state) => ({ needsSnapshot: { ...state.needsSnapshot, [turnId]: false } })),
}))
