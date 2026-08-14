import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Bot, Check, ChevronDown, CircleAlert, CircleCheck, FlaskConical, KeyRound, LoaderCircle, Plus, Save, Search, ShieldCheck, Sparkles } from 'lucide-react'
import { useApp } from '../app/use-app'
import type { ModelProfile, ModelProfileInput, ProviderSpec } from '../lib/backend'

const defaults: Record<ProviderSpec, Pick<ModelProfileInput, 'providerSpec' | 'apiMode' | 'baseUrl' | 'contextWindow' | 'maxOutputTokens' | 'temperature'>> = {
  openai: { providerSpec: 'openai', apiMode: 'responses', baseUrl: 'https://api.openai.com/v1', contextWindow: 128000, maxOutputTokens: 16384, temperature: 0.2 },
  anthropic: { providerSpec: 'anthropic', apiMode: 'messages', baseUrl: 'https://api.anthropic.com/v1', contextWindow: 200000, maxOutputTokens: 16384, temperature: 0.2 },
}

type Draft = ModelProfileInput & { hasStoredKey: boolean }
type ConnectionState = { kind: 'idle' | 'testing' | 'success' | 'error'; message?: string; latencyMs?: number }

function fromProfile(profile: ModelProfile): Draft {
  return { ...profile, apiKey: '', hasStoredKey: profile.hasApiKey }
}

function newDraft(): Draft {
  return {
    id: `model-${Date.now()}`,
    name: '',
    model: '',
    apiKey: '',
    hasStoredKey: false,
    ...defaults.openai,
  }
}

