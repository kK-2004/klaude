import { useEffect, useRef, useState } from 'react'
import { ArrowUp, Bot, Check, ChevronDown, Clock, Folder, PencilLine, ShieldCheck, Sparkles, Square, UserRoundCheck, Zap } from 'lucide-react'
import { useApp } from '../app/use-app'
import type { ApprovalMode } from '../lib/backend'
import { GitBranchSelector } from './GitBranchSelector'

export function Composer() {
  const {
    appState, composer, setComposer, running, submitMessage, stopAgent,
    messages, model, project, openProject,
  } = useApp()
  const showContext = messages.length === 0

  return (
    <div className="composer-dock">
      {showContext && (
        <div className="context-bar">
          <button type="button" className="context-pill context-project" title={project?.rootPath ?? '选择项目目录'} onClick={() => void openProject()}>
            <Folder size={13} strokeWidth={1.75} />{project?.name ?? '选择项目'}
          </button>
          <GitBranchSelector gitRoot={project?.gitRoot} />
        </div>
      )}
      <div className="composer">
        <textarea
          disabled={appState !== 'ready'}
          value={composer}
          onChange={(event) => setComposer(event.target.value)}
          onKeyDown={(event) => {
            if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') void submitMessage()
          }}
          placeholder="随心输入"
          rows={3}
        />
        <div className="composer-toolbar">
          <div className="toolbar-left">
            <ApprovalModeSelector />
          </div>
          <div className="toolbar-right">
            <ModelSelector model={model} />
            <button
              type="button"
              className={running ? 'send-btn stop' : 'send-btn'}
              disabled={appState !== 'ready' && !running}
              onClick={() => running ? void stopAgent() : void submitMessage()}
              aria-label={running ? '停止' : '发送'}
            >
              {running ? <Square size={13} /> : <ArrowUp size={16} strokeWidth={2.4} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

const approvalOptions: { value: ApprovalMode; label: string; description: string; icon: typeof Clock }[] = [
  { value: 'ask', label: '帮我批准', description: '读取自动，写入和命令先询问', icon: Clock },
  { value: 'manual', label: '手动', description: '每次工具操作都需要批准', icon: UserRoundCheck },
  { value: 'full', label: '完全允许', description: '工作区内自动执行，高危命令仍拒绝', icon: Zap },
]

function ApprovalModeSelector() {
  const { approvalMode, applyApprovalMode } = useApp()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const selected = approvalOptions.find((item) => item.value === approvalMode) ?? approvalOptions[0]
  const SelectedIcon = selected.icon

  useEffect(() => {
    const close = (event: MouseEvent) => { if (!ref.current?.contains(event.target as Node)) setOpen(false) }
    window.addEventListener('mousedown', close)
    return () => window.removeEventListener('mousedown', close)
  }, [])

  return (
    <div className="composer-control" ref={ref}>
      <button type="button" className="text-chip active" aria-expanded={open} onClick={() => setOpen((value) => !value)}>
        <SelectedIcon size={14} strokeWidth={1.75} />{selected.label}<ChevronDown size={12} />
      </button>
      {open && (
        <div className="composer-popover approval-popover">
          <div className="composer-popover-title"><ShieldCheck size={14} />批准模式</div>
          {approvalOptions.map((item) => {
            const Icon = item.icon
            return (
              <button key={item.value} type="button" className={item.value === approvalMode ? 'selected' : ''} onClick={async () => { if (await applyApprovalMode(item.value)) setOpen(false) }}>
                <Icon size={15} />
                <span><strong>{item.label}</strong><small>{item.description}</small></span>
                {item.value === approvalMode && <Check size={14} />}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}

function ModelSelector({ model }: { model: string }) {
  const { modelCatalog, selectModelProfile, openModelSettings } = useApp()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const active = modelCatalog.profiles.find((profile) => profile.id === modelCatalog.activeId)

  useEffect(() => {
    const close = (event: MouseEvent) => { if (!ref.current?.contains(event.target as Node)) setOpen(false) }
    window.addEventListener('mousedown', close)
    return () => window.removeEventListener('mousedown', close)
  }, [])

  return (
    <div className="composer-control model-control" ref={ref}>
      <button type="button" className="model-chip" aria-expanded={open} onClick={() => setOpen((current) => !current)}>
        {active?.name ?? (model === 'not configured' ? '选择模型' : model)}<ChevronDown size={13} strokeWidth={1.75} />
      </button>
      {open && (
        <div className="composer-popover model-popover">
          <div className="model-popover-list" role="listbox" aria-label="选择模型">
            {modelCatalog.profiles.map((profile) => {
              const ProviderIcon = profile.providerSpec === 'anthropic' ? Sparkles : Bot
              const selected = profile.id === modelCatalog.activeId
              const mode = profile.apiMode === 'chat_completions' ? 'Chat' : profile.apiMode === 'responses' ? 'Responses' : 'Messages'
              return (
                <button
                  key={profile.id}
                  type="button"
                  role="option"
                  aria-selected={selected}
                  className={selected ? 'selected' : ''}
                  onClick={async () => { if (await selectModelProfile(profile.id)) setOpen(false) }}
                >
                  <span className={`model-provider-icon ${profile.providerSpec}`}><ProviderIcon size={18} strokeWidth={1.8} /></span>
                  <span className="model-option-copy"><strong>{profile.name}</strong><small>{profile.model}</small></span>
                  <span className="model-option-meta"><small>{mode} · {formatContext(profile.contextWindow)}</small>{selected && <Check size={14} strokeWidth={2.2} />}</span>
                </button>
              )
            })}
            {modelCatalog.profiles.length === 0 && <div className="model-popover-empty">还没有可用模型</div>}
          </div>
          <button type="button" className="configure-model-button" onClick={() => { setOpen(false); openModelSettings() }}><PencilLine size={17} />配置自定义模型</button>
        </div>
      )}
    </div>
  )
}

function formatContext(value: number) {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(value % 1_000_000 === 0 ? 0 : 1)}m`
  if (value >= 1_000) return `${Math.round(value / 1_000)}k`
  return String(value)
}
