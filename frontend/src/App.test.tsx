import { describe, expect, it } from 'vitest'
import { boundToolOutput, safeLink } from './lib/presentation'

describe('safe presentation helpers', () => {
  it('rejects executable links and allows https links', () => {
    expect(safeLink('javascript:alert(1)')).toBe(false)
    expect(safeLink('https://example.com/docs')).toBe(true)
  })

  it('bounds large tool output with an explicit marker', () => {
    const output = boundToolOutput('x'.repeat(20), 8)
    expect(output).toContain('output truncated')
    expect(output.length).toBeLessThan(80)
  })
})
