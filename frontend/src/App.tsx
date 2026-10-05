import React, { useState, useEffect } from 'react'
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
import type { FleetQuotaSummary, RuleConfig, SystemStatus } from './types'
import { api } from './api'

export const App: React.FC = () => {
  const [currentTool, setCurrentTool] = useState<number>(0) // 0: Switcher, 1: Marketplace, 2: Settings, 3: Custom Models, 4: Enhancements, 5: Automations
  const [currentTab, setCurrentTab] = useState<number>(0) // 0: Dashboard, 1: MFA, 2: FP, 3: Cache, 4: Rules
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [fleet, setFleet] = useState<FleetQuotaSummary | null>(null)
  const [rules, setRules] = useState<RuleConfig | null>(null)
  const [, setLoading] = useState<boolean>(true)

  const loadAllData = async () => {
    try {
      const [s, f, r] = await Promise.allSettled([
        api.getStatus(),
        api.getFleetQuota(),
        api.getRules(),
      ])

      if (s.status === 'fulfilled') setStatus(s.value)
      if (f.status === 'fulfilled') setFleet(f.value)
      if (r.status === 'fulfilled') setRules(r.value)
    } catch (err) {
      console.error('Data loading error:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAllData()
    // Poll status and fleet metrics periodically
    const interval = setInterval(loadAllData, 10000)
    return () => clearInterval(interval)
  }, [])

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
              height: '56px',
              backgroundColor: '#ffffff',
              borderBottom: '1px solid var(--border)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '0 24px',
              flexShrink: 0,
            }}
          >
            <h1 style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              {currentTool === 1
                ? 'Tools Marketplace & Extensions'
                : currentTool === 2
                ? 'System & Runtime Settings'
                : currentTool === 3
                ? 'Custom Model Providers & Endpoints'
                : currentTool === 4
                ? 'App Enhancements & Usability'
                : currentTool === 5
                ? 'Scheduled Task Automations'
                : 'Archived Projects'}
            </h1>
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
            <SystemSettingsPage status={status} onRefresh={loadAllData} />
          )}
          {currentTool === 3 && <CustomModelsPage />}
          {currentTool === 4 && <AppEnhancementsPage />}
          {currentTool === 5 && <ScheduledTemplatesPage />}
          {currentTool === 6 && <ArchivedProjectsPage />}
        </main>
      </div>
    </div>
  )
}

export default App
