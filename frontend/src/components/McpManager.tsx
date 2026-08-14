import { useEffect, useMemo, useState } from 'react'
import { Cable, Check, CircleAlert, Link, LoaderCircle, Plus, Save, Server, Terminal, Trash2, Unplug } from 'lucide-react'
import { useApp } from '../app/use-app'
import type { MCPServer, MCPServerInput, MCPTransport } from '../lib/backend'

type Draft = {
  id?: string
  name: string
  transport: MCPTransport
  url: string
  command: string
  args: string
  env: string
  enabled: boolean
}

const emptyDraft = (): Draft => ({ id: undefined, name: '', transport: 'streamable_http', url: '', command: '', args: '', env: '', enabled: true })

function draftFromServer(server: MCPServer): Draft {
  return {
    id: server.id,
    name: server.name,
    transport: server.transport,
    url: server.url ?? '',
    command: server.command ?? '',
    args: (server.args ?? []).join('\n'),
    env: (server.env ?? []).join('\n'),
    enabled: server.enabled,
  }
}

function lines(value: string) {
  return value.split('\n').map((item) => item.trim()).filter(Boolean)
}

export function McpManager({ compact = false }: { compact?: boolean }) {
  const { mcpServers, saveMCPServer, deleteMCPServer, connectMCPServer, disconnectMCPServer } = useApp()
  const [selectedID, setSelectedID] = useState<string>()
  const [draft, setDraft] = useState<Draft>(emptyDraft)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')

  const selected = useMemo(() => mcpServers.find((server) => server.id === selectedID), [mcpServers, selectedID])

  useEffect(() => {
    if (selectedID && selected) setDraft(draftFromServer(selected))
  }, [selected, selectedID])

  const select = (server: MCPServer) => {
    setSelectedID(server.id)
    setDraft(draftFromServer(server))
    setNotice('')
  }

  const add = () => {
    setSelectedID(undefined)
    setDraft(emptyDraft())
    setNotice('')
  }

  const update = <K extends keyof Draft>(key: K, value: Draft[K]) => setDraft((current) => ({ ...current, [key]: value }))

  const toInput = (): MCPServerInput => ({
    id: draft.id,
    name: draft.name,
    transport: draft.transport,
    url: draft.transport === 'streamable_http' ? draft.url : '',
    command: draft.transport === 'stdio' ? draft.command : '',
    args: draft.transport === 'stdio' ? lines(draft.args) : [],
    env: draft.transport === 'stdio' ? lines(draft.env) : [],
    enabled: draft.enabled,
  })

  const save = async () => {
    setBusy(true)
    setNotice('')
    const input = toInput()
    if (!input.id) {
      input.id = globalThis.crypto?.randomUUID?.() ?? `mcp-${Date.now()}`
      update('id', input.id)
    }
    const ok = await saveMCPServer(input)
    setBusy(false)
    if (ok) setNotice('已保存 MCP 服务')
  }

  const remove = async () => {
    if (!draft.id || !window.confirm(`删除 MCP 服务「${draft.name || '未命名'}」？`)) return
    setBusy(true)
    const ok = await deleteMCPServer(draft.id)
    setBusy(false)
    if (ok) { add(); setNotice('已删除 MCP 服务') }
  }

  const toggleConnection = async () => {
    if (!draft.id) return
    setBusy(true)
    const ok = draft.id && selected?.status === 'connected'
      ? await disconnectMCPServer(draft.id)
      : await connectMCPServer(draft.id)
    setBusy(false)
    if (ok) setNotice(selected?.status === 'connected' ? '已断开 MCP 服务' : '已连接并发现工具')
  }

  return (
    <div className={`mcp-manager ${compact ? 'compact' : ''}`}>
      <div className="mcp-layout">
        <aside className="mcp-server-list">
          <div className="mcp-list-heading"><span>已配置服务</span><button type="button" className="icon-btn" aria-label="新增 MCP 服务" onClick={add}><Plus size={15} /></button></div>
          {mcpServers.length === 0 && <p className="mcp-empty-list">还没有 MCP 服务</p>}
          {mcpServers.map((server) => (
            <button key={server.id} type="button" className={`mcp-server-row ${server.id === draft.id ? 'selected' : ''}`} onClick={() => select(server)}>
              <span className="mcp-server-icon"><Server size={15} /></span>
              <span><strong>{server.name}</strong><small>{server.transport === 'stdio' ? 'stdio' : 'Streamable HTTP'}</small></span>
              <span className={`mcp-status-dot ${server.status}`} />
            </button>
          ))}
        </aside>

        <section className="mcp-editor">
          <div className="mcp-editor-heading">
            <div><span className="mcp-heading-icon"><Cable size={17} /></span><div><h3>{draft.id ? '编辑 MCP 服务' : '新增 MCP 服务'}</h3><p>连接后，服务提供的工具会加入 Agent 工具列表。</p></div></div>
            {draft.id && <button type="button" className="danger-action" disabled={busy} onClick={() => void remove()}><Trash2 size={14} />删除</button>}
          </div>

          <div className="mcp-form-grid">
            <label className="mcp-field"><span>名称</span><input value={draft.name} onChange={(event) => update('name', event.target.value)} placeholder="例如 本地文件工具" /></label>
            <label className="mcp-enabled"><input type="checkbox" checked={draft.enabled} onChange={(event) => update('enabled', event.target.checked)} />启用此服务</label>
          </div>

          <div className="mcp-transport-tabs" role="tablist" aria-label="MCP 传输方式">
            <button type="button" role="tab" aria-selected={draft.transport === 'streamable_http'} className={draft.transport === 'streamable_http' ? 'selected' : ''} onClick={() => update('transport', 'streamable_http')}><Link size={14} />Streamable HTTP</button>
            <button type="button" role="tab" aria-selected={draft.transport === 'stdio'} className={draft.transport === 'stdio' ? 'selected' : ''} onClick={() => update('transport', 'stdio')}><Terminal size={14} />stdio</button>
          </div>

          {draft.transport === 'streamable_http' ? (
            <label className="mcp-field"><span>服务地址</span><input className="mono" value={draft.url} onChange={(event) => update('url', event.target.value)} placeholder="https://example.com/mcp" /></label>
          ) : (
            <div className="mcp-form-grid">
              <label className="mcp-field"><span>启动命令</span><input className="mono" value={draft.command} onChange={(event) => update('command', event.target.value)} placeholder="npx" /></label>
              <label className="mcp-field"><span>参数（每行一个）</span><textarea className="mono" rows={3} value={draft.args} onChange={(event) => update('args', event.target.value)} placeholder="-y\n@modelcontextprotocol/server-filesystem\n/Users/me/project" /></label>
              <label className="mcp-field wide"><span>环境变量引用（每行一个名称）</span><textarea className="mono" rows={2} value={draft.env} onChange={(event) => update('env', event.target.value)} placeholder="OPENAI_API_KEY" /></label>
            </div>
          )}

          {selected?.tools && selected.tools.length > 0 && <div className="mcp-tools"><strong>已发现工具</strong><div>{selected.tools.map((tool) => <span key={tool.name} title={tool.description}>{tool.name}</span>)}</div></div>}
          {selected?.status === 'error' && <div className="mcp-error"><CircleAlert size={15} />{selected.error || '连接失败，请检查配置。'}</div>}
          {notice && <div className="mcp-success"><Check size={15} />{notice}</div>}

          <div className="mcp-actions">
            {draft.id && <button type="button" className="secondary-action" disabled={busy} onClick={() => void toggleConnection()}>{busy ? <LoaderCircle className="spin" size={14} /> : selected?.status === 'connected' ? <Unplug size={14} /> : <Cable size={14} />}{selected?.status === 'connected' ? '断开连接' : '连接并发现工具'}</button>}
            <button type="button" className="primary-save" disabled={busy} onClick={() => void save()}>{busy ? <LoaderCircle className="spin" size={14} /> : <Save size={14} />}保存配置</button>
          </div>
        </section>
      </div>
    </div>
  )
}
