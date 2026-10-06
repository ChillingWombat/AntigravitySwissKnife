import React, { useState, useEffect } from 'react'
import {
  Copy,
  Check,
  Save,
  Trash2,
} from 'lucide-react'
import type { AccountState } from '../types'
import { api } from '../api'

interface MfaVaultPageProps {
  accounts: AccountState[]
  onRefresh: () => void
}

export const MfaVaultPage: React.FC<MfaVaultPageProps> = ({
  accounts,
  onRefresh,
}) => {
  const [selectedAccount, setSelectedAccount] = useState<string>(accounts[0]?.email || '')
  const [totpCode, setTotpCode] = useState<string>('------')
  const [remainingSec, setRemainingSec] = useState<number>(30)
  const [copied, setCopied] = useState<boolean>(false)
  const [secretInput, setSecretInput] = useState<string>('')
  const [backupCodes, setBackupCodes] = useState<string>('')
  const [feedback, setFeedback] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState<boolean>(false)

  // Fetch TOTP every second
  useEffect(() => {
    let timer: any
    const fetchCode = async () => {
      if (!selectedAccount) return
      try {
        const res = await api.getTOTP(selectedAccount)
        setTotpCode(res.code || '------')
        setRemainingSec(res.remaining_seconds || 30)
      } catch (err) {
        setTotpCode('------')
      }
    }

    fetchCode()
    timer = setInterval(fetchCode, 1000)
    return () => clearInterval(timer)
  }, [selectedAccount])

  const handleCopy = () => {
    if (!totpCode || totpCode === '------') return
    navigator.clipboard.writeText(totpCode.replace(/\s+/g, ''))
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  const handleSaveSecret = async () => {
    if (!selectedAccount) return
    setIsSaving(true)
    setFeedback(null)
    try {
      await api.saveTOTP(selectedAccount, secretInput.trim().toUpperCase())
      setFeedback('TOTP Secret saved successfully.')
      onRefresh()
    } catch (err: any) {
      setFeedback(`Save error: ${err.message}`)
    } finally {
      setIsSaving(false)
    }
  }

  const handleClearSecret = async () => {
    if (!selectedAccount) return
    setIsSaving(true)
    setFeedback(null)
    try {
      await api.saveTOTP(selectedAccount, '')
      setSecretInput('')
      setTotpCode('------')
      setFeedback('MFA protection removed.')
      onRefresh()
    } catch (err: any) {
      setFeedback(`Error: ${err.message}`)
    } finally {
      setIsSaving(false)
    }
  }

  // Format code display: 123 456
  const formattedCode =
    totpCode && totpCode.length === 6
      ? `${totpCode.slice(0, 3)} ${totpCode.slice(3)}`
      : totpCode

  const ringRadius = 24
  const ringCircumference = 2 * Math.PI * ringRadius
  const ringOffset = ringCircumference - (remainingSec / 30) * ringCircumference

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. TOP SECTION: Live MFA Authenticator Card */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Live MFA / TOTP Authenticator
        </div>

        <div
          style={{
            backgroundColor: 'var(--canvas)',
            border: '1px solid var(--border)',
            borderRadius: '12px',
            padding: '20px 24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          {/* Ring + Code Display */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '24px' }}>
            {/* 30s Countdown Ring */}
            <div style={{ position: 'relative', width: '56px', height: '56px' }}>
              <svg width="56" height="56" style={{ transform: 'rotate(-90deg)' }}>
                <circle
                  cx="28"
                  cy="28"
                  r={ringRadius}
                  fill="none"
                  stroke="#e5e9f0"
                  strokeWidth="4"
                />
                <circle
                  cx="28"
                  cy="28"
                  r={ringRadius}
                  fill="none"
                  stroke={remainingSec <= 5 ? 'var(--red)' : 'var(--primary)'}
                  strokeWidth="4"
                  strokeDasharray={ringCircumference}
                  strokeDashoffset={ringOffset}
                  strokeLinecap="round"
                  style={{ transition: 'stroke-dashoffset 0.8s linear' }}
                />
              </svg>
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '13px',
                  fontWeight: 700,
                  color: 'var(--text)',
                }}
              >
                {remainingSec}s
              </div>
            </div>

            {/* Account & Code */}
            <div>
              <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '2px' }}>
                Account: {selectedAccount || 'None Selected'}
              </div>
              <div
                style={{
                  fontFamily: "'Roboto Mono', monospace",
                  fontSize: '34px',
                  fontWeight: 700,
                  letterSpacing: '5px',
                  color: 'var(--primary)',
                }}
              >
                {formattedCode}
              </div>
            </div>
          </div>

          {/* Copy Button */}
          <button
            onClick={handleCopy}
            disabled={!totpCode || totpCode === '------'}
            className="btn-pill-tonal"
            style={{ padding: '9px 20px' }}
          >
            {copied ? <Check size={16} /> : <Copy size={16} />}
            {copied ? 'Copied!' : 'Copy Code'}
          </button>
        </div>
      </div>

      {/* 2. MIDDLE SECTION: Registered Accounts Inventory Table */}
      <div className="google-card" style={{ padding: '0px', overflow: 'hidden' }}>
        <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--border)' }}>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Registered Accounts Inventory
          </div>
        </div>

        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              <th style={{ padding: '12px 20px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase' }}>
                Account Email
              </th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase' }}>
                Account Alias
              </th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase' }}>
                MFA Status
              </th>
              <th style={{ padding: '12px 20px', textAlign: 'right', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase' }}>
                Action
              </th>
            </tr>
          </thead>
          <tbody>
            {accounts.map((acc) => {
              const isSel = selectedAccount === acc.email
              return (
                <tr
                  key={acc.email}
                  onClick={() => setSelectedAccount(acc.email)}
                  style={{
                    cursor: 'pointer',
                    backgroundColor: isSel ? 'var(--canvas)' : 'transparent',
                    borderBottom: '1px solid var(--border-subtle)',
                  }}
                >
                  <td style={{ padding: '12px 20px', fontWeight: 600, color: 'var(--text)' }}>
                    {acc.email}
                  </td>
                  <td style={{ padding: '12px 16px', fontSize: '12px', color: 'var(--text-muted)' }}>
                    {acc.label || 'Standard'}
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    <span className={`badge-chip ${acc.has_mfa ? 'badge-green' : 'badge-neutral'}`}>
                      {acc.has_mfa ? 'PROTECTED' : 'NO MFA'}
                    </span>
                  </td>
                  <td style={{ padding: '12px 20px', textAlign: 'right' }}>
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        setSelectedAccount(acc.email)
                      }}
                      className={isSel ? 'btn-pill-primary' : 'btn-pill-tonal'}
                      style={{ padding: '4px 12px', fontSize: '11px' }}
                    >
                      {isSel ? 'Active' : 'Select'}
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* 3. BOTTOM SECTION: Configure TOTP Secret Key & Backup Codes */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Configure TOTP Secret Key
        </div>

        <div style={{ display: 'flex', gap: '12px', alignItems: 'center', marginBottom: '12px' }}>
          <input
            type="text"
            placeholder="Enter Base32 TOTP secret (e.g. JBSWY3DPEHPK3PXP)"
            value={secretInput}
            onChange={(e) => setSecretInput(e.target.value)}
            style={{ flex: 1, fontFamily: 'monospace' }}
          />

          <button
            onClick={handleSaveSecret}
            disabled={isSaving}
            className="btn-pill-primary"
          >
            <Save size={15} /> Save Secret
          </button>

          <button
            onClick={handleClearSecret}
            disabled={isSaving}
            className="btn-pill-danger"
          >
            <Trash2 size={15} /> Remove MFA
          </button>
        </div>

        {feedback && (
          <div style={{ fontSize: '12px', color: 'var(--primary)', marginBottom: '16px', fontWeight: 500 }}>
            {feedback}
          </div>
        )}

        {/* Emergency Backup Codes */}
        <div style={{ marginTop: '16px', borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '8px' }}>
            Emergency Backup Codes (Optional)
          </div>
          <textarea
            placeholder="Store one-time 8-digit backup codes here (one per line)..."
            value={backupCodes}
            onChange={(e) => setBackupCodes(e.target.value)}
            rows={3}
            style={{ width: '100%', fontFamily: 'monospace' }}
          />
        </div>
      </div>
    </div>
  )
}
