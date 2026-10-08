import React, { useState } from 'react'
import { Lock, Eye, EyeOff, KeyRound, ArrowRight } from 'lucide-react'
import { api } from '../api'

interface AppLockScreenProps {
  onUnlocked: () => void
}

export const AppLockScreen: React.FC<AppLockScreenProps> = ({ onUnlocked }) => {
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleUnlock = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    if (!password.trim()) {
      setError('Please enter your password.')
      return
    }

    setIsSubmitting(true)
    setError(null)
    try {
      const res = await api.unlockApp(password)
      if (res.success) {
        onUnlocked()
      } else {
        setError(res.error || 'Incorrect password. Please try again.')
      }
    } catch (err: any) {
      setError(err.message || 'Authentication failed.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: '#f8fafc',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 9999,
        padding: '20px',
      }}
    >
      <div
        className="google-card"
        style={{
          width: '420px',
          maxWidth: '100%',
          padding: '36px 32px',
          backgroundColor: '#ffffff',
          borderRadius: '10px',
          boxShadow: 'var(--shadow-md)',
          textAlign: 'center',
        }}
      >
        <div
          style={{
            width: '60px',
            height: '60px',
            margin: '0 auto 20px',
            backgroundColor: 'var(--primary-light)',
            borderRadius: '50%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'var(--primary)',
          }}
        >
          <Lock size={28} />
        </div>

        <h2 style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', margin: '0 0 6px' }}>
          Antigravity Swiss Knife
        </h2>
        <p style={{ fontSize: '13px', color: 'var(--text-muted)', margin: '0 0 24px' }}>
          This application is protected. Enter your access password to unlock.
        </p>

        {error && (
          <div
            style={{
              backgroundColor: 'var(--red-bg)',
              color: 'var(--red)',
              padding: '10px 14px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '16px',
              textAlign: 'left',
            }}
          >
            {error}
          </div>
        )}

        <form onSubmit={handleUnlock} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
            <span style={{ position: 'absolute', left: '12px', color: 'var(--text-muted)' }}>
              <KeyRound size={16} />
            </span>
            <input
              type={showPassword ? 'text' : 'password'}
              placeholder="Enter access password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoFocus
              style={{
                width: '100%',
                paddingLeft: '38px',
                paddingRight: '40px',
                paddingTop: '10px',
                paddingBottom: '10px',
                fontSize: '14px',
                borderRadius: '10px',
                border: '1px solid var(--border)',
                backgroundColor: 'var(--canvas)',
              }}
            />
            <button
              type="button"
              onClick={() => setShowPassword(!showPassword)}
              style={{
                position: 'absolute',
                right: '10px',
                background: 'none',
                border: 'none',
                color: 'var(--text-muted)',
                cursor: 'pointer',
                padding: '4px',
                display: 'flex',
                alignItems: 'center',
              }}
              title={showPassword ? 'Hide password' : 'Show password'}
            >
              {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
            </button>
          </div>

          <button
            type="submit"
            disabled={isSubmitting}
            className="btn-pill-primary"
            style={{
              width: '100%',
              padding: '11px',
              fontSize: '14px',
              fontWeight: 600,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '8px',
            }}
          >
            <span>{isSubmitting ? 'Verifying...' : 'Unlock Application'}</span>
            <ArrowRight size={16} />
          </button>
        </form>
      </div>
    </div>
  )
}
