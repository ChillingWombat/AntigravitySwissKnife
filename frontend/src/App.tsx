import React, { useState, useEffect } from 'react'
import { ChevronDown } from 'lucide-react'
import { NavRail } from './components/NavRail'
import { TopRibbon } from './components/TopRibbon'
import { QuotaDashboardPage } from './pages/QuotaDashboardPage'
import { MfaVaultPage } from './pages/MfaVaultPage'
import { FingerprintsPage } from './pages/FingerprintsPage'
import { BrainCachePage } from './pages/BrainCachePage'
import { SwitcherSettingsPage } from './pages/SwitcherSettingsPage'
import { ToolsMarketplacePage } from './pages/ToolsMarketplacePage'
import { SystemSettingsPage } from './pages/SystemSettingsPage'
import { CustomModelsPage } from './pages/CustomModelsPage'
import { AppEnhancementsPage } from './pages/AppEnhancementsPage'
import { ScheduledTemplatesPage } from './pages/ScheduledTemplatesPage'
import { ArchivedProjectsPage } from './pages/ArchivedProjectsPage'
import { ExtensionsPage } from './pages/ExtensionsPage'
import { TokenMonitorPage } from './pages/TokenMonitorPage'
import { UtilitiesPage } from './pages/UtilitiesPage'
import { AppLockScreen } from './components/AppLockScreen'
import type { FleetQuotaSummary, RuleConfig, SystemStatus } from './types'
import { api } from './api'

