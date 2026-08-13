import { useEffect, useRef, useState } from 'react'
import {
  AtSign, Bell, Check, ChevronDown, Clock, Folder, FolderOpen, GitPullRequest,
  LayoutGrid, ListChecks, MessageSquare, Moon, PanelLeft, Plus, Search, Settings,
  Sun, User,
} from 'lucide-react'
import { useApp } from '../app/use-app'
import type { AppPage } from '../app/types'
import { useThemeStore } from '../stores/theme'

const nav: { id: AppPage; label: string; icon: typeof Plus }[] = [
  { id: 'home', label: '新对话', icon: Plus },
  { id: 'pull-requests', label: '拉取请求', icon: GitPullRequest },
  { id: 'sites', label: '站点', icon: LayoutGrid },
  { id: 'scheduled', label: '已安排', icon: Clock },
  { id: 'plugins', label: '插件', icon: AtSign },
]

export function Sidebar() {
  const {
    page, setPage, startNewChat, project, projects, sessions, session,
    selectProject, selectSession, renameCurrentSession, openProject,
    diagnostic, setupDone, setupTotal, model, messages, files, capabilities, setSidebarOpen,
  } = useApp()
  const theme = useThemeStore((state) => state.theme)
  const setTheme = useThemeStore((state) => state.setTheme)
  const [menuOpen, setMenuOpen] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [setupOpen, setSetupOpen] = useState(false)
  const searchRef = useRef<HTMLInputElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (searchOpen) searchRef.current?.focus()
  }, [searchOpen])

  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) setMenuOpen(false)
    }
    window.addEventListener('mousedown', onClick)
    return () => window.removeEventListener('mousedown', onClick)
  }, [])

  const q = query.trim().toLowerCase()
  const visibleProjects = q ? projects.filter((item) => item.name.toLowerCase().includes(q) || item.rootPath.toLowerCase().includes(q)) : projects
  const visibleSessions = q ? sessions.filter((item) => item.title.toLowerCase().includes(q)) : sessions

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <div className="brand-wrap" ref={menuRef}>
          <button
            type="button"
            className="brand-button"
            aria-expanded={menuOpen}
            aria-haspopup="menu"
            onClick={() => setMenuOpen((open) => !open)}
          >
            Klaude
            <ChevronDown size={14} strokeWidth={2} />
          </button>
          {menuOpen && (
            <div className="brand-menu">
              <button type="button" onClick={() => { void openProject(); setMenuOpen(false) }}>
                <FolderOpen size={14} /> 打开项目
              </button>
              <button type="button" onClick={() => { setPage('settings'); setMenuOpen(false) }}>
                <Settings size={14} /> 设置
              </button>
              <button type="button" onClick={() => { setTheme(theme === 'dark' ? 'light' : 'dark'); setMenuOpen(false) }}>
                {theme === 'dark' ? <Sun size={14} /> : <Moon size={14} />}
                {theme === 'dark' ? '浅色外观' : '深色外观'}
              </button>
              <button type="button" onClick={() => { setSidebarOpen(false); setMenuOpen(false) }}>
                <PanelLeft size={14} /> 收起侧栏
              </button>
            </div>
          )}
        </div>
        <div className="header-actions">
          <button
            type="button"
            className="icon-btn"
            aria-label="搜索"
            aria-expanded={searchOpen}
            onClick={() => setSearchOpen((open) => !open)}
          >
            <Search size={16} strokeWidth={1.75} />
          </button>
          <button type="button" className={`icon-btn ${diagnostic ? 'has-dot' : ''}`} aria-label="通知" title={diagnostic || '暂无通知'}>
            <Bell size={16} strokeWidth={1.75} />
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
              <button type="button" className="project-row" onClick={() => void selectProject(item)} title={item.rootPath}>
                <Folder size={15} strokeWidth={1.75} />
                <span>{item.name}</span>
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
          {visibleSessions.slice(0, 6).map((item) => (
            <button key={item.id} type="button" className="recent-row" onClick={() => void selectSession(item)}>
              <MessageSquare size={13} strokeWidth={1.75} />
              {item.title}
            </button>
          ))}
        </div>
      </section>

      <div className="sidebar-footer">
        {setupDone < setupTotal && (
          <button
            type="button"
            className="getting-started"
            aria-expanded={setupOpen}
            onClick={() => setSetupOpen((open) => !open)}
          >
            <ListChecks size={15} strokeWidth={1.75} />
            <span className="setup-count">{setupDone}/{setupTotal}</span>
            开始使用
            <ChevronDown className="disclosure-icon" size={14} strokeWidth={1.75} aria-hidden />
          </button>
        )}
        {setupOpen && (
          <ul className="setup-list">
            <li className={project ? 'done' : ''}><Check size={12} strokeWidth={2} /> 打开项目</li>
            <li className={model !== 'not configured' ? 'done' : ''}><Check size={12} strokeWidth={2} /> 配置模型</li>
            <li className={sessions.length > 0 ? 'done' : ''}><Check size={12} strokeWidth={2} /> 新建对话</li>
            <li className={messages.length > 0 ? 'done' : ''}><Check size={12} strokeWidth={2} /> 发送消息</li>
            <li className={files.length > 0 || capabilities.length > 0 ? 'done' : ''}><Check size={12} strokeWidth={2} /> 检查工作区</li>
          </ul>
        )}
        <button type="button" className="user-row" onClick={() => setPage('settings')}>
          <span className="avatar"><User size={14} strokeWidth={1.75} /></span>
          <span className="user-name">Klaude</span>
          <Settings size={15} strokeWidth={1.75} />
        </button>
      </div>
    </aside>
  )
}
