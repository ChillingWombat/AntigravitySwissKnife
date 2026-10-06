import React, { useState, useEffect, useRef } from 'react'
import { X, Trash2, Save, KeyRound, Tag, RefreshCw, Eye, EyeOff, Lock, LogIn, FileText, ShieldAlert, Copy, Check, Mail } from 'lucide-react'
import type { AccountState } from '../types'
import { renderPlanTierBadge } from '../pages/QuotaDashboardPage'
import { HorizontalQuotaBar } from './HorizontalQuotaBar'
import { ToggleSwitch } from './ToggleSwitch'
import { api } from '../api'
import { generateTOTP } from '../utils/totp'
import {
  getAccountHeaderDisplay,
  resolveDefaultAlias,
  normalizeMfaSecret,
  ACCOUNT_SETUP_TEXTS,
} from '../utils/accountPresentation'

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
  const isNewAccount = !account.email?.trim()
  const [email, setEmail] = useState(account.email || '')
  const [label, setLabel] = useState(account.label || '')
  const [aliasAutoFilled, setAliasAutoFilled] = useState(false)
  const [priority, setPriority] = useState<string>(account.priority || 'High')
  const [allowClaudeGpt, setAllowClaudeGpt] = useState<boolean>(account.allow_claude_gpt ?? false)
  const [enableCreditOverages, setEnableCreditOverages] = useState<boolean>(
    account.enable_credit_overages ?? false
  )
  const [password, setPassword] = useState(account.password || '')
  const [showPassword, setShowPassword] = useState(false)
  const [totpSecret, setTotpSecret] = useState(account.totp_secret || '')
  const [showTotp, setShowTotp] = useState(false)
  const [refreshToken, setRefreshToken] = useState(account.refresh_token || '')
  const [showOAuth, setShowOAuth] = useState(false)
  const [isExtractingOAuth, setIsExtractingOAuth] = useState(false)
  const [oauthAuthUrl, setOauthAuthUrl] = useState<string | null>(null)
  const [copiedOAuthUrl, setCopiedOAuthUrl] = useState(false)
  const [oauthSuccessMsg, setOauthSuccessMsg] = useState<string | null>(null)
  const oauthAbortControllerRef = useRef<AbortController | null>(null)
  const status = account.status || (account.is_active ? 'ACTIVE' : 'STANDBY')
  const [notes, setNotes] = useState(account.notes || '')
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const RIGHT_ACTION_WIDTH = '172px'
  const CONTROL_HEIGHT = '36px'

  const isQuotaConnected = Boolean(
    (!isNewAccount &&
      (account.is_active ||
        account.status === 'ACTIVE' ||
        Boolean(account.refresh_token?.trim()) ||
        (Boolean(account.reset_horizon_text?.trim()) &&
          account.reset_horizon_text !== 'Not Polled'))) ||
      Boolean(oauthSuccessMsg)
  )

  const headerDisplay = getAccountHeaderDisplay(isNewAccount, label, email)

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

  useEffect(() => {
    return () => {
      if (oauthAbortControllerRef.current) {
        oauthAbortControllerRef.current.abort()
        oauthAbortControllerRef.current = null
        api.cancelGoogleOAuth().catch(() => {})
      }
    }
  }, [])

  const handleClose = () => {
    if (oauthAbortControllerRef.current) {
      oauthAbortControllerRef.current.abort()
      oauthAbortControllerRef.current = null
      api.cancelGoogleOAuth().catch(() => {})
    }
    onClose()
  }

  const handleCancelGoogleOAuth = async () => {
    if (oauthAbortControllerRef.current) {
      oauthAbortControllerRef.current.abort()
      oauthAbortControllerRef.current = null
    }
    setIsExtractingOAuth(false)
    setOauthAuthUrl(null)
    setCopiedOAuthUrl(false)
    try {
      await api.cancelGoogleOAuth()
    } catch {}
  }

  const handleCopyOAuthUrl = async () => {
    let urlToCopy = oauthAuthUrl
    if (!urlToCopy) {
      try {
        const info = await api.getGoogleOAuthURL()
        if (info && info.auth_url) {
          urlToCopy = info.auth_url
          setOauthAuthUrl(info.auth_url)
        }
      } catch {}
    }
    if (urlToCopy) {
      try {
        await navigator.clipboard.writeText(urlToCopy)
        setCopiedOAuthUrl(true)
        setOauthSuccessMsg('Login address copied to clipboard! Paste it into your fingerprint browser to complete sign-in.')
        setTimeout(() => setCopiedOAuthUrl(false), 3000)
      } catch (err: any) {
        setError('Failed to copy to clipboard: ' + (err?.message || err))
      }
    } else {
      setError('Login address is generating. Please click again in a moment.')
    }
  }

  const handleExtractGoogleOAuth = async () => {
    if (isExtractingOAuth) {
      await handleCopyOAuthUrl()
      return
    }
    setIsExtractingOAuth(true)
    setError(null)
    setOauthSuccessMsg(null)
    setOauthAuthUrl(null)
    setCopiedOAuthUrl(false)
    const controller = new AbortController()
    oauthAbortControllerRef.current = controller

    // Asynchronously poll for the generated auth URL so it is immediately ready for clipboard copying
    ;(async () => {
      for (let i = 0; i < 20; i++) {
        if (controller.signal.aborted) break
        try {
          const info = await api.getGoogleOAuthURL()
          if (info && info.auth_url) {
            setOauthAuthUrl(info.auth_url)
            break
          }
        } catch {}
        await new Promise((r) => setTimeout(r, 100))
      }
    })()

    try {
      const res = await api.startGoogleOAuth(controller.signal)
      if (res.success && res.refresh_token) {
        setRefreshToken(res.refresh_token)
        if (res.email && !email.trim()) {
          setEmail(res.email)
          if (!label.trim() || aliasAutoFilled) {
            setLabel(res.email)
            setAliasAutoFilled(true)
          }
        }
        setOauthSuccessMsg(`Extracted token successfully for ${res.email || email || account.email}`)
      } else if (!controller.signal.aborted) {
        setError(res.error || 'Failed to extract OAuth token from Google')
      }
    } catch (err: any) {
      if (controller.signal.aborted || err.name === 'AbortError') {
        return
      }
      setError(err.message || 'Google OAuth extraction failed or timed out')
    } finally {
      if (oauthAbortControllerRef.current === controller) {
        oauthAbortControllerRef.current = null
        setIsExtractingOAuth(false)
        setOauthAuthUrl(null)
      }
    }
  }

  const handleEmailBlur = () => {
    const cleanEmail = email.trim()
    if (cleanEmail && (!label.trim() || aliasAutoFilled)) {
      setLabel(cleanEmail)
      setAliasAutoFilled(true)
    }
  }

  const handleSave = async () => {
    setIsSaving(true)
    setError(null)
    try {
      const finalEmail = email.trim() || account.email
      const finalLabel = resolveDefaultAlias(label, email)
      await api.updateAccount({
        email: finalEmail,
        label: finalLabel,
        plan_tier: account.plan_tier || '',
        status: account.status || status,
        priority: priority,
        password: password,
        notes: notes.trim(),
        totp_secret: normalizeMfaSecret(totpSecret).toUpperCase(),
        refresh_token: refreshToken.trim(),
        credits: account.credits !== undefined && account.credits !== null ? account.credits : 0,
        enable_credit_overages: enableCreditOverages,
        allow_claude_gpt: allowClaudeGpt,
      })
      onSaved()
      handleClose()
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
      const targetEmail = account.email || email.trim()
      await api.deleteAccount(targetEmail)
      onSaved()
      handleClose()
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
      onClick={handleClose}
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
              <h2
                style={{
                  fontSize: '18px',
                  fontWeight: 700,
                  color: headerDisplay.isPlaceholder ? 'var(--text-muted)' : 'var(--text)',
                }}
              >
                {headerDisplay.text}
              </h2>
              {headerDisplay.showStatusBadge && (
                <>
                  <span
                    className={`badge-chip ${
                      st === 'BANNED'
                        ? 'badge-red'
                        : st === 'ERROR'
                        ? 'badge-yellow'
                        : account.is_active
                        ? 'badge-green'
                        : st === 'COOLDOWN'
                        ? 'badge-blue'
                        : 'badge-neutral'
                    }`}
                  >
                    {account.is_active && st !== 'BANNED' && st !== 'ERROR' ? 'ACTIVE' : st === 'COOLDOWN' ? 'COOL DOWN' : st}
                  </span>
                  {renderPlanTierBadge(account.plan_tier)}
                  {account.credits !== undefined && account.credits !== null && account.credits > 0 && (
                    <span
                      className="badge-chip badge-neutral"
                      style={{ fontSize: '11px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
                      title={`${account.credits} Available AI Credits`}
                    >
                      {account.credits} Credits
                    </span>
                  )}
                </>
              )}
            </div>
          </div>
          <button
            onClick={handleClose}
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

        {/* Gemini Quota Section */}
        <div
          style={{
            backgroundColor: 'var(--canvas)',
            border: '1px solid var(--border)',
            borderRadius: '12px',
            padding: '16px 20px',
            marginBottom: '24px',
            opacity: isQuotaConnected ? 1 : 0.48,
            filter: isQuotaConnected ? 'none' : 'grayscale(1)',
            pointerEvents: isQuotaConnected ? 'auto' : 'none',
            userSelect: isQuotaConnected ? 'auto' : 'none',
            transition: 'opacity 0.2s ease, filter 0.2s ease',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
              Gemini Quota
            </div>
            {!isQuotaConnected && (
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontStyle: 'italic' }}>
                Pending connection
              </span>
            )}
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                <span style={{ fontWeight: 500, color: 'var(--text)' }}>5H Quota:</span>
                <span style={{ color: 'var(--text-muted)' }}>
                  {isQuotaConnected ? (account.reset_horizon_text || 'Ready') : 'Not connected'}
                </span>
              </div>
              <HorizontalQuotaBar
                fraction={isQuotaConnected ? (account.quota_5h_available ?? account.quota_5h_current ?? 0) : 0}
                disabled={!isQuotaConnected}
                maxWidth="100%"
                title={isQuotaConnected ? (account.reset_horizon_text || 'Resets in 5h cycle') : 'Pending login and connection'}
              />
            </div>

            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                <span style={{ fontWeight: 500, color: 'var(--text)' }}>Weekly Quota:</span>
                <span style={{ color: 'var(--text-muted)' }}>
                  {isQuotaConnected ? (account.reset_horizon_weekly_text || account.reset_horizon_text || 'Ready') : 'Not connected'}
                </span>
              </div>
              <HorizontalQuotaBar
                fraction={isQuotaConnected ? (account.quota_weekly ?? 0) : 0}
                disabled={!isQuotaConnected}
                maxWidth="100%"
                title={isQuotaConnected ? (account.reset_horizon_weekly_text || 'Resets on 7-day rolling cycle') : 'Pending login and connection'}
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
              onBlur={handleEmailBlur}
              style={{ width: '100%', height: CONTROL_HEIGHT, boxSizing: 'border-box' }}
            />
          </div>

          {/* Account Alias and Account Priority on the same row/level with aligned right-column width */}
          <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-end' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                <Tag size={14} /> Account Alias:
              </label>
              <input
                type="text"
                placeholder="e.g. Account 1, Primary, Backup"
                value={label}
                onChange={(e) => {
                  setLabel(e.target.value)
                  setAliasAutoFilled(false)
                }}
                style={{ width: '100%', height: CONTROL_HEIGHT, boxSizing: 'border-box' }}
              />
            </div>

            <div style={{ width: RIGHT_ACTION_WIDTH, flexShrink: 0 }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                Account Priority:
              </label>
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value)}
                style={{
                  width: '100%',
                  height: CONTROL_HEIGHT,
                  boxSizing: 'border-box',
                  backgroundColor: 'var(--canvas)',
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '0 12px',
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

          {/* Optional Password field (hidden by default with eye toggle) */}
          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
              <Lock size={14} /> {ACCOUNT_SETUP_TEXTS.PASSWORD_LABEL}
            </label>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
              <input
                type={showPassword ? 'text' : 'password'}
                placeholder={ACCOUNT_SETUP_TEXTS.PASSWORD_PLACEHOLDER}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                style={{ width: '100%', height: CONTROL_HEIGHT, boxSizing: 'border-box', paddingRight: '40px' }}
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
              <RefreshCw size={14} /> {ACCOUNT_SETUP_TEXTS.OAUTH_REFRESH_LABEL}
            </label>
            <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
              <div style={{ position: 'relative', flex: 1, minWidth: 0, display: 'flex', alignItems: 'center' }}>
                <input
                  type={showOAuth ? 'text' : 'password'}
                  placeholder={ACCOUNT_SETUP_TEXTS.OAUTH_REFRESH_PLACEHOLDER}
                  value={refreshToken}
                  onChange={(e) => setRefreshToken(e.target.value)}
                  style={{ width: '100%', height: CONTROL_HEIGHT, boxSizing: 'border-box', fontFamily: 'monospace', paddingRight: '40px' }}
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
              {isExtractingOAuth ? (
                <div style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', width: RIGHT_ACTION_WIDTH, flexShrink: 0 }}>
                  <button
                    type="button"
                    onClick={handleExtractGoogleOAuth}
                    className="btn-pill-outlined"
                    style={{
                      flex: 1,
                      height: CONTROL_HEIGHT,
                      boxSizing: 'border-box',
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      gap: '4px',
                      whiteSpace: 'nowrap',
                      padding: '0 6px',
                      fontSize: '11px',
                      fontWeight: 600,
                      backgroundColor: copiedOAuthUrl ? 'rgba(52, 168, 83, 0.08)' : 'var(--primary-light)',
                      color: copiedOAuthUrl ? '#188038' : 'var(--primary)',
                      borderColor: copiedOAuthUrl ? '#188038' : 'var(--primary)',
                      cursor: 'pointer',
                    }}
                    title={copiedOAuthUrl ? 'Login address copied!' : 'Waiting for response. Click again to copy login address for fingerprint browser.'}
                  >
                    {copiedOAuthUrl ? <Check size={12} /> : <Copy size={12} />}
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {copiedOAuthUrl ? 'Copied Address!' : 'Waiting for response...'}
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={handleCancelGoogleOAuth}
                    className="btn-pill-outlined"
                    style={{
                      width: '28px',
                      height: CONTROL_HEIGHT,
                      boxSizing: 'border-box',
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      padding: 0,
                      flexShrink: 0,
                      color: '#d93025',
                      borderColor: '#fce8e6',
                      backgroundColor: '#fdf2f2',
                      cursor: 'pointer',
                    }}
                    title="Cancel Google sign in"
                  >
                    <X size={13} />
                  </button>
                </div>
              ) : (
                <button
                  type="button"
                  onClick={handleExtractGoogleOAuth}
                  className="btn-pill-outlined"
                  style={{
                    width: RIGHT_ACTION_WIDTH,
                    flexShrink: 0,
                    height: CONTROL_HEIGHT,
                    boxSizing: 'border-box',
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    gap: '6px',
                    whiteSpace: 'nowrap',
                    padding: '0 12px',
                    fontSize: '12px',
                    fontWeight: 600,
                    backgroundColor: 'var(--primary-light)',
                    color: 'var(--primary)',
                    borderColor: 'var(--primary)',
                    cursor: 'pointer',
                  }}
                  title="Open browser to login with Google and extract token"
                >
                  <LogIn size={14} />
                  Sign in with Google
                </button>
              )}
            </div>
            {isExtractingOAuth && (
              <div
                style={{
                  marginTop: '6px',
                  fontSize: '11px',
                  color: 'var(--text-muted)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                }}
              >
                <Copy size={11} />
                <span>
                  Waiting for response. Click <strong>Waiting for response...</strong> to copy the login URL for your fingerprint browser.
                </span>
              </div>
            )}
          </div>

          {/* MFA / TOTP Secret Key with input-level verification code timer, shower and copier gadget */}
          <div>
            <label
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                fontSize: '12px',
                fontWeight: 600,
                color: 'var(--text-muted)',
                marginBottom: '8px',
              }}
            >
              <KeyRound size={14} /> MFA Secret Key:
            </label>
            <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
              <div style={{ position: 'relative', flex: 1, minWidth: 0, display: 'flex', alignItems: 'center' }}>
                <input
                  type={showTotp ? 'text' : 'password'}
                  placeholder="e.g. JBSWY3DPEHPK3PXP"
                  value={totpSecret}
                  onChange={(e) => setTotpSecret(e.target.value)}
                  style={{ width: '100%', height: CONTROL_HEIGHT, boxSizing: 'border-box', fontFamily: 'monospace', paddingRight: '40px' }}
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

              {/* Right end: MFA Code Timer, Shower and Copier button set aligned with Google button & Priority */}
              <div
                style={{
                  width: RIGHT_ACTION_WIDTH,
                  flexShrink: 0,
                  height: CONTROL_HEIGHT,
                  boxSizing: 'border-box',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                }}
              >
                {/* Blue circular countdown progress ring */}
                <div
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: '28px',
                    height: '28px',
                    position: 'relative',
                    flexShrink: 0,
                  }}
                  title={
                    derivedCode
                      ? `${remainingSeconds}s remaining until code refreshes`
                      : 'No MFA secret configured'
                  }
                >
                  <svg width="28" height="28" viewBox="0 0 28 28" style={{ overflow: 'visible' }}>
                    <circle
                      cx="14"
                      cy="14"
                      r="11"
                      fill="none"
                      stroke={derivedCode ? 'rgba(26, 115, 232, 0.16)' : 'var(--border)'}
                      strokeWidth="2.4"
                    />
                    {derivedCode && (
                      <circle
                        cx="14"
                        cy="14"
                        r="11"
                        fill="none"
                        stroke="#1a73e8"
                        strokeWidth="2.4"
                        strokeLinecap="round"
                        strokeDasharray={69.12}
                        strokeDashoffset={69.12 * (1 - remainingSeconds / 30)}
                        transform="rotate(-90 14 14)"
                        style={{ transition: 'stroke-dashoffset 0.8s linear' }}
                      />
                    )}
                    <text
                      x="14"
                      y="14"
                      textAnchor="middle"
                      dominantBaseline="central"
                      fontSize="9.5"
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
                    flex: 1,
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    border: '1px solid var(--border)',
                    borderRadius: '8px',
                    overflow: 'hidden',
                    backgroundColor: derivedCode ? 'var(--green-bg, rgba(52, 168, 83, 0.08))' : 'var(--hover)',
                    height: CONTROL_HEIGHT,
                    boxSizing: 'border-box',
                    boxShadow: '0 1px 2px rgba(0,0,0,0.03)',
                  }}
                >
                  <span
                    style={{
                      flex: 1,
                      textAlign: 'center',
                      fontFamily: 'monospace',
                      fontSize: '12px',
                      fontWeight: derivedCode ? 700 : 500,
                      letterSpacing: '1px',
                      color: derivedCode ? 'var(--green, #34a853)' : 'var(--text-muted)',
                      padding: '0 6px',
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
                      width: '32px',
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
                    {copiedTotp ? <Check size={14} /> : <Copy size={14} />}
                  </button>
                </div>
              </div>
            </div>
          </div>

          {/* Claude & GPT Models and AI Credits Toggles (no header, toggle right of text, vertical divider between) */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '16px',
              padding: '4px 0',
            }}
          >
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                flex: 1,
                gap: '12px',
              }}
            >
              <span style={{ fontSize: '13px', color: 'var(--text)', fontWeight: 500 }}>
                Allow Claude &amp; GPT models
              </span>
              <ToggleSwitch
                checked={allowClaudeGpt}
                onChange={(val) => setAllowClaudeGpt(val)}
              />
            </div>

            <div
              aria-hidden="true"
              style={{
                width: '1px',
                height: '24px',
                backgroundColor: 'var(--border)',
                flexShrink: 0,
              }}
            />

            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                flex: 1,
                gap: '12px',
              }}
            >
              <span style={{ fontSize: '13px', color: 'var(--text)', fontWeight: 500 }}>
                Allow using AI credits
              </span>
              <ToggleSwitch
                checked={enableCreditOverages}
                onChange={(val) => setEnableCreditOverages(val)}
              />
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
          {(!isNewAccount || Boolean(email.trim())) ? (
            <button
              onClick={handleDelete}
              disabled={isSaving}
              className="btn-pill-danger"
            >
              <Trash2 size={15} /> Delete Account
            </button>
          ) : (
            <div />
          )}

          <div style={{ display: 'flex', gap: '10px' }}>
            <button
              onClick={handleClose}
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
