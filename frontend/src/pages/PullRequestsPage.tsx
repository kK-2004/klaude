import Editor from '@monaco-editor/react'
import { File, GitPullRequest, Undo2, X } from 'lucide-react'
import { useApp } from '../app/use-app'
import { useThemeStore } from '../stores/theme'

export function PullRequestsPage() {
  const { changes, groupedChanges, selectedChange, setSelectedChange, undoSelectedTurn } = useApp()
  const theme = useThemeStore((state) => state.theme)

  return (
    <div className="page-plain">
      <header className="page-heading">
        <h1>拉取请求</h1>
        <p>Agent 对工作区的文件变更会显示在这里，可按 Turn 审查和撤销。</p>
      </header>
      {changes.length === 0 ? (
        <p className="page-muted"><GitPullRequest size={16} strokeWidth={1.75} /> 还没有文件变更。</p>
      ) : (
        <div className="review-layout">
          <div className="change-list">
            {Object.entries(groupedChanges).map(([turnId, turnChanges]) => (
              <section key={turnId} className="change-group">
                <div className="change-group-heading">Turn {turnId.slice(0, 8)} · {turnChanges.length} 个文件</div>
                {turnChanges.map((change) => (
                  <button
                    key={change.id}
                    type="button"
                    className={`change-row ${selectedChange?.id === change.id ? 'selected' : ''}`}
                    aria-pressed={selectedChange?.id === change.id}
                    onClick={() => setSelectedChange(change)}
                  >
                    <strong><File size={13} strokeWidth={1.75} /> {change.path}</strong>
                    <span><em>+{change.addedLines}</em> <b>−{change.deletedLines}</b> · {change.status}</span>
                  </button>
                ))}
              </section>
            ))}
            <button type="button" className="undo-button" onClick={() => void undoSelectedTurn()} disabled={!selectedChange}>
              <Undo2 size={14} /> 撤销所选 Turn
            </button>
          </div>
          {selectedChange && (
            <div className="diff-card">
              <div className="diff-heading">
                <strong>{selectedChange.path}</strong>
                <button type="button" className="icon-btn" onClick={() => setSelectedChange(undefined)} aria-label="关闭 diff">
                  <X size={14} />
                </button>
              </div>
              <div className="diff-note">已记录基线；继续修改前请核对当前文件内容。</div>
              <Editor
                height="420px"
                language="diff"
                theme={theme === 'dark' ? 'vs-dark' : 'light'}
                value={selectedChange.diff && selectedChange.diff.length < 200000 && !selectedChange.diff.includes('\u0000')
                  ? selectedChange.diff
                  : '仅元数据 diff：二进制或体积过大。请在编辑器中打开文件查看。'}
                options={{ readOnly: true, minimap: { enabled: false }, wordWrap: 'on', lineNumbers: 'off', scrollBeyondLastLine: false }}
              />
            </div>
          )}
        </div>
      )}
    </div>
  )
}
