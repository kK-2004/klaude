import { useEffect, useRef, useState } from 'react'
import {
  Cable, Folder, FolderOpen, FolderSearch, MessageSquare, PanelLeft, PencilLine, Pin, PinOff,
  Plus, Search, Settings, Trash2, User,
} from 'lucide-react'
import { useApp } from '../app/use-app'
import type { AppPage } from '../app/types'
import type { Project } from '../types/backend'
import { ContextMenu } from './ContextMenu'
import type { ContextMenuItem } from './ContextMenu'

const nav: { id: AppPage; label: string; icon: typeof Plus }[] = [
  { id: 'home', label: '新对话', icon: Plus },
  { id: 'mcp', label: 'MCP', icon: Cable },
]

export function Sidebar() {
  const {
	page, setPage, startNewChat, project, projects, sessions, recentSessions, session, platform,
    selectProject, selectSession, renameCurrentSession, openProject,
    renameProject, toggleProjectPinned, deleteProject, revealProject,
    setSidebarOpen,
  } = useApp()
  const [searchOpen, setSearchOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [projectMenu, setProjectMenu] = useState<{ project: Project; x: number; y: number }>()
  const searchRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (searchOpen) searchRef.current?.focus()
  }, [searchOpen])

  const revealLabel = platform === 'windows' ? '在资源管理器中打开' : platform === 'darwin' ? '在 Finder 中打开' : '在文件管理器中打开'
  const projectMenuItems = (item: Project): ContextMenuItem[] => [
    { id: 'pin', label: item.pinned ? '取消置顶' : '置顶', icon: item.pinned ? PinOff : Pin, onSelect: () => void toggleProjectPinned(item) },
    { id: 'rename', label: '重命名项目', icon: PencilLine, onSelect: () => void renameProject(item) },
    { id: 'reveal', label: revealLabel, icon: platform === 'windows' ? FolderSearch : FolderOpen, onSelect: () => void revealProject(item) },
    { id: 'delete', label: '删除', icon: Trash2, danger: true, onSelect: () => void deleteProject(item) },
  ]

  const q = query.trim().toLowerCase()
  const visibleProjects = q ? projects.filter((item) => item.name.toLowerCase().includes(q) || item.rootPath.toLowerCase().includes(q)) : projects
	const visibleSessions = q ? sessions.filter((item) => item.title.toLowerCase().includes(q)) : sessions
	const visibleRecentSessions = q ? recentSessions.filter((item) => item.title.toLowerCase().includes(q)) : recentSessions

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <div className="brand-label">Klaude</div>
        <div className="header-actions">
          <button type="button" className="icon-btn sidebar-collapse" aria-label="收起侧栏" onClick={() => setSidebarOpen(false)}>
            <PanelLeft size={16} strokeWidth={1.75} />
          </button>
          <button
            type="button"
            className="icon-btn"
            aria-label="搜索"
            aria-expanded={searchOpen}
            onClick={() => setSearchOpen((open) => !open)}
          >
            <Search size={16} strokeWidth={1.75} />
          </button>
        </div>
      </div>

      {searchOpen && (
        <input
          ref={searchRef}
          className="sidebar-search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="搜索项目和对话"
        />
      )}

      <nav className="sidebar-nav">
        {nav.map((item) => {
          const Icon = item.icon
          const active = item.id !== 'home' && page === item.id
          return (
            <button
              key={item.id}
              type="button"
              className={`nav-item ${active ? 'active' : ''}`}
              onClick={() => item.id === 'home' ? void startNewChat() : setPage(item.id)}
            >
              <Icon size={16} strokeWidth={1.75} />
              {item.label}
            </button>
          )
        })}
      </nav>

      <section className="sidebar-section">
        <div className="section-label">
          项目
          <button type="button" className="icon-btn" aria-label="打开项目" onClick={() => void openProject()}>
            <FolderOpen size={14} strokeWidth={1.75} />
          </button>
        </div>
        <div className="project-list">
          {visibleProjects.map((item) => (
            <div key={item.id} className={`project-block ${item.id === project?.id ? 'active' : ''}`}>
              <button
                type="button"
                className="project-row"
                onClick={() => void selectProject(item)}
                onContextMenu={(event) => { event.preventDefault(); setProjectMenu({ project: item, x: event.clientX, y: event.clientY }) }}
                title={item.rootPath}
              >
                <Folder size={15} strokeWidth={1.75} />
                <span>{item.name}</span>
                {item.pinned && <Pin className="project-pin" size={12} strokeWidth={1.75} aria-label="已置顶" />}
              </button>
              {item.id === project?.id && visibleSessions.slice(0, 4).map((entry) => (
                <button
                  key={entry.id}
                  type="button"
                  className={`session-row ${entry.id === session?.id ? 'current' : ''}`}
                  onClick={() => void selectSession(entry)}
                  onContextMenu={(event) => { event.preventDefault(); void renameCurrentSession() }}
                >
                  <MessageSquare size={12} strokeWidth={1.75} />
                  {entry.title}
                </button>
              ))}
            </div>
          ))}
        </div>
      </section>

      <section className="sidebar-section grow">
        <div className="section-label">最近</div>
        <div className="recent-list">
		  {visibleRecentSessions.slice(0, 10).map((item) => (
            <button key={item.id} type="button" className="recent-row" onClick={() => void selectSession(item)}>
              <MessageSquare size={13} strokeWidth={1.75} />
              {item.title}
            </button>
          ))}
        </div>
      </section>

      <div className="sidebar-footer">
        <div className="user-row">
          <span className="avatar"><User size={14} strokeWidth={1.75} /></span>
          <span className="user-name">Klaude</span>
          <button type="button" className="icon-btn user-settings" aria-label="设置" onClick={() => setPage('settings')}>
            <Settings size={15} strokeWidth={1.75} />
          </button>
        </div>
      </div>

      {projectMenu && (
        <ContextMenu
          anchor={projectMenu}
          label={`项目 ${projectMenu.project.name}`}
          items={projectMenuItems(projectMenu.project)}
          onClose={() => setProjectMenu(undefined)}
        />
      )}
    </aside>
  )
}
