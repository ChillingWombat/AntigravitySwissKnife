import React, { useState, useEffect } from 'react'
import { routeFromNavigationTarget, routeToTool } from './navigation/routes'
import { useRouteNavigation } from './navigation/useRouteNavigation'
import { ArrowRightLeft, FolderArchive, HardDrive } from 'lucide-react'
import { NavRail } from './components/NavRail'
import { TopRibbon } from './components/TopRibbon'
import { QuotaDashboardPage } from './pages/QuotaDashboardPage'
import { FingerprintsPage } from './pages/FingerprintsPage'
import { SwitcherSettingsPage } from './pages/SwitcherSettingsPage'
import { SystemSettingsPage } from './pages/SystemSettingsPage'
import { CustomModelsPage } from './pages/CustomModelsPage'
import { AppEnhancementsPage } from './pages/AppEnhancementsPage'
import { ExtensionsPage } from './pages/ExtensionsPage'
import { TokenMonitorPage } from './pages/TokenMonitorPage'
import { UtilitiesPage } from './pages/UtilitiesPage'
import { BridgePage } from './pages/BridgePage'
import { ACPAgentMeshPage } from './pages/ACPAgentMeshPage'
import { AppLockScreen } from './components/AppLockScreen'
import { ErrorBoundary } from './components/ErrorBoundary'
import type { AccountState, FleetQuotaSummary, RuleConfig, SystemStatus } from './types'
import { toAccountState } from './types'
import { api, getSessionToken, runSingleFlightPoll, shouldRunBackgroundPoll } from './api'

