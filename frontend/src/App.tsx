import { AppProvider } from './app/context'
import { useApp } from './app/use-app'
import { useAppController } from './app/use-app-controller'
import { AppShell } from './components/AppShell'
import { ErrorPage } from './pages/ErrorPage'
import { HomePage } from './pages/HomePage'
import { LoadingPage } from './pages/LoadingPage'
import { PluginsPage } from './pages/PluginsPage'
import { PullRequestsPage } from './pages/PullRequestsPage'
import { ScheduledPage } from './pages/ScheduledPage'
import { SettingsPage } from './pages/SettingsPage'
import { SitesPage } from './pages/SitesPage'

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
      {page === 'pull-requests' && <PullRequestsPage />}
      {page === 'sites' && <SitesPage />}
      {page === 'scheduled' && <ScheduledPage />}
      {page === 'plugins' && <PluginsPage />}
      {page === 'settings' && <SettingsPage />}
    </AppShell>
  )
}
