import React, { useState } from 'react'
import { X, Trash2, Save, KeyRound, Tag, RefreshCw } from 'lucide-react'
import type { AccountState } from '../types'
import { HorizontalQuotaBar } from './HorizontalQuotaBar'
import { api } from '../api'

interface AccountDetailModalProps {
  account: AccountState
  onClose: () => void
  onSaved: () => void
}

export const AccountDetailModal: React.FC<AccountDetailModalProps> = ({
  account,
  onClose,
  onSaved,
}) => {
  const [label, setLabel] = useState(account.label || '')
  const [planTier, setPlanTier] = useState(account.plan_tier || 'Pro')
  const [totpSecret, setTotpSecret] = useState(account.totp_secret || '')
  const [refreshToken, setRefreshToken] = useState(account.refresh_token || '')
  const [setActive, setSetActive] = useState(account.is_active)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSave = async () => {
    setIsSaving(true)
    setError(null)
    try {
      await api.updateAccount({
        email: account.email,
        label: label.trim(),
        plan_tier: planTier,
        totp_secret: totpSecret.trim().toUpperCase(),
        refresh_token: refreshToken.trim(),
        set_active: setActive,
      })
      onSaved()
      onClose()
    } catch (err: any) {
      setError(err.message || 'Failed to update account')
    } finally {
      setIsSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!window.confirm(`Are you sure you want to remove account ${account.email}?`)) {
      return
    }
    setIsSaving(true)
    setError(null)
    try {
      await api.deleteAccount(account.email)
      onSaved()
      onClose()
    } catch (err: any) {
      setError(err.message || 'Failed to remove account')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.45)',
        backdropFilter: 'blur(3px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
      }}
      onClick={onClose}
    >
      <div
        className="google-card"
        style={{
          width: '560px',
          maxWidth: '92vw',
          maxHeight: '90vh',
          overflowY: 'auto',
          padding: '24px',
          backgroundColor: '#ffffff',
          borderRadius: '16px',
          boxShadow: 'var(--shadow-md)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <h2 style={{ fontSize: '18px', fontWeight: 700, color: 'var(--text)' }}>
                {account.email}
              </h2>
              <span className={`badge-chip ${account.is_active ? 'badge-green' : 'badge-neutral'}`}>
                {account.is_active ? 'ACTIVE' : 'STANDBY'}
              </span>
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
              Managed Session Account • {account.reset_horizon_text || 'Reset horizon calculating...'}
            </div>
          </div>
          <button
            onClick={onClose}
            style={{
              padding: '6px',
              borderRadius: '50%',
              color: 'var(--text-muted)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <X size={20} />
          </button>
        </div>

        {error && (
          <div
            style={{
              backgroundColor: 'var(--red-bg)',
              color: 'var(--red)',
              padding: '10px 14px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '16px',
            }}
          >
            {error}
          </div>
        )}

        {/* Live Quota Metrics Section */}
        <div
          style={{
            backgroundColor: 'var(--canvas)',
            border: '1px solid var(--border)',
            borderRadius: '12px',
            padding: '16px',
            marginBottom: '20px',
          }}
        >
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', marginBottom: '12px', textTransform: 'uppercase' }}>
            Live Quota Metrics
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '4px' }}>
                <span style={{ fontWeight: 500, color: 'var(--text)' }}>Next 5 Hours Quota:</span>
                <span style={{ color: 'var(--text-muted)' }}>{account.reset_horizon_text}</span>
              </div>
              <HorizontalQuotaBar fraction={account.quota_5h_available} maxWidth="100%" />
            </div>

            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '4px' }}>
                <span style={{ fontWeight: 500, color: 'var(--text)' }}>Weekly Horizon Quota:</span>
                <span style={{ color: 'var(--text-muted)' }}>7-day allowance</span>
              </div>
              <HorizontalQuotaBar fraction={account.quota_weekly} maxWidth="100%" />
            </div>
          </div>
        </div>

        {/* Form Fields */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', marginBottom: '24px' }}>
          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
              <Tag size={14} /> Friendly Label / Role:
            </label>
            <input
              type="text"
              placeholder="e.g. Lead Systems Architect, Primary Backup"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              style={{ width: '100%' }}
            />
          </div>

          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
              Plan Tier / Membership:
            </label>
            <select
              value={planTier}
              onChange={(e) => setPlanTier(e.target.value)}
              style={{
                width: '100%',
                backgroundColor: 'var(--canvas)',
                border: '1px solid var(--border)',
                borderRadius: '8px',
                padding: '8px 12px',
                fontSize: '13px',
                color: 'var(--text)',
              }}
            >
              <option value="Free">Free (Standard free tier)</option>
              <option value="Plus">Plus (Paid Plus)</option>
              <option value="Pro">Pro (Full paid Pro)</option>
              <option value="Pro - Trial">Pro - Trial (Pro trial membership)</option>
              <option value="Edu">Edu (Educational paid subscription)</option>
              <option value="Ultra 5X">Ultra 5X (5X quota multiplier)</option>
              <option value="Ultra 10X">Ultra 10X (10X quota multiplier)</option>
              <option value="Ultra 20X">Ultra 20X (20X maximum tier)</option>
            </select>
          </div>

          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
              <KeyRound size={14} /> RFC 6238 TOTP MFA Secret (Base32):
            </label>
            <input
              type="text"
              placeholder="e.g. JBSWY3DPEHPK3PXP"
              value={totpSecret}
              onChange={(e) => setTotpSecret(e.target.value)}
              style={{ width: '100%', fontFamily: 'monospace' }}
            />
          </div>

          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
              <RefreshCw size={14} /> OAuth Refresh Token:
            </label>
            <input
              type="password"
              placeholder="1//... (Leave unchanged to preserve active credential)"
              value={refreshToken}
              onChange={(e) => setRefreshToken(e.target.value)}
              style={{ width: '100%', fontFamily: 'monospace' }}
            />
          </div>

          <label style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '13px', cursor: 'pointer', marginTop: '4px' }}>
            <input
              type="checkbox"
              checked={setActive}
              onChange={(e) => setSetActive(e.target.checked)}
              style={{ width: '16px', height: '16px' }}
            />
            <span>Set as active Antigravity account upon save</span>
          </label>
        </div>

        {/* Footer Actions */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
          <button
            onClick={handleDelete}
            disabled={isSaving}
            className="btn-pill-danger"
          >
            <Trash2 size={15} /> Delete Account
          </button>

          <div style={{ display: 'flex', gap: '10px' }}>
            <button
              onClick={onClose}
              disabled={isSaving}
              className="btn-pill-outlined"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={isSaving}
              className="btn-pill-primary"
            >
              <Save size={15} /> {isSaving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
