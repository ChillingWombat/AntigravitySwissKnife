import React, { useState, useEffect, useRef } from 'react'
import {
  RotateCw,
  CheckCircle2,
  ArrowRightLeft,
  Edit2,
  Copy,
  AlertTriangle,
  AlertCircle,
  ArrowUpDown,
  X,
  Search,
  Plus,
  Timer,
  Trash2,
  ExternalLink,
  ChevronDown,
  Monitor,
  Terminal,
  Puzzle,
  Layers,
} from 'lucide-react'
import type { AccountState, FleetQuotaSummary, RuleConfig, DiscoveredAccount, TargetApp } from '../types'
import { normalizePlanTier, toAccountState } from '../types'
import { TABLE_MIN_WIDTH } from '../utils/layoutTokens'
import { getAccountTableDisplay, parseAccountErrorAlert } from '../utils/accountPresentation'
import { CircularGauge } from '../components/CircularGauge'
import { HorizontalQuotaBar } from '../components/HorizontalQuotaBar'
import { AccountDetailModal } from '../components/AccountDetailModal'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'
import antigravityLogo from '../assets/antigravity_logo.png'

import { sortAccounts, type SortMode } from '../utils/accountSorting'

export { sortAccounts, type SortMode }


interface QuotaDashboardPageProps {
  fleet: FleetQuotaSummary | null
  directAccounts?: any[]
  activeAccountEmail?: string
  rules: RuleConfig | null
  onRefresh: () => void
  onAutoSwitchToggled: (enabled: boolean) => void
}

const renderPriorityBadge = (priority?: string) => {
  const p = priority || 'High'
  const isHigh = p === 'High'
  const isMid = p === 'Mid'

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        padding: '2px 8px',
        borderRadius: '6px',
        fontSize: '11px',
        fontWeight: 600,
        backgroundColor: isHigh
          ? 'rgba(26, 115, 232, 0.1)'
          : isMid
          ? 'rgba(95, 99, 104, 0.08)'
          : 'var(--canvas)',
        color: isHigh
          ? 'var(--primary)'
          : isMid
          ? 'var(--text)'
          : 'var(--text-muted)',
        border: `1px solid ${isHigh ? 'rgba(26, 115, 232, 0.25)' : 'var(--border)'}`,
      }}
    >
      {p}
    </span>
  )
}

export const renderPlanTierBadge = (tier?: string) => {
  const t = normalizePlanTier(tier)
  let style: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    padding: '3px 10px',
    borderRadius: '12px',
    fontSize: '11px',
    fontWeight: 600,
    letterSpacing: '0.3px',
    whiteSpace: 'nowrap',
  }

  switch (t) {
    case 'Plus':
      style = { ...style, backgroundColor: '#e6f4ea', color: '#137333' }
      break
    case 'Pro':
      style = { ...style, backgroundColor: '#e8f0fe', color: '#1a73e8' }
      break
    case 'Pro - Trial':
      style = { ...style, backgroundColor: '#e0f7fa', color: '#007b83' }
      break
    case 'Edu':
      style = { ...style, backgroundColor: '#ede7f6', color: '#512da8' }
      break
    case 'Ultra 5X':
      style = { ...style, backgroundColor: '#f3e8fd', color: '#7b1fa2' }
      break
    case 'Ultra 10X':
      style = { ...style, backgroundColor: '#f3e8fd', color: '#6a1b9a' }
      break
    case 'Ultra 20X':
      style = {
        ...style,
        background: 'linear-gradient(135deg, #f3e8fd 0%, #fff8e1 100%)',
        color: '#5c1b8b',
        border: '1px solid #d1c4e9',
        fontWeight: 700,
      }
      break
    case 'Enterprise':
      style = { ...style, backgroundColor: '#e0f2fe', color: '#0369a1', fontWeight: 700 }
      break
    case 'Free':
    default:
      style = { ...style, backgroundColor: 'var(--tonal)', color: 'var(--text-muted)' }
      break
  }

  return <span style={style}>{t}</span>
}

