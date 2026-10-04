import React, { useState } from 'react'
import {
  RotateCw,
  Zap,
  CheckCircle2,
  Clock,
  ArrowRightLeft,
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
                  {acc.email} {acc.label ? `(${acc.label})` : ''}
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
              title="5h Available"
            />
            <CircularGauge
              percentage={(fleet?.fleet_weekly_available ?? 0.94) * 100}
              title="Weekly Available"
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
            justifyContent: 'space-between',
          }}
        >
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            All Managed Accounts (Status & Quotas)
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
            Click any row to inspect details, modify credentials, or configure MFA
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
                Next 5h Quota
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Weekly Available
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Reset Horizon
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
                      {acc.email}
                    </div>
                    {acc.label && (
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        {acc.label}
                      </div>
                    )}
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
                    <HorizontalQuotaBar fraction={acc.quota_5h_available} />
                  </td>

                  <td style={{ padding: '14px' }}>
                    <HorizontalQuotaBar fraction={acc.quota_weekly} />
                  </td>

                  <td style={{ padding: '14px', fontSize: '12px', color: 'var(--text)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <Clock size={13} color="var(--text-subtle)" />
                      <span>{acc.reset_horizon_text || 'Active cycle'}</span>
                    </div>
                  </td>

                  <td style={{ padding: '14px 20px', textAlign: 'right' }}>
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        setSelectedRowAccount(acc)
                      }}
                      className="btn-pill-tonal"
                      style={{ padding: '4px 12px', fontSize: '11px' }}
                    >
                      Edit Details
                    </button>
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
    </div>
  )
}
