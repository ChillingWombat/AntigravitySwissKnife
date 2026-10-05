import React, { useState, useEffect } from 'react'
import {
  RotateCw,
  Zap,
  CheckCircle2,
  ArrowRightLeft,
  Edit2,
  Copy,
} from 'lucide-react'
import type { AccountState, FleetQuotaSummary, RuleConfig } from '../types'
import { CircularGauge } from '../components/CircularGauge'
import { HorizontalQuotaBar } from '../components/HorizontalQuotaBar'
import { AccountDetailModal } from '../components/AccountDetailModal'
import { api } from '../api'

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
            padding: '18px 20px 14px',
            borderBottom: '1px solid var(--border)',
            display: 'flex',
            alignItems: 'center',
          }}
        >
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            All Managed Accounts (Status & Quotas)
          </div>
        </div>

        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              <th style={{ padding: '12px 20px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Account Identity
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Plan Tier
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Status
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                5H QUOTA
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                WEEKLY QUOTA
              </th>
              <th style={{ padding: '12px 20px', textAlign: 'right', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Action
              </th>
            </tr>
          </thead>
          <tbody>
            {accounts.map((acc) => {
              const isActive = acc.is_active || acc.email === activeAccount
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
                    <div style={{ fontWeight: 600, color: 'var(--text)', fontSize: '13px' }}>
                      {acc.label ? acc.label : acc.email}
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
                    <span className={`badge-chip ${isActive ? 'badge-green' : 'badge-neutral'}`}>
                      {isActive ? 'ACTIVE' : 'STANDBY'}
                    </span>
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
                    {isActive ? (
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