export const QuotaDashboardPage: React.FC<QuotaDashboardPageProps> = ({
  fleet,
  directAccounts,
  activeAccountEmail,
  rules,
  onRefresh,
  onAutoSwitchToggled,
}) => {
  const [selectedRowAccount, setSelectedRowAccount] = useState<AccountState | null>(null)
  const [sortMode, setSortMode] = useState<SortMode>('auto')
  const [isTogglingRules, setIsTogglingRules] = useState(false)
  const [isScanning, setIsScanning] = useState(false)
  const [switchFeedback, setSwitchFeedback] = useState<string | null>(null)
  const [errorDetailAccount, setErrorDetailAccount] = useState<AccountState | null>(null)
  const [isVerifyingErrorAccount, setIsVerifyingErrorAccount] = useState(false)
  const isVerifyingRef = React.useRef<boolean>(false)
  isVerifyingRef.current = isVerifyingErrorAccount
  const errorDetailFetchedAtRef = React.useRef<number>(0)
  const errorDetailPollingRef = React.useRef<boolean>(false)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; account: AccountState } | null>(null)

  // Discovered Accounts Import Modal State
  const [isScanModalOpen, setIsScanModalOpen] = useState(false)
  const [discoveredAccounts, setDiscoveredAccounts] = useState<DiscoveredAccount[]>([])
  const [selectedDiscoveredEmails, setSelectedDiscoveredEmails] = useState<Set<string>>(new Set())
  const [isImportingDiscovered, setIsImportingDiscovered] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [isLogoHovered, setIsLogoHovered] = useState(false)
  const [isLaunchingIDE, setIsLaunchingIDE] = useState(false)

  const mountedRef = useRef(true)
  useEffect(() => {
    mountedRef.current = true
    return () => { mountedRef.current = false }
  }, [])

  const handleLaunchAntigravity = async () => {
    if (isLaunchingIDE) return
    try {
      setIsLaunchingIDE(true)
      setSwitchFeedback('Launching Antigravity 2.0...')
      const electronAPI = (window as any).electronAPI
      let res: any
      if (electronAPI?.launchAntigravity) {
        res = await electronAPI.launchAntigravity()
      } else {
        res = await api.launchHostIDE()
      }
      if (res && res.success === false) {
        throw new Error(res.error || res.message || 'Launch request failed')
      }
      setTimeout(() => {
        if (mountedRef.current) {
          setSwitchFeedback('Antigravity 2.0 launch command dispatched')
        }
      }, 1000)
    } catch (err: any) {
      console.warn('Could not launch Antigravity:', err)
      if (mountedRef.current) {
        setSwitchFeedback('Could not launch Antigravity: ' + (err.message || 'Error'))
      }
    } finally {
      setTimeout(() => {
        if (mountedRef.current) {
          setIsLaunchingIDE(false)
          setSwitchFeedback(null)
        }
      }, 3500)
    }
  }

  const handleRefreshClick = async () => {
    if (isRefreshing) return
    setIsRefreshing(true)
    try {
      await api.refreshFleetQuota().catch(() => {})
      // Fleet refresh runs async on the daemon; poll until it reports done.
      const deadline = Date.now() + 120_000
      for (;;) {
        await new Promise((r) => setTimeout(r, 1500))
        if (!mountedRef.current) return
        if (Date.now() > deadline) break
        const f = await api.getFleetQuota().catch(() => null)
        if (f && !f.refreshing) break
      }
      if (mountedRef.current) onRefresh()
    } finally {
      if (mountedRef.current) setIsRefreshing(false)
    }
  }

  useEffect(() => {
    const handleGlobalClick = () => setContextMenu(null)
    window.addEventListener('click', handleGlobalClick)
    return () => window.removeEventListener('click', handleGlobalClick)
  }, [])

  const accounts: AccountState[] = React.useMemo(() => {
    if (fleet?.accounts && fleet.accounts.length > 0) {
      return fleet.accounts
    }
    if (directAccounts && directAccounts.length > 0) {
      const active = activeAccountEmail || fleet?.active_account || ''
      return directAccounts.map((a) => toAccountState(a, active))
    }
    return []
  }, [fleet?.accounts, directAccounts, activeAccountEmail, fleet?.active_account])

  const pollErrorDetailAccount = React.useCallback(
    async (targetEmail: string, updateUrl: boolean = false): Promise<string | null> => {
      if (!targetEmail || errorDetailPollingRef.current) return null
      errorDetailPollingRef.current = true
      try {
        const q = await api.refreshAccountQuota(targetEmail)
        if (!q) return null
        const hasError = Boolean(q.error_status || q.error_message)
        if (!hasError && q.reset_horizon_text !== 'Not Polled' && !q.reset_horizon_text?.startsWith('Error')) {
          if (isVerifyingRef.current) {
            setIsVerifyingErrorAccount(false)
            setErrorDetailAccount((prev) =>
              prev && prev.email.toLowerCase() === targetEmail.toLowerCase() ? null : prev
            )
            onRefresh()
          }
          return null
        }
        if (q.error_message) {
          errorDetailFetchedAtRef.current = Date.now()
          if (updateUrl) {
            setErrorDetailAccount((prev) =>
              prev && prev.email.toLowerCase() === targetEmail.toLowerCase()
                ? {
                    ...prev,
                    error_message: q.error_message || prev.error_message,
                    status: (q.error_status as any) || prev.status,
                  }
                : prev
            )
          }
          return q.error_message
        }
        return null
      } catch {
        return null
      } finally {
        errorDetailPollingRef.current = false
      }
    },
    [onRefresh]
  )

  useEffect(() => {
    if (!errorDetailAccount?.email) {
      setIsVerifyingErrorAccount(false)
      return
    }
    if (errorDetailAccount.status?.toUpperCase() === 'BANNED') return
    const targetEmail = errorDetailAccount.email
    pollErrorDetailAccount(targetEmail, true)
  }, [errorDetailAccount?.email, errorDetailAccount?.status, pollErrorDetailAccount])

  useEffect(() => {
    if (!errorDetailAccount?.email || errorDetailAccount.status?.toUpperCase() === 'BANNED') return
    const targetEmail = errorDetailAccount.email
    const checkRecovery = () => {
      pollErrorDetailAccount(targetEmail, false)
    }
    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        checkRecovery()
      }
    }
    window.addEventListener('focus', checkRecovery)
    document.addEventListener('visibilitychange', onVisibilityChange)
    const interval = setInterval(checkRecovery, isVerifyingErrorAccount ? 2500 : 4000)
    return () => {
      window.removeEventListener('focus', checkRecovery)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      clearInterval(interval)
    }
  }, [errorDetailAccount?.email, errorDetailAccount?.status, isVerifyingErrorAccount, pollErrorDetailAccount])

  useEffect(() => {
    if (!errorDetailAccount?.email || !isVerifyingErrorAccount) return
    const matching = accounts.find((a) => a.email.toLowerCase() === errorDetailAccount.email.toLowerCase())
    if (
      matching &&
      !matching.error_message &&
      !matching.status?.toUpperCase().includes('ERROR') &&
      !matching.status?.toUpperCase().includes('NEEDS_REAUTH') &&
      !matching.status?.toUpperCase().includes('BANNED') &&
      Boolean(matching.refresh_token?.trim())
    ) {
      setIsVerifyingErrorAccount(false)
      setErrorDetailAccount(null)
    }
  }, [accounts, errorDetailAccount?.email, isVerifyingErrorAccount])
  const activeAccount = activeAccountEmail || fleet?.active_account || accounts.find((a) => a.is_active)?.email || ''
  const autoSwitchOn = rules?.auto_switch_enabled ?? false
  const threshold = rules?.auto_switch_threshold ?? 0.10
  const thresholdWeekly = rules?.auto_switch_weekly_threshold ?? 0.05

  const [switchMenuEmail, setSwitchMenuEmail] = useState<string | null>(null)

  useEffect(() => {
    if (!switchMenuEmail) return
    const handleOutside = () => setSwitchMenuEmail(null)
    window.addEventListener('click', handleOutside)
    return () => window.removeEventListener('click', handleOutside)
  }, [switchMenuEmail])

  const isIndividualMode = rules?.multi_app_sync_mode === 'individual'
  const activeAppMap = rules?.active_app_accounts || {}
  const installed = rules?.installed_apps || { desktop: true, agy: true, vscode: true }

  interface InstalledAppItem {
    key: 'desktop' | 'agy' | 'vscode'
    target: TargetApp
    label: string
    shortLabel: string
    icon: React.ReactNode
  }

  const installedAppsList: InstalledAppItem[] = []
  if (installed.desktop) {
    installedAppsList.push({
      key: 'desktop',
      target: 'desktop',
      label: 'AGY 2.0',
      shortLabel: '2.0',
      icon: <Monitor size={12} />,
    })
  }
  if (installed.agy) {
    installedAppsList.push({
      key: 'agy',
      target: 'agy',
      label: 'AGY CLI',
      shortLabel: 'CLI',
      icon: <Terminal size={12} />,
    })
  }
  if (installed.vscode) {
    installedAppsList.push({
      key: 'vscode',
      target: 'vscode',
      label: 'AGY EXT',
      shortLabel: 'EXT',
      icon: <Puzzle size={12} />,
    })
  }
  if (installedAppsList.length === 0) {
    installedAppsList.push({
      key: 'desktop',
      target: 'desktop',
      label: 'AGY 2.0',
      shortLabel: '2.0',
      icon: <Monitor size={12} />,
    })
  }

  const sortedAccounts = sortAccounts(accounts, activeAccount, threshold, sortMode, rules?.switch_mode, thresholdWeekly)

  const errorAccountsCount = accounts.filter((a) =>
    a.status?.toUpperCase().includes('ERROR')
  ).length
  const bannedAccountsCount = accounts.filter((a) =>
    a.status?.toUpperCase().includes('BANNED')
  ).length

  const fallback5h = accounts.length > 0 ? (accounts.reduce((sum, a) => sum + (a.quota_5h_available ?? 1), 0) / accounts.length) : 0
  const fallbackWeekly = accounts.length > 0 ? (accounts.reduce((sum, a) => sum + (a.quota_weekly ?? 1), 0) / accounts.length) : 0
  const fallback5hClaude = accounts.length > 0 ? (accounts.reduce((sum, a) => sum + (a.quota_5h_claude_gpt ?? 1), 0) / accounts.length) : 0
  const fallbackWeeklyClaude = accounts.length > 0 ? (accounts.reduce((sum, a) => sum + (a.quota_weekly_claude_gpt ?? 1), 0) / accounts.length) : 0

  const gauge5hGemini = (fleet?.fleet_5h_gemini_available ?? fleet?.fleet_5h_available ?? fallback5h) * 100
  const gaugeWeeklyGemini = (fleet?.fleet_weekly_gemini_available ?? fleet?.fleet_weekly_available ?? fallbackWeekly) * 100
  const gauge5hClaude = (fleet?.fleet_5h_claude_gpt_available ?? fallback5hClaude) * 100
  const gaugeWeeklyClaude = (fleet?.fleet_weekly_claude_gpt_available ?? fallbackWeeklyClaude) * 100
  const gauge5h = (fleet?.fleet_5h_available ?? fallback5h) * 100
  const gaugeWeekly = (fleet?.fleet_weekly_available ?? fallbackWeekly) * 100

  const handleToggleAutoSwitch = async () => {
    setIsTogglingRules(true)
    try {
      const nextVal = !autoSwitchOn
      await api.setAutoSwitch(nextVal)
      onAutoSwitchToggled(nextVal)
    } catch (err: any) {
      setSwitchFeedback(`Could not toggle auto-switch: ${err.message}`)
    } finally {
      setIsTogglingRules(false)
    }
  }

  const handleScanLocalAccounts = async () => {
    setIsScanning(true)
    setSwitchFeedback(null)
    try {
      const scanned = await api.scanLocalAccounts()
      const existingEmails = new Set(accounts.map((a) => (a.email || '').toLowerCase()))
      const allScanned = Array.isArray(scanned) ? scanned : []
      const unimported = allScanned.filter((s) => s.email && !existingEmails.has(s.email.toLowerCase()))

      if (unimported.length === 0) {
        setSwitchFeedback('Scan complete: All accounts detected on this machine are already in your fleet.')
      } else {
        setDiscoveredAccounts(unimported)
        setSelectedDiscoveredEmails(new Set(unimported.map((s) => s.email)))
        setIsScanModalOpen(true)
      }
    } catch (err: any) {
      setSwitchFeedback(`Scan error: ${err.message}`)
    } finally {
      setIsScanning(false)
    }
  }

  const handleToggleSelectAllDiscovered = () => {
    if (selectedDiscoveredEmails.size === discoveredAccounts.length) {
      setSelectedDiscoveredEmails(new Set())
    } else {
      setSelectedDiscoveredEmails(new Set(discoveredAccounts.map((d) => d.email)))
    }
  }

  const handleToggleDiscoveredEmail = (email: string) => {
    const next = new Set(selectedDiscoveredEmails)
    if (next.has(email)) {
      next.delete(email)
    } else {
      next.add(email)
    }
    setSelectedDiscoveredEmails(next)
  }

  const handleImportSelectedDiscovered = async () => {
    if (selectedDiscoveredEmails.size === 0) return
    setIsImportingDiscovered(true)
    try {
      let count = 0
      for (const item of discoveredAccounts) {
        if (selectedDiscoveredEmails.has(item.email)) {
          await api.importAccount({
            email: item.email,
            refresh_token: item.refresh_token || '',
            access_token: item.access_token || '',
            label: item.source ? `Discovered (${item.source})` : 'Discovered Account',
          })
          count++
        }
      }
      setIsScanModalOpen(false)
      setDiscoveredAccounts([])
      setSwitchFeedback(`Successfully imported ${count} account(s) into your fleet.`)
      onRefresh()
    } catch (err: any) {
      setSwitchFeedback(`Import error: ${err.message}`)
    } finally {
      setIsImportingDiscovered(false)
    }
  }

  const handleAddNewAccount = () => {
    setSelectedRowAccount({
      email: '',
      label: '',
      priority: 'High',
      plan_tier: '',
      is_active: false,
      status: 'STANDBY',
      quota_5h_current: 0,
      quota_5h_available: 0,
      quota_weekly: 0,
      reset_horizon_text: '',
      reset_horizon_weekly_text: '',
      has_mfa: false,
    })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* ============================================================== */}
      {/* 1. TOP SECTION: Fleet Overview & Merged Total Quota Gadget     */}
      {/* ============================================================== */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1.2fr 1.2fr',
          gap: '16px',
        }}
      >
        {/* Card A: Switcher Status Fleet Gadget */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                Switcher Status
              </div>

              {/* Auto-Switch Toggle Button */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>
                  Auto-Switch
                </span>
                <ToggleSwitch
                  checked={autoSwitchOn}
                  onChange={handleToggleAutoSwitch}
                  disabled={isTogglingRules}
                  size="sm"
                />
              </div>
            </div>

            <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginBottom: (errorAccountsCount > 0 || bannedAccountsCount > 0) ? '6px' : '8px' }}>
              {fleet?.total_accounts || accounts.length} Accounts Managed
            </div>

            {(errorAccountsCount > 0 || bannedAccountsCount > 0) && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
                {errorAccountsCount > 0 && (
                  <span
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '4px',
                      fontSize: '11px',
                      fontWeight: 600,
                      color: '#b06000',
                      backgroundColor: '#fef7e0',
                      border: '1px solid #feefc3',
                      borderRadius: '12px',
                      padding: '2px 8px',
                    }}
                    title={`${errorAccountsCount} account${errorAccountsCount > 1 ? 's' : ''} with error`}
                  >
                    <AlertCircle size={12} />
                    {errorAccountsCount} {errorAccountsCount > 1 ? 'Errors' : 'Error'}
                  </span>
                )}
                {bannedAccountsCount > 0 && (
                  <span
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '4px',
                      fontSize: '11px',
                      fontWeight: 600,
                      color: '#c5221f',
                      backgroundColor: '#fce8e6',
                      border: '1px solid #fad2cf',
                      borderRadius: '12px',
                      padding: '2px 8px',
                    }}
                    title={`${bannedAccountsCount} account${bannedAccountsCount > 1 ? 's' : ''} banned or suspended`}
                  >
                    <AlertTriangle size={12} />
                    {bannedAccountsCount} Banned
                  </span>
                )}
              </div>
            )}

            {switchFeedback && (
              <div style={{ fontSize: '12px', color: 'var(--primary)', marginTop: '6px', fontWeight: 500, display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                <span>{switchFeedback}</span>
              </div>
            )}
          </div>

          {/* Action Row: Scan Local Accounts, Add Account, and Launch Antigravity Logo Button */}
          <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', marginTop: '16px', gap: '8px', flexWrap: 'wrap' }}>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
              <button
                onClick={handleScanLocalAccounts}
                disabled={isScanning}
                className="btn-pill-outlined"
                style={{
                  width: '172px',
                  height: '32px',
                  padding: '7px 16px',
                  fontSize: '12px',
                  fontWeight: 600,
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: '6px',
                  whiteSpace: 'nowrap',
                  boxSizing: 'border-box',
                }}
                title="Scan system for local Antigravity and Google accounts."
              >
                <Search size={14} />
                {isScanning ? 'Scanning...' : 'Scan Local Accounts'}
              </button>

              <button
                onClick={handleAddNewAccount}
                className="btn-pill-primary"
                style={{
                  width: '172px',
                  height: '32px',
                  padding: '8px 16px',
                  fontSize: '12px',
                  fontWeight: 600,
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: '6px',
                  whiteSpace: 'nowrap',
                  boxSizing: 'border-box',
                }}
                title="Add and configure an account manually."
              >
                <Plus size={14} /> Add Account
              </button>
            </div>

            {/* Antigravity 2.0 Launch Logo Button pinned at bottom-right corner */}
            <div style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', marginLeft: 'auto', alignSelf: 'flex-end' }}>
              <button
                type="button"
                id="btnLaunchAntigravity"
                onClick={handleLaunchAntigravity}
                onMouseEnter={() => setIsLogoHovered(true)}
                onMouseLeave={() => setIsLogoHovered(false)}
                onFocus={() => setIsLogoHovered(true)}
                onBlur={() => setIsLogoHovered(false)}
                disabled={isLaunchingIDE}
                title="Launch Antigravity 2.0"
                aria-label="Launch Antigravity 2.0"
                style={{
                  background: 'none',
                  backgroundColor: 'transparent',
                  border: 'none',
                  outline: 'none',
                  padding: 0,
                  margin: 0,
                  height: '32px',
                  width: '32px',
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  cursor: isLaunchingIDE ? 'wait' : 'pointer',
                  boxShadow: 'none',
                  transform: isLaunchingIDE ? 'scale(0.95)' : isLogoHovered ? 'scale(1.12)' : 'scale(1)',
                  filter: isLogoHovered
                    ? 'drop-shadow(0 2px 6px rgba(66, 133, 244, 0.38)) brightness(1.08)'
                    : 'drop-shadow(0 1px 2px rgba(0, 0, 0, 0.06))',
                  transition: 'transform 0.18s cubic-bezier(0.2, 0, 0, 1), filter 0.18s cubic-bezier(0.2, 0, 0, 1), opacity 0.18s ease',
                  opacity: isLaunchingIDE ? 0.6 : 1,
                }}
              >
                <img
                  src={antigravityLogo}
                  alt="Antigravity 2.0"
                  style={{
                    height: '24px',
                    width: 'auto',
                    aspectRatio: '200 / 184',
                    display: 'block',
                    objectFit: 'contain',
                    pointerEvents: 'none',
                    userSelect: 'none',
                  }}
                />
              </button>

              {/* Hover Tooltip: Launch Antigravity 2.0 */}
              {isLogoHovered && (
                <div
                  role="tooltip"
                  style={{
                    position: 'absolute',
                    bottom: 'calc(100% + 8px)',
                    right: 0,
                    backgroundColor: 'rgba(32, 33, 36, 0.95)',
                    color: '#ffffff',
                    fontSize: '11px',
                    fontWeight: 600,
                    padding: '4px 10px',
                    borderRadius: '6px',
                    border: '1px solid rgba(255, 255, 255, 0.12)',
                    whiteSpace: 'nowrap',
                    pointerEvents: 'none',
                    zIndex: 50,
                    boxShadow: '0 2px 8px rgba(0, 0, 0, 0.25)',
                    animation: 'fadeIn 0.15s ease',
                  }}
                >
                  {isLaunchingIDE ? 'Launching Antigravity 2.0...' : 'Launch Antigravity 2.0'}
                  <div
                    style={{
                      position: 'absolute',
                      top: '100%',
                      right: '11px',
                      width: 0,
                      height: 0,
                      borderLeft: '5px solid transparent',
                      borderRight: '5px solid transparent',
                      borderTop: '5px solid rgba(32, 33, 36, 0.95)',
                    }}
                  />
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Card B: Merged Total Quota Progress Rings */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              marginBottom: rules?.allow_non_gemini_native_models ? '8px' : '12px',
            }}
          >
            <div
              style={{
                fontSize: '11px',
                fontWeight: 700,
                color: 'var(--text-muted)',
                letterSpacing: '0.8px',
                textTransform: 'uppercase',
              }}
            >
              Quota Overview
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              {rules?.allow_non_gemini_native_models && (
                <span
                  style={{
                    fontSize: '10px',
                    fontWeight: 700,
                    backgroundColor: 'rgba(26, 115, 232, 0.1)',
                    color: 'var(--primary)',
                    padding: '2px 8px',
                    borderRadius: '10px',
                    border: '1px solid rgba(26, 115, 232, 0.25)',
                  }}
                >
                  Dual Engine (Gemini + Claude/GPT)
                </span>
              )}
              <button
                onClick={handleRefreshClick}
                className="btn-pill-tonal"
                style={{
                  border: 'none',
                  padding: '5px 8px',
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  cursor: 'pointer',
                }}
                title="Refresh Quota"
              >
                <RotateCw
                  size={13}
                  className={isRefreshing ? 'animate-spin' : ''}
                />
              </button>
            </div>
          </div>

          {rules?.allow_non_gemini_native_models ? (
            /* 2x2 Progress Rings Layout with grey horizontal split line */
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', flex: 1, justifyContent: 'center' }}>
              {/* Row 1: Gemini Models */}
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.8px' }}>
                  Gemini Models
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-around', gap: '12px' }}>
                  <CircularGauge
                    percentage={gauge5hGemini}
                    title="Gemini 5H"
                    size={72}
                    strokeWidth={8}
                  />
                  <CircularGauge
                    percentage={gaugeWeeklyGemini}
                    title="Gemini Weekly"
                    size={72}
                    strokeWidth={8}
                  />
                </div>
              </div>

              {/* Grey horizontal split line in between */}
              <div style={{ height: '1px', backgroundColor: 'var(--border)', width: '100%', margin: '4px 0' }} />

              {/* Row 2: Claude & GPT Models */}
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.8px' }}>
                  Claude &amp; GPT Models
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-around', gap: '12px' }}>
                  <CircularGauge
                    percentage={gauge5hClaude}
                    title="Claude/GPT 5H"
                    size={72}
                    strokeWidth={8}
                  />
                  <CircularGauge
                    percentage={gaugeWeeklyClaude}
                    title="Claude/GPT Weekly"
                    size={72}
                    strokeWidth={8}
                  />
                </div>
              </div>
            </div>
          ) : (
            /* Standard 1x2 Progress Rings */
            <div
              style={{
                flex: 1,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-around',
                gap: '16px',
              }}
            >
              <CircularGauge
                percentage={gauge5h}
                title="5H"
              />
              <CircularGauge
                percentage={gaugeWeekly}
                title="Weekly"
              />
            </div>
          )}
        </div>
      </div>

      {/* ============================================================== */}
      {/* 2. BOTTOM SECTION: All Accounts Table (Whole Page Scrolls) */}
      {/* ============================================================== */}
      <div className="google-card" style={{ padding: '0px', overflow: 'hidden' }}>
        <div
          style={{
            padding: '12px 20px',
            borderBottom: '1px solid var(--border)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Account Fleet
          </div>

          {/* Sort Dropdown at Right End */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
              Sort:
            </span>
            <div style={{ position: 'relative', display: 'inline-flex', alignItems: 'center' }}>
              <select
                value={sortMode}
                onChange={(e) => setSortMode(e.target.value as SortMode)}
                style={{
                  padding: '5px 28px 5px 12px',
                  borderRadius: '8px',
                  border: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  color: 'var(--text)',
                  fontSize: '12px',
                  fontWeight: 600,
                  cursor: 'pointer',
                  appearance: 'none',
                  WebkitAppearance: 'none',
                  outline: 'none',
                }}
              >
                <option value="auto">Auto</option>
                <option value="identity">Account Name</option>
                <option value="credits">Credit</option>
                <option value="priority">Priority</option>
                <option value="quota_5h">5-Hour Quota</option>
                <option value="quota_weekly">Weekly Quota</option>
              </select>
              <ArrowUpDown
                size={13}
                style={{
                  position: 'absolute',
                  right: '10px',
                  pointerEvents: 'none',
                  color: 'var(--text-muted)',
                }}
              />
            </div>
          </div>
        </div>

        <div style={{ overflowX: 'auto', width: '100%' }}>
          <table style={{ width: '100%', minWidth: `${TABLE_MIN_WIDTH}px`, borderCollapse: 'collapse', tableLayout: 'fixed' }}>
            <thead>
              <tr>
                <th
                  onClick={() => setSortMode('identity')}
                  style={{
                    width: '244px',
                    minWidth: '244px',
                    padding: '12px 16px',
                    textAlign: 'left',
                    fontSize: '11px',
                    fontWeight: 600,
                    color: sortMode === 'identity' ? 'var(--primary)' : 'var(--text-muted)',
                    borderBottom: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                    textTransform: 'uppercase',
                    letterSpacing: '0.5px',
                    cursor: 'pointer',
                    userSelect: 'none',
                  }}
                  title="Click to sort by Account Name"
                >
                <div style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span>Account</span>
                  {sortMode === 'identity' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th style={{ width: '80px', minWidth: '80px', padding: '12px 8px', textAlign: 'center', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Plan
              </th>
              <th
                onClick={() => setSortMode('quota_5h')}
                style={{
                  width: '156px',
                  minWidth: '156px',
                  padding: '12px 12px',
                  textAlign: 'left',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: sortMode === 'quota_5h' ? 'var(--primary)' : 'var(--text-muted)',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.5px',
                  cursor: 'pointer',
                  userSelect: 'none',
                }}
                title="Click to sort by 5-Hour Quota"
              >
                <div style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span>5-Hour Quota</span>
                  {sortMode === 'quota_5h' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th
                onClick={() => setSortMode('quota_weekly')}
                style={{
                  width: '156px',
                  minWidth: '156px',
                  padding: '12px 12px',
                  textAlign: 'left',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: sortMode === 'quota_weekly' ? 'var(--primary)' : 'var(--text-muted)',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.5px',
                  cursor: 'pointer',
                  userSelect: 'none',
                }}
                title="Click to sort by Weekly Quota"
              >
                <div style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span>Weekly Quota</span>
                  {sortMode === 'quota_weekly' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th
                onClick={() => setSortMode('credits')}
                style={{
                  width: '76px',
                  minWidth: '76px',
                  padding: '12px 8px',
                  textAlign: 'center',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: sortMode === 'credits' ? 'var(--primary)' : 'var(--text-muted)',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.5px',
                  cursor: 'pointer',
                  userSelect: 'none',
                }}
                title="Click to sort by Available Credit"
              >
                <div style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}>
                  <span>Credit</span>
                  {sortMode === 'credits' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th
                onClick={() => setSortMode('priority')}
                style={{
                  width: '76px',
                  minWidth: '76px',
                  padding: '12px 8px',
                  textAlign: 'center',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: sortMode === 'priority' ? 'var(--primary)' : 'var(--text-muted)',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.5px',
                  cursor: 'pointer',
                  userSelect: 'none',
                }}
                title="Click to sort by Priority"
              >
                <div style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}>
                  <span>Priority</span>
                  {sortMode === 'priority' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th style={{ width: '130px', minWidth: '130px', padding: '12px 16px', textAlign: 'center', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Action
              </th>
            </tr>
          </thead>
          <tbody>
            {sortedAccounts.map((acc, index) => {
              const accEmailLower = acc.email.toLowerCase()
              const usingApps = isIndividualMode
                ? installedAppsList.filter((app) => {
                    if (activeAppMap[app.key]) {
                      return activeAppMap[app.key].toLowerCase() === accEmailLower
                    }
                    if (acc.active_apps && acc.active_apps.length > 0) {
                      return acc.active_apps.includes(app.key)
                    }
                    return accEmailLower === activeAccount.toLowerCase()
                  })
                : []

              const isRowActive = isIndividualMode
                ? usingApps.length > 0
                : (activeAccount ? accEmailLower === activeAccount.toLowerCase() : Boolean(acc.is_active))

              let activeBadgeText = 'Active'
              if (isIndividualMode && usingApps.length > 0) {
                if (usingApps.length === installedAppsList.length) {
                  activeBadgeText = 'All'
                } else if (usingApps.length === 1) {
                  activeBadgeText = usingApps[0].label
                } else if (usingApps.length === 2) {
                  activeBadgeText = `${usingApps[0].shortLabel} & ${usingApps[1].shortLabel}`
                } else {
                  activeBadgeText = usingApps.map((u) => u.shortLabel).join(' & ')
                }
              }

              const isActive = isRowActive
              const current5h = acc.quota_5h_current ?? acc.quota_5h_available ?? 0
              const isHealthy = current5h > threshold && (acc.quota_weekly ?? 0) > thresholdWeekly && !acc.status?.toUpperCase().includes('NEEDS_REAUTH') && !acc.status?.toUpperCase().includes('ERROR') && !acc.status?.toUpperCase().includes('BANNED') && Boolean(acc.refresh_token?.trim())
              const isNextSwitch = autoSwitchOn && sortMode === 'auto' && !isActive && !acc.status?.toUpperCase().includes('BANNED') && !acc.status?.toUpperCase().includes('ERROR') && !acc.status?.toUpperCase().includes('COOLDOWN') && !acc.status?.toUpperCase().includes('COOLING') && !acc.status?.toUpperCase().includes('NEEDS_REAUTH') && Boolean(acc.refresh_token?.trim()) && index === 1 && isHealthy
              return (
                <tr
                  key={acc.email}
                  onClick={() => setSelectedRowAccount(acc)}
                  onContextMenu={(e) => {
                    e.preventDefault()
                    e.stopPropagation()
                    setContextMenu({ x: e.clientX, y: e.clientY, account: acc })
                  }}
                  style={{
                    cursor: 'pointer',
                    transition: 'background-color 0.15s ease',
                    borderBottom: '1px solid var(--border-subtle)',
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--canvas)')}
                  onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                >
                  <td style={{ padding: '12px 16px', overflow: 'hidden' }}>
                    {(() => {
                      const display = getAccountTableDisplay(acc.label, acc.email)
                      return (
                        <>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <span style={{ fontWeight: 600, color: 'var(--text)', fontSize: '13px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                              {display.primaryText}
                            </span>
                            {isNextSwitch && (
                              <span
                                style={{
                                  fontSize: '10px',
                                  fontWeight: 700,
                                  color: '#137333',
                                  backgroundColor: '#e6f4ea',
                                  border: '1px solid #ceead6',
                                  padding: '1px 6px',
                                  borderRadius: '10px',
                                  flexShrink: 0,
                                }}
                                title="Next account in continuous rotation queue"
                              >
                                Next
                              </span>
                            )}
                          </div>
                          {display.secondaryText && (
                            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                              {display.secondaryText}
                            </div>
                          )}
                        </>
                      )
                    })()}
                  </td>

                  <td style={{ padding: '12px 8px', textAlign: 'center' }}>
                    {renderPlanTierBadge(acc.plan_tier)}
                  </td>

                  <td style={{ padding: '12px', verticalAlign: 'middle' }}>
                    {(() => {
                      const avail5h = acc.quota_5h_available ?? current5h
                      const hasDiff = Math.abs(avail5h - current5h) > 0.001
                      const tooltip5h = hasDiff
                        ? `${acc.reset_horizon_text || 'Resets in 5h cycle'} (Projected next 5h: ${Math.round(avail5h * 100)}%)`
                        : (acc.reset_horizon_text || 'Resets in 5h cycle')
                      const tooltip5hGemini = hasDiff
                        ? `${acc.reset_horizon_text || 'Gemini 5h cycle'} (Projected next 5h: ${Math.round(avail5h * 100)}%)`
                        : (acc.reset_horizon_text || 'Gemini 5h cycle')

                      if (rules?.allow_non_gemini_native_models) {
                        return (
                          <div style={{ display: 'flex', flexDirection: 'column', gap: '5px' }}>
                            <div>
                              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '10px', color: 'var(--text-muted)', marginBottom: '1px', fontWeight: 600 }}>
                                <span>Gemini</span>
                                <span style={{ color: current5h <= 0.10 ? 'var(--red)' : 'inherit' }}>{Math.round(current5h * 100)}%</span>
                              </div>
                              <HorizontalQuotaBar
                                fraction={current5h}
                                title={tooltip5hGemini}
                              />
                            </div>
                            <div>
                              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '10px', color: 'var(--text-muted)', marginBottom: '1px', fontWeight: 600 }}>
                                <span>Claude &amp; GPT</span>
                                <span>{acc.quota_5h_claude_gpt !== undefined && acc.quota_5h_claude_gpt !== null ? `${Math.round(acc.quota_5h_claude_gpt * 100)}%` : '0%'}</span>
                              </div>
                              <HorizontalQuotaBar
                                fraction={acc.quota_5h_claude_gpt ?? 0}
                                title="Claude/GPT 5h cycle"
                              />
                            </div>
                          </div>
                        )
                      }
                      return (
                        <HorizontalQuotaBar
                          fraction={current5h}
                          title={tooltip5h}
                        />
                      )
                    })()}
                  </td>

                  <td style={{ padding: '12px', verticalAlign: 'middle' }}>
                    {rules?.allow_non_gemini_native_models ? (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '5px' }}>
                        <div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '10px', color: 'var(--text-muted)', marginBottom: '1px', fontWeight: 600 }}>
                            <span>Gemini</span>
                            <span>{Math.round((acc.quota_weekly ?? 0) * 100)}%</span>
                          </div>
                          <HorizontalQuotaBar
                            fraction={acc.quota_weekly ?? 0}
                            title={acc.reset_horizon_weekly_text || 'Gemini 7-day rolling cycle'}
                          />
                        </div>
                        <div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '10px', color: 'var(--text-muted)', marginBottom: '1px', fontWeight: 600 }}>
                            <span>Claude &amp; GPT</span>
                            <span>{acc.quota_weekly_claude_gpt !== undefined && acc.quota_weekly_claude_gpt !== null ? `${Math.round(acc.quota_weekly_claude_gpt * 100)}%` : '0%'}</span>
                          </div>
                          <HorizontalQuotaBar
                            fraction={acc.quota_weekly_claude_gpt ?? 0}
                            title="Claude/GPT 7-day cycle"
                          />
                        </div>
                      </div>
                    ) : (
                      <HorizontalQuotaBar
                        fraction={acc.quota_weekly ?? 0}
                        title={acc.reset_horizon_weekly_text || '7-day rolling cycle'}
                      />
                    )}
                  </td>

                  <td style={{ padding: '12px 8px', textAlign: 'center' }}>
                    <div style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '6px' }}>
                      <span
                        style={{
                          fontWeight: acc.enable_credit_overages ? 600 : 500,
                          fontSize: '13px',
                          color: acc.enable_credit_overages ? 'var(--text)' : 'var(--text-subtle)',
                        }}
                        title={acc.enable_credit_overages ? 'AI Credits usage enabled' : 'Credit usage disabled (do not use credits)'}
                      >
                        {((acc.credits !== undefined && acc.credits !== null) ? Number(acc.credits) : 0)}
                      </span>
                      {acc.enable_credit_overages ? (
                        <span
                          style={{
                            fontSize: '10px',
                            fontWeight: 700,
                            padding: '1px 5px',
                            borderRadius: '4px',
                            backgroundColor: '#ecfdf5',
                            color: '#059669',
                            border: '1px solid #a7f3d0',
                          }}
                          title="AI Credit Overages Enabled"
                        >
                          +Overage
                        </span>
                      ) : null}
                    </div>
                  </td>

                  <td style={{ padding: '12px 8px', textAlign: 'center' }}>
                    {renderPriorityBadge(acc.priority)}
                  </td>

                  <td style={{ padding: '12px 16px', textAlign: 'center' }}>
                    {acc.status?.toUpperCase() === 'BANNED' ? (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setErrorDetailAccount(acc)
                        }}
                        style={{
                          padding: '4px 10px',
                          borderRadius: '12px',
                          fontSize: '11px',
                          fontWeight: 700,
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '4px',
                          color: '#c5221f',
                          backgroundColor: '#fce8e6',
                          border: '1px solid #fad2cf',
                          cursor: 'pointer',
                        }}
                        title="Account suspended or banned (Click to view details)"
                      >
                        <AlertTriangle size={12} /> BANNED
                      </button>
                    ) : acc.status?.toUpperCase() === 'ERROR' || acc.status?.toUpperCase() === 'NEEDS_REAUTH' || !acc.refresh_token?.trim() ? (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setErrorDetailAccount(acc)
                        }}
                        style={{
                          padding: '4px 10px',
                          borderRadius: '12px',
                          fontSize: '11px',
                          fontWeight: 700,
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '4px',
                          color: '#b06000',
                          backgroundColor: '#fef7e0',
                          border: '1px solid #feefc3',
                          cursor: 'pointer',
                        }}
                        title="Authentication or verification error (Click to view details)"
                      >
                        <AlertCircle size={12} /> ERROR
                      </button>
                    ) : (() => {
                      const renderSwitchControl = () => {
                        if (isIndividualMode) {
                          const isMenuOpen = switchMenuEmail === acc.email
                          return (
                            <div style={{ position: 'relative', display: 'inline-block' }}>
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  setSwitchMenuEmail((prev) => (prev === acc.email ? null : acc.email))
                                }}
                                className="btn-pill-tonal"
                                style={{
                                  padding: '4px 10px',
                                  fontSize: '11px',
                                  display: 'inline-flex',
                                  alignItems: 'center',
                                  gap: '4px',
                                  whiteSpace: 'nowrap',
                                }}
                                title="Select target application to switch"
                              >
                                <ArrowRightLeft size={11} /> Switch <ChevronDown size={10} />
                              </button>
                              {isMenuOpen && (
                                <div
                                  onClick={(e) => e.stopPropagation()}
                                  style={{
                                    position: 'absolute',
                                    right: 0,
                                    top: 'calc(100% + 4px)',
                                    backgroundColor: 'var(--surface)',
                                    border: '1px solid var(--border)',
                                    borderRadius: '6px',
                                    boxShadow: '0 4px 14px rgba(0,0,0,0.12)',
                                    zIndex: 50,
                                    minWidth: '130px',
                                    display: 'flex',
                                    flexDirection: 'column',
                                    padding: '4px',
                                    gap: '2px',
                                  }}
                                >
                                  <button
                                    type="button"
                                    onClick={async (e) => {
                                      e.stopPropagation()
                                      setSwitchMenuEmail(null)
                                      try {
                                        await api.switchAccount(acc.email, true, 'all')
                                        setSwitchFeedback(null)
                                        onRefresh()
                                        setTimeout(onRefresh, 3000)
                                      } catch (err: any) {
                                        setSwitchFeedback('Switch failed: ' + err.message)
                                      }
                                    }}
                                    style={{
                                      display: 'flex',
                                      alignItems: 'center',
                                      gap: '6px',
                                      padding: '6px 10px',
                                      borderRadius: '4px',
                                      border: 'none',
                                      backgroundColor: 'transparent',
                                      color: 'var(--text)',
                                      fontSize: '11px',
                                      fontWeight: 600,
                                      cursor: 'pointer',
                                      textAlign: 'left',
                                      width: '100%',
                                      whiteSpace: 'nowrap',
                                    }}
                                    onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal, rgba(0,0,0,0.05))')}
                                    onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                                  >
                                    <Layers size={12} color="var(--primary)" />
                                    <span>All</span>
                                  </button>
                                  <div style={{ height: '1px', backgroundColor: 'var(--border)', margin: '2px 0' }} />
                                  {installedAppsList.map((app) => (
                                    <button
                                      key={app.target}
                                      type="button"
                                      onClick={async (e) => {
                                        e.stopPropagation()
                                        setSwitchMenuEmail(null)
                                        try {
                                          const shouldRelaunch = app.target === 'desktop'
                                          await api.switchAccount(acc.email, shouldRelaunch, app.target)
                                          setSwitchFeedback(null)
                                          onRefresh()
                                          setTimeout(onRefresh, 3000)
                                        } catch (err: any) {
                                          setSwitchFeedback('Switch failed: ' + err.message)
                                        }
                                      }}
                                      style={{
                                        display: 'flex',
                                        alignItems: 'center',
                                        gap: '6px',
                                        padding: '6px 10px',
                                        borderRadius: '4px',
                                        border: 'none',
                                        backgroundColor: 'transparent',
                                        color: 'var(--text)',
                                        fontSize: '11px',
                                        fontWeight: 500,
                                        cursor: 'pointer',
                                        textAlign: 'left',
                                        width: '100%',
                                        whiteSpace: 'nowrap',
                                      }}
                                      onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal, rgba(0,0,0,0.05))')}
                                      onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                                    >
                                      {app.icon}
                                      <span>{app.label}</span>
                                    </button>
                                  ))}
                                </div>
                              )}
                            </div>
                          )
                        }

                        return (
                          <button
                            onClick={async (e) => {
                              e.stopPropagation()
                              try {
                                await api.switchAccount(acc.email, true)
                                setSwitchFeedback(null)
                                onRefresh()
                                setTimeout(onRefresh, 3000)
                              } catch (err: any) {
                                setSwitchFeedback('Switch failed: ' + err.message)
                              }
                            }}
                            className="btn-pill-tonal"
                            style={{
                              padding: '4px 12px',
                              fontSize: '11px',
                              display: 'inline-flex',
                              alignItems: 'center',
                              gap: '4px',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            <ArrowRightLeft size={11} /> Switch
                          </button>
                        )
                      }

                      if (isRowActive) {
                        return (
                          <div style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                            {!isHealthy ? (
                              <span
                                className="badge-chip"
                                style={{
                                  fontSize: '11px',
                                  padding: '4px 10px',
                                  display: 'inline-flex',
                                  alignItems: 'center',
                                  gap: '4px',
                                  color: '#b06000',
                                  backgroundColor: '#fef7e0',
                                  border: '1px solid #feefc3',
                                  fontWeight: 600,
                                }}
                                title={
                                  (acc.quota_weekly ?? 0) <= thresholdWeekly
                                    ? 'Active account 7-day (weekly) quota is depleted below threshold'
                                    : 'Active account 5-hour quota is depleted below threshold'
                                }
                              >
                                <AlertTriangle size={12} />{' '}
                                {isIndividualMode
                                  ? `${activeBadgeText} (${(acc.quota_weekly ?? 0) <= thresholdWeekly ? 'Weekly Low' : 'Quota Low'})`
                                  : ((acc.quota_weekly ?? 0) <= thresholdWeekly ? 'Active (Weekly Low)' : 'Active (Quota Low)')}
                              </span>
                            ) : (
                              <span
                                className="badge-chip badge-green"
                                style={{
                                  fontSize: '11px',
                                  padding: '4px 10px',
                                  display: 'inline-flex',
                                  alignItems: 'center',
                                  gap: '4px',
                                }}
                              >
                                <CheckCircle2 size={12} /> {isIndividualMode ? activeBadgeText : 'Active'}
                              </span>
                            )}
                            {isIndividualMode && usingApps.length < installedAppsList.length && renderSwitchControl()}
                          </div>
                        )
                      }

                      return (
                        <div style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                          {(acc.status?.toUpperCase() === 'COOLDOWN' || acc.status?.toUpperCase() === 'COOLING') ? (
                            <span
                              className="badge-chip"
                              style={{
                                fontSize: '11px',
                                padding: '3px 8px',
                                display: 'inline-flex',
                                alignItems: 'center',
                                gap: '4px',
                                color: '#1a73e8',
                                backgroundColor: '#e8f0fe',
                                border: '1px solid #d2e3fc',
                                fontWeight: 600,
                                cursor: 'default',
                              }}
                              title="Quota exhausted below threshold; cooling until reset."
                            >
                              <Timer size={11} /> Cooling
                            </span>
                          ) : (
                            renderSwitchControl()
                          )}
                        </div>
                      )
                    })()}
                  </td>
                </tr>
              )
            })}
            {sortedAccounts.length === 0 && (
              <tr>
                <td colSpan={6} style={{ textAlign: 'center', padding: '32px 16px', color: 'var(--text-muted)', fontSize: '13px' }}>
                  No accounts configured. Scan local accounts or click Add Account to begin.
                </td>
              </tr>
            )}
          </tbody>
        </table>
        </div>
      </div>

      {/* Account Detail Modal Window */}
      {selectedRowAccount && (
        <AccountDetailModal
          account={selectedRowAccount}
          onClose={() => setSelectedRowAccount(null)}
          onSaved={async () => {
            onRefresh()
          }}
        />
      )}

      {/* Error / Ban Details Modal Window */}
      {errorDetailAccount && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.45)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            backdropFilter: 'blur(2px)',
          }}
          onClick={() => setErrorDetailAccount(null)}
        >
          <div
            style={{
              backgroundColor: '#ffffff',
              borderRadius: '10px',
              width: '460px',
              maxWidth: '90vw',
              padding: '24px',
              boxShadow: 'var(--shadow-lg)',
              border: '1px solid var(--border)',
              display: 'flex',
              flexDirection: 'column',
              gap: '16px',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                {errorDetailAccount.status?.toUpperCase() === 'BANNED' ? (
                  <AlertTriangle size={20} color="#c5221f" />
                ) : (
                  <AlertCircle size={20} color="#b06000" />
                )}
                <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                  {errorDetailAccount.status?.toUpperCase() === 'BANNED'
                    ? 'Account Suspended / Banned'
                    : 'Account Error Details'}
                </h3>
              </div>
              <button
                onClick={() => setErrorDetailAccount(null)}
                style={{
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  color: 'var(--text-muted)',
                  padding: '4px',
                  display: 'flex',
                  alignItems: 'center',
                  borderRadius: '50%',
                }}
                title="Close"
              >
                <X size={18} />
              </button>
            </div>

            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
                {errorDetailAccount.label
                  ? `${errorDetailAccount.label} (${errorDetailAccount.email})`
                  : errorDetailAccount.email}
              </div>
              {(() => {
                const detailAlert = parseAccountErrorAlert(
                  errorDetailAccount.error_message || errorDetailAccount.status_reason,
                  errorDetailAccount.email
                )
                return (
                  <div
                    style={{
                      backgroundColor:
                        errorDetailAccount.status?.toUpperCase() === 'BANNED' ? '#fce8e6' : '#fef7e0',
                      color:
                        errorDetailAccount.status?.toUpperCase() === 'BANNED' ? '#c5221f' : '#b06000',
                      padding: '12px 16px',
                      borderRadius: '10px',
                      fontSize: '12px',
                      lineHeight: 1.5,
                      wordBreak: 'break-word',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: '8px',
                    }}
                  >
                    <div style={{ fontFamily: 'monospace' }}>
                      {errorDetailAccount.error_message || errorDetailAccount.status_reason
                        ? detailAlert.summaryText
                        : errorDetailAccount.status?.toUpperCase() === 'BANNED'
                        ? 'This account has been flagged or suspended by Google Antigravity services. Quota requests cannot be serviced.'
                        : 'Authentication failure or token expired. Please re-authenticate or update credentials.'}
                    </div>
                    {detailAlert.verificationUrl && (
                      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '8px', marginTop: '2px' }}>
                        <a
                          href={detailAlert.verificationUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          onClick={async (e) => {
                            setIsVerifyingErrorAccount(true)
                            const isStale = Date.now() - errorDetailFetchedAtRef.current > 45_000
                            const electronOpen = (window as any).electronAPI?.openExternal
                            if (electronOpen || isStale) {
                              e.preventDefault()
                              let freshUrl = detailAlert.verificationUrl
                              if (isStale) {
                                const msg = await pollErrorDetailAccount(errorDetailAccount.email, true)
                                if (!msg) return
                                const parsed = parseAccountErrorAlert(msg, errorDetailAccount.email)
                                if (parsed.verificationUrl) {
                                  freshUrl = parsed.verificationUrl
                                }
                              }
                              if (!freshUrl) return
                              if (electronOpen) {
                                electronOpen(freshUrl)
                              } else {
                                window.open(freshUrl, '_blank', 'noopener,noreferrer')
                              }
                            }
                          }}
                          style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '6px',
                            padding: '6px 12px',
                            borderRadius: '9999px',
                            backgroundColor: '#1a73e8',
                            color: '#ffffff',
                            fontSize: '12px',
                            fontWeight: 600,
                            textDecoration: 'none',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          <ExternalLink size={12} />
                          <span>Verify Account in Browser</span>
                        </a>
                        {isVerifyingErrorAccount && (
                          <span
                            style={{
                              display: 'inline-flex',
                              alignItems: 'center',
                              gap: '6px',
                              fontSize: '12px',
                              fontWeight: 500,
                              color: '#1a73e8',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            <RotateCw size={12} className="spin" />
                            <span>Waiting for browser verification...</span>
                          </span>
                        )}
                      </div>
                    )}
                  </div>
                )
              })()}
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '4px' }}>
              <button
                onClick={() => setErrorDetailAccount(null)}
                className="btn-pill-outlined"
                style={{ padding: '6px 16px', fontSize: '12px' }}
              >
                Dismiss
              </button>
              <button
                onClick={() => {
                  const target = errorDetailAccount
                  setErrorDetailAccount(null)
                  setSelectedRowAccount(target)
                }}
                className="btn-pill-primary"
                style={{ padding: '6px 16px', fontSize: '12px' }}
              >
                Open Account Setup
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Right-Click Context Menu */}
      {contextMenu && (
        <div
          style={{
            position: 'fixed',
            top: contextMenu.y,
            left: contextMenu.x,
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            boxShadow: 'var(--shadow-md)',
            zIndex: 2000,
            padding: '6px 0',
            minWidth: '200px',
          }}
          onClick={(e) => e.stopPropagation()}
        >
          <div
            style={{
              padding: '6px 16px',
              fontSize: '11px',
              fontWeight: 700,
              color: 'var(--text-muted)',
              borderBottom: '1px solid var(--border-subtle)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {contextMenu.account.email}
          </div>
          <button
            onClick={() => {
              setSelectedRowAccount(contextMenu.account)
              setContextMenu(null)
            }}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              width: '100%',
              padding: '8px 16px',
              fontSize: '12px',
              textAlign: 'left',
              color: 'var(--text)',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
            onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
          >
            <Edit2 size={13} /> Edit Account Details
          </button>
          {!contextMenu.account.status?.trim().toUpperCase().includes('BANNED') &&
            !contextMenu.account.status?.trim().toUpperCase().includes('COOLDOWN') &&
            !contextMenu.account.status?.trim().toUpperCase().includes('COOLING') && (
            isIndividualMode ? (
              <>
                <button
                  onClick={async () => {
                    const target = contextMenu.account.email
                    setContextMenu(null)
                    try {
                      await api.switchAccount(target, true, 'all')
                      setSwitchFeedback(null)
                      onRefresh()
                      setTimeout(onRefresh, 3000)
                    } catch (err: any) {
                      setSwitchFeedback('Switch failed: ' + err.message)
                    }
                  }}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    width: '100%',
                    padding: '8px 16px',
                    fontSize: '12px',
                    textAlign: 'left',
                    color: 'var(--primary)',
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                  onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                >
                  <Layers size={13} /> Switch All to this Account
                </button>
                {installedAppsList.map((app) => (
                  <button
                    key={app.target}
                    onClick={async () => {
                      const target = contextMenu.account.email
                      setContextMenu(null)
                      try {
                        const shouldRelaunch = app.target === 'desktop'
                        await api.switchAccount(target, shouldRelaunch, app.target)
                        setSwitchFeedback(null)
                        onRefresh()
                        setTimeout(onRefresh, 3000)
                      } catch (err: any) {
                        setSwitchFeedback('Switch failed: ' + err.message)
                      }
                    }}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '8px',
                      width: '100%',
                      padding: '8px 16px',
                      fontSize: '12px',
                      textAlign: 'left',
                      color: 'var(--text)',
                    }}
                    onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                    onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                  >
                    {app.icon} Switch {app.label} to this Account
                  </button>
                ))}
              </>
            ) : (
              !contextMenu.account.is_active &&
              contextMenu.account.email !== activeAccount && (
                <button
                  onClick={async () => {
                    const target = contextMenu.account.email
                    setContextMenu(null)
                    try {
                      await api.switchAccount(target, true)
                      setSwitchFeedback(null)
                      onRefresh()
                      setTimeout(onRefresh, 3000)
                    } catch (err: any) {
                      setSwitchFeedback('Switch failed: ' + err.message)
                    }
                  }}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    width: '100%',
                    padding: '8px 16px',
                    fontSize: '12px',
                    textAlign: 'left',
                    color: 'var(--primary)',
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                  onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                >
                  <ArrowRightLeft size={13} /> Switch to this Account
                </button>
              )
            )
          )}
          <button
            onClick={() => {
              navigator.clipboard.writeText(contextMenu.account.email)
              setContextMenu(null)
            }}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              width: '100%',
              padding: '8px 16px',
              fontSize: '12px',
              textAlign: 'left',
              color: 'var(--text)',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
            onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
          >
            <Copy size={13} /> Copy Email Address
          </button>
          <div style={{ height: '1px', backgroundColor: 'var(--border)', margin: '4px 0' }} />
          <button
            onClick={async () => {
              const target = contextMenu.account.email
              const isActive = contextMenu.account.is_active || target === activeAccount
              setContextMenu(null)
              const msg = isActive
                ? `This account is currently ACTIVE. Removing it will switch to another available account in your fleet.\n\nAre you sure you want to remove ${target}?`
                : `Are you sure you want to remove account ${target}?`
              if (window.confirm(msg)) {
                try {
                  await api.deleteAccount(target)
                  onRefresh()
                } catch (err: any) {
                  setSwitchFeedback('Failed to remove account: ' + err.message)
                }
              }
            }}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              width: '100%',
              padding: '8px 16px',
              fontSize: '12px',
              textAlign: 'left',
              color: '#d93025',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = '#fdf2f2')}
            onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
          >
            <Trash2 size={13} /> Remove Account
          </button>
        </div>
      )}

      {/* Discovered Accounts Popup Modal */}
      {isScanModalOpen && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.45)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            backdropFilter: 'blur(2px)',
          }}
          onClick={() => !isImportingDiscovered && setIsScanModalOpen(false)}
        >
          <div
            style={{
              backgroundColor: '#ffffff',
              borderRadius: '10px',
              width: '540px',
              maxWidth: '92vw',
              maxHeight: '85vh',
              padding: '24px',
              boxShadow: 'var(--shadow-lg)',
              border: '1px solid var(--border)',
              display: 'flex',
              flexDirection: 'column',
              gap: '16px',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Search size={20} color="#1a73e8" />
                <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                  Discovered Local Accounts ({discoveredAccounts.length})
                </h3>
              </div>
              <button
                onClick={() => setIsScanModalOpen(false)}
                disabled={isImportingDiscovered}
                style={{
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  color: 'var(--text-muted)',
                  padding: '4px',
                  display: 'flex',
                  alignItems: 'center',
                }}
              >
                <X size={18} />
              </button>
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '12px' }}>
              <button
                onClick={handleToggleSelectAllDiscovered}
                style={{
                  background: 'none',
                  border: 'none',
                  color: 'var(--primary)',
                  fontWeight: 600,
                  cursor: 'pointer',
                  padding: 0,
                }}
              >
                {selectedDiscoveredEmails.size === discoveredAccounts.length ? 'Deselect All' : 'Select All'}
              </button>
              <span style={{ color: 'var(--text-muted)', fontWeight: 500 }}>
                {selectedDiscoveredEmails.size} of {discoveredAccounts.length} selected
              </span>
            </div>

            {/* Scrollable list of accounts */}
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '8px',
                overflowY: 'auto',
                maxHeight: '340px',
                paddingRight: '4px',
              }}
            >
              {discoveredAccounts.map((item) => {
                const isSelected = selectedDiscoveredEmails.has(item.email)
                return (
                  <div
                    key={item.email}
                    onClick={() => handleToggleDiscoveredEmail(item.email)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '10px 16px',
                      borderRadius: '8px',
                      border: `1px solid ${isSelected ? 'var(--primary)' : 'var(--border)'}`,
                      backgroundColor: isSelected ? 'rgba(26, 115, 232, 0.04)' : 'var(--card-bg, #ffffff)',
                      cursor: 'pointer',
                      transition: 'all 0.15s ease',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => {}}
                        style={{ accentColor: '#1a73e8', cursor: 'pointer' }}
                      />
                      <div>
                        <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                          {item.email}
                        </div>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', gap: '8px', marginTop: '2px', flexWrap: 'wrap' }}>
                          <span>Source: {item.source || 'Local IDE'}</span>
                          {item.has_refresh && <span style={{ color: '#137333', fontWeight: 500 }}>• Refresh Token Available</span>}
                          {item.has_tokens && !item.has_refresh && <span>• OAuth Token</span>}
                        </div>
                      </div>
                    </div>

                    {item.is_active_in_ide && (
                      <span
                        style={{
                          fontSize: '10px',
                          fontWeight: 700,
                          padding: '2px 8px',
                          borderRadius: '10px',
                          backgroundColor: 'rgba(19, 115, 51, 0.1)',
                          color: '#137333',
                        }}
                      >
                        Active in IDE
                      </span>
                    )}
                  </div>
                )
              })}
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '8px' }}>
              <button
                onClick={() => setIsScanModalOpen(false)}
                disabled={isImportingDiscovered}
                className="btn-pill-tonal"
              >
                Cancel
              </button>
              <button
                onClick={handleImportSelectedDiscovered}
                disabled={isImportingDiscovered || selectedDiscoveredEmails.size === 0}
                className="btn-pill-primary"
              >
                {isImportingDiscovered
                  ? 'Importing...'
                  : `Import ${selectedDiscoveredEmails.size} Account(s)`}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
