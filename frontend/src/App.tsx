import React, { useState, useEffect, useRef } from 'react'
import {
  BRIDGE_TABS,
  ENHANCEMENT_TABS,
  EXTENSIONS_TABS,
  SYSTEM_SETTINGS_TABS,
  TOKEN_MONITOR_TABS,
  routeFromNavigationTarget,
  routeToTool,
  type BridgeTab,
  type EnhancementTab,
  type ExtensionsTab,
  type SystemSettingsTab,
  type TokenMonitorTab,
  type UtilitiesTab,
} from './navigation/routes'
import { useRouteNavigation } from './navigation/useRouteNavigation'
import { ArrowRightLeft, FolderArchive, HardDrive, type LucideIcon } from 'lucide-react'
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
import { api, getSessionToken, shouldRunBackgroundPoll } from './api'
import {
  dashboardIsOffline,
  type DashboardEndpoint,
} from './utils/dashboardLoadState'
import {
  useDashboardData,
  type DashboardLoaders,
} from './hooks/useDashboardData'

interface SecondaryTabOption<T extends string> {
  id: T
  label: string
  Icon?: LucideIcon
}

interface SecondaryTabsProps<T extends string> {
  options: readonly SecondaryTabOption<T>[]
  activeId: T
  onSelect: (tab: T) => void
}

function SecondaryTabs<T extends string>({ options, activeId, onSelect }: SecondaryTabsProps<T>) {
  return (
    <div
      style={{
        display: 'flex',
        backgroundColor: 'var(--tonal)',
        borderRadius: '8px',
        padding: '3px',
        gap: '2px',
      }}
    >
      {options.map(({ id, label, Icon }) => {
        const isActive = activeId === id
        return (
          <button
            key={id}
            type="button"
            data-route-tab={id}
            aria-pressed={isActive}
            onClick={() => onSelect(id)}
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
              {Icon ? <Icon size={13} /> : null}
              {label}
            </span>
          </button>
        )
      })}
    </div>
  )
}

const SYSTEM_SETTINGS_TAB_OPTIONS = [
  { id: 'general', label: 'General' },
  { id: 'storage', label: 'Path & Storage' },
  { id: 'privacy', label: 'Error & Privacy' },
  { id: 'about', label: 'About' },
] as const satisfies ReadonlyArray<SecondaryTabOption<SystemSettingsTab>>

const ENHANCEMENT_TAB_OPTIONS = [
  { id: 'chat-view', label: 'Chat View' },
  { id: 'project-panel', label: 'Project Panel' },
  { id: 'auxiliary-panel', label: 'Auxiliary Panel' },
] as const satisfies ReadonlyArray<SecondaryTabOption<EnhancementTab>>

const EXTENSIONS_TAB_OPTIONS = [
  { id: 'built-in', label: 'Built-in' },
  { id: 'community', label: 'Community' },
] as const satisfies ReadonlyArray<SecondaryTabOption<ExtensionsTab>>

const TOKEN_MONITOR_TAB_OPTIONS = [
  { id: 'overview', label: 'Consumption Overview' },
  { id: 'telemetry', label: 'Telemetry & Logs' },
  { id: 'token-price', label: 'Token Price' },
] as const satisfies ReadonlyArray<SecondaryTabOption<TokenMonitorTab>>

const UTILITIES_TAB_OPTIONS = [
  { id: 'cache', label: 'Storage Manager', Icon: HardDrive },
  { id: 'archives', label: 'Archived Projects', Icon: FolderArchive },
  { id: 'import', label: 'Chat & Project Importer', Icon: ArrowRightLeft },
] as const satisfies ReadonlyArray<SecondaryTabOption<UtilitiesTab>>

const BRIDGE_TAB_OPTIONS = [
  { id: 'remote', label: 'Remote' },
  { id: 'gemini', label: 'Gemini' },
  { id: 'whatsapp', label: 'WhatsApp' },
  { id: 'telegram', label: 'Telegram' },
  { id: 'discord', label: 'Discord' },
] as const satisfies ReadonlyArray<SecondaryTabOption<BridgeTab>>

export interface AppProps {
  dashboardLoaders?: DashboardLoaders
}

