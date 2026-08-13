import { useCallback, useEffect, useMemo, useState } from 'react'
import { backend } from '../lib/backend'
import { demoFiles, demoSession } from '../lib/demo'
import { useAgentStore } from '../stores/agent'
import { useThemeStore } from '../stores/theme'
import { useWorkspaceStore } from '../stores/workspace'
import type { AgentEvent, FileChange, Project, Session } from '../types/backend'
import type { AppController, AppPage, AppState, PendingApproval } from './types'
import type { ApprovalMode, BackendSettings, ModelCatalog, ModelConnectionResult, ModelProfileInput, SettingsUpdate } from '../lib/backend'

const previewModelCatalog: ModelCatalog = {
  activeId: 'preview-openai',
  profiles: [
    { id: 'preview-openai', name: 'GPT-5.6-Sol', providerSpec: 'openai', apiMode: 'responses', baseUrl: 'https://api.openai.com/v1', model: 'gpt-5.6-sol', contextWindow: 400000, maxOutputTokens: 32768, temperature: 0.2, hasApiKey: false },
    { id: 'preview-anthropic', name: 'Claude Sonnet', providerSpec: 'anthropic', apiMode: 'messages', baseUrl: 'https://api.anthropic.com/v1', model: 'claude-sonnet', contextWindow: 200000, maxOutputTokens: 16384, temperature: 0.2, hasApiKey: false },
  ],
}

