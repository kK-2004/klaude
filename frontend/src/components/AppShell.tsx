import type { ReactNode } from 'react'
import { PanelLeft, X } from 'lucide-react'
import { useApp } from '../app/use-app'
import { Sidebar } from './Sidebar'

export function AppShell({ children }: { children: ReactNode }) {
  const { diagnostic, dismissDiagnostic, sidebarOpen, setSidebarOpen } = useApp()

  return (
    <div className={`app-shell ${sidebarOpen ? '' : 'sidebar-collapsed'}`}>
      {sidebarOpen && <div className="sidebar-backdrop" onClick={() => setSidebarOpen(false)} />}
      {sidebarOpen && <Sidebar />}
      <div className="main">
        {diagnostic && (
          <div className="notice-bar">
            <span>{diagnostic}</span>
            <button type="button" className="icon-btn" aria-label="关闭" onClick={dismissDiagnostic}><X size={14} /></button>
          </div>
        )}
        {!sidebarOpen && (
          <button type="button" className="sidebar-reopen" aria-label="打开侧栏" onClick={() => setSidebarOpen(true)}>
            <PanelLeft size={16} />
          </button>
        )}
        {children}
      </div>
    </div>
  )
}
