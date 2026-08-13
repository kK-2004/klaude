/** @vitest-environment jsdom */

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { backend } from '../lib/backend'
import type { GitBranchSnapshot } from '../types/backend'
import { GitBranchSelector } from './GitBranchSelector'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const snapshot: GitBranchSnapshot = {
  current: 'main',
  worktreeBase: '/repos',
  branches: [
    { name: 'main', remote: false, current: true },
    { name: 'feature/local', remote: false, current: false },
    { name: 'origin/main', remote: true, current: false },
    { name: 'origin/topic', remote: true, current: false },
  ],
}

describe('GitBranchSelector', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(async () => {
    vi.restoreAllMocks()
    vi.spyOn(backend, 'gitBranches').mockResolvedValue(snapshot)
    vi.spyOn(backend, 'checkoutGitBranch').mockResolvedValue(undefined)
    vi.spyOn(backend, 'deleteGitBranch').mockResolvedValue(undefined)
    vi.spyOn(backend, 'createGitWorktree').mockResolvedValue('/custom/topic')
    vi.spyOn(backend, 'selectDirectory').mockResolvedValue('/custom')
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => {
      root.render(<GitBranchSelector gitRoot="/repo" />)
      await Promise.resolve()
    })
  })

  afterEach(async () => {
    await act(async () => root.unmount())
    container.remove()
  })

  it('lists local and remote branches and checks out on single click', async () => {
    await clickButton('main')
    expect(container.textContent).toContain('本地分支')
    expect(container.textContent).toContain('远端分支')

    await clickButton('feature/local')
    expect(backend.checkoutGitBranch).toHaveBeenCalledWith('/repo', 'feature/local', false)
  })

  it('requires confirmation and emphasizes remote deletion', async () => {
    await clickButton('main')
    await contextMenuButton('origin/topic')
    await clickButton('删除分支')

    expect(container.textContent).toContain('远端分支也会被删除！')
    await clickButton('确认删除')
    expect(backend.deleteGitBranch).toHaveBeenCalledWith('/repo', 'origin/topic', true)
  })

  it('uses the selected directory as the worktree path prefix', async () => {
    await clickButton('main')
    await contextMenuButton('origin/topic')
    await clickButton('创建 Worktree 并签出')
    await clickButton('选择目录')

    expect(container.textContent).toContain('/custom/topic')
    await clickButton('创建并签出')
    expect(backend.createGitWorktree).toHaveBeenCalledWith('/repo', 'origin/topic', 'topic', '/custom/topic')
  })

  async function clickButton(text: string) {
    const button = Array.from(container.querySelectorAll('button')).find((item) => item.textContent?.trim().includes(text))
    if (!button) throw new Error(`button not found: ${text}`)
    await act(async () => {
      button.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      await Promise.resolve()
    })
  }

  async function contextMenuButton(text: string) {
    const button = Array.from(container.querySelectorAll('button')).find((item) => item.textContent?.trim() === text)
    if (!button) throw new Error(`branch not found: ${text}`)
    await act(async () => {
      button.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: 120, clientY: 120 }))
      await Promise.resolve()
    })
  }
})
