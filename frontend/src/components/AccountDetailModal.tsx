import React, { useState, useEffect } from 'react'
import { X, Trash2, Save, KeyRound, Tag, RefreshCw, Eye, EyeOff, Lock, LogIn, FileText, ShieldAlert, Copy, Check, Mail, Sparkles } from 'lucide-react'
import type { AccountState } from '../types'
import { CANONICAL_PLAN_TIERS, normalizePlanTier } from '../types'
import { renderPlanTierBadge } from '../pages/QuotaDashboardPage'
import { HorizontalQuotaBar } from './HorizontalQuotaBar'
import { ToggleSwitch } from './ToggleSwitch'
import { api } from '../api'
import { generateTOTP } from '../utils/totp'

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
  const [email, setEmail] = useState(account.email || '')
  const [label, setLabel] = useState(account.label || '')
  const [priority, setPriority] = useState<string>(account.priority || 'High')
  const [password, setPassword] = useState(account.password || '')
  const [showPassword, setShowPassword] = useState(false)
  const [totpSecret, setTotpSecret] = useState(account.totp_secret || '')
  const [showTotp, setShowTotp] = useState(false)
  const [refreshToken, setRefreshToken] = useState(account.refresh_token || '')
  const [showOAuth, setShowOAuth] = useState(false)
  const [isExtractingOAuth, setIsExtractingOAuth] = useState(false)
  const [oauthSuccessMsg, setOauthSuccessMsg] = useState<string | null>(null)
  const status = account.status || (account.is_active ? 'ACTIVE' : 'STANDBY')
  const [planTier, setPlanTier] = useState<string>(normalizePlanTier(account.plan_tier || 'Pro'))
  const credits =
    account.credits !== undefined && account.credits !== null
      ? account.credits
      : 0
  const [enableCreditOverages, setEnableCreditOverages] = useState<boolean>(account.enable_credit_overages ?? false)
  const [notes, setNotes] = useState(account.notes || '')
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  // Real-time derived 6-number verification code
  const [derivedCode, setDerivedCode] = useState<string | null>(null)
  const [formattedCode, setFormattedCode] = useState<string>('--- ---')
  const [remainingSeconds, setRemainingSeconds] = useState<number>(30)
  const [copiedTotp, setCopiedTotp] = useState(false)

  useEffect(() => {
    let timer: ReturnType<typeof setInterval>
    const updateCode = async () => {
      const clean = totpSecret?.trim()
      if (!clean) {
        setDerivedCode(null)
        setFormattedCode('--- ---')
        return
      }
      const res = await generateTOTP(clean)
      if (res) {
        setDerivedCode(res.code)
        setFormattedCode(res.formattedCode)
        setRemainingSeconds(res.remainingSeconds)
      } else {
        setDerivedCode(null)
        setFormattedCode('--- ---')
      }
    }

    updateCode()
    timer = setInterval(updateCode, 1000)
    return () => clearInterval(timer)
  }, [totpSecret])

  const handleCopyTotpCode = () => {
    if (!derivedCode) return
    navigator.clipboard.writeText(derivedCode)
    setCopiedTotp(true)
    setTimeout(() => setCopiedTotp(false), 1500)
  }

  const handleExtractGoogleOAuth = async () => {
    setIsExtractingOAuth(true)
    setError(null)
    setOauthSuccessMsg(null)
    try {
      const res = await api.startGoogleOAuth()
      if (res.success && res.refresh_token) {
        setRefreshToken(res.refresh_token)
        setOauthSuccessMsg(`Extracted token successfully for ${res.email || account.email}`)
      } else {
        setError(res.error || 'Failed to extract OAuth token from Google')
      }
    } catch (err: any) {
      setError(err.message || 'Google OAuth extraction failed or timed out')
    } finally {
      setIsExtractingOAuth(false)
    }
  }

  const handleSave = async () => {
    setIsSaving(true)
    setError(null)
    try {
      await api.updateAccount({
        email: email.trim() || account.email,
        label: label.trim(),
        plan_tier: planTier,
        status: status,
        priority: priority,
        password: password,
        notes: notes.trim(),
        totp_secret: totpSecret.trim().toUpperCase(),
        refresh_token: refreshToken.trim(),
        credits: Number(credits) || 0,
        enable_credit_overages: enableCreditOverages,
      })
      onSaved()
      onClose()
    } catch (err: any) {
      setError(err.message || 'Failed to update account')
    } finally {
      setIsSaving(false)
    }
  }

  const handleDelete = () => {
    setShowDeleteConfirm(true)
  }

  const confirmDelete = async () => {
    setShowDeleteConfirm(false)
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

  const st = (status || (account.is_active ? 'ACTIVE' : 'STANDBY')).toUpperCase()

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
          width: '580px',
          maxWidth: '94vw',
          maxHeight: '92vh',
          overflowY: 'auto',
          padding: '28px',
          backgroundColor: '#ffffff',
          borderRadius: '16px',
          boxShadow: 'var(--shadow-md)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '20px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <h2 style={{ fontSize: '18px', fontWeight: 700, color: 'var(--text)' }}>
                {label || account.email}
              </h2>
              <span
                className={`badge-chip ${
                  st === 'BANNED'
                    ? 'badge-red'
                    : st === 'ERROR'
                    ? 'badge-yellow'
                    : account.is_active
                    ? 'badge-green'
                    : 'badge-neutral'
                }`}
              >
                {st}
              </span>
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
              {account.email} • Priority: <span style={{ fontWeight: 600 }}>{priority}</span>
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

        {st === 'BANNED' && (
          <div
            style={{
              backgroundColor: 'var(--red-bg)',
              color: 'var(--red)',
              padding: '10px 16px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '16px',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
            }}
          >
            <ShieldAlert size={16} />
            <span>Account banned or suspended upstream. Please appeal via Google or discard this account.</span>
          </div>
        )}

        {st === 'ERROR' && (
          <div
            style={{
              backgroundColor: 'var(--yellow-bg)',
              color: 'var(--yellow)',
              padding: '10px 16px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '16px',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
            }}
          >
            <ShieldAlert size={16} />
            <span>Authentication error. Verification or token re-extraction required.</span>
          </div>
        )}

        {error && (
          <div
            style={{
              backgroundColor: 'var(--red-bg)',
              color: 'var(--red)',
              padding: '10px 16px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '16px',
            }}
          >
            {error}
          </div>
        )}

        {oauthSuccessMsg && (
          <div
            style={{
              backgroundColor: 'var(--green-bg)',
              color: 'var(--green)',
              padding: '10px 16px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '20px',
            }}
          >
            {oauthSuccessMsg}
          </div>
        )}

        {/* Live Quota Metrics Section */}
        <div
          style={{
            backgroundColor: 'var(--canvas)',
            border: '1px solid var(--border)',
            borderRadius: '12px',
            padding: '16px 20px',
            marginBottom: '24px',
          }}
        >
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', marginBottom: '12px', textTransform: 'uppercase' }}>
            Live Quota Metrics
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                <span style={{ fontWeight: 500, color: 'var(--text)' }}>5H Quota:</span>
                <span style={{ color: 'var(--text-muted)' }}>{account.reset_horizon_text}</span>
              </div>
              <HorizontalQuotaBar
                fraction={account.quota_5h_available}
                maxWidth="100%"
                title={account.reset_horizon_text || 'Resets in 5h cycle'}
              />
            </div>

            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                <span style={{ fontWeight: 500, color: 'var(--text)' }}>Weekly Quota:</span>
                <span style={{ color: 'var(--text-muted)' }}>7-day allowance</span>
              </div>
              <HorizontalQuotaBar
                fraction={account.quota_weekly}
                maxWidth="100%"
                title="Resets on 7-day rolling cycle"
              />
            </div>
          </div>
        </div>

        {/* Form Fields */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '22px', marginBottom: '28px' }}>
          {/* Account ID / Email Address */}
          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
              <Mail size={14} /> Account ID / Email Address:
            </label>
            <input
              type="email"
              placeholder="e.g. user@gmail.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              style={{ width: '100%' }}
            />
          </div>

          {/* Account Alias and Account Priority on the same row/level */}
          <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 0.8fr', gap: '16px' }}>
            <div>
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                <Tag size={14} /> Account Alias:
              </label>
              <input
                type="text"
                placeholder="e.g. Account 1, Primary, Backup"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                style={{ width: '100%' }}
              />
            </div>

            <div>
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                Account Priority:
              </label>
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value)}
                style={{
                  width: '100%',
                  backgroundColor: 'var(--canvas)',
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '8px 12px',
                  fontSize: '13px',
                  color: 'var(--text)',
                  cursor: 'pointer',
                }}
              >
                <option value="High">High</option>
                <option value="Mid">Mid</option>
                <option value="Low">Low</option>
              </select>
            </div>
          </div>

          {/* Optional Password / Vault field (hidden by default with eye toggle) */}
          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
              <Lock size={14} /> Password (Optional / Vault):
            </label>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
              <input
                type={showPassword ? 'text' : 'password'}
                placeholder="Optional login password or vault credential"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                style={{ width: '100%', paddingRight: '40px' }}
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                style={{
                  position: 'absolute',
                  right: '8px',
                  background: 'none',
                  border: 'none',
                  color: 'var(--text-muted)',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  padding: '4px',
                }}
                title={showPassword ? 'Hide password' : 'Show password'}
              >
                {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          {/* OAuth Refresh Token with eye toggle & Google extraction button */}
          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
              <RefreshCw size={14} /> OAuth Refresh Token:
            </label>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <div style={{ position: 'relative', flex: 1, display: 'flex', alignItems: 'center' }}>
                <input
                  type={showOAuth ? 'text' : 'password'}
                  placeholder="1//... (Leave unchanged or extract via Google)"
                  value={refreshToken}
                  onChange={(e) => setRefreshToken(e.target.value)}
                  style={{ width: '100%', fontFamily: 'monospace', paddingRight: '40px' }}
                />
                <button
                  type="button"
                  onClick={() => setShowOAuth(!showOAuth)}
                  style={{
                    position: 'absolute',
                    right: '8px',
                    background: 'none',
                    border: 'none',
                    color: 'var(--text-muted)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    padding: '4px',
                  }}
                  title={showOAuth ? 'Hide OAuth token' : 'Show OAuth token'}
                >
                  {showOAuth ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
              <button
                type="button"
                onClick={handleExtractGoogleOAuth}
                disabled={isExtractingOAuth}
                className="btn-pill-outlined"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  whiteSpace: 'nowrap',
                  padding: '7px 12px',
                  fontSize: '12px',
                  fontWeight: 600,
                  backgroundColor: 'var(--primary-light)',
                  color: 'var(--primary)',
                  borderColor: 'var(--primary)',
                }}
                title="Open browser to login with Google and extract token"
              >
                <LogIn size={14} />
                {isExtractingOAuth ? 'Waiting...' : 'Sign in with Google'}
              </button>
            </div>
          </div>

          {/* MFA / TOTP Secret Key with header-level verification code and copy button */}
          <div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                marginBottom: '8px',
              }}
            >
              <label
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  fontSize: '12px',
                  fontWeight: 600,
                  color: 'var(--text-muted)',
                }}
              >
                <KeyRound size={14} /> MFA Secret Key:
              </label>

              {/* Right end: Countdown progress ring & connected 6-digit code widget */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                {/* Blue circular countdown progress ring */}
                <div
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: '24px',
                    height: '24px',
                    position: 'relative',
                    flexShrink: 0,
                  }}
                  title={
                    derivedCode
                      ? `${remainingSeconds}s remaining until code refreshes`
                      : 'No MFA secret configured'
                  }
                >
                  <svg width="24" height="24" viewBox="0 0 24 24" style={{ overflow: 'visible' }}>
                    <circle
                      cx="12"
                      cy="12"
                      r="9"
                      fill="none"
                      stroke={derivedCode ? 'rgba(26, 115, 232, 0.16)' : 'var(--border)'}
                      strokeWidth="2.2"
                    />
                    {derivedCode && (
                      <circle
                        cx="12"
                        cy="12"
                        r="9"
                        fill="none"
                        stroke="#1a73e8"
                        strokeWidth="2.2"
                        strokeLinecap="round"
                        strokeDasharray={56.55}
                        strokeDashoffset={56.55 * (1 - remainingSeconds / 30)}
                        transform="rotate(-90 12 12)"
                        style={{ transition: 'stroke-dashoffset 0.8s linear' }}
                      />
                    )}
                    <text
                      x="12"
                      y="12"
                      textAnchor="middle"
                      dominantBaseline="central"
                      fontSize="9"
                      fontWeight="700"
                      fill={derivedCode ? '#1a73e8' : 'var(--text-muted)'}
                      style={{ fontFamily: 'system-ui, -apple-system, sans-serif' }}
                    >
                      {derivedCode ? remainingSeconds : '--'}
                    </text>
                  </svg>
                </div>

                {/* Connected 6-digit verification code & icon-only copy button */}
                <div
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    border: '1px solid var(--border)',
                    borderRadius: '6px',
                    overflow: 'hidden',
                    backgroundColor: derivedCode ? 'var(--green-bg, rgba(52, 168, 83, 0.08))' : 'var(--hover)',
                    height: '24px',
                    boxShadow: '0 1px 2px rgba(0,0,0,0.03)',
                  }}
                >
                  <span
                    style={{
                      fontFamily: 'monospace',
                      fontSize: '12px',
                      fontWeight: derivedCode ? 700 : 500,
                      letterSpacing: '1px',
                      color: derivedCode ? 'var(--green, #34a853)' : 'var(--text-muted)',
                      padding: '0 8px',
                      lineHeight: '22px',
                      userSelect: 'all',
                      borderRight: '1px solid var(--border)',
                    }}
                    title={
                      derivedCode
                        ? `Current derived 6-number verification code (${remainingSeconds}s remaining)`
                        : 'No MFA secret configured'
                    }
                  >
                    {formattedCode}
                  </span>

                  <button
                    type="button"
                    onClick={handleCopyTotpCode}
                    disabled={!derivedCode}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      width: '26px',
                      height: '100%',
                      padding: 0,
                      backgroundColor: copiedTotp ? 'var(--green-bg, rgba(52,168,83,0.22))' : 'transparent',
                      color: copiedTotp ? 'var(--green, #34a853)' : (!derivedCode ? 'var(--text-muted)' : 'var(--text)'),
                      border: 'none',
                      cursor: derivedCode ? 'pointer' : 'not-allowed',
                      opacity: derivedCode ? 1 : 0.4,
                      transition: 'background-color 0.15s ease, color 0.15s ease',
                    }}
                    title={derivedCode ? (copiedTotp ? 'Copied to clipboard!' : 'Copy verification code') : 'No code to copy'}
                  >
                    {copiedTotp ? <Check size={13} /> : <Copy size={13} />}
                  </button>
                </div>
              </div>
            </div>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
              <input
                type={showTotp ? 'text' : 'password'}
                placeholder="e.g. JBSWY3DPEHPK3PXP"
                value={totpSecret}
                onChange={(e) => setTotpSecret(e.target.value)}
                style={{ width: '100%', fontFamily: 'monospace', paddingRight: '40px' }}
              />
              <button
                type="button"
                onClick={() => setShowTotp(!showTotp)}
                style={{
                  position: 'absolute',
                  right: '8px',
                  background: 'none',
                  border: 'none',
                  color: 'var(--text-muted)',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  padding: '4px',
                }}
                title={showTotp ? 'Hide MFA secret' : 'Show MFA secret'}
              >
                {showTotp ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          {/* Status & Plan Tier (Auto-ingested) */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                  Account Status:
                </label>
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontStyle: 'italic' }}>
                  Auto-detected
                </span>
              </div>
              <div
                style={{
                  width: '100%',
                  backgroundColor: 'var(--canvas)',
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '9px 12px',
                  fontSize: '13px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  boxSizing: 'border-box',
                }}
              >
                <span
                  style={{
                    display: 'inline-block',
                    width: '8px',
                    height: '8px',
                    borderRadius: '50%',
                    backgroundColor: status === 'ACTIVE' ? '#137333' : status === 'ERROR' || status === 'BANNED' ? '#b3261e' : '#b06000',
                  }}
                />
                <span style={{ fontWeight: 700, color: 'var(--text)' }}>
                  {status || 'STANDBY'}
                </span>
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto' }}>
                  Runtime State
                </span>
              </div>
            </div>

            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                  Plan Tier / Membership:
                </label>
                <div style={{ display: 'flex', alignItems: 'center' }}>
                  {renderPlanTierBadge(planTier)}
                </div>
              </div>
              <select
                value={normalizePlanTier(planTier)}
                onChange={(e) => setPlanTier(e.target.value)}
                style={{
                  width: '100%',
                  backgroundColor: 'var(--canvas)',
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '9px 12px',
                  fontSize: '13px',
                  fontWeight: 600,
                  color: 'var(--text)',
                  cursor: 'pointer',
                  boxSizing: 'border-box',
                }}
              >
                {CANONICAL_PLAN_TIERS.map((tier) => (
                  <option key={tier} value={tier}>
                    {tier}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* AI Credits (Auto-ingested, no dollar sign) & Overage Setting */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', alignItems: 'center' }}>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                  <Sparkles size={14} color="#f59e0b" /> Available AI Credits:
                </label>
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontStyle: 'italic' }}>
                  Auto-ingested
                </span>
              </div>
              <div
                style={{
                  width: '100%',
                  backgroundColor: 'var(--canvas)',
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '9px 12px',
                  fontSize: '13px',
                  fontWeight: 700,
                  color: 'var(--text)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  boxSizing: 'border-box',
                }}
              >
                <span>{credits !== undefined && credits !== null ? `${credits} Credits` : '0 Credits'}</span>
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 500 }}>
                  Credit Pool
                </span>
              </div>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                Credit Overages:
              </label>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', height: '38px' }}>
                <ToggleSwitch
                  checked={enableCreditOverages}
                  onChange={(val) => setEnableCreditOverages(val)}
                />
                <span style={{ fontSize: '13px', color: 'var(--text)', fontWeight: 500 }}>
                  Enable AI Credit Overages
                </span>
              </div>
            </div>
          </div>

          {/* Section at bottom to write notes */}
          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
              <FileText size={14} /> Account Notes:
            </label>
            <textarea
              placeholder="Write private notes, recovery details, or usage reminders for this account..."
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={3}
              style={{
                width: '100%',
                backgroundColor: 'var(--canvas)',
                border: '1px solid var(--border)',
                borderRadius: '8px',
                padding: '10px 12px',
                fontSize: '12px',
                color: 'var(--text)',
                resize: 'vertical',
                minHeight: '65px',
                fontFamily: 'inherit',
              }}
            />
          </div>
        </div>

        {/* Footer Actions */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: '1px solid var(--border)', paddingTop: '20px' }}>
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

      {/* Delete Confirmation In-App Modal */}
      {showDeleteConfirm && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.45)',
            backdropFilter: 'blur(3px)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1100,
          }}
          onClick={() => setShowDeleteConfirm(false)}
        >
          <div
            className="google-card"
            style={{
              width: '440px',
              maxWidth: '92vw',
              padding: '24px',
              boxShadow: 'var(--shadow-md)',
              backgroundColor: '#ffffff',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', color: '#b3261e', marginBottom: '12px' }}>
              <Trash2 size={20} />
              <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                Delete Account
              </h3>
            </div>
            <p style={{ margin: '0 0 20px', fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Are you sure you want to permanently remove account <strong>"{account.email}"</strong>? This will remove stored credentials and quota history.
            </p>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
              <button
                type="button"
                onClick={() => setShowDeleteConfirm(false)}
                className="btn-pill-tonal"
                style={{ padding: '7px 16px', fontSize: '12px' }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={confirmDelete}
                disabled={isSaving}
                className="btn-pill-danger"
                style={{ padding: '7px 16px', fontSize: '12px' }}
              >
                {isSaving ? 'Deleting...' : 'Delete Account'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
