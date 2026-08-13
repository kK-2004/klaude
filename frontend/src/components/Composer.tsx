import { ArrowUp, ChevronDown, Clock, Folder, Mic, Square } from 'lucide-react'
import { useApp } from '../app/use-app'
import { GitBranchSelector } from './GitBranchSelector'

export function Composer() {
  const {
    appState, composer, setComposer, running, submitMessage, stopAgent,
    messages, model, project, setPage,
  } = useApp()
  const showContext = messages.length === 0

  return (
    <div className="composer-dock">
      {showContext && (
        <div className="context-bar">
          <span className="context-pill" title={project?.rootPath}><Folder size={13} strokeWidth={1.75} />{project?.name ?? '未打开项目'}</span>
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
            <button type="button" className="text-chip active" title="写入和命令需要批准">
              <Clock size={14} strokeWidth={1.75} />
              帮我批准
            </button>
          </div>
          <div className="toolbar-right">
            <button type="button" className="model-chip" title="在设置中更改模型" onClick={() => setPage('settings')}>
              {model === 'not configured' ? '选择模型' : model}
              <ChevronDown size={13} strokeWidth={1.75} />
            </button>
            <button type="button" className="icon-btn" disabled title="语音输入尚未接入">
              <Mic size={16} strokeWidth={1.75} />
            </button>
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
