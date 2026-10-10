import React, { useState, useEffect, useRef } from 'react'
import { X, Trash2, Save, KeyRound, Tag, RefreshCw, Eye, EyeOff, Lock, LogIn, FileText, ShieldAlert, Copy, Check, Mail, Link, ExternalLink } from 'lucide-react'
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
  parseAccountErrorAlert,
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
  const [extractedAccessToken, setExtractedAccessToken] = useState<string>('')
  const [showOAuth, setShowOAuth] = useState(false)
  const [isExtractingOAuth, setIsExtractingOAuth] = useState(false)
  const [showManualCallbackField, setShowManualCallbackField] = useState(false)
  const [manualCallbackUrl, setManualCallbackUrl] = useState('')
  const [isExchangingOAuth, setIsExchangingOAuth] = useState(false)
  const [oauthSuccessMsg, setOauthSuccessMsg] = useState<string | null>(null)
  const oauthAbortControllerRef = useRef<AbortController | null>(null)
  const [currentStatus, setCurrentStatus] = useState<string>(account.status || (account.is_active ? 'ACTIVE' : 'STANDBY'))
  const [liveErrorMessage, setLiveErrorMessage] = useState<string | null>(account.error_message || null)
  const [copiedVerificationUrl, setCopiedVerificationUrl] = useState(false)
  const [notes, setNotes] = useState(account.notes || '')
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const [liveQuota5h, setLiveQuota5h] = useState<number | null>(
    account.quota_5h_available ?? account.quota_5h_current ?? null
  )
  const [liveQuotaWeekly, setLiveQuotaWeekly] = useState<number | null>(
    account.quota_weekly ?? null
  )
  const [liveResetHorizon, setLiveResetHorizon] = useState<string | null>(
    account.reset_horizon_text || null
  )
  const [liveResetHorizonWeekly, setLiveResetHorizonWeekly] = useState<string | null>(
    account.reset_horizon_weekly_text || null
  )
  const [currentPlanTier, setCurrentPlanTier] = useState<string>(account.plan_tier || '')
  const [isRefreshingQuota, setIsRefreshingQuota] = useState(false)

  const RIGHT_ACTION_WIDTH = '172px'
  const CONTROL_HEIGHT = '36px'

  const isAccountInError = Boolean(
    currentStatus?.toUpperCase() === 'ERROR' ||
    currentStatus?.toUpperCase() === 'BANNED' ||
    currentStatus?.toUpperCase() === 'NEEDS_REAUTH' ||
    account.status?.toUpperCase() === 'ERROR' ||
    account.status?.toUpperCase() === 'BANNED' ||
    account.status?.toUpperCase() === 'NEEDS_REAUTH' ||
    Boolean(account.error_message) ||
    Boolean(liveErrorMessage) ||
    (!isNewAccount && !account.refresh_token?.trim() && !refreshToken?.trim())
  )

  const isQuotaConnected = Boolean(
    !isAccountInError &&
      (liveQuota5h !== null ||
        (!isNewAccount &&
          (account.is_active ||
            currentStatus === 'ACTIVE' ||
            account.status === 'ACTIVE' ||
            Boolean(account.refresh_token?.trim()) ||
            Boolean(refreshToken?.trim()) ||
            (Boolean(account.reset_horizon_text?.trim()) &&
              account.reset_horizon_text !== 'Not Polled' &&
              account.reset_horizon_text !== 'Error'))) ||
        Boolean(oauthSuccessMsg))
  )

  const headerDisplay = getAccountHeaderDisplay(isNewAccount, label, email)

  const lastVerificationUrlFetchedAtRef = useRef<number>(0)
  const [isWaitingForVerification, setIsWaitingForVerification] = useState(false)
  const isWaitingForVerificationRef = useRef<boolean>(false)

  const applyQuotaSummaryUpdate = (q: any, isLiveRefresh: boolean = false) => {
    if (!q) return
    if (q.quota_5h_fraction !== undefined) setLiveQuota5h(q.quota_5h_fraction)
    if (q.quota_weekly_fraction !== undefined) setLiveQuotaWeekly(q.quota_weekly_fraction)
    if (q.reset_horizon_text) setLiveResetHorizon(q.reset_horizon_text)
    if (q.reset_horizon_weekly_text) setLiveResetHorizonWeekly(q.reset_horizon_weekly_text)
    if (q.plan_tier) {
      account.plan_tier = q.plan_tier
      setCurrentPlanTier(q.plan_tier)
    }
    if (q.error_message) {
      setLiveErrorMessage(q.error_message)
      account.error_message = q.error_message
      if (isLiveRefresh) {
        lastVerificationUrlFetchedAtRef.current = Date.now()
      }
    } else {
      setLiveErrorMessage(null)
      account.error_message = ''
    }
    if (q.error_status) {
      setCurrentStatus(q.error_status)
      account.status = q.error_status
    } else if (
      (currentStatus === 'ERROR' || currentStatus === 'BANNED') &&
      (!q.reset_horizon_text ||
        (q.reset_horizon_text !== 'Not Polled' && !q.reset_horizon_text.startsWith('Error')))
    ) {
      const nextSt = account.is_active ? 'ACTIVE' : 'STANDBY'
      setCurrentStatus(nextSt)
      account.status = nextSt
    }
  }

  const handleRefreshLiveQuota = async (): Promise<any> => {
    const targetEmail = email.trim() || account.email
    if (!targetEmail) return null
    setIsRefreshingQuota(true)
    setError(null)
    try {
      const q = await api.refreshAccountQuota(targetEmail)
      if (q) {
        const hadError = (currentStatus || '').toUpperCase() === 'ERROR' || (account.status || '').toUpperCase() === 'ERROR'
        const isRecovered =
          !q.error_status &&
          !q.error_message &&
          q.reset_horizon_text !== 'Not Polled' &&
          !q.reset_horizon_text?.startsWith('Error')
        applyQuotaSummaryUpdate(q, true)
        if (hadError || !q.error_message) {
          onSaved()
        }
        if (hadError && isRecovered) {
          setIsWaitingForVerification(false)
          if (isWaitingForVerificationRef.current) {
            isWaitingForVerificationRef.current = false
            onClose()
          }
        }
      }
      return q
    } catch (err: any) {
      console.warn('Quota refresh warning:', err)
      return null
    } finally {
      setIsRefreshingQuota(false)
    }
  }

  const resolveFreshVerificationUrl = async (): Promise<string | null> => {
    const targetEmail = email.trim() || account.email
    const currentAlert = parseAccountErrorAlert(
      liveErrorMessage || account.error_message || account.status_reason,
      targetEmail
    )
    if (Date.now() - lastVerificationUrlFetchedAtRef.current <= 45_000 && currentAlert.verificationUrl) {
      return currentAlert.verificationUrl
    }
    const q = await handleRefreshLiveQuota()
    if (q && !(q as any).error_status && !(q as any).error_message) {
      return null
    }
    const updatedAlert = parseAccountErrorAlert(
      (q as any)?.error_message || liveErrorMessage || account.error_message || account.status_reason,
      targetEmail
    )
    return updatedAlert.verificationUrl
  }

  useEffect(() => {
    if (!isNewAccount && account.email && (account.refresh_token || account.access_token)) {
      if (account.status?.toUpperCase() === 'ERROR') {
        handleRefreshLiveQuota()
      } else if (
        account.reset_horizon_text === 'Not Polled' ||
        liveQuota5h === null ||
        (account.status?.toUpperCase() === 'BANNED' && !account.error_message)
      ) {
        api.getQuotaSummary(account.email).then((q) => {
          if (q) {
            applyQuotaSummaryUpdate(q, false)
          }
        }).catch(() => {})
      }
    }
  }, [account.email])

  useEffect(() => {
    if (isNewAccount || (currentStatus || '').toUpperCase() !== 'ERROR') return
    const checkRecovery = () => {
      if (!isRefreshingQuota) {
        handleRefreshLiveQuota()
      }
    }
    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        checkRecovery()
      }
    }
    window.addEventListener('focus', checkRecovery)
    document.addEventListener('visibilitychange', onVisibilityChange)
    const interval = setInterval(checkRecovery, isWaitingForVerification ? 2500 : 4000)
    return () => {
      window.removeEventListener('focus', checkRecovery)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      clearInterval(interval)
    }
  }, [isNewAccount, currentStatus, isRefreshingQuota, isWaitingForVerification, email, account.email])

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
    setShowManualCallbackField(false)
    setManualCallbackUrl('')
    setIsExchangingOAuth(false)
    onClose()
  }

  const handleCancelGoogleOAuth = async () => {
    if (oauthAbortControllerRef.current) {
      oauthAbortControllerRef.current.abort()
      oauthAbortControllerRef.current = null
    }
    setIsExtractingOAuth(false)
    if (!manualCallbackUrl.trim()) {
      setShowManualCallbackField(false)
    }
    try {
      await api.cancelGoogleOAuth()
    } catch {}
  }

  const persistExtractedOAuthIfExisting = async (
    newRefreshToken: string,
    newAccessToken?: string,
    extractedEmail?: string
  ) => {
    const targetEmail = (email.trim() || account.email || extractedEmail || '').trim()
    if (isNewAccount || !targetEmail) return
    if (extractedEmail && extractedEmail.toLowerCase() !== targetEmail.toLowerCase()) return

    try {
      const finalLabel = resolveDefaultAlias(label, targetEmail)
      const targetStatus =
        currentStatus === 'ERROR' || currentStatus === 'BANNED'
          ? account.is_active
            ? 'ACTIVE'
            : 'STANDBY'
          : currentStatus || (account.is_active ? 'ACTIVE' : 'STANDBY')
      const updateRes = await api.updateAccount({
        email: targetEmail,
        label: finalLabel,
        plan_tier: currentPlanTier || account.plan_tier || '',
        status: targetStatus,
        priority: priority,
        password: password,
        notes: notes.trim(),
        totp_secret: normalizeMfaSecret(totpSecret).toUpperCase(),
        refresh_token: newRefreshToken.trim(),
        access_token: newAccessToken?.trim() || undefined,
        credits: account.credits !== undefined && account.credits !== null ? account.credits : 0,
        enable_credit_overages: enableCreditOverages,
        allow_claude_gpt: allowClaudeGpt,
      })
      if (updateRes?.quota) {
        applyQuotaSummaryUpdate(updateRes.quota)
      } else {
        if ((updateRes as any)?.status) {
          setCurrentStatus((updateRes as any).status)
          account.status = (updateRes as any).status
        }
        if ((updateRes as any)?.error_message !== undefined) {
          const msg = (updateRes as any).error_message || null
          setLiveErrorMessage(msg)
          account.error_message = msg || ''
        }
      }
      onSaved()
    } catch (err) {
      console.warn('Auto-persist OAuth token warning:', err)
    }
  }

  const handleExtractGoogleOAuth = async () => {
    if (isExtractingOAuth) {
      await handleCancelGoogleOAuth()
      return
    }
    setIsExtractingOAuth(true)
    setShowManualCallbackField(true)
    setError(null)
    setOauthSuccessMsg(null)
    const controller = new AbortController()
    oauthAbortControllerRef.current = controller

    try {
      const res = await api.startGoogleOAuth(controller.signal)
      if (res.success && res.refresh_token) {
        setRefreshToken(res.refresh_token)
        if (res.access_token) {
          setExtractedAccessToken(res.access_token)
        }
        if (res.email && !email.trim()) {
          setEmail(res.email)
          if (!label.trim() || aliasAutoFilled) {
            setLabel(res.email)
            setAliasAutoFilled(true)
          }
        }
        setOauthSuccessMsg(`Extracted refresh token successfully for ${res.email || email || account.email}`)
        setShowManualCallbackField(false)
        setManualCallbackUrl('')
        await persistExtractedOAuthIfExisting(res.refresh_token, res.access_token, res.email)
      } else if (!controller.signal.aborted) {
        setError(res.error || 'Failed to extract OAuth refresh token from Google')
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
      }
    }
  }

  const handleExchangeManualCallback = async (overrideUrl?: string) => {
    const urlToExchange = (overrideUrl !== undefined ? overrideUrl : manualCallbackUrl).trim()
    if (!urlToExchange) return

    setIsExchangingOAuth(true)
    setError(null)
    setOauthSuccessMsg(null)

    try {
      const res = await api.exchangeGoogleOAuth({ callback_url: urlToExchange })
      if (res.success && res.refresh_token) {
        setRefreshToken(res.refresh_token)
        if (res.access_token) {
          setExtractedAccessToken(res.access_token)
        }
        if (res.email && !email.trim()) {
          setEmail(res.email)
          if (!label.trim() || aliasAutoFilled) {
            setLabel(res.email)
            setAliasAutoFilled(true)
          }
        }
        setOauthSuccessMsg(`Extracted refresh token successfully for ${res.email || email || account.email}`)
        setShowManualCallbackField(false)
        setManualCallbackUrl('')
        setIsExtractingOAuth(false)
        if (oauthAbortControllerRef.current) {
          oauthAbortControllerRef.current.abort()
          oauthAbortControllerRef.current = null
        }
        api.cancelGoogleOAuth().catch(() => {})
        await persistExtractedOAuthIfExisting(res.refresh_token, res.access_token, res.email)
      } else {
        setError(res.error || 'Failed to extract refresh token from return URL')
      }
    } catch (err: any) {
      setError(err.message || 'Token exchange failed')
    } finally {
      setIsExchangingOAuth(false)
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
      const targetStatus = (currentStatus === 'ERROR' || currentStatus === 'BANNED')
        ? (account.is_active ? 'ACTIVE' : 'STANDBY')
        : (currentStatus || (account.is_active ? 'ACTIVE' : 'STANDBY'))
      const res = await api.updateAccount({
        email: finalEmail,
        label: finalLabel,
        plan_tier: currentPlanTier || account.plan_tier || '',
        status: targetStatus,
        priority: priority,
        password: password,
        notes: notes.trim(),
        totp_secret: normalizeMfaSecret(totpSecret).toUpperCase(),
        refresh_token: refreshToken.trim(),
        access_token: extractedAccessToken.trim() || undefined,
        credits: account.credits !== undefined && account.credits !== null ? account.credits : 0,
        enable_credit_overages: enableCreditOverages,
        allow_claude_gpt: allowClaudeGpt,
      })
      if (res && res.quota) {
        applyQuotaSummaryUpdate(res.quota)
      } else if (res && res.plan_tier) {
        account.plan_tier = res.plan_tier
        setCurrentPlanTier(res.plan_tier)
      }
      await onSaved()
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

  const st = (
    currentStatus?.toUpperCase() === 'BANNED' || account.status?.toUpperCase() === 'BANNED'
      ? 'BANNED'
      : isAccountInError
      ? 'ERROR'
      : currentStatus || (account.is_active ? 'ACTIVE' : 'STANDBY')
  ).toUpperCase()

  const errorAlert = parseAccountErrorAlert(
    liveErrorMessage || account.error_message || account.status_reason || (isAccountInError ? 'Missing credentials / re-authentication required' : ''),
    email.trim() || account.email
  )

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
          borderRadius: '10px',
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
                        : st === 'COOLDOWN' || st === 'COOLING'
                        ? 'badge-blue'
                        : 'badge-neutral'
                    }`}
                  >
                    {st === 'BANNED' ? 'BANNED' : st === 'ERROR' ? 'ERROR' : account.is_active ? 'ACTIVE' : (st === 'COOLDOWN' || st === 'COOLING') ? 'COOLING' : st}
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
              padding: '12px 16px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '16px',
              display: 'flex',
              flexDirection: 'column',
              gap: '6px',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600 }}>
              <ShieldAlert size={16} />
              <span>Account Suspended Upstream (Banned)</span>
            </div>
            <div style={{ fontSize: '11px', opacity: 0.95, paddingLeft: '24px', lineHeight: 1.4, wordBreak: 'break-word', fontFamily: 'monospace' }}>
              {liveErrorMessage || account.error_message || account.status_reason || 'This account has been flagged or disabled upstream by Google (e.g. TOS_VIOLATION). Quota requests cannot be serviced.'}
            </div>
          </div>
        )}

        {st === 'ERROR' && (
          <div
            style={{
              backgroundColor: 'var(--yellow-bg)',
              color: 'var(--yellow)',
              padding: '12px 16px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '16px',
              display: 'flex',
              flexDirection: 'column',
              gap: '8px',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600 }}>
              <ShieldAlert size={16} />
              <span>
                {errorAlert.isValidationRequired
                  ? 'Google Account Verification Required (VALIDATION_REQUIRED)'
                  : 'Authentication / Quota Error'}
              </span>
            </div>
            <div style={{ fontSize: '11px', opacity: 0.95, paddingLeft: '24px', lineHeight: 1.45, wordBreak: 'break-word', fontFamily: 'monospace' }}>
              {errorAlert.summaryText}
            </div>
            {errorAlert.isValidationRequired && (
              <div style={{ fontSize: '11.5px', paddingLeft: '24px', lineHeight: 1.45, color: 'var(--text)' }}>
                Google Cloud Code API requires a one-time security verification in the browser for{' '}
                <strong>{email.trim() || account.email}</strong> (standard OAuth sign-in alone does not clear this challenge).
              </div>
            )}
            <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '8px', paddingLeft: '24px', marginTop: '2px' }}>
              {errorAlert.verificationUrl && (
                <>
                  <a
                    href={errorAlert.verificationUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={async (e) => {
                      setIsWaitingForVerification(true)
                      isWaitingForVerificationRef.current = true
                      const isStale = Date.now() - lastVerificationUrlFetchedAtRef.current > 45_000
                      const electronOpen = (window as any).electronAPI?.openExternal
                      if (electronOpen || isStale) {
                        e.preventDefault()
                        const freshUrl = (await resolveFreshVerificationUrl()) || errorAlert.verificationUrl
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
                      padding: '5px 12px',
                      borderRadius: '9999px',
                      backgroundColor: '#1a73e8',
                      color: '#ffffff',
                      fontSize: '11.5px',
                      fontWeight: 600,
                      textDecoration: 'none',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    <ExternalLink size={12} />
                    <span>Verify Account in Browser</span>
                  </a>
                  <button
                    type="button"
                    onClick={async () => {
                      const freshUrl = (await resolveFreshVerificationUrl()) || errorAlert.verificationUrl
                      if (!freshUrl) return
                      navigator.clipboard.writeText(freshUrl)
                      setCopiedVerificationUrl(true)
                      setTimeout(() => setCopiedVerificationUrl(false), 1800)
                    }}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '5px',
                      padding: '4px 10px',
                      borderRadius: '9999px',
                      backgroundColor: '#ffffff',
                      border: '1px solid var(--border)',
                      color: 'var(--text)',
                      fontSize: '11.5px',
                      fontWeight: 500,
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                    title="Copy verification URL to open in the browser profile signed into this Google account"
                  >
                    {copiedVerificationUrl ? <Check size={12} color="var(--green)" /> : <Copy size={12} />}
                    <span>{copiedVerificationUrl ? 'Copied Link' : 'Copy Verification Link'}</span>
                  </button>
                </>
              )}
              <button
                type="button"
                onClick={handleRefreshLiveQuota}
                disabled={isRefreshingQuota}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '5px',
                  padding: '4px 10px',
                  borderRadius: '9999px',
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  color: 'var(--text)',
                  fontSize: '11.5px',
                  fontWeight: 500,
                  cursor: isRefreshingQuota ? 'wait' : 'pointer',
                  whiteSpace: 'nowrap',
                }}
              >
                <RefreshCw size={12} className={isRefreshingQuota ? 'spin' : ''} />
                <span>{isRefreshingQuota ? 'Checking...' : 'Re-check Quota'}</span>
              </button>
            </div>
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
            {!isQuotaConnected ? (
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontStyle: 'italic' }}>
                Pending connection
              </span>
            ) : (
              <button
                type="button"
                onClick={handleRefreshLiveQuota}
                disabled={isRefreshingQuota || isSaving}
                title="Fetch and refresh live quota"
                style={{
                  background: 'none',
                  border: 'none',
                  cursor: isRefreshingQuota ? 'wait' : 'pointer',
                  padding: '2px 6px',
                  color: 'var(--primary)',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '4px',
                  fontSize: '11px',
                  fontWeight: 600,
                  borderRadius: '4px',
                }}
              >
                <RefreshCw size={12} className={isRefreshingQuota ? 'spin' : ''} />
                <span>{isRefreshingQuota ? 'Refreshing...' : 'Refresh Quota'}</span>
              </button>
            )}
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                <span style={{ fontWeight: 500, color: 'var(--text)' }}>5H Quota:</span>
                <span style={{ color: 'var(--text-muted)' }}>
                  {isQuotaConnected ? (liveResetHorizon || account.reset_horizon_text || 'Ready') : 'Not connected'}
                </span>
              </div>
              <HorizontalQuotaBar
                fraction={isQuotaConnected ? (liveQuota5h ?? (account.quota_5h_available ?? account.quota_5h_current ?? 0)) : 0}
                disabled={!isQuotaConnected}
                maxWidth="100%"
                title={isQuotaConnected ? (liveResetHorizon || account.reset_horizon_text || 'Resets in 5h cycle') : 'Pending login and connection'}
              />
            </div>

            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                <span style={{ fontWeight: 500, color: 'var(--text)' }}>Weekly Quota:</span>
                <span style={{ color: 'var(--text-muted)' }}>
                  {isQuotaConnected ? (liveResetHorizonWeekly || account.reset_horizon_weekly_text || liveResetHorizon || account.reset_horizon_text || 'Ready') : 'Not connected'}
                </span>
              </div>
              <HorizontalQuotaBar
                fraction={isQuotaConnected ? (liveQuotaWeekly ?? (account.quota_weekly ?? 0)) : 0}
                disabled={!isQuotaConnected}
                maxWidth="100%"
                title={isQuotaConnected ? (liveResetHorizonWeekly || account.reset_horizon_weekly_text || 'Resets on 7-day rolling cycle') : 'Pending login and connection'}
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
                  title={showOAuth ? 'Hide OAuth refresh token' : 'Show OAuth refresh token'}
                >
                  {showOAuth ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
              <button
                type="button"
                onClick={isExtractingOAuth ? handleCancelGoogleOAuth : handleExtractGoogleOAuth}
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
                title={isExtractingOAuth ? 'Click to stop waiting' : 'Open browser to login with Google and extract long-term refresh token'}
              >
                {isExtractingOAuth ? (
                  <>
                    <RefreshCw size={13} className="animate-spin" />
                    <span>Waiting for response...</span>
                  </>
                ) : (
                  <>
                    <LogIn size={14} />
                    <span>Sign in with Google</span>
                  </>
                )}
              </button>
            </div>
            {refreshToken.trim().startsWith('ya29.') && (
              <div
                style={{
                  marginTop: '6px',
                  fontSize: '11px',
                  color: 'var(--yellow, #b06000)',
                  lineHeight: 1.4,
                }}
              >
                Notice: Token starts with &apos;ya29&apos; (short-lived 1-hour Access Token). For background quota polling and autonomous rotation, enter a long-term Refresh Token (starts with &apos;1//&apos;) or click &apos;Sign in with Google&apos;.
              </div>
            )}
            {(showManualCallbackField || Boolean(manualCallbackUrl.trim())) && (
              <div style={{ marginTop: '10px' }}>
                <label
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    fontSize: '12px',
                    fontWeight: 600,
                    color: 'var(--text-muted)',
                    marginBottom: '6px',
                  }}
                >
                  <Link size={13} /> Return URL (OAuth Callback):
                </label>
                <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
                  <input
                    type="text"
                    placeholder="Paste return URL (e.g. http://127.0.0.1:.../oauth/callback?code=...)"
                    value={manualCallbackUrl}
                    onChange={(e) => setManualCallbackUrl(e.target.value)}
                    onPaste={(e) => {
                      const pasted = e.clipboardData.getData('text')?.trim()
                      if (pasted && (pasted.includes('code=') || pasted.startsWith('http://') || pasted.startsWith('https://'))) {
                        setManualCallbackUrl(pasted)
                        handleExchangeManualCallback(pasted)
                      }
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        handleExchangeManualCallback()
                      }
                    }}
                    style={{
                      flex: 1,
                      minWidth: 0,
                      height: CONTROL_HEIGHT,
                      boxSizing: 'border-box',
                      fontFamily: 'monospace',
                      fontSize: '11px',
                      padding: '0 10px',
                    }}
                  />
                  <button
                    type="button"
                    onClick={() => handleExchangeManualCallback()}
                    disabled={!manualCallbackUrl.trim() || isExchangingOAuth}
                    className="btn-pill-primary"
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
                      cursor: !manualCallbackUrl.trim() || isExchangingOAuth ? 'not-allowed' : 'pointer',
                      opacity: !manualCallbackUrl.trim() || isExchangingOAuth ? 0.6 : 1,
                    }}
                    title="Exchange return URL for OAuth refresh token"
                  >
                    {isExchangingOAuth ? (
                      <>
                        <RefreshCw size={13} className="animate-spin" />
                        <span>Exchanging...</span>
                      </>
                    ) : (
                      <>
                        <Check size={14} />
                        <span>Extract Token</span>
                      </>
                    )}
                  </button>
                </div>
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
              gap: '8px',
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
              <Save size={15} className={isSaving ? 'spin' : ''} /> {isSaving ? 'Saving & Refreshing...' : 'Save Changes'}
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
