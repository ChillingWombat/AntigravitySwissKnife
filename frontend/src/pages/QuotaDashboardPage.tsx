import React, { useState, useEffect } from 'react'
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
} from 'lucide-react'
import type { AccountState, FleetQuotaSummary, RuleConfig, DiscoveredAccount } from '../types'
import { CircularGauge } from '../components/CircularGauge'
import { HorizontalQuotaBar } from '../components/HorizontalQuotaBar'
import { AccountDetailModal } from '../components/AccountDetailModal'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'

export type SortMode = 'auto' | 'identity' | 'priority' | 'quota_5h' | 'quota_weekly' | 'credits'

export function sortAccounts(
  accounts: AccountState[],
  activeEmail: string,
  threshold: number,
  mode: SortMode
): AccountState[] {
  const copy = [...accounts]

  if (mode === 'identity') {
    return copy.sort((a, b) => {
      const nameA = (a.label || a.email).toLowerCase()
      const nameB = (b.label || b.email).toLowerCase()
      if (nameA !== nameB) {
        return nameA.localeCompare(nameB)
      }
      return a.email.localeCompare(b.email)
    })
  }

  if (mode === 'priority') {
    const order: Record<string, number> = { High: 0, Mid: 1, Low: 2 }
    return copy.sort((a, b) => {
      const pa = order[a.priority || 'High'] ?? 1
      const pb = order[b.priority || 'High'] ?? 1
      if (pa !== pb) return pa - pb
      const diff = (b.quota_5h_available ?? 0) - (a.quota_5h_available ?? 0)
      if (Math.abs(diff) > 0.0001) return diff
      return (b.quota_weekly ?? 0) - (a.quota_weekly ?? 0)
    })
  }

  if (mode === 'credits') {
    return copy.sort((a, b) => {
      const credA = (a.credits !== undefined && a.credits !== null) ? Number(a.credits) : 0
      const credB = (b.credits !== undefined && b.credits !== null) ? Number(b.credits) : 0
      return credB - credA
    })
  }

  if (mode === 'quota_5h') {
    return copy.sort((a, b) => {
      const diff = (b.quota_5h_available ?? 0) - (a.quota_5h_available ?? 0)
      if (Math.abs(diff) > 0.0001) return diff
      return (b.quota_weekly ?? 0) - (a.quota_weekly ?? 0)
    })
  }

  if (mode === 'quota_weekly') {
    return copy.sort((a, b) => {
      const diff = (b.quota_weekly ?? 0) - (a.quota_weekly ?? 0)
      if (Math.abs(diff) > 0.0001) return diff
      return (b.quota_5h_available ?? 0) - (a.quota_5h_available ?? 0)
    })
  }

  // mode === 'auto' (Default)
  // 1. Row 1: Active account (unconditionally at top)
  // 2. Row 2+: Next standby accounts to rotate into, arranged by 5H & weekly capacity
  // 3. Last rows: Switched-off or cooling-down accounts below threshold
  // 4. End: Broken (ERROR/BANNED) accounts
  return copy.sort((a, b) => {
    const isActA = a.is_active || a.email === activeEmail
    const isActB = b.is_active || b.email === activeEmail

    // Unconditionally pin the active account to Row 1 (index 0)
    if (isActA && !isActB) return -1
    if (!isActA && isActB) return 1

    const stA = (a.status || '').toUpperCase()
    const stB = (b.status || '').toUpperCase()
    const isBannedA = stA === 'BANNED'
    const isBannedB = stB === 'BANNED'
    const isErrorA = stA === 'ERROR'
    const isErrorB = stB === 'ERROR'
    const isBrokenA = isBannedA || isErrorA
    const isBrokenB = isBannedB || isErrorB

    const q5hA = a.quota_5h_available ?? 0
    const q5hB = b.quota_5h_available ?? 0
    const qWkA = a.quota_weekly ?? 0
    const qWkB = b.quota_weekly ?? 0

    const isBelowA = q5hA <= threshold || qWkA <= 0.05
    const isBelowB = q5hB <= threshold || qWkB <= 0.05

    const getTier = (isBroken: boolean, isErr: boolean, isBan: boolean, isBelow: boolean) => {
      if (isBan) return 4
      if (isErr) return 3
      if (!isBelow && !isBroken) return 1
      return 2
    }

    const tierA = getTier(isBrokenA, isErrorA, isBannedA, isBelowA)
    const tierB = getTier(isBrokenB, isErrorB, isBannedB, isBelowB)

    if (tierA !== tierB) {
      return tierA - tierB
    }

    // Within Tier 1 (Healthy Standby): Rank by continuous usage score (60% 5h + 40% weekly)
    if (tierA === 1) {
      const scoreA = q5hA * 0.6 + qWkA * 0.4
      const scoreB = q5hB * 0.6 + qWkB * 0.4
      if (Math.abs(scoreB - scoreA) > 0.001) {
        return scoreB - scoreA
      }
      if (Math.abs(q5hB - q5hA) > 0.001) {
        return q5hB - q5hA
      }
      return qWkB - qWkA
    }

    // Within Tier 2 (Below threshold / cooling down): Rank by remaining capacity
    if (tierA === 2) {
      if (Math.abs(q5hB - q5hA) > 0.001) {
        return q5hB - q5hA
      }
      return qWkB - qWkA
    }

    const labelA = (a.label || a.email).toLowerCase()
    const labelB = (b.label || b.email).toLowerCase()
    return labelA.localeCompare(labelB)
  })
}


