import type { Message, Session } from '../types/backend'

export function draftConversationState(): { session?: Session; messages: Message[] } {
  return { session: undefined, messages: [] }
}
