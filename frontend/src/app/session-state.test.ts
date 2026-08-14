import { describe, expect, it } from 'vitest'
import { draftConversationState } from './session-state'

describe('session state', () => {
  it('starts a new conversation as an unsaved draft', () => {
    expect(draftConversationState()).toEqual({ session: undefined, messages: [] })
  })
})
