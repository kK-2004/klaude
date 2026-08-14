export function shouldSubmitComposerKey(event: Pick<KeyboardEvent, 'key' | 'shiftKey'>): boolean {
  return event.key === 'Enter' && !event.shiftKey
}
