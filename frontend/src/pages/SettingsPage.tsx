import { useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowLeft, Bot, Check, ChevronRight, Gauge, GitBranch, Layers, Moon,
  Palette, Search, Settings, ShieldCheck, SlidersHorizontal, Sun, Zap,
} from 'lucide-react'
import { useApp } from '../app/use-app'
import type { ApprovalMode } from '../lib/backend'
import { useThemeStore } from '../stores/theme'
import { ModelSettings } from '../components/ModelSettings'

const approvalModes: { value: ApprovalMode; title: string; detail: string }[] = [
  { value: 'ask', title: '帮我批准', detail: '可自动读取工作区；写入文件和执行命令前会请求批准。' },
  { value: 'manual', title: '手动', detail: '包括读取在内的每次工具操作都需要你确认。' },
  { value: 'full', title: '完全允许', detail: '在工作区边界内自动读写并执行命令；显式高危命令仍会被拒绝。' },
]

const navItems = [
  { id: 'permissions', label: '权限', icon: ShieldCheck },
  { id: 'appearance', label: '外观', icon: Palette },
  { id: 'model', label: '模型', icon: Bot },
  { id: 'tools', label: '工具并发', icon: Layers },
  { id: 'workspace', label: '工作区', icon: Gauge },
]

export function SettingsPage() {
  const {
    setPage, contextLimit, setContextLimit, turnLimit, setTurnLimit,
    parallelTools, setParallelTools, llmSchedule, setLLMSchedule,
    approvalMode, setApprovalMode, saveSettings,
  } = useApp()
  const theme = useThemeStore((state) => state.theme)
  const setTheme = useThemeStore((state) => state.setTheme)
  const [query, setQuery] = useState('')
  const [activeId, setActiveId] = useState(navItems[0].id)
  const contentRef = useRef<HTMLElement>(null)
  const visibleNav = useMemo(() => navItems.filter((item) => item.label.includes(query.trim())), [query])
  const currentId = visibleNav.some((item) => item.id === activeId) ? activeId : visibleNav[0]?.id

  const goTo = (id: string) => {
    setActiveId(id)
    document.getElementById(`settings-${id}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  useEffect(() => {
    const root = contentRef.current
    if (!root) return
    const observer = new IntersectionObserver((entries) => {
      const visible = entries
        .filter((entry) => entry.isIntersecting)
        .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
      const id = visible[0]?.target.id.replace(/^settings-/, '')
      if (id && navItems.some((item) => item.id === id)) setActiveId(id)
    }, { root, rootMargin: '-18% 0px -68% 0px', threshold: 0 })
    for (const item of navItems) {
      const section = document.getElementById(`settings-${item.id}`)
      if (section) observer.observe(section)
    }
    return () => observer.disconnect()
  }, [])

  return (
    <div className="settings-shell">
      <aside className="settings-sidebar">
        <button type="button" className="settings-back" onClick={() => setPage('home')}><ArrowLeft size={15} />返回应用</button>
        <label className="settings-search">
          <Search size={14} />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索设置…" />
        </label>
        <div className="settings-nav-label">常规</div>
        <nav className="settings-nav">
          {visibleNav.map((item) => {
            const Icon = item.icon
            const active = item.id === currentId
            return (
              <button
                key={item.id}
                type="button"
                className={active ? 'active' : ''}
                aria-current={active ? 'page' : undefined}
                onClick={() => goTo(item.id)}
              >
                <Icon size={14} />{item.label}<ChevronRight size={12} />
              </button>
            )
          })}
          {visibleNav.length === 0 && <p>没有匹配的设置</p>}
        </nav>
      </aside>

      <main className="settings-content" ref={contentRef}>
        <div className="settings-content-inner">
          <header className="settings-heading">
            <span><Settings size={18} /></span>
            <div><h1>常规</h1><p>管理 Klaude 的权限、模型、工具调度和工作区限制。</p></div>
          </header>

          <SettingsGroup id="permissions" title="权限" description="设置新会话中工具调用的默认批准策略。">
            <div className="permission-options">
              {approvalModes.map((item) => (
                <button key={item.value} type="button" className={approvalMode === item.value ? 'selected' : ''} onClick={() => setApprovalMode(item.value)}>
                  <span className="radio-mark">{approvalMode === item.value && <Check size={12} />}</span>
                  <span><strong>{item.title}</strong><small>{item.detail}</small></span>
                  {item.value === 'full' && <Zap className="permission-accent" size={15} />}
                </button>
              ))}
            </div>
          </SettingsGroup>

          <SettingsGroup id="appearance" title="外观" description="主题会应用到整个桌面应用。">
            <div className="settings-row">
              <div><strong>应用主题</strong><small>切换浅色或深色外观</small></div>
              <div className="segmented-control">
                <button type="button" className={theme === 'light' ? 'selected' : ''} onClick={() => setTheme('light')}><Sun size={14} />浅色</button>
                <button type="button" className={theme === 'dark' ? 'selected' : ''} onClick={() => setTheme('dark')}><Moon size={14} />深色</button>
              </div>
            </div>
          </SettingsGroup>

          <SettingsGroup id="model" title="模型" description="配置 OpenAI 或 Anthropic 规范、API 模式与推理参数；可在保存前发送真实请求验证连接。">
            <ModelSettings />
          </SettingsGroup>

          <SettingsGroup id="tools" title="工具并发" description="无资源冲突的工具可并行执行，涉及同一文件的修改仍会串行。">
            <ToggleRow icon={Layers} title="并行工具调度" detail="同层执行可安全并行的读取和写入工具" checked={parallelTools} onChange={setParallelTools} />
            <ToggleRow icon={GitBranch} title="LLM 拓扑回退" detail="当依赖关系不明确时，让模型补全调度边" checked={llmSchedule} disabled={!parallelTools} onChange={setLLMSchedule} />
          </SettingsGroup>

          <SettingsGroup id="workspace" title="工作区" description="限制单次会话的上下文和 Agent 循环。">
            <div className="settings-number-grid">
              <SettingsField label="上下文字符" value={contextLimit} onChange={(value) => setContextLimit(value.replace(/\D/g, ''))} inputMode="numeric" />
              <SettingsField label="最大 Turns" value={turnLimit} onChange={(value) => setTurnLimit(value.replace(/\D/g, ''))} inputMode="numeric" />
            </div>
            <div className="workspace-boundary"><SlidersHorizontal size={15} /><span>文件和命令被限制在当前项目工作区；权限模式不会绕过路径边界和高危命令拦截。</span></div>
          </SettingsGroup>

          <div className="settings-save-bar">
            <span><ShieldCheck size={14} />设置保存到本机 config.toml</span>
            <button type="button" className="primary-save" onClick={() => void saveSettings()}><Check size={15} />保存更改</button>
          </div>
        </div>
      </main>
    </div>
  )
}

function SettingsGroup({ id, title, description, children }: { id: string; title: string; description: string; children: React.ReactNode }) {
  return <section id={`settings-${id}`} className="settings-group"><header><h2>{title}</h2><p>{description}</p></header><div className="settings-card">{children}</div></section>
}

function SettingsField({ label, value, onChange, placeholder, mono, inputMode }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string; mono?: boolean; inputMode?: 'numeric' }) {
  return <label className="settings-field"><span>{label}</span><input className={mono ? 'mono' : ''} inputMode={inputMode} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} /></label>
}

function ToggleRow({ icon: Icon, title, detail, checked, disabled, onChange }: { icon: typeof Layers; title: string; detail: string; checked: boolean; disabled?: boolean; onChange: (value: boolean) => void }) {
  return <div className={`settings-row ${disabled ? 'disabled' : ''}`}><div className="row-with-icon"><span><Icon size={15} /></span><div><strong>{title}</strong><small>{detail}</small></div></div><button type="button" role="switch" aria-checked={checked} className="switch" disabled={disabled} onClick={() => onChange(!checked)}><span /></button></div>
}
