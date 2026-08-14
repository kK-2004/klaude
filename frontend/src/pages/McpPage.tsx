import { Cable } from 'lucide-react'
import { McpManager } from '../components/McpManager'

export function McpPage() {
  return (
    <div className="page-plain mcp-page">
      <header className="page-heading">
        <div><span className="page-heading-icon"><Cable size={19} /></span><div><h1>MCP</h1><p>配置 Streamable HTTP 或 stdio 服务，并管理可供 Agent 使用的工具。</p></div></div>
      </header>
      <McpManager />
    </div>
  )
}
