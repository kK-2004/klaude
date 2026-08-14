import { useCallback, useEffect, useMemo, useState } from 'react'
import { backend } from '../lib/backend'
import { useAgentStore } from '../stores/agent'
import { useThemeStore } from '../stores/theme'
import { useWorkspaceStore } from '../stores/workspace'
import type { AgentEvent, FileChange, Project, Session } from '../types/backend'
import type { AppController, AppPage, AppState, PendingApproval } from './types'
import type { ApprovalMode, BackendSettings, MCPServer, MCPServerInput, ModelCatalog, ModelConnectionResult, ModelProfileInput, Platform, SettingsUpdate } from '../lib/backend'
import { emptyModelCatalog } from './model-state'
import { removeProject } from './project-state'
import { draftConversationState } from './session-state'

function isBackendUnavailable(error: unknown) {
  const message = error instanceof Error ? error.message : String(error)
  return message.includes('unavailable') || message.includes('Wails')
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}

function detectPlatform(): Platform {
  const agent = typeof navigator === 'undefined' ? '' : navigator.userAgent
  if (agent.includes('Win')) return 'windows'
  if (agent.includes('Linux') && !agent.includes('Android')) return 'linux'
  return 'darwin'
}

// 与后端 ListProjects 的排序保持一致：置顶优先，其余按最近使用。
function sortProjects(projects: Project[]): Project[] {
  return [...projects].sort((left, right) => {
    if (Boolean(left.pinned) !== Boolean(right.pinned)) return left.pinned ? -1 : 1
    return right.updatedAt.localeCompare(left.updatedAt)
  })
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
  const [model, setModel] = useState('')
  const [modelCatalog, setModelCatalog] = useState<ModelCatalog>(emptyModelCatalog)
  const [mcpServers, setMCPServers] = useState<MCPServer[]>([])
  const [endpoint, setEndpoint] = useState('')
  const [credentialEnv, setCredentialEnv] = useState('')
  const [contextLimit, setContextLimit] = useState('120000')
  const [turnLimit, setTurnLimit] = useState('50')
  const [parallelTools, setParallelTools] = useState(false)
  const [llmSchedule, setLLMSchedule] = useState(false)
  const [approvalMode, setApprovalMode] = useState<ApprovalMode>('ask')
  const [turnStatus, setTurnStatus] = useState('idle')
  const [usage, setUsage] = useState<{ input?: number; output?: number }>({})
  const [platform, setPlatform] = useState<Platform>(detectPlatform)

	const project = useWorkspaceStore((state) => state.project)
	const projects = useWorkspaceStore((state) => state.projects)
	const sessions = useWorkspaceStore((state) => state.sessions)
	const recentSessions = useWorkspaceStore((state) => state.recentSessions)
  const session = useWorkspaceStore((state) => state.session)
  const files = useWorkspaceStore((state) => state.files)
  const capabilities = useWorkspaceStore((state) => state.capabilities)
  const messages = useWorkspaceStore((state) => state.messages)
  const setProject = useWorkspaceStore((state) => state.setProject)
	const setProjects = useWorkspaceStore((state) => state.setProjects)
	const setSessions = useWorkspaceStore((state) => state.setSessions)
	const setRecentSessions = useWorkspaceStore((state) => state.setRecentSessions)
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
      if (health.platform) setPlatform(health.platform)
      if (!health.ready) {
        setAppState('diagnostic')
        setDiagnostic('应用以只读诊断模式启动。')
        return
      }
		const [recentProjects, globalRecentSessions, caps, settings, catalog, configuredMCPServers] = await Promise.all([
			backend.listProjects(), backend.listRecentSessions(), backend.capabilities(), backend.settings().catch(() => undefined), backend.modelProfiles().catch(() => undefined), backend.mcpServers().catch(() => []),
		])
		setRecentSessions(globalRecentSessions)
      setMCPServers(configuredMCPServers)
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
        } else {
          setSession(undefined)
          setMessages([])
        }
        setFiles(await backend.browseProject(selected.rootPath, '.'))
      } else {
		setProject(undefined)
		setSessions([])
		setRecentSessions([])
        setSession(undefined)
        setMessages([])
        setFiles([])
      }
      setAppState('ready')
    } catch (error) {
      const message = errorMessage(error)
      if (isBackendUnavailable(error)) {
        setModelCatalog(emptyModelCatalog)
        setMCPServers([])
        setModel('')
        setProject(undefined)
        setProjects([])
        setSession(undefined)
        setSessions([])
        setMessages([])
        setFiles([])
        setAppState('ready')
        setDiagnostic('预览模式：请通过 Wails 启动桌面应用以连接后端。')
      } else {
        setAppState('fatal')
        setDiagnostic(message)
      }
    }
	}, [setCapabilities, setFiles, setMessages, setProject, setProjects, setRecentSessions, setSession, setSessions])

  useEffect(() => { void initialize() }, [initialize])

  useEffect(() => {
    const unsubscribe = window.runtime?.EventsOn?.('klaude:agent-event', (event: AgentEvent) => {
      applyEvent(event)
      const payload = event.payload as { delta?: string; text?: string; message?: string; inputTokens?: number; outputTokens?: number; approval?: PendingApproval } | undefined
      if (payload?.delta || payload?.text) appendAssistantDelta(payload.delta ?? payload.text ?? '')
      if (payload?.approval) setApproval(payload.approval)
      if (payload?.inputTokens !== undefined || payload?.outputTokens !== undefined) setUsage({ input: payload.inputTokens, output: payload.outputTokens })
      if (event.type.endsWith('started')) { setTurnStatus('running'); setRunning(true) }
      if (event.type.includes('approval')) setTurnStatus('waiting_approval')
      if (event.type.endsWith('finished')) { setTurnStatus('completed'); setRunning(false) }
      if (event.type.endsWith('cancelled')) { setTurnStatus('cancelled'); setRunning(false) }
      if (event.type.endsWith('failed')) { setTurnStatus('failed'); setRunning(false) }
      if (event.type.endsWith('error')) {
        setTurnStatus('failed')
        setRunning(false)
        if (payload?.message) setDiagnostic(payload.message)
      }
    })
    return unsubscribe
  }, [appendAssistantDelta, applyEvent])

  useEffect(() => {
    if (!needsSnapshot || !session) return
    void backend.loadConversation(session.id).then((snapshot) => setMessages(snapshot.messages)).catch(() => undefined)
  }, [needsSnapshot, session, setMessages])

  // loadProjectWorkspace 是切换项目的唯一入口：同时刷新文件树、会话列表与会话内容。
  // session 用于重选项目后停留在同一个会话；keepDraft 保持「新对话」状态，不自动打开旧会话。
  const loadProjectWorkspace = useCallback(async (item: Project, options: { session?: Session; keepDraft?: boolean } = {}) => {
    setProject(item)
    setPage('home')
    const [entries, projectSessions, globalRecentSessions] = await Promise.all([
      backend.browseProject(item.rootPath, '.').catch(() => []),
      backend.listSessions(item.id).catch(() => []),
      backend.listRecentSessions().catch(() => []),
    ])
    setFiles(entries)
    setSessions(projectSessions)
    setRecentSessions(globalRecentSessions)
    const preferred = options.session
    const next = preferred
      ? projectSessions.find((entry) => entry.id === preferred.id) ?? preferred
      : options.keepDraft ? undefined : projectSessions[0]
    setSession(next)
    if (!next) {
      setMessages([])
      return
    }
    try {
      setMessages((await backend.loadConversation(next.id)).messages)
    } catch {
      setMessages([])
    }
  }, [setFiles, setMessages, setProject, setRecentSessions, setSession, setSessions])

  const closeProject = useCallback(() => {
    setProject(undefined)
    setSessions([])
    setSession(undefined)
    setMessages([])
    setFiles([])
    setChanges([])
    setSelectedChange(undefined)
    setPage('home')
  }, [setFiles, setMessages, setProject, setSession, setSessions])

  const selectProject = useCallback(async (item: Project) => {
    await loadProjectWorkspace(item)
  }, [loadProjectWorkspace])

  const pickProjectDirectory = useCallback(async () => {
    const path = await backend.selectDirectory(project?.rootPath ?? '')
    if (!path) return undefined
    const opened = await backend.openProject(path)
    setProjects(sortProjects([opened, ...projects.filter((item) => item.rootPath !== opened.rootPath)]))
    return opened
  }, [project?.rootPath, projects, setProjects])

  const openProject = useCallback(async () => {
    try {
      const opened = await pickProjectDirectory()
      if (!opened) return
      await loadProjectWorkspace(opened)
      setAppState('ready')
    } catch (error) {
      setDiagnostic(errorMessage(error))
    }
  }, [loadProjectWorkspace, pickProjectDirectory])

  // 输入框上的项目胶囊只切换工作区，不创建或移动会话。
  const chooseProjectForSession = useCallback(async () => {
    try {
      const opened = await pickProjectDirectory()
      if (!opened) return
      await loadProjectWorkspace(opened, { keepDraft: true })
      setAppState('ready')
    } catch (error) {
      setDiagnostic(errorMessage(error))
    }
  }, [loadProjectWorkspace, pickProjectDirectory])

  const refreshProjects = useCallback(async (fallback: Project[]) => {
    try {
      setProjects(await backend.listProjects())
    } catch {
      setProjects(sortProjects(fallback))
    }
  }, [setProjects])

  const refreshRecentSessions = useCallback(async () => {
    try {
      setRecentSessions((await backend.listRecentSessions()).slice(0, 10))
    } catch {
      // 预览模式没有后端，保留当前内存列表。
    }
  }, [setRecentSessions])

  const renameProject = useCallback(async (item: Project) => {
    const name = window.prompt('重命名项目', item.name)?.trim()
    if (!name || name === item.name) return
    try {
      await backend.renameProject(item.id, name)
    } catch (error) {
      if (!isBackendUnavailable(error)) {
        setDiagnostic(errorMessage(error))
        return
      }
    }
    const renamed = { ...item, name }
    await refreshProjects(projects.map((entry) => entry.id === item.id ? renamed : entry))
    if (project?.id === item.id) setProject(renamed)
  }, [project?.id, projects, refreshProjects, setProject])

  const toggleProjectPinned = useCallback(async (item: Project) => {
    const pinned = !item.pinned
    try {
      await backend.setProjectPinned(item.id, pinned)
    } catch (error) {
      if (!isBackendUnavailable(error)) {
        setDiagnostic(errorMessage(error))
        return
      }
    }
    await refreshProjects(projects.map((entry) => entry.id === item.id ? { ...entry, pinned } : entry))
  }, [projects, refreshProjects])

  const deleteProject = useCallback(async (item: Project) => {
    if (!window.confirm(`删除项目「${item.name}」？该项目下的对话记录会一并移除，磁盘文件不受影响。`)) return
    try {
      await backend.deleteProject(item.id)
    } catch (error) {
      if (!isBackendUnavailable(error)) {
        setDiagnostic(errorMessage(error))
        return
      }
    }
    const remaining = removeProject(projects, item.id)
    setProjects(remaining)
    setRecentSessions(recentSessions.filter((entry) => entry.projectId !== item.id))
    if (project?.id !== item.id) return
    const next = remaining[0]
    if (next) await loadProjectWorkspace(next)
    else closeProject()
  }, [closeProject, loadProjectWorkspace, project?.id, projects, recentSessions, setProjects, setRecentSessions])

  const revealProject = useCallback(async (item: Project) => {
    try {
      await backend.revealProject(item.id)
    } catch (error) {
      setDiagnostic(isBackendUnavailable(error) ? '请通过 Klaude 桌面应用打开项目目录。' : errorMessage(error))
    }
  }, [])

  const selectSession = useCallback(async (next: Session) => {
    const owner = projects.find((item) => item.id === next.projectId)
    if (owner && owner.id !== project?.id) {
      await loadProjectWorkspace(owner, { session: next })
      return
    }
    setSession(next)
    setPage('home')
    try {
      setMessages((await backend.loadConversation(next.id)).messages)
    } catch {
      setMessages([])
    }
  }, [loadProjectWorkspace, project?.id, projects, setMessages, setSession])

  const activeProviderName = useMemo(() => {
    const active = modelCatalog.profiles.find((profile) => profile.id === modelCatalog.activeId)
    return active ? `${active.providerSpec}-compatible` : ''
  }, [modelCatalog])

  const createSession = useCallback(async (title = '新对话') => {
    if (!project) {
      setDiagnostic('请先打开一个项目。')
      return
    }
    try {
      const created = await backend.createSession(project.id, title, activeProviderName, model)
      setSessions([created, ...sessions])
      setRecentSessions([created, ...recentSessions.filter((item) => item.id !== created.id)].slice(0, 10))
      setSession(created)
      setMessages([])
    } catch (error) {
      setDiagnostic(error instanceof Error ? error.message : String(error))
    }
  }, [activeProviderName, model, project, recentSessions, sessions, setMessages, setRecentSessions, setSession, setSessions])

  const startNewChat = useCallback(async () => {
    setPage('home')
    setComposer('')
    const draft = draftConversationState()
    setSession(draft.session)
    setMessages(draft.messages)
    setChanges([])
    setSelectedChange(undefined)
  }, [setMessages, setSession])

  const renameCurrentSession = useCallback(async () => {
    if (!session) return
    const title = window.prompt('重命名对话', session.title)?.trim()
    if (!title || title === session.title) return
    try { await backend.renameSession(session.id, title) } catch { /* preview mode */ }
    const renamed = { ...session, title }
    setSession(renamed)
    setSessions(sessions.map((item) => item.id === session.id ? renamed : item))
    setRecentSessions(recentSessions.map((item) => item.id === session.id ? renamed : item))
  }, [recentSessions, session, sessions, setRecentSessions, setSession, setSessions])

  const browse = useCallback(async (path: string) => {
    if (!project) return
    try {
      setFiles(await backend.browseProject(project.rootPath, path))
    } catch (error) {
      setFiles([])
      setDiagnostic(error instanceof Error ? error.message : String(error))
    }
  }, [project, setFiles])

  const submitMessage = useCallback(async () => {
    const text = composer.trim()
    if (!text || running) return
    let active = session
    if (!active && project) {
      try {
        active = await backend.createSession(project.id, text.slice(0, 36), activeProviderName, model)
      } catch (error) {
        setDiagnostic(error instanceof Error ? error.message : String(error))
        return
      }
      setSessions([active, ...sessions])
      setRecentSessions([active, ...recentSessions.filter((item) => item.id !== active?.id)].slice(0, 10))
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
      await refreshRecentSessions()
    } catch (error) {
      const message = errorMessage(error)
      addMessage({ role: 'assistant', content: `无法开始此 Turn：${message}` })
      setRunning(false)
      setTurnStatus('failed')
      setDiagnostic(message)
    }
  }, [activeProviderName, addMessage, composer, model, project, recentSessions, refreshRecentSessions, running, session, sessions, setRecentSessions, setSession, setSessions])

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
    setEndpoint(saved?.Provider?.Endpoint ?? '')
    setModel(saved?.Provider?.Model ?? '')
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
      const message = errorMessage(error)
      if (isBackendUnavailable(error)) {
        setPage('home')
        return
      }
      setDiagnostic(message)
    }
  }, [persistSettings])

  const refreshMCPServers = useCallback(async () => {
    try {
      setMCPServers(await backend.mcpServers())
    } catch (error) {
      if (!isBackendUnavailable(error)) setDiagnostic(errorMessage(error))
    }
  }, [])

  const saveMCPServer = useCallback(async (input: MCPServerInput) => {
    try {
      setMCPServers(await backend.saveMCPServer(input))
      return true
    } catch (error) {
      setDiagnostic(errorMessage(error))
      return false
    }
  }, [])

  const deleteMCPServer = useCallback(async (id: string) => {
    try {
      setMCPServers(await backend.deleteMCPServer(id))
      return true
    } catch (error) {
      setDiagnostic(errorMessage(error))
      return false
    }
  }, [])

  const connectMCPServer = useCallback(async (id: string) => {
    try {
      setMCPServers(await backend.connectMCPServer(id))
      return true
    } catch (error) {
      const message = errorMessage(error)
      setDiagnostic(message)
      try { setMCPServers(await backend.mcpServers()) } catch { /* keep the error state */ }
      return false
    }
  }, [])

  const disconnectMCPServer = useCallback(async (id: string) => {
    try {
      setMCPServers(await backend.disconnectMCPServer(id))
      return true
    } catch (error) {
      setDiagnostic(errorMessage(error))
      return false
    }
  }, [])

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
      const message = errorMessage(error)
      if (isBackendUnavailable(error)) return true
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
      const message = errorMessage(error)
      if (isBackendUnavailable(error)) {
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
      const message = errorMessage(error)
      if (isBackendUnavailable(error)) {
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
      const message = errorMessage(error)
      if (isBackendUnavailable(error)) return true
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
    mcpServers,
    refreshMCPServers,
    saveMCPServer,
    deleteMCPServer,
    connectMCPServer,
    disconnectMCPServer,
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
    platform,
    project,
    projects,
    sessions,
    recentSessions,
    session,
    files,
    capabilities,
    messages,
    groupedChanges,
    openProject,
    chooseProjectForSession,
    closeProject,
    selectProject,
    renameProject,
    toggleProjectPinned,
    deleteProject,
    revealProject,
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