export function ModelSettings() {
  const { modelCatalog, saveModelProfile, testModelConnection } = useApp()
  const active = modelCatalog.profiles.find((profile) => profile.id === modelCatalog.activeId) ?? modelCatalog.profiles[0]
  const [draft, setDraft] = useState<Draft>(() => active ? fromProfile(active) : newDraft())
  const [connection, setConnection] = useState<ConnectionState>({ kind: 'idle' })
  const [saving, setSaving] = useState(false)
  const [profileMenuOpen, setProfileMenuOpen] = useState(false)
  const [profileQuery, setProfileQuery] = useState('')
  const [profileScrollbar, setProfileScrollbar] = useState({ top: 0, height: 32 })
  const profilePickerRef = useRef<HTMLDivElement>(null)
  const profileListRef = useRef<HTMLDivElement>(null)
  const selectedProfile = modelCatalog.profiles.find((profile) => profile.id === draft.id)
  const filteredProfiles = useMemo(() => {
    const query = profileQuery.trim().toLocaleLowerCase()
    if (!query) return modelCatalog.profiles
    return modelCatalog.profiles.filter((profile) => `${profile.name} ${profile.model} ${profile.providerSpec} ${profile.apiMode}`.toLocaleLowerCase().includes(query))
  }, [modelCatalog.profiles, profileQuery])
  const profileListScrollable = filteredProfiles.length > 5

  const syncProfileScrollbar = useCallback((element: HTMLDivElement | null) => {
    if (!element || element.scrollHeight <= element.clientHeight) return
    const trackHeight = Math.max(0, element.clientHeight - 8)
    const height = Math.max(28, Math.round(trackHeight * element.clientHeight / element.scrollHeight))
    const progress = element.scrollTop / (element.scrollHeight - element.clientHeight)
    setProfileScrollbar({ height, top: Math.round(progress * (trackHeight - height)) })
  }, [])

  useEffect(() => {
    if (active) setDraft(fromProfile(active))
  }, [active])
  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!profilePickerRef.current?.contains(event.target as Node)) setProfileMenuOpen(false)
    }
    window.addEventListener('mousedown', close)
    return () => window.removeEventListener('mousedown', close)
  }, [])
  useEffect(() => {
    if (!profileMenuOpen || !profileListScrollable) return
    const frame = window.requestAnimationFrame(() => syncProfileScrollbar(profileListRef.current))
    return () => window.cancelAnimationFrame(frame)
  }, [filteredProfiles.length, profileListScrollable, profileMenuOpen, syncProfileScrollbar])

  const update = <K extends keyof Draft>(key: K, value: Draft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }))
    setConnection({ kind: 'idle' })
  }
  const switchProvider = (providerSpec: ProviderSpec) => {
    setDraft((current) => ({ ...current, ...defaults[providerSpec], apiKey: '', hasStoredKey: current.providerSpec === providerSpec && current.hasStoredKey }))
    setConnection({ kind: 'idle' })
  }
  const input: ModelProfileInput = {
    id: draft.id,
    name: draft.name.trim(),
    providerSpec: draft.providerSpec,
    apiMode: draft.providerSpec === 'anthropic' ? 'messages' : draft.apiMode,
    baseUrl: draft.baseUrl.trim(),
    model: draft.model.trim(),
    apiKey: draft.apiKey.trim(),
    contextWindow: draft.contextWindow,
    maxOutputTokens: draft.maxOutputTokens,
    temperature: draft.temperature,
  }
  const complete = Boolean(input.name && input.baseUrl && input.model && input.contextWindow > 0 && input.maxOutputTokens > 0 && (draft.hasStoredKey || input.apiKey))

  const test = async () => {
    if (!complete) return
    setConnection({ kind: 'testing' })
    const result = await testModelConnection(input)
    setConnection({ kind: result.success ? 'success' : 'error', message: result.message, latencyMs: result.latencyMs })
  }

  const save = async () => {
    if (!complete || saving) return
    setSaving(true)
    const catalog = await saveModelProfile(input)
    setSaving(false)
    if (!catalog) {
      setConnection({ kind: 'error', message: '模型配置保存失败。' })
      return
    }
    const saved = catalog.profiles.find((profile) => profile.id === catalog.activeId)
    if (saved) setDraft(fromProfile(saved))
    setConnection({ kind: 'success', message: '配置已安全保存，并设为当前模型。' })
  }

  return (
    <div className="model-settings">
      <div className="model-profile-toolbar">
        <div className="model-profile-picker" ref={profilePickerRef}>
          <button type="button" className="model-profile-trigger" aria-expanded={profileMenuOpen} onClick={() => setProfileMenuOpen((value) => !value)}>
            <span className={`model-profile-trigger-icon ${draft.providerSpec}`}>
              {draft.providerSpec === 'anthropic' ? <Sparkles size={16} /> : <Bot size={16} />}
            </span>
            <span className="model-profile-trigger-copy">
              <strong>{selectedProfile?.name || draft.name || '新模型配置'}</strong>
              <small>{selectedProfile ? `${selectedProfile.model} · ${selectedProfile.providerSpec === 'anthropic' ? 'Anthropic' : 'OpenAI'}` : '尚未保存'}</small>
            </span>
            <ChevronDown size={14} />
          </button>
          {profileMenuOpen && (
            <div className="model-profile-menu">
              <label className="model-profile-search">
                <Search size={13} />
                <input autoFocus value={profileQuery} onChange={(event) => setProfileQuery(event.target.value)} onKeyDown={(event) => { if (event.key === 'Escape') setProfileMenuOpen(false) }} placeholder="搜索名称、模型 ID…" />
              </label>
              <div className="model-profile-menu-summary"><span>{filteredProfiles.length} 个模型配置</span>{modelCatalog.activeId && <small>勾选项为当前使用</small>}</div>
              <div className="model-profile-menu-list-shell">
                <div
                  ref={profileListRef}
                  className={`model-profile-menu-list${profileListScrollable ? ' scrollable' : ''}`}
                  onScroll={(event) => syncProfileScrollbar(event.currentTarget)}
                >
                  {filteredProfiles.map((profile) => {
                    const Icon = profile.providerSpec === 'anthropic' ? Sparkles : Bot
                    const isActive = profile.id === modelCatalog.activeId
                    return (
                      <button
                        key={profile.id}
                        type="button"
                        className={draft.id === profile.id ? 'selected' : ''}
                        onClick={() => { setDraft(fromProfile(profile)); setConnection({ kind: 'idle' }); setProfileMenuOpen(false); setProfileQuery('') }}
                      >
                        <span className={`model-profile-option-icon ${profile.providerSpec}`}><Icon size={15} /></span>
                        <span><strong>{profile.name}</strong><small>{profile.model} · {profile.apiMode === 'chat_completions' ? 'Chat' : profile.apiMode === 'responses' ? 'Responses' : 'Messages'}</small></span>
                        <span className="model-profile-option-status">{isActive && <Check size={13} />}</span>
                      </button>
                    )
                  })}
                  {filteredProfiles.length === 0 && <div className="model-profile-no-results">没有匹配的模型配置</div>}
                </div>
                {profileListScrollable && <span className="model-scroll-track" aria-hidden="true"><span style={{ height: profileScrollbar.height, top: profileScrollbar.top }} /></span>}
              </div>
            </div>
          )}
        </div>
        <button type="button" className="model-add-button" onClick={() => { setDraft(newDraft()); setConnection({ kind: 'idle' }); setProfileMenuOpen(false); setProfileQuery('') }}><Plus size={14} />新增模型</button>
      </div>

      <div className="model-editor">
        <div className="model-editor-section">
          <div className="model-editor-label"><strong>接口规范</strong><small>请求会严格按所选提供方规范发送</small></div>
          <div className="provider-segments">
            <button type="button" className={draft.providerSpec === 'openai' ? 'selected openai' : ''} onClick={() => switchProvider('openai')}><Bot size={15} />OpenAI</button>
            <button type="button" className={draft.providerSpec === 'anthropic' ? 'selected anthropic' : ''} onClick={() => switchProvider('anthropic')}><Sparkles size={15} />Anthropic</button>
          </div>
        </div>

        <div className="model-editor-section">
          <div className="model-editor-label"><strong>API 模式</strong><small>{draft.providerSpec === 'openai' ? '选择模型端点使用的请求格式' : 'Anthropic 使用 Messages API'}</small></div>
          <div className="provider-segments">
            {draft.providerSpec === 'openai' ? (
              <>
                <button type="button" className={draft.apiMode === 'chat_completions' ? 'selected' : ''} onClick={() => update('apiMode', 'chat_completions')}>Chat Completions</button>
                <button type="button" className={draft.apiMode === 'responses' ? 'selected' : ''} onClick={() => update('apiMode', 'responses')}>Responses API</button>
              </>
            ) : <button type="button" className="selected">Messages API</button>}
          </div>
        </div>

        <div className="model-form-grid">
          <ModelField label="配置名称" value={draft.name} onChange={(value) => update('name', value)} placeholder="例如 GPT 主力模型" />
          <ModelField label="模型 ID" value={draft.model} onChange={(value) => update('model', value)} placeholder={draft.providerSpec === 'openai' ? 'gpt-4.1' : 'claude-sonnet-4-5'} mono />
          <ModelField className="wide" label="Base URL" value={draft.baseUrl} onChange={(value) => update('baseUrl', value)} placeholder={defaults[draft.providerSpec].baseUrl} mono />
          <ModelField className="wide" label="API Key" value={draft.apiKey} onChange={(value) => update('apiKey', value)} placeholder={draft.hasStoredKey ? '凭据已配置；留空则保持不变' : '输入提供方 API Key'} type="password" mono />
          <ModelField label="上下文窗口" value={String(draft.contextWindow)} onChange={(value) => update('contextWindow', Number(value.replace(/\D/g, '')))} inputMode="numeric" />
          <ModelField label="最大输出 Tokens" value={String(draft.maxOutputTokens)} onChange={(value) => update('maxOutputTokens', Number(value.replace(/\D/g, '')))} inputMode="numeric" />
          <ModelField label="Temperature" value={String(draft.temperature)} onChange={(value) => update('temperature', Number(value))} type="number" step="0.1" min="0" max="2" />
        </div>

        <div className="model-security-note"><KeyRound size={15} /><span>API Key 仅写入 macOS 钥匙串；配置文件和本地数据库只保存凭据引用。</span></div>

        <div className="model-editor-actions">
          <ConnectionStatus state={connection} />
          <div>
            <button type="button" className="secondary-action" disabled={!complete || connection.kind === 'testing'} onClick={() => void test()}>
              {connection.kind === 'testing' ? <LoaderCircle className="spin" size={14} /> : <FlaskConical size={14} />}测试连接
            </button>
            <button type="button" className="primary-save" disabled={!complete || saving} onClick={() => void save()}>
              {saving ? <LoaderCircle className="spin" size={14} /> : <Save size={14} />}保存并使用
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function ModelField({ label, value, onChange, className = '', mono, ...inputProps }: {
  label: string
  value: string
  onChange: (value: string) => void
  className?: string
  mono?: boolean
  placeholder?: string
  type?: string
  inputMode?: 'numeric'
  step?: string
  min?: string
  max?: string
}) {
  return <label className={`model-form-field ${className}`}><span>{label}</span><input {...inputProps} className={mono ? 'mono' : ''} value={value} onChange={(event) => onChange(event.target.value)} /></label>
}

function ConnectionStatus({ state }: { state: ConnectionState }) {
  if (state.kind === 'idle') return <span className="connection-status idle"><ShieldCheck size={14} />保存前可验证端点和凭据</span>
  if (state.kind === 'testing') return <span className="connection-status"><LoaderCircle className="spin" size={14} />正在发送真实模型请求…</span>
  const Icon = state.kind === 'success' ? CircleCheck : CircleAlert
  return <span className={`connection-status ${state.kind}`}><Icon size={14} /><span>{state.message || (state.kind === 'success' ? '连接成功' : '连接失败')}{state.latencyMs ? ` · ${state.latencyMs} ms` : ''}</span></span>
}