export const App: React.FC = () => {
  const [currentTool, setCurrentTool] = useState<number>(0) // 0: Switcher, 1: Marketplace, 2: Settings, 3: Custom Models, 4: Enhancements, 5: Automations, 6: Archived, 7: Extensions, 8: Token Monitor, 9: Utilities
  const [currentTab, setCurrentTab] = useState<number>(0) // 0: Dashboard, 1: MFA, 2: FP, 3: Cache, 4: Rules
  const [systemSettingsTab, setSystemSettingsTab] = useState<number>(0) // 0: General, 1: Path & Storage, 2: Error & Privacy, 3: About
  const [enhancementTab, setEnhancementTab] = useState<number>(0) // 0: Chat View, 1: Project Panel, 2: Overview Panel, 3: Chat History
  const [automationTab, setAutomationTab] = useState<'catalog' | 'created'>('catalog')
  const [extensionTab, setExtensionTab] = useState<number>(0) // 0: Preview, 1: File Explorer, 2: Memos, 3: GitHub Workspace, 4: Mobile, 5: Computer Use
  const [extensionScope, setExtensionScope] = useState<string>(() => {
    return localStorage.getItem('antigravity_extension_scope') || 'GLOBAL'
  })
  const [availableProjects, setAvailableProjects] = useState<Array<{ name: string; color: string; order: number; is_archived: boolean }>>([])
  const [isScopeDropdownOpen, setIsScopeDropdownOpen] = useState<boolean>(false)
  const [utilitiesTab, setUtilitiesTab] = useState<number>(0) // 0: Importer, 1: ACP Inspector
  const [tokenMonitorTab, setTokenMonitorTab] = useState<number>(0) // 0: Overview, 1: Telemetry & Logs, 2: Pricing Matrix
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [fleet, setFleet] = useState<FleetQuotaSummary | null>(null)
  const [rules, setRules] = useState<RuleConfig | null>(null)
  const [, setLoading] = useState<boolean>(true)
  const [isLocked, setIsLocked] = useState<boolean>(false)

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
      const [s, f, r, p] = await Promise.allSettled([
        api.getStatus(),
        api.getFleetQuota(),
        api.getRules(),
        api.getGUIProjects(),
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
    } catch (err) {
      console.error('Data loading error:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!isScopeDropdownOpen) return
    const handleOutsideClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement | null
      if (!target) return
      if (target.closest('#btn-extension-scope-dropdown') || target.closest('#extension-scope-menu')) {
        return
      }
      setIsScopeDropdownOpen(false)
    }
    document.addEventListener('mousedown', handleOutsideClick)
    return () => document.removeEventListener('mousedown', handleOutsideClick)
  }, [isScopeDropdownOpen])

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
            setExtensionTab(3)
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

  if (isLocked) {
    return <AppLockScreen onUnlocked={() => setIsLocked(false)} />
  }

  return (
    <div style={{ display: 'flex', width: '100vw', height: '100vh', overflow: 'hidden' }}>
      {/* 1. Left Navigation Rail (Fixed 220px) */}
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
        ) : (
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
                  {['Chat View', 'Project Panel', 'Auxiliary & Overview Panel', 'Chat History'].map((tab, idx) => {
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

              {currentTool === 5 && (
                /* Task Automations Segmented Tabs */
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
                    { id: 'catalog', label: 'Task Templates' },
                    { id: 'created', label: 'Created Tasks' },
                  ].map((tab) => {
                    const isActive = automationTab === tab.id
                    return (
                      <button
                        key={tab.id}
                        onClick={() => setAutomationTab(tab.id as 'catalog' | 'created')}
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
                        {tab.label}
                      </button>
                    )
                  })}
                </div>
              )}

              {(currentTool === 7 || currentTool === 10) && (
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  {/* Extension Scope Dropdown (GLOBAL vs Specific Project) */}
                  <div style={{ position: 'relative' }}>
                    <button
                      id="btn-extension-scope-dropdown"
                      onClick={() => setIsScopeDropdownOpen(!isScopeDropdownOpen)}
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '6px',
                        padding: '5px 12px',
                        borderRadius: '6px',
                        fontSize: '12px',
                        fontWeight: 500,
                        backgroundColor: 'var(--card)',
                        color: 'var(--text)',
                        border: '1px solid var(--border)',
                        cursor: 'pointer',
                        transition: 'all 0.15s ease',
                        whiteSpace: 'nowrap',
                      }}
                      title="Select Extension Scope (GLOBAL or specific project)"
                    >
                      <span style={{ color: 'var(--text-muted)' }}>Scope:</span>
                      <strong style={{ color: extensionScope === 'GLOBAL' ? 'var(--primary)' : 'var(--text)' }}>{extensionScope}</strong>
                      <ChevronDown size={12} style={{ opacity: 0.7 }} />
                    </button>

                    {isScopeDropdownOpen && (
                      <div
                        id="extension-scope-menu"
                        style={{
                          position: 'absolute',
                          top: '100%',
                          left: 0,
                          marginTop: '4px',
                          minWidth: '220px',
                          maxHeight: '280px',
                          overflowY: 'auto',
                          backgroundColor: '#ffffff',
                          border: '1px solid var(--border)',
                          borderRadius: '10px',
                          boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
                          zIndex: 1000,
                          padding: '4px',
                        }}
                      >
                        <button
                          onClick={() => {
                            setExtensionScope('GLOBAL')
                            localStorage.setItem('antigravity_extension_scope', 'GLOBAL')
                            setIsScopeDropdownOpen(false)
                          }}
                          style={{
                            width: '100%',
                            textAlign: 'left',
                            padding: '7px 10px',
                            borderRadius: '6px',
                            border: 'none',
                            backgroundColor: extensionScope === 'GLOBAL' ? 'rgba(26, 115, 232, 0.1)' : 'transparent',
                            color: extensionScope === 'GLOBAL' ? 'var(--primary)' : 'var(--text)',
                            fontWeight: extensionScope === 'GLOBAL' ? 600 : 500,
                            fontSize: '12px',
                            cursor: 'pointer',
                            display: 'flex',
                            alignItems: 'center',
                            gap: '6px',
                          }}
                        >
                          <span style={{ fontWeight: 600 }}>GLOBAL</span>
                          <span style={{ fontSize: '10px', color: 'var(--text-muted)', marginLeft: 'auto' }}>(Global Scope)</span>
                        </button>

                        {availableProjects.map((proj) => (
                          <button
                            key={proj.name}
                            onClick={() => {
                              setExtensionScope(proj.name)
                              localStorage.setItem('antigravity_extension_scope', proj.name)
                              localStorage.setItem('antigravity_last_active_project', proj.name)
                              setIsScopeDropdownOpen(false)
                            }}
                            style={{
                              width: '100%',
                              textAlign: 'left',
                              padding: '7px 10px',
                              borderRadius: '6px',
                              border: 'none',
                              backgroundColor: extensionScope === proj.name ? 'rgba(26, 115, 232, 0.1)' : 'transparent',
                              color: extensionScope === proj.name ? 'var(--primary)' : 'var(--text)',
                              fontWeight: extensionScope === proj.name ? 600 : 500,
                              fontSize: '12px',
                              cursor: 'pointer',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '6px',
                            }}
                          >
                            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{proj.name}</span>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Extensions Segmented Tabs */}
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
                      'Preview Browser',
                      'Auxiliary File Explorer',
                      'Quick Memos',
                      'GitHub Workspace',
                      'Mobile Simulator',
                      'Computer Use Enhancer',
                    ].map((tab, idx) => {
                      const effectiveActiveTab = currentTool === 10 ? 3 : extensionTab
                      const isActive = effectiveActiveTab === idx
                      return (
                        <button
                          key={tab}
                          onClick={() => {
                            if (currentTool === 10) setCurrentTool(7)
                            setExtensionTab(idx)
                          }}
                          style={{
                            borderRadius: '6px',
                            padding: '6px 14px',
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
                  {['Consumption Overview', 'Telemetry & Logs', 'Pricing Matrix'].map((tab, idx) => {
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
                  {['Chat & Project Importer', 'ACP Agent Mesh'].map((tab, idx) => {
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

              {currentTool !== 2 && currentTool !== 3 && currentTool !== 4 && currentTool !== 5 && currentTool !== 7 && currentTool !== 8 && currentTool !== 9 && currentTool !== 10 && (
                <h1 style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', margin: 0 }}>
                  {currentTool === 1
                    ? 'Tools Marketplace'
                    : currentTool === 6
                    ? 'Archived Projects'
                    : ''}
                </h1>
              )}
            </div>

            <div id="top-bar-right" style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              {(currentTool === 7 || currentTool === 10) && (
                <span
                  style={{
                    fontSize: '11.5px',
                    fontWeight: 600,
                    backgroundColor: 'rgba(26, 115, 232, 0.1)',
                    color: 'var(--primary)',
                    padding: '5px 12px',
                    borderRadius: '6px',
                    border: '1px solid rgba(26, 115, 232, 0.25)',
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '6px',
                    whiteSpace: 'nowrap',
                  }}
                >
                  Antigravity 2.0 Desktop Exclusive
                </span>
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
          {currentTool === 0 && (
            <>
              {currentTab === 0 && (
                <QuotaDashboardPage
                  fleet={fleet}
                  rules={rules}
                  onRefresh={loadAllData}
                  onAutoSwitchToggled={(val) => {
                    if (rules) setRules({ ...rules, auto_switch_enabled: val })
                    loadAllData()
                  }}
                />
              )}
              {currentTab === 1 && (
                <MfaVaultPage
                  accounts={fleet?.accounts || []}
                  onRefresh={loadAllData}
                />
              )}
              {currentTab === 2 && (
                <FingerprintsPage accounts={fleet?.accounts || []} />
              )}
              {currentTab === 3 && <BrainCachePage />}
              {currentTab === 4 && (
                <SwitcherSettingsPage
                  initialRules={rules}
                  onSaved={loadAllData}
                />
              )}
            </>
          )}

          {currentTool === 1 && <ToolsMarketplacePage onSelectTool={setCurrentTool} />}
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
          {currentTool === 5 && <ScheduledTemplatesPage activeTab={automationTab} onTabChange={setAutomationTab} />}
          {currentTool === 6 && <ArchivedProjectsPage />}
          {(currentTool === 7 || currentTool === 10) && (
            <ExtensionsPage
              activeTab={currentTool === 10 ? 3 : extensionTab}
              onTabChange={(t) => {
                if (currentTool === 10) setCurrentTool(7)
                setExtensionTab(t)
              }}
              scope={extensionScope}
              onScopeChange={setExtensionScope}
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
        </main>
      </div>
    </div>
  )
}

export default App
