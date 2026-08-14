/** @vitest-environment jsdom */

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.hoisted(() => {
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: { getItem: () => null, setItem: () => undefined },
  })
  Object.defineProperty(globalThis, 'matchMedia', {
    configurable: true,
    value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined }),
  })
})

import { backend } from '../lib/backend'
import { useWorkspaceStore } from '../stores/workspace'
import type { Project } from '../types/backend'
import { useAppController } from './use-app-controller'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const project: Project = {
  id: 'project-1',
  name: '项目一',
  rootPath: '/tmp/project-1',
  pinned: false,
  createdAt: '',
  updatedAt: '',
}

describe('useAppController', () => {
  let container: HTMLDivElement
  let root: Root
  let controller: ReturnType<typeof useAppController> | undefined

  beforeEach(async () => {
    vi.restoreAllMocks()
    useWorkspaceStore.setState({ project, projects: [project], session: undefined, messages: [], sessions: [] })
    vi.spyOn(backend, 'health').mockResolvedValue({ ready: false, product: 'Klaude', version: 'test' })
    vi.spyOn(backend, 'createSession').mockResolvedValue({
      id: 'session-created', projectId: project.id, title: '新对话', provider: '', model: '', status: 'idle', createdAt: '', updatedAt: '',
    })
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    function Probe() {
      controller = useAppController()
      return null
    }
    await act(async () => {
      root.render(<Probe />)
      await Promise.resolve()
    })
  })

  afterEach(async () => {
    await act(async () => root.unmount())
    container.remove()
    controller = undefined
  })

  it('starts an unsaved draft without creating a sidebar session', async () => {
    await act(async () => {
      await controller?.startNewChat()
    })

    expect(backend.createSession).not.toHaveBeenCalled()
    expect(useWorkspaceStore.getState().session).toBeUndefined()
    expect(useWorkspaceStore.getState().messages).toEqual([])
  })
})
