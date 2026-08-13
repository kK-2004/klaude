import {
  Check, ChevronDown, CircleAlert, Cloud, FolderGit2, FolderOpen, GitBranch,
  Loader2, LogIn, RefreshCw, Trash2, TriangleAlert, X,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { backend } from '../lib/backend'
import type { GitBranch as GitBranchInfo, GitBranchSnapshot } from '../types/backend'

type BranchMenu = { branch: GitBranchInfo; x: number; y: number }
type WorktreeDraft = { branch: GitBranchInfo; branchName: string; prefix: string }

export function GitBranchSelector({ gitRoot }: { gitRoot?: string }) {
  const [open, setOpen] = useState(false)
  const [snapshot, setSnapshot] = useState<GitBranchSnapshot>()
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [branchMenu, setBranchMenu] = useState<BranchMenu>()
  const [deleteTarget, setDeleteTarget] = useState<GitBranchInfo>()
  const [worktree, setWorktree] = useState<WorktreeDraft>()
  const rootRef = useRef<HTMLDivElement>(null)

  const refresh = useCallback(async () => {
    if (!gitRoot) {
      setSnapshot(undefined)
      return
    }
    setLoading(true)
    setError('')
    try {
      setSnapshot(await backend.gitBranches(gitRoot))
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setLoading(false)
    }
  }, [gitRoot])

  useEffect(() => { void refresh() }, [refresh])

  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false)
        setBranchMenu(undefined)
      }
    }
    const escape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      if (worktree) setWorktree(undefined)
      else if (deleteTarget) setDeleteTarget(undefined)
      else if (branchMenu) setBranchMenu(undefined)
      else setOpen(false)
    }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', escape)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', escape)
    }
  }, [branchMenu, deleteTarget, worktree])

  const localBranches = useMemo(() => snapshot?.branches.filter((branch) => !branch.remote) ?? [], [snapshot])
  const remoteBranches = useMemo(() => snapshot?.branches.filter((branch) => branch.remote) ?? [], [snapshot])
  const current = snapshot?.current || (gitRoot ? 'Git' : '非 Git')

  const checkout = async (branch: GitBranchInfo) => {
    if (!gitRoot || branch.current) return
    setBusy(`checkout:${branch.name}`)
    setError('')
    setNotice('')
    try {
      await backend.checkoutGitBranch(gitRoot, branch.name, branch.remote)
      await refresh()
      setOpen(false)
      setBranchMenu(undefined)
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setBusy('')
    }
  }

  const removeBranch = async () => {
    if (!gitRoot || !deleteTarget) return
    const branch = deleteTarget
    setBusy(`delete:${branch.name}`)
    setError('')
    setNotice('')
    try {
      await backend.deleteGitBranch(gitRoot, branch.name, branch.remote)
      setDeleteTarget(undefined)
      setBranchMenu(undefined)
      setNotice(`已删除${branch.remote ? '远端' : '本地'}分支 ${branch.name}`)
      await refresh()
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setBusy('')
    }
  }

  const openWorktree = (branch: GitBranchInfo) => {
    const baseName = branch.remote ? branch.name.split('/').slice(1).join('/') : branch.name
    const branchName = branch.current ? `${baseName}-worktree` : baseName
    setError('')
    setNotice('')
    setWorktree({ branch, branchName, prefix: snapshot?.worktreeBase ?? '' })
    setBranchMenu(undefined)
  }

  const chooseDirectory = async () => {
    if (!worktree) return
    try {
      const selected = await backend.selectDirectory(worktree.prefix)
      if (selected) setWorktree({ ...worktree, prefix: selected })
    } catch (reason) {
      setError(errorMessage(reason))
    }
  }

  const createWorktree = async () => {
    if (!gitRoot || !worktree) return
    const target = worktreePath(worktree.prefix, worktree.branchName)
    setBusy(`worktree:${worktree.branch.name}`)
    setError('')
    setNotice('')
    try {
      const created = await backend.createGitWorktree(gitRoot, worktree.branch.name, worktree.branchName.trim(), target)
      setWorktree(undefined)
      setOpen(false)
      setNotice(`Worktree 已创建：${created}`)
      await refresh()
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setBusy('')
    }
  }

  return (
    <div className="branch-selector" ref={rootRef}>
      <button
        type="button"
        className="context-pill branch-trigger"
        disabled={!gitRoot}
        aria-expanded={open}
        aria-haspopup="menu"
        title={gitRoot ? `Git 仓库：${gitRoot}` : '当前目录不是 Git 仓库'}
        onClick={() => {
          setOpen((value) => !value)
          setBranchMenu(undefined)
          if (!open) void refresh()
        }}
      >
        <GitBranch size={13} strokeWidth={1.75} />
        <span>{current}</span>
        {loading ? <Loader2 className="spin" size={12} /> : <ChevronDown className="branch-chevron" size={12} />}
      </button>

      {open && (
        <div className="branch-popover" role="menu" aria-label="Git 分支">
          <div className="branch-popover-heading">
            <span><GitBranch size={14} /> 分支</span>
            <button type="button" className="icon-btn" aria-label="刷新分支" onClick={() => void refresh()}>
              <RefreshCw className={loading ? 'spin' : ''} size={14} />
            </button>
          </div>
          {error && <div className="branch-feedback error"><CircleAlert size={14} /> <span>{error}</span></div>}
          {notice && <div className="branch-feedback success"><Check size={14} /> <span>{notice}</span></div>}
          <BranchGroup title="本地分支" branches={localBranches} busy={busy} onCheckout={checkout} onContextMenu={setBranchMenu} />
          <BranchGroup title="远端分支" branches={remoteBranches} busy={busy} remote onCheckout={checkout} onContextMenu={setBranchMenu} />
        </div>
      )}

      {branchMenu && (
        <div className="branch-context-menu" style={{ left: branchMenu.x, top: branchMenu.y }} role="menu">
          <button type="button" onClick={() => void checkout(branchMenu.branch)} disabled={branchMenu.branch.current}>
            <LogIn size={14} /> 签出
          </button>
          <button type="button" onClick={() => openWorktree(branchMenu.branch)}>
            <FolderGit2 size={14} /> 创建 Worktree 并签出
          </button>
          <button type="button" className="danger" disabled={branchMenu.branch.current} onClick={() => { setError(''); setNotice(''); setDeleteTarget(branchMenu.branch); setBranchMenu(undefined) }}>
            <Trash2 size={14} /> 删除分支
          </button>
        </div>
      )}

      {deleteTarget && (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setDeleteTarget(undefined) }}>
          <div className="modal-card branch-modal" role="dialog" aria-modal="true" aria-labelledby="delete-branch-title">
            <div className="modal-heading">
              <div>
                <span className="modal-kicker">删除分支</span>
                <h2 id="delete-branch-title">删除 {deleteTarget.name}？</h2>
              </div>
              <button type="button" className="icon-btn" aria-label="关闭" onClick={() => setDeleteTarget(undefined)}><X size={15} /></button>
            </div>
            <p>此操作无法由 Klaude 自动撤销。本地未合并分支会受到 Git 的安全保护。</p>
            {deleteTarget.remote && (
              <div className="remote-danger">
                <TriangleAlert size={18} />
                <div><strong>远端分支也会被删除！</strong><span>该操作会推送删除请求到远端仓库，其他协作者也会受到影响。</span></div>
              </div>
            )}
            {error && <div className="branch-feedback error"><CircleAlert size={14} /> <span>{error}</span></div>}
            <div className="modal-actions">
              <button type="button" onClick={() => setDeleteTarget(undefined)}><X size={14} /> 取消</button>
              <button type="button" className="danger-button" disabled={busy.startsWith('delete:')} onClick={() => void removeBranch()}>
                {busy.startsWith('delete:') ? <Loader2 className="spin" size={14} /> : <Trash2 size={14} />} 确认删除
              </button>
            </div>
          </div>
        </div>
      )}

      {worktree && (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setWorktree(undefined) }}>
          <div className="modal-card branch-modal worktree-modal" role="dialog" aria-modal="true" aria-labelledby="worktree-title">
            <div className="modal-heading">
              <div>
                <span className="modal-kicker">Git Worktree</span>
                <h2 id="worktree-title">创建 Worktree 并签出</h2>
              </div>
              <button type="button" className="icon-btn" aria-label="关闭" onClick={() => setWorktree(undefined)}><X size={15} /></button>
            </div>
            <p>基于 <strong>{worktree.branch.name}</strong> 创建独立工作目录。</p>
            <label className="branch-field">
              <span><GitBranch size={14} /> 分支名</span>
              <input value={worktree.branchName} onChange={(event) => setWorktree({ ...worktree, branchName: event.target.value })} placeholder="feature/my-branch" />
            </label>
            <label className="branch-field">
              <span><FolderOpen size={14} /> 目录前缀</span>
              <div className="path-picker">
                <input value={worktree.prefix} onChange={(event) => setWorktree({ ...worktree, prefix: event.target.value })} placeholder="选择或输入父目录" />
                <button type="button" onClick={() => void chooseDirectory()}><FolderOpen size={14} /> 选择目录</button>
              </div>
            </label>
            <div className="worktree-target"><FolderGit2 size={14} /><span>将创建在</span><code>{worktreePath(worktree.prefix, worktree.branchName) || '请填写目录和分支名'}</code></div>
            {error && <div className="branch-feedback error"><CircleAlert size={14} /> <span>{error}</span></div>}
            <div className="modal-actions">
              <button type="button" onClick={() => setWorktree(undefined)}><X size={14} /> 取消</button>
              <button
                type="button"
                className="primary"
                disabled={!worktree.prefix.trim() || !worktree.branchName.trim() || busy.startsWith('worktree:')}
                onClick={() => void createWorktree()}
              >
                {busy.startsWith('worktree:') ? <Loader2 className="spin" size={14} /> : <FolderGit2 size={14} />} 创建并签出
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function BranchGroup({
  title, branches, busy, remote = false, onCheckout, onContextMenu,
}: {
  title: string
  branches: GitBranchInfo[]
  busy: string
  remote?: boolean
  onCheckout: (branch: GitBranchInfo) => Promise<void>
  onContextMenu: (menu: BranchMenu) => void
}) {
  return (
    <section className="branch-group">
      <div className="branch-group-title">{remote ? <Cloud size={13} /> : <GitBranch size={13} />}{title}<span>{branches.length}</span></div>
      {branches.length === 0 ? <div className="branch-empty">没有{title}</div> : branches.map((branch) => (
        <button
          type="button"
          key={`${branch.remote ? 'remote' : 'local'}:${branch.name}`}
          className={`branch-row ${branch.current ? 'current' : ''}`}
          aria-current={branch.current ? 'true' : undefined}
          disabled={busy !== ''}
          onClick={() => void onCheckout(branch)}
          onContextMenu={(event) => {
            event.preventDefault()
            onContextMenu({ branch, x: Math.min(event.clientX, window.innerWidth - 220), y: Math.min(event.clientY, window.innerHeight - 132) })
          }}
        >
          {busy === `checkout:${branch.name}` ? <Loader2 className="spin" size={14} /> : branch.remote ? <Cloud size={14} /> : <GitBranch size={14} />}
          <span>{branch.name}</span>
          {branch.current && <Check size={14} />}
        </button>
      ))}
    </section>
  )
}

function worktreePath(prefix: string, branchName: string) {
  const base = prefix.trim().replace(/[\\/]+$/, '')
  const suffix = branchName.trim().replace(/[\\/]+/g, '-')
  if (!base || !suffix) return ''
  const separator = base.includes('\\') && !base.includes('/') ? '\\' : '/'
  return `${base}${separator}${suffix}`
}

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : String(reason)
}