function approvalModeFromSettings(settings?: BackendSettings): ApprovalMode {
  const permissions = settings?.Permissions
  if (permissions?.Read === 'allow' && permissions?.Write === 'allow' && permissions?.Shell === 'allow' && permissions?.Network === 'allow') return 'full'
  if (permissions?.Read === 'ask') return 'manual'
  return 'ask'
}

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
  const [modelCatalog, setModelCatalog] = useState<ModelCatalog>(previewModelCatalog)
  const [endpoint, setEndpoint] = useState('https://api.openai.com/v1')
  const [credentialEnv, setCredentialEnv] = useState('OPENAI_API_KEY')
  const [contextLimit, setContextLimit] = useState('120000')
  const [turnLimit, setTurnLimit] = useState('50')
  const [parallelTools, setParallelTools] = useState(false)
  const [llmSchedule, setLLMSchedule] = useState(false)
  const [approvalMode, setApprovalMode] = useState<ApprovalMode>('ask')
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
      const [recentProjects, caps, settings, catalog] = await Promise.all([
        backend.listProjects(), backend.capabilities(), backend.settings().catch(() => undefined), backend.modelProfiles().catch(() => undefined),
      ])
      if (catalog) {
        setModelCatalog(catalog)
        const activeProfile = catalog.profiles.find((profile) => profile.id === catalog.activeId)
        if (activeProfile) setModel(activeProfile.model)
      }
      if (settings?.Provider?.Endpoint) setEndpoint(settings.Provider.Endpoint)
      if (settings?.Provider?.Model) setModel(settings.Provider.Model)
      setCredentialEnv(settings?.Provider?.CredentialEnv ?? '')
      if (settings?.Agent?.ContextBudgetChars) setContextLimit(String(settings.Agent.ContextBudgetChars))
      if (settings?.Agent?.MaxTurns) setTurnLimit(String(settings.Agent.MaxTurns))
      setParallelTools(Boolean(settings?.Agent?.ParallelTools))
      setLLMSchedule(Boolean(settings?.Agent?.LLMSchedule))
      setApprovalMode(approvalModeFromSettings(settings))
      if (settings?.UI?.Theme === 'light' || settings?.UI?.Theme === 'dark') useThemeStore.getState().setTheme(settings.UI.Theme)
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
        }
        setFiles(await backend.browseProject(selected.rootPath, '.'))
      }
      setAppState('ready')
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      if (message.includes('unavailable') || message.includes('Wails')) {
        setModelCatalog(previewModelCatalog)
        setModel(previewModelCatalog.profiles[0].model)
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
      if (event.type.endsWith('error')) { setTurnStatus('failed'); setRunning(false) }
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
        setMessages((await backend.loadConversation(next.id)).messages)
      } else {
        setMessages([])
      }
    } catch {
      setFiles(demoFiles)
    }
  }, [setFiles, setMessages, setProject, setSession, setSessions])

  const openProject = useCallback(async () => {
    try {
      const path = await backend.selectDirectory(project?.rootPath ?? '')
      if (!path) return
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
  }, [project?.rootPath, projects, setFiles, setProject, setProjects, setSession, setSessions])

  const selectSession = useCallback(async (next: Session) => {
    setSession(next)
    setPage('home')
    try {
      setMessages((await backend.loadConversation(next.id)).messages)
    } catch {
      setMessages([])
    }
  }, [setMessages, setSession])

  const activeProviderName = useMemo(() => {
    const active = modelCatalog.profiles.find((profile) => profile.id === modelCatalog.activeId)
    return `${active?.providerSpec ?? 'openai'}-compatible`
  }, [modelCatalog])

  const createSession = useCallback(async (title = '新对话') => {
    if (!project) {
      setDiagnostic('请先打开一个项目。')
      return
    }
    try {
      const created = await backend.createSession(project.id, title, activeProviderName, model)
      setSessions([created, ...sessions])
      setSession(created)
      setMessages([])
    } catch {
      const created = { ...demoSession, id: `local-${Date.now()}`, projectId: project.id, title }
      setSessions([created, ...sessions])
      setSession(created)
      setMessages([])
    }
  }, [activeProviderName, model, project, sessions, setMessages, setSession, setSessions])

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
        active = await backend.createSession(project.id, text.slice(0, 36), activeProviderName, model)
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
      const turn = await backend.sendMessage(active.id, text, activeProviderName, model)
      const nextChanges = await backend.turnChanges(turn.id).catch(() => [])
      setChanges(nextChanges)
    } catch (error) {
      addMessage({ role: 'assistant', content: `无法开始此 Turn：${error instanceof Error ? error.message : String(error)}` })
      setRunning(false)
      setDiagnostic('发送前请先在设置中配置凭据引用。')
    }
  }, [activeProviderName, addMessage, composer, model, project, running, session, sessions, setSession, setSessions])

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

  const toggleParallelTools = useCallback((value: boolean) => {
    setParallelTools(value)
    if (!value) setLLMSchedule(false)
  }, [])

  const settingsPayload = useCallback((overrides: Partial<SettingsUpdate> = {}): SettingsUpdate => ({
    theme: useThemeStore.getState().theme,
    endpoint: endpoint.trim(),
    model: model.trim(),
    credentialEnv: credentialEnv.trim(),
    contextBudgetChars: Number(contextLimit),
    maxTurns: Number(turnLimit),
    parallelTools,
    llmSchedule: parallelTools && llmSchedule,
    approvalMode,
    ...overrides,
  }), [approvalMode, contextLimit, credentialEnv, endpoint, llmSchedule, model, parallelTools, turnLimit])

  const persistSettings = useCallback(async (overrides: Partial<SettingsUpdate> = {}) => {
    const payload = settingsPayload(overrides)
    const saved = await backend.updateSettings(payload)
    if (saved?.Provider?.Endpoint) setEndpoint(saved.Provider.Endpoint)
    if (saved?.Provider?.Model) setModel(saved.Provider.Model)
    setCredentialEnv(saved?.Provider?.CredentialEnv ?? '')
    if (saved?.Agent?.ContextBudgetChars) setContextLimit(String(saved.Agent.ContextBudgetChars))
    if (saved?.Agent?.MaxTurns) setTurnLimit(String(saved.Agent.MaxTurns))
    setParallelTools(Boolean(saved?.Agent?.ParallelTools))
    setLLMSchedule(Boolean(saved?.Agent?.LLMSchedule))
    setApprovalMode(approvalModeFromSettings(saved))
    return saved
  }, [settingsPayload])

  const saveSettings = useCallback(async () => {
    try {
      await persistSettings()
      setPage('home')
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      if (message.includes('unavailable') || message.includes('Wails')) {
        setPage('home')
        return
      }
      setDiagnostic(message)
    }
  }, [persistSettings])

  const selectModelProfile = useCallback(async (profileId: string) => {
    const previousCatalog = modelCatalog
    const selected = modelCatalog.profiles.find((profile) => profile.id === profileId)
    if (!selected) return false
    setModelCatalog({ ...modelCatalog, activeId: profileId })
    setModel(selected.model)
    try {
      const catalog = await backend.selectModelProfile(profileId)
      setModelCatalog(catalog)
      const active = catalog.profiles.find((profile) => profile.id === catalog.activeId)
      if (active) {
        setModel(active.model)
        setEndpoint(active.baseUrl)
      }
      return true
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      if (message.includes('unavailable') || message.includes('Wails')) return true
      setModelCatalog(previousCatalog)
      const previous = previousCatalog.profiles.find((profile) => profile.id === previousCatalog.activeId)
      if (previous) setModel(previous.model)
      setDiagnostic(message)
      return false
    }
  }, [modelCatalog])

  const saveModelProfile = useCallback(async (input: ModelProfileInput) => {
    try {
      const catalog = await backend.saveModelProfile(input)
      setModelCatalog(catalog)
      const active = catalog.profiles.find((profile) => profile.id === catalog.activeId)
      if (active) {
        setModel(active.model)
        setEndpoint(active.baseUrl)
        setCredentialEnv('')
      }
      return catalog
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      if (message.includes('unavailable') || message.includes('Wails')) {
        const profile = { ...input, hasApiKey: Boolean(input.apiKey) }
        const catalog = {
          activeId: input.id,
          profiles: [...modelCatalog.profiles.filter((item) => item.id !== input.id), profile],
        }
        setModelCatalog(catalog)
        setModel(input.model)
        setEndpoint(input.baseUrl)
        return catalog
      }
      setDiagnostic(message)
      return undefined
    }
  }, [modelCatalog])

  const testModelConnection = useCallback(async (input: ModelProfileInput): Promise<ModelConnectionResult> => {
    try {
      return await backend.testModelConnection(input)
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      if (message.includes('unavailable') || message.includes('Wails')) {
        return { success: false, latencyMs: 0, message: '请通过 Klaude 桌面应用测试真实连接。' }
      }
      return { success: false, latencyMs: 0, message }
    }
  }, [])

  const openModelSettings = useCallback(() => {
    setPage('settings')
    window.setTimeout(() => document.getElementById('settings-model')?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 80)
  }, [])

  const applyApprovalMode = useCallback(async (value: ApprovalMode) => {
    const previous = approvalMode
    setApprovalMode(value)
    try {
      await persistSettings({ approvalMode: value })
      return true
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      if (message.includes('unavailable') || message.includes('Wails')) return true
      setApprovalMode(previous)
      setDiagnostic(message)
      return false
    }
  }, [approvalMode, persistSettings])

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
    modelCatalog,
    selectModelProfile,
    saveModelProfile,
    testModelConnection,
    openModelSettings,
    contextLimit,
    setContextLimit,
    turnLimit,
    setTurnLimit,
    parallelTools,
    setParallelTools: toggleParallelTools,
    llmSchedule,
    setLLMSchedule,
    approvalMode,
    setApprovalMode,
    applyApprovalMode,
    saveSettings,
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