export const App: React.FC<AppProps> = ({ dashboardLoaders }) => {
  const { route, navigate } = useRouteNavigation()
  const currentTool = routeToTool(route)
  const currentTab = route.tool === 'switcher' ? route.tab : 'dashboard'
  const utilitiesTab = route.tool === 'utilities' ? route.tab : 'cache'
  const systemSettingsTab = route.tool === 'system-settings' ? route.tab : 'general'
  const enhancementTab = route.tool === 'enhancements' ? route.tab : 'chat-view'
  const extensionsTab = route.tool === 'extensions' ? route.tab : 'built-in'
  const bridgeTab = route.tool === 'remote-connect' ? route.tab : 'remote'
  const tokenMonitorTab = route.tool === 'token-monitor' ? route.tab : 'overview'
  const onRulesSurface = route.tool === 'system-settings' ||
    (route.tool === 'switcher' && route.tab === 'switcher-settings')
  const onProjectsSurface = route.tool === 'switcher'
  const {
    state,
    isLoading,
    failedEndpoints,
    effectiveAccounts,
    directAccounts,
    load: loadAllData,
    refreshLive,
    retry,
  } = useDashboardData({ loaders: dashboardLoaders })
  const status = state.status.data
  const fleet = state.quota.data
  const rules = state.rules.data
  const availableProjects = state.projects.data ?? []
  const [isLocked, setIsLocked] = useState<boolean>(false)

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

  useEffect(() => {
    checkAuth()
    void loadAllData()
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
      void refreshLive()
    }, 10000)
    const handleVisibilityChange = () => {
      if (shouldRunBackgroundPoll()) {
        void refreshLive()
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
  }, [loadAllData, navigate, refreshLive])

  const accountsFallbackRef = useRef<string | null>(null)

  useEffect(() => {
    const quota = state.quota
    const fleetHasAccounts = (quota.data?.accounts.length ?? 0) > 0
    const needsAccounts = quota.status === 'error' ||
      quota.status === 'degraded' ||
      (quota.status === 'ready' && !fleetHasAccounts)
    const key = `${quota.status}:${quota.updatedAt ?? 0}:${quota.error ?? ''}`
    if (needsAccounts && accountsFallbackRef.current !== key) {
      accountsFallbackRef.current = key
      void retry(['accounts'])
    } else if (fleetHasAccounts) {
      accountsFallbackRef.current = null
    }
  }, [retry, state.quota])

  const rulesSurfaceRef = useRef(onRulesSurface)
  const projectsSurfaceRef = useRef(onProjectsSurface)

  useEffect(() => {
    const entered = !rulesSurfaceRef.current && onRulesSurface
    rulesSurfaceRef.current = onRulesSurface
    if (entered) {
      void retry(['rules'])
    }
  }, [onRulesSurface, retry])

  useEffect(() => {
    const entered = !projectsSurfaceRef.current && onProjectsSurface
    projectsSurfaceRef.current = onProjectsSurface
    if (entered) {
      void retry(['projects'])
    }
  }, [onProjectsSurface, retry])

  useEffect(() => {
    const onAccountSwitched = () => { void refreshLive() }
    const onDashboardMutation = (event: Event) => {
      const endpoint = (event as CustomEvent<{ endpoint?: string }>).detail?.endpoint
      if (endpoint === 'rules' || endpoint === 'projects' || endpoint === 'accounts') {
        void retry([endpoint])
      }
    }
    window.addEventListener('swiss:account-switched', onAccountSwitched)
    window.addEventListener('swiss:dashboard-mutated', onDashboardMutation)
    return () => {
      window.removeEventListener('swiss:account-switched', onAccountSwitched)
      window.removeEventListener('swiss:dashboard-mutated', onDashboardMutation)
    }
  }, [refreshLive, retry])

  useEffect(() => {
    const activeProjects = availableProjects.filter((project) => !project.is_archived)
    if (
      activeProjects.length > 0 &&
      !localStorage.getItem('antigravity_last_active_project')
    ) {
      localStorage.setItem('antigravity_last_active_project', activeProjects[0].name)
    }
  }, [availableProjects])

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
          void loadAllData()
        }}
      />
    )
  }

  return (
    <div data-active-tool={currentTool} style={{ display: 'flex', width: '100vw', height: '100vh', overflow: 'hidden' }}>
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
                <SecondaryTabs
                  options={ENHANCEMENT_TAB_OPTIONS}
                  activeId={enhancementTab}
                  onSelect={(tab) => navigate({ tool: 'enhancements', tab })}
                />
              )}

              {currentTool === 'extensions' && (
                <SecondaryTabs
                  options={EXTENSIONS_TAB_OPTIONS}
                  activeId={extensionsTab}
                  onSelect={(tab) => navigate({ tool: 'extensions', tab })}
                />
              )}

              {currentTool === 'token-monitor' && (
                <SecondaryTabs
                  options={TOKEN_MONITOR_TAB_OPTIONS}
                  activeId={tokenMonitorTab}
                  onSelect={(tab) => navigate({ tool: 'token-monitor', tab })}
                />
              )}

              {currentTool === 'utilities' && (
                <SecondaryTabs
                  options={UTILITIES_TAB_OPTIONS}
                  activeId={utilitiesTab}
                  onSelect={(tab) => navigate({ tool: 'utilities', tab })}
                />
              )}

              {currentTool === 'remote-connect' && (
                <SecondaryTabs
                  options={BRIDGE_TAB_OPTIONS}
                  activeId={bridgeTab}
                  onSelect={(tab) => navigate({ tool: 'remote-connect', tab })}
                />
              )}

              {currentTool === 'system-settings' && (
                <SecondaryTabs
                  options={SYSTEM_SETTINGS_TAB_OPTIONS}
                  activeId={systemSettingsTab}
                  onSelect={(tab) => navigate({ tool: 'system-settings', tab })}
                />
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

        {failedEndpoints.length > 0 && (
          <div
            data-dashboard-health
            role="status"
            aria-live="polite"
            style={{
              minHeight: '36px',
              boxSizing: 'border-box',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '16px',
              padding: '0 24px',
              backgroundColor: 'var(--tonal)',
              borderBottom: '1px solid var(--border)',
              color: 'var(--text-muted)',
              fontSize: '12px',
              whiteSpace: 'nowrap',
            }}
          >
            <span>
              {dashboardIsOffline(state)
                ? 'Daemon offline'
                : `${failedEndpoints.length} source${failedEndpoints.length === 1 ? '' : 's'} unavailable \u00b7 Stale data`}
            </span>
            <button
              data-dashboard-retry
              type="button"
              disabled={isLoading}
              onClick={() => void retry(failedEndpoints as DashboardEndpoint[])}
              style={{
                border: '1px solid var(--border)',
                borderRadius: '6px',
                padding: '4px 12px',
                backgroundColor: 'var(--surface)',
                color: 'var(--text)',
                fontSize: '12px',
                fontWeight: 600,
                whiteSpace: 'nowrap',
                cursor: isLoading ? 'default' : 'pointer',
              }}
            >
              Retry
            </button>
          </div>
        )}

        {/* Scrollable Feature Page View (Whole Page Scrolls) */}
        <main
          aria-busy={isLoading}
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
                      void val
                      void loadAllData()
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
                  activeTab={SYSTEM_SETTINGS_TABS.indexOf(systemSettingsTab)}
                  onTabChange={(tab) => {
                    const nextTab = SYSTEM_SETTINGS_TABS[tab]
                    if (nextTab) navigate({ tool: 'system-settings', tab: nextTab })
                  }}
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
                <AppEnhancementsPage activeCategoryTab={ENHANCEMENT_TABS.indexOf(enhancementTab)} />
              </ErrorBoundary>
            )}
            {currentTool === 'extensions' && (
              <ErrorBoundary>
                <ExtensionsPage
                  activeTab={EXTENSIONS_TABS.indexOf(extensionsTab)}
                  onTabChange={(tab) => {
                    const nextTab = EXTENSIONS_TABS[tab]
                    if (nextTab) navigate({ tool: 'extensions', tab: nextTab })
                  }}
                  fallbackProject={availableProjects[0]?.name}
                />
              </ErrorBoundary>
            )}
            {currentTool === 'token-monitor' && (
              <ErrorBoundary>
                <TokenMonitorPage
                  activeTab={TOKEN_MONITOR_TABS.indexOf(tokenMonitorTab)}
                  onTabChange={(tab) => {
                    const nextTab = TOKEN_MONITOR_TABS[tab]
                    if (nextTab) navigate({ tool: 'token-monitor', tab: nextTab })
                  }}
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
                  activeTab={BRIDGE_TABS.indexOf(bridgeTab)}
                  onTabChange={(tab) => {
                    const nextTab = BRIDGE_TABS[tab]
                    if (nextTab) navigate({ tool: 'remote-connect', tab: nextTab })
                  }}
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
