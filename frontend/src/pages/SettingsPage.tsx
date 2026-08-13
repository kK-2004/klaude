import { Check, GitBranch, Layers, Moon, ShieldCheck, Sun } from 'lucide-react'
import { useApp } from '../app/use-app'
import { useThemeStore } from '../stores/theme'

export function SettingsPage() {
  const {
    endpoint, setEndpoint, model, setModel, credentialEnv, setCredentialEnv,
    contextLimit, setContextLimit, turnLimit, setTurnLimit,
    parallelTools, setParallelTools, llmSchedule, setLLMSchedule, saveSettings,
  } = useApp()
  const theme = useThemeStore((state) => state.theme)
  const setTheme = useThemeStore((state) => state.setTheme)

  return (
    <div className="page-plain settings-page">
      <header className="page-heading">
        <h1>设置</h1>
        <p>外观、模型、工具并发和工作区限制。凭据只通过环境变量引用，不会以明文保存。</p>
      </header>

      <section className="settings-section">
        <h2>外观</h2>
        <div className="theme-picks">
          <button type="button" className={theme === 'light' ? 'selected' : ''} aria-pressed={theme === 'light'} onClick={() => setTheme('light')}>
            <Sun size={15} strokeWidth={1.75} /> 浅色
          </button>
          <button type="button" className={theme === 'dark' ? 'selected' : ''} aria-pressed={theme === 'dark'} onClick={() => setTheme('dark')}>
            <Moon size={15} strokeWidth={1.75} /> 深色
          </button>
        </div>
      </section>

      <section className="settings-section">
        <h2>模型</h2>
        <label>提供方端点
          <input value={endpoint} onChange={(event) => setEndpoint(event.target.value)} placeholder="https://api.openai.com/v1" />
        </label>
        <label>模型
          <input value={model} onChange={(event) => setModel(event.target.value)} placeholder="gpt-4.1-mini" />
        </label>
        <label>凭据环境变量
          <input value={credentialEnv} onChange={(event) => setCredentialEnv(event.target.value.replace(/[^A-Za-z0-9_]/g, ''))} placeholder="OPENAI_API_KEY" />
        </label>
      </section>

      <section className="settings-section">
        <h2>工具并发</h2>
        <p className="settings-hint">默认串行执行。开启后，无资源冲突的只读与写工具可同层并行；依赖不明时可让模型补全调度边。</p>
        <div className="theme-picks">
          <button
            type="button"
            className={parallelTools ? 'selected' : ''}
            aria-pressed={parallelTools}
            onClick={() => setParallelTools(!parallelTools)}
          >
            <Layers size={15} strokeWidth={1.75} /> 并行工具调度
          </button>
          <button
            type="button"
            className={llmSchedule ? 'selected' : ''}
            aria-pressed={llmSchedule}
            disabled={!parallelTools}
            title={parallelTools ? '依赖不明时用模型补全拓扑边' : '需先开启并行工具调度'}
            onClick={() => setLLMSchedule(!llmSchedule)}
          >
            <GitBranch size={15} strokeWidth={1.75} /> LLM 拓扑回退
          </button>
        </div>
      </section>

      <section className="settings-section">
        <h2>工作区</h2>
        <div className="settings-grid">
          <label>上下文字符
            <input inputMode="numeric" value={contextLimit} onChange={(event) => setContextLimit(event.target.value.replace(/\D/g, ''))} />
          </label>
          <label>最大 Turns
            <input inputMode="numeric" value={turnLimit} onChange={(event) => setTurnLimit(event.target.value.replace(/\D/g, ''))} />
          </label>
        </div>
        <div className="logical-boundary">
          <ShieldCheck size={16} />
          <span>执行逻辑上限制在工作区内；当前版本不是操作系统沙箱。默认允许读取，写入和命令会先询问。</span>
        </div>
      </section>

      <button type="button" className="primary-save" onClick={() => void saveSettings()}>
        <Check size={15} strokeWidth={2} /> 保存并完成
      </button>
    </div>
  )
}
