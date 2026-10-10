import React, { useState, useEffect } from 'react'
import { NavRail } from './components/NavRail'
import { TopRibbon } from './components/TopRibbon'
import { QuotaDashboardPage } from './pages/QuotaDashboardPage'
import { FingerprintsPage } from './pages/FingerprintsPage'
import { SwitcherSettingsPage } from './pages/SwitcherSettingsPage'
import { SystemSettingsPage } from './pages/SystemSettingsPage'
import { CustomModelsPage } from './pages/CustomModelsPage'
import { AppEnhancementsPage } from './pages/AppEnhancementsPage'
import { ScheduledTemplatesPage } from './pages/ScheduledTemplatesPage'
import { ArchivedProjectsPage } from './pages/ArchivedProjectsPage'
import { ExtensionsPage } from './pages/ExtensionsPage'
import { TokenMonitorPage } from './pages/TokenMonitorPage'
import { UtilitiesPage } from './pages/UtilitiesPage'
import { AppLockScreen } from './components/AppLockScreen'
import { ErrorBoundary } from './components/ErrorBoundary'
import type { AccountState, FleetQuotaSummary, RuleConfig, SystemStatus } from './types'
import { toAccountState } from './types'
import { api } from './api'

export const App: React.FC = () => {
  const [currentTool, setCurrentTool] = useState<number>(0) // 0: Switcher, 2: Settings, 3: Custom Models, 4: Enhancements, 5: Automations, 6: Archived, 7: Extensions, 8: Token Monitor, 9: Utilities
  const [currentTab, setCurrentTab] = useState<number>(0) // 0: Dashboard, 1: FP, 2: Rules
  const [systemSettingsTab, setSystemSettingsTab] = useState<number>(0) // 0: General, 1: Path & Storage, 2: Error & Privacy, 3: About
  const [enhancementTab, setEnhancementTab] = useState<number>(0) // 0: Chat View, 1: Project Panel, 2: Overview Panel
  const [availableProjects, setAvailableProjects] = useState<Array<{ name: string; color: string; order: number; is_archived: boolean }>>([])
  const [utilitiesTab, setUtilitiesTab] = useState<number>(0) // 0: Cache Manager, 1: ACP Agent Mesh, 2: Chat & Project Importer
  const [tokenMonitorTab, setTokenMonitorTab] = useState<number>(0) // 0: Overview, 1: Telemetry & Logs, 2: Token Price
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [fleet, setFleet] = useState<FleetQuotaSummary | null>(null)
  const [directAccounts, setDirectAccounts] = useState<any[]>([])
  const [rules, setRules] = useState<RuleConfig | null>(null)
  const [, setLoading] = useState<boolean>(true)
  const [isLocked, setIsLocked] = useState<boolean>(false)

  const effectiveAccounts: AccountState[] = React.useMemo(() => {
    if (fleet?.accounts && fleet.accounts.length > 0) {
      return fleet.accounts
    }
    if (directAccounts && directAccounts.length > 0) {
      const active = status?.active_account || fleet?.active_account || ''
      return directAccounts.map(a => toAccountState(a, active))
    }
    return []
  }, [fleet?.accounts, directAccounts, status?.active_account, fleet?.active_account])

  const checkAuth = async () => {
    try {
      const res = await api.getAuthStatus()
      if (res.password_required) {
        setIsLocked(true)
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


  useEffect(() => {
    checkAuth()
    loadAllData()
    // Listen for navigation requests from Electron system tray context menu
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.onNavigate) {
      electronAPI.onNavigate((toolIdx: number) => {
        if (typeof toolIdx === 'number') {
          if (toolIdx === 10) {
            setCurrentTool(7)
          } else if (toolIdx === 1) {
            setCurrentTool(0)
          } else {
            setCurrentTool(toolIdx)
          }
        }
      })
    }
    // Poll status and fleet metrics periodically
    const interval = setInterval(loadAllData, 10000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    if (status && !status.daemon_running && (currentTool === 7 || currentTool === 10)) {
      setCurrentTool(0)
    }
    if (currentTool === 1) {
      setCurrentTool(0)
    }
  }, [status?.daemon_running, currentTool])

  if (isLocked) {
    return <AppLockScreen onUnlocked={() => setIsLocked(false)} />
  }

  return (
    <div style={{ display: 'flex', width: '100vw', height: '100vh', overflow: 'hidden' }}>
      {/* 1. Left Navigation Rail (Fixed 200px) */}
      <NavRail
        currentTool={currentTool}
        onSelectTool={setCurrentTool}
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
        {currentTool === 0 ? (
          <TopRibbon
            currentTab={currentTab}
            onSelectTab={setCurrentTab}
            activeAccount={status?.active_account || fleet?.active_account || null}
          />
        ) : (currentTool !== 5 && currentTool !== 6 && currentTool !== 7 && currentTool !== 10) ? (
          <header
            style={{
              height: '72px',
              backgroundColor: '#ffffff',
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
              {currentTool === 4 && (
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
                  {['Chat View', 'Project Panel', 'Auxiliary & Overview Panel'].map((tab, idx) => {
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
                          backgroundColor: isActive ? '#ffffff' : 'transparent',
                          boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                          border: 'none',
                          cursor: 'pointer',
                        }}
                      >
                        {tab}
                      </button>
                    )
                  })}
                </div>
              )}

              {currentTool === 8 && (
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
                          backgroundColor: isActive ? '#ffffff' : 'transparent',
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

              {currentTool === 9 && (
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
                  {['Storage Manager', 'ACP Agent Mesh', 'Chat & Project Importer'].map((tab, idx) => {
                    const isActive = utilitiesTab === idx
                    return (
                      <button
                        key={tab}
                        onClick={() => setUtilitiesTab(idx)}
                        style={{
                          borderRadius: '6px',
                          padding: '6px 16px',
                          fontSize: '12px',
                          fontWeight: isActive ? 600 : 500,
                          color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                          backgroundColor: isActive ? '#ffffff' : 'transparent',
                          boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                          border: 'none',
                          cursor: 'pointer',
                        }}
                      >
                        {tab}
                      </button>
                    )
                  })}
                </div>
              )}

              {currentTool === 2 && (
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
                          backgroundColor: isActive ? '#ffffff' : 'transparent',
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

            <div id="top-bar-right" style={{ display: 'flex', alignItems: 'center', gap: '10px' }} />
          </header>
        ) : null}

        {/* Scrollable Feature Page View (Whole Page Scrolls) */}
        <main
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: '24px',
          }}
        >
          <ErrorBoundary>
            {currentTool === 0 && (
              <>
                {currentTab === 0 && (
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
                {currentTab === 1 && (
                  <FingerprintsPage accounts={effectiveAccounts} />
                )}
                {currentTab === 2 && (
                  <SwitcherSettingsPage
                    initialRules={rules}
                    onSaved={loadAllData}
                  />
                )}
              </>
            )}

            {currentTool === 2 && (
              <SystemSettingsPage
                status={status}
                onRefresh={loadAllData}
                activeTab={systemSettingsTab}
                onTabChange={setSystemSettingsTab}
              />
            )}
            {currentTool === 3 && <CustomModelsPage />}
            {currentTool === 4 && <AppEnhancementsPage activeCategoryTab={enhancementTab} />}
            {currentTool === 5 && <ScheduledTemplatesPage />}
            {currentTool === 6 && <ArchivedProjectsPage />}
            {Boolean(status?.daemon_running) && (currentTool === 7 || currentTool === 10) && (
              <ExtensionsPage
                fallbackProject={availableProjects[0]?.name}
              />
            )}
            {currentTool === 8 && (
              <TokenMonitorPage
                activeTab={tokenMonitorTab}
                onTabChange={setTokenMonitorTab}
              />
            )}
            {currentTool === 9 && (
              <UtilitiesPage
                activeTab={utilitiesTab}
                onTabChange={setUtilitiesTab}
              />
            )}
          </ErrorBoundary>
        </main>
      </div>
    </div>
  )
}

export default App
