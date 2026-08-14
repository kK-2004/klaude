import { describe, expect, it } from 'vitest'
import { shouldSubmitComposerKey } from './composer-input'

describe('composer keyboard shortcuts', () => {
  it('sends with Enter and preserves a newline with Shift+Enter', () => {
    expect(shouldSubmitComposerKey({ key: 'Enter', shiftKey: false })).toBe(true)
    expect(shouldSubmitComposerKey({ key: 'Enter', shiftKey: true })).toBe(false)
    expect(shouldSubmitComposerKey({ key: 'a', shiftKey: false })).toBe(false)
  })
})