export const App: React.FC = () => {
  const { route, navigate } = useRouteNavigation()
  const currentTool = routeToTool(route)
  const currentTab = route.tool === 'switcher' ? route.tab : 'dashboard'
  const utilitiesTab = route.tool === 'utilities' ? route.tab : 'cache'
  const [systemSettingsTab, setSystemSettingsTab] = useState<number>(0) // 0: General, 1: Path & Storage, 2: Error & Privacy, 3: About
  const [enhancementTab, setEnhancementTab] = useState<number>(0) // 0: Chat View, 1: Project Panel, 2: Auxiliary Panel
  const [extensionsTab, setExtensionsTab] = useState<number>(0) // 0: Built-in, 1: Community
  const [availableProjects, setAvailableProjects] = useState<Array<{ name: string; color: string; order: number; is_archived: boolean }>>([])
  const [bridgeTab, setBridgeTab] = useState<number>(0) // 0: Remote, 1: Gemini, 2: WhatsApp, 3: Telegram, 4: Discord
  const [tokenMonitorTab, setTokenMonitorTab] = useState<number>(0) // 0: Overview, 1: Telemetry & Logs, 2: Token Price
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [fleet, setFleet] = useState<FleetQuotaSummary | null>(null)
  const [directAccounts, setDirectAccounts] = useState<any[]>([])
  const [rules, setRules] = useState<RuleConfig | null>(null)
  const [, setLoading] = useState<boolean>(true)
  const [isLocked, setIsLocked] = useState<boolean>(false)
  const isPollingRef = React.useRef(false)

  const effectiveAccounts: AccountState[] = React.useMemo(() => {
    if (fleet?.accounts && fleet.accounts.length > 0) {
      return fleet.accounts
    }
    if (directAccounts && directAccounts.length > 0) {
      const active = status?.active_account || fleet?.active_account || ''
      return directAccounts.map(a => toAccountState(a, active, rules?.allow_non_gemini_native_models))
    }
    return []
  }, [fleet?.accounts, directAccounts, status?.active_account, fleet?.active_account, rules?.allow_non_gemini_native_models])

  const checkAuth = async () => {
    try {
      const electronAPI = (window as any).electronAPI
      const usesDaemonBroker = electronAPI?.isElectron === true &&
        typeof (window as any).swiss?.request === 'function'
      if (usesDaemonBroker) {
        setIsLocked(false)
        return
      }
      const res = await api.getAuthStatus()
      if (res.password_required) {
        // Keep the lock only when no token can be presented; a stale stored token
        // will surface as swiss:auth-required via 401 handling instead.
        if (!getSessionToken()) setIsLocked(true)
      }
    } catch (err) {
      console.error('Auth check error:', err)
    }
  }

  const loadAllData = async () => {
    try {
      const [s, f, r, p, accs] = await Promise.allSettled([
        api.getStatus(),
        api.getFleetQuota(),
        api.getRules(),
        api.getGUIProjects(),
        api.getAccounts(),
      ])

      if (s.status === 'fulfilled') setStatus(s.value)
      if (f.status === 'fulfilled') setFleet(f.value)
      if (r.status === 'fulfilled') setRules(r.value)
      if (p.status === 'fulfilled' && Array.isArray(p.value)) {
        const activeProjs = p.value.filter(proj => !proj.is_archived)
        setAvailableProjects(activeProjs)
        if (activeProjs.length > 0 && !localStorage.getItem('antigravity_last_active_project')) {
          localStorage.setItem('antigravity_last_active_project', activeProjs[0].name)
        }
      }
      if (accs.status === 'fulfilled' && Array.isArray(accs.value)) {
        setDirectAccounts(accs.value)
      }
    } catch (err) {
      console.error('Data loading error:', err)
    } finally {
      setLoading(false)
    }
  }


  const pollStatus = () => runSingleFlightPoll(isPollingRef, loadAllData)

  useEffect(() => {
    checkAuth()
    void pollStatus()
    // Listen for navigation requests from Electron system tray context menu
    let unsubNavigate: (() => void) | undefined
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.onNavigate) {
      unsubNavigate = electronAPI.onNavigate((target: unknown) => {
        const nextRoute = routeFromNavigationTarget(target)
        if (nextRoute) navigate(nextRoute)
      })
    }
    // Poll status and fleet metrics periodically without overlapping requests.
    const interval = setInterval(() => {
      if (!shouldRunBackgroundPoll()) return
      void pollStatus()
    }, 10000)
    const handleVisibilityChange = () => {
      if (shouldRunBackgroundPoll()) {
        void pollStatus()
      }
    }
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', handleVisibilityChange)
    }
    return () => {
      if (typeof unsubNavigate === 'function') {
        unsubNavigate()
      }
      clearInterval(interval)
      if (typeof document !== 'undefined') {
        document.removeEventListener('visibilitychange', handleVisibilityChange)
      }
    }
  }, [navigate])

  useEffect(() => {
    // A 401 that could not be auto-recovered means interactive unlock is needed.
    const onAuthRequired = () => setIsLocked(true)
    window.addEventListener('swiss:auth-required', onAuthRequired)
    return () => window.removeEventListener('swiss:auth-required', onAuthRequired)
  }, [])

  if (isLocked) {
    return (
      <AppLockScreen
        onUnlocked={() => {
          setIsLocked(false)
          loadAllData()
        }}
      />
    )
  }

  return (
    <div style={{ display: 'flex', width: '100vw', height: '100vh', overflow: 'hidden' }}>
      {/* 1. Left Navigation Rail (Fixed 200px) */}
      <NavRail
        currentTool={currentTool}
        navigate={navigate}
        status={status}
      />

      {/* 2. Main Right Column */}
      <div
        style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          height: '100vh',
          overflow: 'hidden',
          backgroundColor: 'var(--canvas)',
        }}
      >
        {/* Top Header / Ribbon */}
        {currentTool === 'switcher' ? (
          <TopRibbon
            currentTab={currentTab}
            navigate={navigate}
            activeAccount={status?.active_account || fleet?.active_account || null}
          />
        ) : (
          <header
            style={{
              height: '72px',
              backgroundColor: 'var(--surface)',
              borderBottom: '1px solid var(--border)',
              boxSizing: 'border-box',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '0 24px',
              flexShrink: 0,
            }}
          >
            <div id="top-bar-left" style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
              {currentTool === 'custom-models' && (
                <div
                  id="custom-models-top-bar-portal"
                  style={{ display: 'flex', alignItems: 'center', gap: '12px' }}
                />
              )}
              {currentTool === 'acp-adapter' && (
                <div
                  id="acp-adapter-top-bar-portal"
                  style={{ display: 'flex', alignItems: 'center', gap: '12px' }}
                />
              )}
              {currentTool === 'enhancements' && (
                /* UI Enhancement Segmented Tabs */
                <div
                  style={{
                    display: 'flex',
                    backgroundColor: 'var(--tonal)',
                    borderRadius: '8px',
                    padding: '3px',
                    gap: '2px',
                  }}
                >
                  {['Chat View', 'Project Panel', 'Auxiliary Panel'].map((tab, idx) => {
                    const isActive = enhancementTab === idx
                    return (
                      <button
                        key={tab}
                        onClick={() => setEnhancementTab(idx)}
                        style={{
                          borderRadius: '6px',
                          padding: '6px 16px',
                          fontSize: '12px',
                          fontWeight: isActive ? 600 : 500,
                          color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                          backgroundColor: isActive ? 'var(--surface)' : 'transparent',
                          boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                          border: 'none',
                          cursor: 'pointer',
                          whiteSpace: 'nowrap',
                          transition: 'all 0.15s ease',
                        }}
                      >
                        {tab}
                      </button>
                    )
                  })}
                </div>
              )}

              {currentTool === 'extensions' && (
                /* Extensions Segmented Tabs */
                <div
                  style={{
                    display: 'flex',
                    backgroundColor: 'var(--tonal)',
                    borderRadius: '8px',
                    padding: '3px',
                    gap: '2px',
                  }}
                >
                  {['Built-in', 'Community'].map((tab, idx) => {
                    const isActive = extensionsTab === idx
                    return (
                      <button
                        key={tab}
                        onClick={() => setExtensionsTab(idx)}
                        style={{
                          borderRadius: '6px',
                          padding: '6px 16px',
                          fontSize: '12px',
                          fontWeight: isActive ? 600 : 500,
                          color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                          backgroundColor: isActive ? 'var(--surface)' : 'transparent',
                          boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                          border: 'none',
                          cursor: 'pointer',
                          whiteSpace: 'nowrap',
                          transition: 'all 0.15s ease',
                        }}
                      >
                        {tab}
                      </button>
                    )
                  })}
                </div>
              )}

              {currentTool === 'token-monitor' && (
                /* Token Monitor Segmented Tabs */
                <div
                  style={{
                    display: 'flex',
                    backgroundColor: 'var(--tonal)',
                    borderRadius: '8px',
                    padding: '3px',
                    gap: '2px',
                  }}
                >
                  {['Consumption Overview', 'Telemetry & Logs', 'Token Price'].map((tab, idx) => {
                    const isActive = tokenMonitorTab === idx
                    return (
                      <button
                        key={tab}
                        onClick={() => setTokenMonitorTab(idx)}
                        style={{
                          borderRadius: '6px',
                          padding: '6px 16px',
                          fontSize: '12px',
                          fontWeight: isActive ? 600 : 500,
                          color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                          backgroundColor: isActive ? 'var(--surface)' : 'transparent',
                          boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                          border: 'none',
                          cursor: 'pointer',
                          whiteSpace: 'nowrap',
                          transition: 'all 0.15s ease',
                        }}
                      >
                        {tab}
                      </button>
                    )
                  })}
                </div>
              )}

              {currentTool === 'utilities' && (
                /* Utilities Segmented Tabs */
                <div
                  style={{
                    display: 'flex',
                    backgroundColor: 'var(--tonal)',
                    borderRadius: '8px',
                    padding: '3px',
                    gap: '2px',
                  }}
                >
                  {[
                    { id: 'cache' as const, label: 'Storage Manager', Icon: HardDrive },
                    { id: 'archives' as const, label: 'Archived Projects', Icon: FolderArchive },
                    { id: 'import' as const, label: 'Chat & Project Importer', Icon: ArrowRightLeft },
                  ].map(({ id, label, Icon }) => {
                    const isActive = utilitiesTab === id
                    return (
                      <button
                        key={id}
                        onClick={() => navigate({ tool: 'utilities', tab: id })}
                        style={{
                          borderRadius: '6px',
                          padding: '6px 16px',
                          fontSize: '12px',
                          fontWeight: isActive ? 600 : 500,
                          color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                          backgroundColor: isActive ? 'var(--surface)' : 'transparent',
                          boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                          border: 'none',
                          cursor: 'pointer',
                          whiteSpace: 'nowrap',
                          transition: 'all 0.15s ease',
                        }}
                      >
                        <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                          <Icon size={13} />
                          {label}
                        </span>
                      </button>
                    )
                  })}
                </div>
              )}

              {currentTool === 'remote-connect' && (
                /* Remote Connect Segmented Tabs */
                <div
                  style={{
                    display: 'flex',
                    backgroundColor: 'var(--tonal)',
                    borderRadius: '8px',
                    padding: '3px',
                    gap: '2px',
                  }}
                >
                  {['Remote', 'Gemini', 'WhatsApp', 'Telegram', 'Discord'].map((tab, idx) => {
                    const isActive = bridgeTab === idx
                    return (
                      <button
                        key={tab}
                        onClick={() => setBridgeTab(idx)}
                        style={{
                          borderRadius: '6px',
                          padding: '6px 16px',
                          fontSize: '12px',
                          fontWeight: isActive ? 600 : 500,
                          color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                          backgroundColor: isActive ? 'var(--surface)' : 'transparent',
                          boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                          border: 'none',
                          cursor: 'pointer',
                          whiteSpace: 'nowrap',
                          transition: 'all 0.15s ease',
                        }}
                      >
                        {tab}
                      </button>
                    )
                  })}
                </div>
              )}

              {currentTool === 'system-settings' && (
                /* System Settings Category Tabs */
                <div
                  style={{
                    display: 'flex',
                    backgroundColor: 'var(--tonal)',
                    borderRadius: '8px',
                    padding: '3px',
                    gap: '2px',
                  }}
                >
                  {['General', 'Path & Storage', 'Error & Privacy', 'About'].map((tab, idx) => {
                    const isActive = systemSettingsTab === idx
                    return (
                      <button
                        key={tab}
                        onClick={() => setSystemSettingsTab(idx)}
                        style={{
                          borderRadius: '6px',
                          padding: '6px 16px',
                          fontSize: '12px',
                          fontWeight: isActive ? 600 : 500,
                          color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                          backgroundColor: isActive ? 'var(--surface)' : 'transparent',
                          boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                          border: 'none',
                          cursor: 'pointer',
                          whiteSpace: 'nowrap',
                          transition: 'all 0.15s ease',
                        }}
                      >
                        {tab}
                      </button>
                    )
                  })}
                </div>
              )}

            </div>

            <div id="top-bar-right" style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              {currentTool === 'acp-adapter' && (
                <div
                  id="acp-mesh-top-bar-portal"
                  style={{ display: 'flex', alignItems: 'center', gap: '12px' }}
                />
              )}
            </div>
          </header>
        )}

        {/* Scrollable Feature Page View (Whole Page Scrolls) */}
        <main
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: '24px',
          }}
        >
          <ErrorBoundary>
            {currentTool === 'switcher' && (
              <ErrorBoundary>
                {currentTab === 'dashboard' && (
                  <QuotaDashboardPage
                    fleet={fleet}
                    directAccounts={directAccounts}
                    activeAccountEmail={status?.active_account || fleet?.active_account || ''}
                    rules={rules}
                    onRefresh={loadAllData}
                    onAutoSwitchToggled={(val) => {
                      if (rules) setRules({ ...rules, auto_switch_enabled: val })
                      loadAllData()
                    }}
                  />
                )}
                {currentTab === 'fingerprints' && (
                  <FingerprintsPage accounts={effectiveAccounts} />
                )}
                {currentTab === 'switcher-settings' && (
                  <SwitcherSettingsPage
                    initialRules={rules}
                    onSaved={loadAllData}
                  />
                )}
              </ErrorBoundary>
            )}

            {currentTool === 'system-settings' && (
              <ErrorBoundary>
                <SystemSettingsPage
                  status={status}
                  onRefresh={loadAllData}
                  activeTab={systemSettingsTab}
                  onTabChange={setSystemSettingsTab}
                />
              </ErrorBoundary>
            )}
            {currentTool === 'custom-models' && (
              <ErrorBoundary>
                <CustomModelsPage />
              </ErrorBoundary>
            )}
            {currentTool === 'enhancements' && (
              <ErrorBoundary>
                <AppEnhancementsPage activeCategoryTab={enhancementTab} />
              </ErrorBoundary>
            )}
            {currentTool === 'extensions' && (
              <ErrorBoundary>
                <ExtensionsPage
                  activeTab={extensionsTab}
                  onTabChange={setExtensionsTab}
                  fallbackProject={availableProjects[0]?.name}
                />
              </ErrorBoundary>
            )}
            {currentTool === 'token-monitor' && (
              <ErrorBoundary>
                <TokenMonitorPage
                  activeTab={tokenMonitorTab}
                  onTabChange={setTokenMonitorTab}
                />
              </ErrorBoundary>
            )}
            {currentTool === 'utilities' && (
              <ErrorBoundary>
                <UtilitiesPage
                  activeTab={utilitiesTab}
                  onTabChange={(tab) => navigate({ tool: 'utilities', tab })}
                />
              </ErrorBoundary>
            )}
            {currentTool === 'remote-connect' && (
              <ErrorBoundary>
                <BridgePage
                  activeTab={bridgeTab}
                  onTabChange={setBridgeTab}
                />
              </ErrorBoundary>
            )}
            {currentTool === 'acp-adapter' && (
              <ErrorBoundary>
                <ACPAgentMeshPage />
              </ErrorBoundary>
            )}
          </ErrorBoundary>
        </main>
      </div>
    </div>
  )
}

export default App
