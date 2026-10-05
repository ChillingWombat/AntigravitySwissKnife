import React, { useState, useEffect } from 'react'
import {
  RotateCw,
  Zap,
  CheckCircle2,
  ArrowRightLeft,
  Edit2,
  Copy,
  AlertTriangle,
  AlertCircle,
  ArrowUpDown,
} from 'lucide-react'
import type { AccountState, FleetQuotaSummary, RuleConfig } from '../types'
import { CircularGauge } from '../components/CircularGauge'
import { HorizontalQuotaBar } from '../components/HorizontalQuotaBar'
import { AccountDetailModal } from '../components/AccountDetailModal'
import { api } from '../api'

export type SortMode = 'auto' | 'identity' | 'quota_5h' | 'quota_weekly'

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
  // 1. Row 1: Active healthy account
  // 2. Row 2+: Next standby accounts to rotate into, arranged by 5H & weekly capacity
  // 3. Last rows: Switched-off or cooling-down accounts below threshold
  // 4. End: Broken (ERROR/BANNED) accounts
  return copy.sort((a, b) => {
    const isActA = a.is_active || a.email === activeEmail
    const isActB = b.is_active || b.email === activeEmail

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

    const getTier = (isAct: boolean, isBroken: boolean, isErr: boolean, isBan: boolean, isBelow: boolean) => {
      if (isBan) return 4
      if (isErr) return 3
      if (isAct && !isBelow && !isBroken) return 0
      if (!isAct && !isBelow && !isBroken) return 1
      return 2
    }

    const tierA = getTier(isActA, isBrokenA, isErrorA, isBannedA, isBelowA)
    const tierB = getTier(isActB, isBrokenB, isErrorB, isBannedB, isBelowB)

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

const renderStatusBadge = (status?: string, isActive?: boolean) => {
  const st = (status || '').toUpperCase()
  if (st === 'BANNED') {
    return (
      <span
        className="badge-chip badge-red"
        title="Account suspended or banned (Appeal or discard)"
      >
        BANNED
      </span>
    )
  }
  if (st === 'ERROR') {
    return (
      <span
        className="badge-chip badge-yellow"
        title="Authentication or verification required (Re-authenticate)"
      >
        ERROR
      </span>
    )
  }
  if (isActive) {
    return <span className="badge-chip badge-green">ACTIVE</span>
  }
  return <span className="badge-chip badge-neutral">STANDBY</span>
}

interface QuotaDashboardPageProps {
  fleet: FleetQuotaSummary | null
  rules: RuleConfig | null
  onRefresh: () => void
  onAutoSwitchToggled: (enabled: boolean) => void
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
  const [selectedQuickEmail, setSelectedQuickEmail] = useState<string>('')
  const [sortMode, setSortMode] = useState<SortMode>('auto')
  const [isSwitching, setIsSwitching] = useState(false)
  const [isTogglingRules, setIsTogglingRules] = useState(false)
  const [switchFeedback, setSwitchFeedback] = useState<string | null>(null)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; account: AccountState } | null>(null)

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

  const handleManualSwitch = async () => {
    const target = selectedQuickEmail || (accounts[0]?.email ?? '')
    if (!target || target === activeAccount) return

    setIsSwitching(true)
    setSwitchFeedback(null)
    try {
      await api.switchAccount(target)
      setSwitchFeedback(`Successfully switched to ${target}`)
      onRefresh()
    } catch (err: any) {
      setSwitchFeedback(`Switch error: ${err.message}`)
    } finally {
      setIsSwitching(false)
    }
  }

  const handleToggleAutoSwitch = async () => {
    setIsTogglingRules(true)
    try {
      const nextVal = !autoSwitchOn
      await api.setAutoSwitch(nextVal)
      onAutoSwitchToggled(nextVal)
    } catch (err: any) {
      alert(`Could not toggle auto-switch: ${err.message}`)
    } finally {
      setIsTogglingRules(false)
    }
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
        {/* Card A: Managed Accounts Fleet */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                Managed Accounts Fleet
              </div>

              {/* Auto-Switch Toggle Pill Button */}
              <button
                onClick={handleToggleAutoSwitch}
                disabled={isTogglingRules}
                style={{
                  borderRadius: '20px',
                  padding: '5px 14px',
                  fontSize: '11px',
                  fontWeight: 700,
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  backgroundColor: autoSwitchOn ? 'var(--green-bg)' : 'var(--tonal)',
                  color: autoSwitchOn ? 'var(--green)' : 'var(--text-muted)',
                  border: `1px solid ${autoSwitchOn ? '#ceead6' : 'var(--border)'}`,
                  transition: 'all 0.2s ease',
                }}
              >
                <Zap size={13} fill={autoSwitchOn ? 'var(--green)' : 'none'} />
                Auto-Switch: {autoSwitchOn ? 'ON' : 'OFF'}
              </button>
            </div>

            <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginBottom: '4px' }}>
              {fleet?.total_accounts || accounts.length} Accounts Managed
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '13px', fontWeight: 600, color: 'var(--primary)', marginBottom: '4px' }}>
              <CheckCircle2 size={15} />
              <span>Active: {activeAccount || 'Not Logged In'}</span>
            </div>

            <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
              Last Quota Sync: {new Date().toLocaleTimeString()}
            </div>

            {switchFeedback && (
              <div style={{ fontSize: '12px', color: 'var(--primary)', marginTop: '6px', fontWeight: 500 }}>
                {switchFeedback}
              </div>
            )}
          </div>

          {/* Quick Switch Dropdown & Action Row */}
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center', marginTop: '16px' }}>
            <select
              value={selectedQuickEmail || activeAccount}
              onChange={(e) => setSelectedQuickEmail(e.target.value)}
              style={{ flex: 1, backgroundColor: 'var(--tonal)' }}
            >
              {accounts.map((acc) => (
                <option key={acc.email} value={acc.email}>
                  {acc.label ? `${acc.label} (${acc.email})` : acc.email}
                </option>
              ))}
            </select>

            <button
              onClick={handleManualSwitch}
              disabled={isSwitching}
              className="btn-pill-primary"
              style={{ padding: '7px 16px', fontSize: '12px' }}
            >
              <ArrowRightLeft size={14} /> Switch
            </button>

            <button
              onClick={onRefresh}
              className="btn-pill-outlined"
              style={{ padding: '7px 12px' }}
              title="Refresh Quota"
            >
              <RotateCw size={14} />
            </button>
          </div>
        </div>

        {/* Card B: Merged Total Quota Progress Rings */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column' }}>
          <div
            style={{
              fontSize: '11px',
              fontWeight: 700,
              color: 'var(--text-muted)',
              letterSpacing: '0.8px',
              textTransform: 'uppercase',
              marginBottom: '12px',
            }}
          >
            Total Quota
          </div>
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
              percentage={(fleet?.fleet_5h_available ?? 0.94) * 100}
              title="5H"
            />
            <CircularGauge
              percentage={(fleet?.fleet_weekly_available ?? 0.94) * 100}
              title="Weekly"
            />
          </div>
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
            All Managed Accounts (Status & Quotas)
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
                <option value="auto">Auto (Continuous Rotation)</option>
                <option value="identity">Account Identity</option>
                <option value="quota_5h">5H Quota</option>
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

        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              <th
                onClick={() => setSortMode('identity')}
                style={{
                  padding: '12px 20px',
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
                title="Click to sort by Account Identity"
              >
                <div style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span>Account Identity</span>
                  {sortMode === 'identity' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Plan Tier
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Status
              </th>
              <th
                onClick={() => setSortMode('quota_5h')}
                style={{
                  padding: '12px 14px',
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
                title="Click to sort by 5H Quota"
              >
                <div style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                  <span>5H QUOTA</span>
                  {sortMode === 'quota_5h' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th
                onClick={() => setSortMode('quota_weekly')}
                style={{
                  padding: '12px 14px',
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
                  <span>WEEKLY QUOTA</span>
                  {sortMode === 'quota_weekly' && <ArrowUpDown size={11} />}
                </div>
              </th>
              <th style={{ padding: '12px 20px', textAlign: 'right', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
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
                  <td style={{ padding: '14px 20px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <span style={{ fontWeight: 600, color: 'var(--text)', fontSize: '13px' }}>
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
                          }}
                          title="Next account in continuous rotation queue"
                        >
                          Next Switch
                        </span>
                      )}
                    </div>
                    {acc.label ? (
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        {acc.email}
                      </div>
                    ) : null}
                  </td>

                  <td style={{ padding: '14px' }}>
                    {renderPlanTierBadge(acc.plan_tier)}
                  </td>

                  <td style={{ padding: '14px' }}>
                    {renderStatusBadge(acc.status, isActive)}
                  </td>

                  <td style={{ padding: '14px' }}>
                    <HorizontalQuotaBar
                      fraction={acc.quota_5h_available}
                      title={acc.reset_horizon_text || 'Resets in 5h cycle'}
                    />
                  </td>

                  <td style={{ padding: '14px' }}>
                    <HorizontalQuotaBar
                      fraction={acc.quota_weekly}
                      title="Resets on 7-day rolling cycle"
                    />
                  </td>

                  <td style={{ padding: '14px 20px', textAlign: 'right' }}>
                    {acc.status?.toUpperCase() === 'BANNED' ? (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setSelectedRowAccount(acc)
                        }}
                        className="btn-pill-tonal"
                        style={{
                          padding: '4px 12px',
                          fontSize: '11px',
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '4px',
                          color: 'var(--red)',
                          backgroundColor: '#fce8e6',
                          border: '1px solid #fad2cf',
                        }}
                        title="Account suspended or banned (Click to appeal or discard)"
                      >
                        <AlertTriangle size={11} /> Banned
                      </button>
                    ) : acc.status?.toUpperCase() === 'ERROR' ? (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setSelectedRowAccount(acc)
                        }}
                        className="btn-pill-tonal"
                        style={{
                          padding: '4px 12px',
                          fontSize: '11px',
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '4px',
                          color: '#b06000',
                          backgroundColor: '#fef7e0',
                          border: '1px solid #feefc3',
                        }}
                        title="Authentication or verification required (Click to re-authenticate)"
                      >
                        <AlertCircle size={11} /> Verify
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
                            alert('Switch failed: ' + err.message)
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

      {/* Account Detail Modal Window */}
      {selectedRowAccount && (
        <AccountDetailModal
          account={selectedRowAccount}
          onClose={() => setSelectedRowAccount(null)}
          onSaved={onRefresh}
        />
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
                  alert('Switch failed: ' + err.message)
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
    </div>
  )
}
