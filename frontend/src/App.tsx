import { AppProvider } from './app/context'
import { useApp } from './app/use-app'
import { useAppController } from './app/use-app-controller'
import { AppShell } from './components/AppShell'
import { ErrorPage } from './pages/ErrorPage'
import { HomePage } from './pages/HomePage'
import { LoadingPage } from './pages/LoadingPage'
import { McpPage } from './pages/McpPage'
import { ScheduledPage } from './pages/ScheduledPage'
import { SettingsPage } from './pages/SettingsPage'

export default function App() {
  const controller = useAppController()
  return (
    <AppProvider value={controller}>
      <AppView />
    </AppProvider>
  )
}

function AppView() {
  const { appState, page } = useApp()
  if (appState === 'loading') return <LoadingPage />
  if (appState === 'fatal') return <ErrorPage />
  return (
    <AppShell>
      {page === 'home' && <HomePage />}
      {page === 'mcp' && <McpPage />}
      {page === 'scheduled' && <ScheduledPage />}
      {page === 'settings' && <SettingsPage />}
    </AppShell>
  )
}
