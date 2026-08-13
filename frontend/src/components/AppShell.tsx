import type { ReactNode } from 'react'
import { PanelLeft, X } from 'lucide-react'
import { useApp } from '../app/use-app'
import { Sidebar } from './Sidebar'

export function AppShell({ children }: { children: ReactNode }) {
  const { diagnostic, dismissDiagnostic, page, sidebarOpen, setSidebarOpen } = useApp()
  const settingsMode = page === 'settings'

  return (
    <div className={`app-shell ${sidebarOpen ? '' : 'sidebar-collapsed'} ${settingsMode ? 'settings-mode' : ''}`}>
      <div className="window-drag-region" aria-hidden="true" />
      {!settingsMode && sidebarOpen && <div className="sidebar-backdrop" onClick={() => setSidebarOpen(false)} />}
      {!settingsMode && sidebarOpen && <Sidebar />}
      <div className="main">
        {diagnostic && (
          <div className="notice-bar">
            <span>{diagnostic}</span>
            <button type="button" className="icon-btn" aria-label="关闭" onClick={dismissDiagnostic}><X size={14} /></button>
          </div>
        )}
        {!settingsMode && !sidebarOpen && (
          <button type="button" className="sidebar-reopen" aria-label="打开侧栏" onClick={() => setSidebarOpen(true)}>
            <PanelLeft size={16} />
          </button>
        )}
        {children}
      </div>
    </div>
  )
}
