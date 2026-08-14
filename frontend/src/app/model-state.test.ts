import { describe, expect, it } from 'vitest'
import { emptyModelCatalog, hasConfiguredModel } from './model-state'

describe('model state', () => {
  it('starts without a selected or preview model', () => {
    expect(emptyModelCatalog).toEqual({ activeId: '', profiles: [] })
    expect(hasConfiguredModel('')).toBe(false)
    expect(hasConfiguredModel('gpt-4.1')).toBe(true)
  })
})