interface QuotaDashboardPageProps {
  fleet: FleetQuotaSummary | null
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

const renderPlanTierBadge = (tier?: string) => {
  const t = tier || 'Free'
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
    case 'Free':
    default:
      style = { ...style, backgroundColor: 'var(--tonal)', color: 'var(--text-muted)' }
      break
  }

  return <span style={style}>{t}</span>
}

export const QuotaDashboardPage: React.FC<QuotaDashboardPageProps> = ({
  fleet,
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
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; account: AccountState } | null>(null)

  // Discovered Accounts Import Modal State
  const [isScanModalOpen, setIsScanModalOpen] = useState(false)
  const [discoveredAccounts, setDiscoveredAccounts] = useState<DiscoveredAccount[]>([])
  const [selectedDiscoveredEmails, setSelectedDiscoveredEmails] = useState<Set<string>>(new Set())
  const [isImportingDiscovered, setIsImportingDiscovered] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)

  const handleRefreshClick = () => {
    setIsRefreshing(true)
    try {
      onRefresh()
    } finally {
      setTimeout(() => setIsRefreshing(false), 500)
    }
  }

  useEffect(() => {
    const handleGlobalClick = () => setContextMenu(null)
    window.addEventListener('click', handleGlobalClick)
    return () => window.removeEventListener('click', handleGlobalClick)
  }, [])

  const accounts = fleet?.accounts || []
  const activeAccount = fleet?.active_account || ''
  const autoSwitchOn = rules?.auto_switch_enabled ?? false
  const threshold = rules?.auto_switch_threshold ?? 0.10

  const sortedAccounts = sortAccounts(accounts, activeAccount, threshold, sortMode)

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
      plan_tier: 'Free',
      is_active: false,
      status: 'STANDBY',
      quota_5h_available: 1.0,
      quota_weekly: 1.0,
      reset_horizon_text: '',
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
                <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
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

            <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginBottom: '8px' }}>
              {fleet?.total_accounts || accounts.length} Accounts Managed
            </div>

            {switchFeedback && (
              <div style={{ fontSize: '12px', color: 'var(--primary)', marginTop: '6px', fontWeight: 500 }}>
                {switchFeedback}
              </div>
            )}
          </div>

          {/* Action Row: Scan Local Accounts & Add Account */}
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center', marginTop: '16px' }}>
            <button
              onClick={handleScanLocalAccounts}
              disabled={isScanning}
              className="btn-pill-outlined"
              style={{
                padding: '7px 14px',
                fontSize: '12px',
                fontWeight: 600,
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: '6px',
                width: 'fit-content',
              }}
              title="Scan machine for local Antigravity/Google accounts"
            >
              <Search size={14} />
              {isScanning ? 'Scanning...' : 'Scan Local Accounts'}
            </button>

            <button
              onClick={handleAddNewAccount}
              className="btn-pill-primary"
              style={{
                padding: '7px 16px',
                fontSize: '12px',
                fontWeight: 600,
                display: 'inline-flex',
                alignItems: 'center',
                gap: '6px',
              }}
              title="Manually configure and add a new account"
            >
              <Plus size={14} /> Add Account
            </button>

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
                  style={{
                    transition: 'transform 0.5s ease',
                    transform: isRefreshing ? 'rotate(360deg)' : 'none',
                  }}
                />
              </button>
            </div>
          </div>

          {rules?.allow_non_gemini_native_models ? (
            /* 2x2 Progress Rings Layout with grey horizontal split line */
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', flex: 1, justifyContent: 'center' }}>
              {/* Row 1: Gemini Models */}
              <div>
                <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                  Gemini Models
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-around', gap: '12px' }}>
                  <CircularGauge
                    percentage={(fleet?.fleet_5h_gemini_available ?? fleet?.fleet_5h_available ?? 0) * 100}
                    title="Gemini 5H"
                    size={74}
                    strokeWidth={7}
                  />
                  <CircularGauge
                    percentage={(fleet?.fleet_weekly_gemini_available ?? fleet?.fleet_weekly_available ?? 0) * 100}
                    title="Gemini Weekly"
                    size={74}
                    strokeWidth={7}
                  />
                </div>
              </div>

              {/* Grey horizontal split line in between */}
              <div style={{ height: '1px', backgroundColor: 'var(--border)', width: '100%', margin: '4px 0' }} />

              {/* Row 2: Claude & GPT Models */}
              <div>
                <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                  Claude &amp; GPT Models
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-around', gap: '12px' }}>
                  <CircularGauge
                    percentage={(fleet?.fleet_5h_claude_gpt_available ?? 0) * 100}
                    title="Claude/GPT 5H"
                    size={74}
                    strokeWidth={7}
                  />
                  <CircularGauge
                    percentage={(fleet?.fleet_weekly_claude_gpt_available ?? 0) * 100}
                    title="Claude/GPT Weekly"
                    size={74}
                    strokeWidth={7}
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
                percentage={(fleet?.fleet_5h_available ?? 0) * 100}
                title="5H"
              />
              <CircularGauge
                percentage={(fleet?.fleet_weekly_available ?? 0) * 100}
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
                  borderRadius: '16px',
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
                <option value="credits">AI Credits</option>
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
          <table style={{ width: '100%', minWidth: '960px', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
            <thead>
              <tr>
                <th
                  onClick={() => setSortMode('identity')}
                  style={{
                    width: '26%',
                    minWidth: '220px',
                    padding: '12px 18px',
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
              <th style={{ width: '90px', minWidth: '85px', padding: '12px 10px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Plan
              </th>
              <th
                onClick={() => setSortMode('quota_5h')}
                style={{
                  width: '22%',
                  minWidth: '160px',
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
                  width: '22%',
                  minWidth: '160px',
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
                  width: '110px',
                  minWidth: '100px',
                  padding: '12px 10px',
                  textAlign: 'left',
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
                title="Click to sort by Available AI Credits"
              >
                <div style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span>AI Credits</span>
                  {sortMode === 'credits' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th
                onClick={() => setSortMode('priority')}
                style={{
                  width: '80px',
                  minWidth: '75px',
                  padding: '12px 10px',
                  textAlign: 'left',
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
                <div style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span>Priority</span>
                  {sortMode === 'priority' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th style={{ width: '95px', minWidth: '90px', padding: '12px 16px', textAlign: 'right', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Action
              </th>
            </tr>
          </thead>
          <tbody>
            {sortedAccounts.map((acc, index) => {
              const isActive = acc.is_active || acc.email === activeAccount
              const isNextSwitch = sortMode === 'auto' && !isActive && !acc.status?.toUpperCase().includes('BANNED') && !acc.status?.toUpperCase().includes('ERROR') && index === 1
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
                  <td style={{ padding: '12px 18px', overflow: 'hidden' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <span style={{ fontWeight: 600, color: 'var(--text)', fontSize: '13px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {acc.label ? acc.label : acc.email}
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
                          Next Switch
                        </span>
                      )}
                    </div>
                    {acc.label ? (
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {acc.email}
                      </div>
                    ) : null}
                  </td>

                  <td style={{ padding: '12px 10px' }}>
                    {renderPlanTierBadge(acc.plan_tier)}
                  </td>

                  <td style={{ padding: '12px', verticalAlign: 'middle' }}>
                    {rules?.allow_non_gemini_native_models ? (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '5px' }}>
                        <div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '10px', color: 'var(--text-muted)', marginBottom: '1px', fontWeight: 600 }}>
                            <span>Gemini</span>
                            <span>{Math.round((acc.quota_5h_available ?? 0) * 100)}%</span>
                          </div>
                          <HorizontalQuotaBar
                            fraction={acc.quota_5h_available ?? 0}
                            title={acc.reset_horizon_text || 'Gemini 5h cycle'}
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
                    ) : (
                      <HorizontalQuotaBar
                        fraction={acc.quota_5h_available ?? 0}
                        title={acc.reset_horizon_text || 'Resets in 5h cycle'}
                      />
                    )}
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
                            title="Gemini 7-day rolling cycle"
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
                        title="Resets on 7-day rolling cycle"
                      />
                    )}
                  </td>

                  <td style={{ padding: '14px' }}>
                    <div style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
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

                  <td style={{ padding: '14px' }}>
                    {renderPriorityBadge(acc.priority)}
                  </td>

                  <td style={{ padding: '14px 20px', textAlign: 'right' }}>
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
                    ) : acc.status?.toUpperCase() === 'ERROR' ? (
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
                    ) : isActive ? (
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
                        <CheckCircle2 size={12} /> Active
                      </span>
                    ) : (
                      <button
                        onClick={async (e) => {
                          e.stopPropagation()
                          try {
                            await api.switchAccount(acc.email)
                            onRefresh()
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
                        }}
                      >
                        <ArrowRightLeft size={11} /> Switch
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
        </div>
      </div>

      {/* Account Detail Modal Window */}
      {selectedRowAccount && (
        <AccountDetailModal
          account={selectedRowAccount}
          onClose={() => setSelectedRowAccount(null)}
          onSaved={onRefresh}
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
              borderRadius: '16px',
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
              <div
                style={{
                  backgroundColor:
                    errorDetailAccount.status?.toUpperCase() === 'BANNED' ? '#fce8e6' : '#fef7e0',
                  color:
                    errorDetailAccount.status?.toUpperCase() === 'BANNED' ? '#c5221f' : '#b06000',
                  padding: '12px 14px',
                  borderRadius: '10px',
                  fontSize: '12px',
                  lineHeight: 1.5,
                  wordBreak: 'break-word',
                  fontFamily: 'monospace',
                }}
              >
                {errorDetailAccount.error_message ||
                  errorDetailAccount.status_reason ||
                  (errorDetailAccount.status?.toUpperCase() === 'BANNED'
                    ? 'This account has been flagged or suspended by Google Antigravity services. Quota requests cannot be serviced.'
                    : 'Authentication failure or token expired. Please re-authenticate or update credentials.')}
              </div>
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
              padding: '6px 14px',
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
              padding: '8px 14px',
              fontSize: '12px',
              textAlign: 'left',
              color: 'var(--text)',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
            onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
          >
            <Edit2 size={13} /> Edit Account Details
          </button>
          {!contextMenu.account.is_active && contextMenu.account.email !== activeAccount && (
            <button
              onClick={async () => {
                const target = contextMenu.account.email
                setContextMenu(null)
                try {
                  await api.switchAccount(target)
                  onRefresh()
                } catch (err: any) {
                  setSwitchFeedback('Switch failed: ' + err.message)
                }
              }}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                width: '100%',
                padding: '8px 14px',
                fontSize: '12px',
                textAlign: 'left',
                color: 'var(--primary)',
              }}
              onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
              onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
            >
              <ArrowRightLeft size={13} /> Switch to this Account
            </button>
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
              padding: '8px 14px',
              fontSize: '12px',
              textAlign: 'left',
              color: 'var(--text)',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
            onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
          >
            <Copy size={13} /> Copy Email Address
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
              borderRadius: '16px',
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
                      padding: '10px 14px',
                      borderRadius: '8px',
                      border: `1.5px solid ${isSelected ? 'var(--primary)' : 'var(--border)'}`,
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
