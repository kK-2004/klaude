import { useCallback, useEffect, useMemo, useState } from 'react'
import { backend } from '../lib/backend'
import { demoFiles, demoSession } from '../lib/demo'
import { useAgentStore } from '../stores/agent'
import { useWorkspaceStore } from '../stores/workspace'
import type { AgentEvent, FileChange, Project, Session } from '../types/backend'
import type { AppController, AppPage, AppState, PendingApproval } from './types'

export function useAppController(): AppController {
  const [appState, setAppState] = useState<AppState>('loading')
  const [diagnostic, setDiagnostic] = useState('')
  const [page, setPage] = useState<AppPage>('home')
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [composer, setComposer] = useState('')
  const [running, setRunning] = useState(false)
  const [approval, setApproval] = useState<PendingApproval>()
  const [changes, setChanges] = useState<FileChange[]>([])
  const [selectedChange, setSelectedChange] = useState<FileChange>()
  const [model, setModel] = useState('not configured')
  const [endpoint, setEndpoint] = useState('https://api.openai.com/v1')
  const [credentialEnv, setCredentialEnv] = useState('OPENAI_API_KEY')
  const [contextLimit, setContextLimit] = useState('120000')
  const [turnLimit, setTurnLimit] = useState('50')
  const [turnStatus, setTurnStatus] = useState('idle')
  const [usage, setUsage] = useState<{ input?: number; output?: number }>({})

  const project = useWorkspaceStore((state) => state.project)
  const projects = useWorkspaceStore((state) => state.projects)
  const sessions = useWorkspaceStore((state) => state.sessions)
  const session = useWorkspaceStore((state) => state.session)
  const files = useWorkspaceStore((state) => state.files)
  const capabilities = useWorkspaceStore((state) => state.capabilities)
  const messages = useWorkspaceStore((state) => state.messages)
  const setProject = useWorkspaceStore((state) => state.setProject)
  const setProjects = useWorkspaceStore((state) => state.setProjects)
  const setSessions = useWorkspaceStore((state) => state.setSessions)
  const setSession = useWorkspaceStore((state) => state.setSession)
  const setFiles = useWorkspaceStore((state) => state.setFiles)
  const setCapabilities = useWorkspaceStore((state) => state.setCapabilities)
  const setMessages = useWorkspaceStore((state) => state.setMessages)
  const addMessage = useWorkspaceStore((state) => state.addMessage)
  const appendAssistantDelta = useWorkspaceStore((state) => state.appendAssistantDelta)
  const applyEvent = useAgentStore((state) => state.applyEvent)
  const needsSnapshot = useAgentStore((state) => session ? state.needsSnapshot[session.id] : false)

  const initialize = useCallback(async () => {
    setAppState('loading')
    setDiagnostic('')
    try {
      const health = await backend.health()
      if (!health.ready) {
        setAppState('diagnostic')
        setDiagnostic('应用以只读诊断模式启动。')
        return
      }
      const [recentProjects, caps] = await Promise.all([backend.listProjects(), backend.capabilities()])
      setProjects(recentProjects)
      setCapabilities(caps)
      const selected = recentProjects[0]
      if (selected) {
        setProject(selected)
        const recentSessions = await backend.listSessions(selected.id)
        setSessions(recentSessions)
        const selectedSession = recentSessions[0]
        if (selectedSession) {
          setSession(selectedSession)
          const snapshot = await backend.loadConversation(selectedSession.id)
          setMessages(snapshot.messages)
          setModel(selectedSession.model)
        }
        setFiles(await backend.browseProject(selected.rootPath, '.'))
      }
      setAppState('ready')
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      if (message.includes('unavailable') || message.includes('Wails')) {
        setSession(demoSession)
        setSessions([demoSession])
        setFiles(demoFiles)
        setAppState('ready')
        setDiagnostic('预览模式：请通过 Wails 启动桌面应用以连接后端。')
      } else {
        setAppState('fatal')
        setDiagnostic(message)
      }
    }
  }, [setCapabilities, setFiles, setMessages, setProject, setProjects, setSession, setSessions])

  useEffect(() => { void initialize() }, [initialize])

  useEffect(() => {
    const unsubscribe = window.runtime?.EventsOn?.('klaude:agent-event', (event: AgentEvent) => {
      applyEvent(event)
      const payload = event.payload as { delta?: string; text?: string; inputTokens?: number; outputTokens?: number; approval?: PendingApproval } | undefined
      if (payload?.delta || payload?.text) appendAssistantDelta(payload.delta ?? payload.text ?? '')
      if (payload?.approval) setApproval(payload.approval)
      if (payload?.inputTokens !== undefined || payload?.outputTokens !== undefined) setUsage({ input: payload.inputTokens, output: payload.outputTokens })
      if (event.type.endsWith('started')) { setTurnStatus('running'); setRunning(true) }
      if (event.type.includes('approval')) setTurnStatus('waiting_approval')
      if (event.type.endsWith('finished')) { setTurnStatus('completed'); setRunning(false) }
      if (event.type.endsWith('cancelled')) { setTurnStatus('cancelled'); setRunning(false) }
      if (event.type.endsWith('failed')) { setTurnStatus('failed'); setRunning(false) }
    })
    return unsubscribe
  }, [appendAssistantDelta, applyEvent])

  useEffect(() => {
    if (!needsSnapshot || !session) return
    void backend.loadConversation(session.id).then((snapshot) => setMessages(snapshot.messages)).catch(() => undefined)
  }, [needsSnapshot, session, setMessages])

  const selectProject = useCallback(async (item: Project) => {
    setProject(item)
    setPage('home')
    try {
      setFiles(await backend.browseProject(item.rootPath, '.'))
      const recentSessions = await backend.listSessions(item.id)
      setSessions(recentSessions)
      const next = recentSessions[0]
      setSession(next)
      if (next) {
        setModel(next.model)
        setMessages((await backend.loadConversation(next.id)).messages)
      } else {
        setMessages([])
      }
    } catch {
      setFiles(demoFiles)
    }
  }, [setFiles, setMessages, setProject, setSession, setSessions])

  const openProject = useCallback(async () => {
    const path = window.prompt('项目路径')
    if (!path) return
    try {
      const opened = await backend.openProject(path)
      setProject(opened)
      setProjects([opened, ...projects.filter((item) => item.rootPath !== opened.rootPath)])
      setFiles(await backend.browseProject(opened.rootPath, '.'))
      const recentSessions = await backend.listSessions(opened.id)
      setSessions(recentSessions)
      setSession(recentSessions[0])
      setPage('home')
      setAppState('ready')
    } catch (error) {
      setDiagnostic(error instanceof Error ? error.message : String(error))
      setAppState('diagnostic')
    }
  }, [projects, setFiles, setProject, setProjects, setSession, setSessions])

  const selectSession = useCallback(async (next: Session) => {
    setSession(next)
    setModel(next.model)
    setPage('home')
    try {
      setMessages((await backend.loadConversation(next.id)).messages)
    } catch {
      setMessages([])
    }
  }, [setMessages, setSession])

  const createSession = useCallback(async (title = '新对话') => {
    if (!project) {
      setDiagnostic('请先打开一个项目。')
      return
    }
    try {
      const created = await backend.createSession(project.id, title, 'openai-compatible', model)
      setSessions([created, ...sessions])
      setSession(created)
      setMessages([])
    } catch {
      const created = { ...demoSession, id: `local-${Date.now()}`, projectId: project.id, title }
      setSessions([created, ...sessions])
      setSession(created)
      setMessages([])
    }
  }, [model, project, sessions, setMessages, setSession, setSessions])

  const startNewChat = useCallback(async () => {
    setPage('home')
    setComposer('')
    await createSession('新对话')
  }, [createSession])

  const renameCurrentSession = useCallback(async () => {
    if (!session) return
    const title = window.prompt('重命名对话', session.title)?.trim()
    if (!title || title === session.title) return
    try { await backend.renameSession(session.id, title) } catch { /* preview mode */ }
    const renamed = { ...session, title }
    setSession(renamed)
    setSessions(sessions.map((item) => item.id === session.id ? renamed : item))
  }, [session, sessions, setSession, setSessions])

  const browse = useCallback(async (path: string) => {
    if (!project) return
    try {
      setFiles(await backend.browseProject(project.rootPath, path))
    } catch {
      setFiles(demoFiles.filter((item) => item.path.startsWith(path === '.' ? '' : path)))
    }
  }, [project, setFiles])

  const submitMessage = useCallback(async () => {
    const text = composer.trim()
    if (!text || running) return
    let active = session
    if (!active && project) {
      try {
        active = await backend.createSession(project.id, text.slice(0, 36), 'openai-compatible', model)
      } catch {
        active = { ...demoSession, id: `local-${Date.now()}`, projectId: project.id, title: text.slice(0, 36) }
      }
      setSessions([active, ...sessions])
      setSession(active)
    }
    if (!active) {
      setDiagnostic('请先打开一个项目并创建对话。')
      return
    }
    addMessage({ role: 'user', content: text })
    setComposer('')
    setRunning(true)
    setTurnStatus('running')
    setUsage({})
    setPage('home')
    try {
      const turn = await backend.sendMessage(active.id, text, 'openai-compatible', model)
      const nextChanges = await backend.turnChanges(turn.id).catch(() => [])
      setChanges(nextChanges)
    } catch (error) {
      addMessage({ role: 'assistant', content: `无法开始此 Turn：${error instanceof Error ? error.message : String(error)}` })
      setRunning(false)
      setDiagnostic('发送前请先在设置中配置凭据引用。')
    }
  }, [addMessage, composer, model, project, running, session, sessions, setSession, setSessions])

  const stopAgent = useCallback(async () => {
    if (session) await backend.cancelAgent(session.id).catch(() => undefined)
    setApproval(undefined)
    setRunning(false)
    setTurnStatus('cancelled')
  }, [session])

  const resolveApproval = useCallback(async (status: 'approved' | 'rejected') => {
    if (!approval) return
    const pending = approval
    setApproval(undefined)
    await backend.resolveApproval({ approvalId: pending.id, status, requestHash: pending.hash }).catch(() => undefined)
    if (status === 'rejected') setRunning(false)
  }, [approval])

  const undoSelectedTurn = useCallback(async () => {
    const turnId = selectedChange?.turnId
    if (!turnId || !window.confirm('撤销此 Turn 的全部更改？')) return
    try {
      await backend.undoTurn(turnId)
      setChanges(changes.filter((item) => item.turnId !== turnId))
      setSelectedChange(undefined)
    } catch (error) {
      setDiagnostic(error instanceof Error ? error.message : String(error))
    }
  }, [changes, selectedChange])

  const groupedChanges = useMemo(
    () => changes.reduce<Record<string, FileChange[]>>((groups, change) => {
      (groups[change.turnId] ??= []).push(change)
      return groups
    }, {}),
    [changes],
  )

  const setupTotal = 5
  const setupDone = [
    Boolean(project),
    model !== 'not configured',
    sessions.length > 0,
    messages.length > 0,
    capabilities.length > 0 || files.length > 0,
  ].filter(Boolean).length

  return {
    appState,
    diagnostic,
    dismissDiagnostic: () => setDiagnostic(''),
    retryInit: () => { void initialize() },
    page,
    setPage,
    sidebarOpen,
    setSidebarOpen,
    composer,
    setComposer,
    running,
    approval,
    changes,
    selectedChange,
    setSelectedChange,
    model,
    setModel,
    endpoint,
    setEndpoint,
    credentialEnv,
    setCredentialEnv,
    contextLimit,
    setContextLimit,
    turnLimit,
    setTurnLimit,
    turnStatus,
    usage,
    project,
    projects,
    sessions,
    session,
    files,
    capabilities,
    messages,
    groupedChanges,
    setupDone,
    setupTotal,
    openProject,
    selectProject,
    selectSession,
    createSession,
    renameCurrentSession,
    startNewChat,
    browse,
    submitMessage,
    stopAgent,
    resolveApproval,
    undoSelectedTurn,
  }
}
