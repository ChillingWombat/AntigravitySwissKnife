import React, { useState, useEffect } from 'react'
import {
  ShieldCheck,
  Copy,
  Check,
  RefreshCw,
  Monitor,
  Code,
  Lock,
  Eye,
  EyeOff,
  KeyRound,
} from 'lucide-react'
import type { SystemStatus, SystemInstallations } from '../types'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'

interface SystemSettingsPageProps {
  status: SystemStatus | null
  onRefresh: () => void
}

export const SystemSettingsPage: React.FC<SystemSettingsPageProps> = ({
  status,
  onRefresh,
}) => {
  const [copiedKey, setCopiedKey] = useState<string | null>(null)
  const [installations, setInstallations] = useState<SystemInstallations | null>(null)
  const [isCheckingUpdates, setIsCheckingUpdates] = useState(false)
  const [updateFeedback, setUpdateFeedback] = useState<string | null>(null)

  // App Access Password state
  const [isPasswordEnabled, setIsPasswordEnabled] = useState(false)
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showCurrentPassword, setShowCurrentPassword] = useState(false)
  const [showNewPassword, setShowNewPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)
  const [passwordFeedback, setPasswordFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [isSubmittingPassword, setIsSubmittingPassword] = useState(false)

  // Desktop System Startup state
  const [startupEnabled, setStartupEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_startup_enabled') === 'true'
  })

  useEffect(() => {
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.getStartupSetting) {
      electronAPI.getStartupSetting().then((res: any) => {
        if (res && typeof res.openAtLogin === 'boolean') {
          setStartupEnabled(res.openAtLogin)
          localStorage.setItem('antigravity_startup_enabled', String(res.openAtLogin))
        }
      }).catch((err: any) => console.warn('Could not read startup setting:', err))
    }
  }, [])

  const handleToggleStartup = async (enabled: boolean) => {
    setStartupEnabled(enabled)
    localStorage.setItem('antigravity_startup_enabled', String(enabled))
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.setStartupSetting) {
      try {
        await electronAPI.setStartupSetting(enabled)
      } catch (err: any) {
        console.warn('Failed to update startup setting:', err)
      }
    }
  }

  const loadInstallations = async () => {
    try {
      const data = await api.getInstallations()
      setInstallations(data)
    } catch (err: any) {
      console.error('Failed to load installations:', err)
    }
  }

  const loadPasswordSettings = async () => {
    try {
      const res = await api.getPasswordSettings()
      setIsPasswordEnabled(!!res.enabled)
    } catch (err: any) {
      console.error('Failed to load password settings:', err)
    }
  }

  const handleSetPassword = async () => {
    setPasswordFeedback(null)
    if (newPassword.length < 6) {
      setPasswordFeedback({ text: 'Password must be at least 6 characters long.', isError: true })
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordFeedback({ text: 'New passwords do not match.', isError: true })
      return
    }

    setIsSubmittingPassword(true)
    try {
      const res = await api.setPasswordSettings({
        password: newPassword,
        current_password: currentPassword,
      })
      if (res.success) {
        setIsPasswordEnabled(true)
        setNewPassword('')
        setConfirmPassword('')
        setCurrentPassword('')
        setPasswordFeedback({ text: 'Application access password updated successfully.', isError: false })
      } else {
        setPasswordFeedback({ text: res.error || 'Failed to set password.', isError: true })
      }
    } catch (err: any) {
      setPasswordFeedback({ text: err.message || 'Failed to update password.', isError: true })
    } finally {
      setIsSubmittingPassword(false)
    }
  }

  const handleRemovePassword = async () => {
    setPasswordFeedback(null)
    if (!currentPassword) {
      setPasswordFeedback({ text: 'Please enter your current password to remove protection.', isError: true })
      return
    }

    setIsSubmittingPassword(true)
    try {
      const res = await api.setPasswordSettings({
        remove: true,
        current_password: currentPassword,
      })
      if (res.success) {
        setIsPasswordEnabled(false)
        setNewPassword('')
        setConfirmPassword('')
        setCurrentPassword('')
        setPasswordFeedback({ text: 'Application access password removed.', isError: false })
      } else {
        setPasswordFeedback({ text: res.error || 'Failed to remove password.', isError: true })
      }
    } catch (err: any) {
      setPasswordFeedback({ text: err.message || 'Failed to remove password.', isError: true })
    } finally {
      setIsSubmittingPassword(false)
    }
  }

  const handleCheckUpdates = async () => {
    setIsCheckingUpdates(true)
    setUpdateFeedback(null)
    try {
      const data = await api.checkUpdates()
      setInstallations(data)
      setUpdateFeedback('Update check completed successfully.')
    } catch (err: any) {
      setUpdateFeedback(`Update check failed: ${err.message}`)
    } finally {
      setIsCheckingUpdates(false)
    }
  }

  useEffect(() => {
    loadInstallations()
    loadPasswordSettings()
  }, [])

  const copyPath = (key: string, val: string) => {
    navigator.clipboard.writeText(val)
    setCopiedKey(key)
    setTimeout(() => setCopiedKey(null), 1500)
  }

  const envPaths = [
    { key: 'bin', label: 'Antigravity Binary:', val: '/opt/Antigravity/antigravity' },
    { key: 'antigravity_config', label: 'Antigravity Config:', val: '~/.config/Antigravity' },
    { key: 'swiss_config', label: 'Swiss Knife Config:', val: '~/.config/antigravity-swiss' },
    { key: 'socket', label: 'Daemon IPC Socket:', val: '/run/user/1000/antigravity-swiss/daemon.sock' },
  ]

  const handleRefreshAll = () => {
    onRefresh()
    loadInstallations()
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Header Info Card */}
      <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            System & Process Settings
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
            Runtime diagnostics, IPC Unix domain sockets, and Antigravity process safety shield.
          </div>
        </div>

        <button onClick={handleRefreshAll} className="btn-pill-tonal">
          <RefreshCw size={14} /> Refresh Diagnostics
        </button>
      </div>

      {/* Antigravity Installations & Updates Card */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
              Antigravity Installations & Updates
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Cross-platform host detection ({installations?.platform || 'linux'} / {installations?.arch || 'amd64'})
            </div>
          </div>

          <button
            onClick={handleCheckUpdates}
            disabled={isCheckingUpdates}
            className="btn-pill-primary"
            style={{ padding: '6px 14px', fontSize: '12px', display: 'flex', alignItems: 'center', gap: '6px' }}
          >
            <RefreshCw size={13} />
            {isCheckingUpdates ? 'Checking...' : 'Check for Updates'}
          </button>
        </div>

        {updateFeedback && (
          <div style={{ fontSize: '12px', color: 'var(--primary)', marginBottom: '14px', fontWeight: 500 }}>
            {updateFeedback}
          </div>
        )}

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
          {/* Desktop App */}
          <div
            style={{
              border: '1px solid var(--border)',
              borderRadius: '12px',
              padding: '16px',
              backgroundColor: 'var(--canvas)',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
            }}
          >
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '10px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Monitor size={18} color="var(--primary)" />
                  <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                    Desktop App (2.0)
                  </span>
                </div>
                {installations?.desktop_app?.installed ? (
                  <span className={`badge-chip ${installations.desktop_app.up_to_date ? 'badge-green' : 'badge-yellow'}`}>
                    {installations.desktop_app.up_to_date
                      ? `v${installations.desktop_app.version} (Up to Date)`
                      : `v${installations.desktop_app.version} → v${installations.desktop_app.latest_version} (Update Available)`}
                  </span>
                ) : (
                  <span className="badge-chip badge-neutral">Not Detected</span>
                )}
              </div>

              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '6px' }}>
                <strong>Binary / Package Path:</strong>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '12px' }}>
                <input
                  type="text"
                  readOnly
                  value={installations?.desktop_app?.path || 'Not installed'}
                  style={{
                    flex: 1,
                    backgroundColor: '#ffffff',
                    fontFamily: 'monospace',
                    fontSize: '11px',
                    padding: '6px 10px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    color: 'var(--text)',
                  }}
                />
                {installations?.desktop_app?.path && (
                  <button
                    onClick={() => copyPath('desktop_path', installations.desktop_app.path)}
                    className="btn-pill-tonal"
                    style={{ padding: '5px 10px', fontSize: '11px' }}
                    title="Copy path"
                  >
                    {copiedKey === 'desktop_path' ? <Check size={12} /> : <Copy size={12} />}
                  </button>
                )}
              </div>
            </div>

            <div style={{ fontSize: '11px', color: 'var(--text-subtle)', borderTop: '1px solid var(--border)', paddingTop: '10px' }}>
              <strong>Process State:</strong> {installations?.desktop_app?.process_state || (status?.antigravity_running ? `Running (PID: ${status.antigravity_pid})` : 'Not running')}
            </div>
          </div>

          {/* VS Code Extension */}
          <div
            style={{
              border: '1px solid var(--border)',
              borderRadius: '12px',
              padding: '16px',
              backgroundColor: 'var(--canvas)',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
            }}
          >
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '10px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Code size={18} color="var(--primary)" />
                  <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                    VS Code Extension
                  </span>
                </div>
                {installations?.vscode_extension?.installed ? (
                  <span className={`badge-chip ${installations.vscode_extension.up_to_date ? 'badge-green' : 'badge-yellow'}`}>
                    {installations.vscode_extension.up_to_date
                      ? `v${installations.vscode_extension.version} (Up to Date)`
                      : `v${installations.vscode_extension.version} → v${installations.vscode_extension.latest_version} (Update Available)`}
                  </span>
                ) : (
                  <span className="badge-chip badge-neutral">Not Detected</span>
                )}
              </div>

              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '6px' }}>
                <strong>Extension Directory:</strong>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '12px' }}>
                <input
                  type="text"
                  readOnly
                  value={installations?.vscode_extension?.path || 'Not installed'}
                  style={{
                    flex: 1,
                    backgroundColor: '#ffffff',
                    fontFamily: 'monospace',
                    fontSize: '11px',
                    padding: '6px 10px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    color: 'var(--text)',
                  }}
                />
                {installations?.vscode_extension?.path && (
                  <button
                    onClick={() => copyPath('vscode_path', installations.vscode_extension.path)}
                    className="btn-pill-tonal"
                    style={{ padding: '5px 10px', fontSize: '11px' }}
                    title="Copy path"
                  >
                    {copiedKey === 'vscode_path' ? <Check size={12} /> : <Copy size={12} />}
                  </button>
                )}
              </div>
            </div>

            <div style={{ fontSize: '11px', color: 'var(--text-subtle)', borderTop: '1px solid var(--border)', paddingTop: '10px' }}>
              <strong>Integration Target:</strong> {installations?.vscode_extension?.installed ? 'Webview injection ready' : 'Extension not found'}
            </div>
          </div>
        </div>
      </div>

      {/* Safety Shield Card */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '12px' }}>
          Process Safety Shield & Protection
        </div>

        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
            backgroundColor: 'var(--green-bg)',
            border: '1px solid #ceead6',
            borderRadius: '12px',
            padding: '16px 20px',
          }}
        >
          <ShieldCheck size={28} color="var(--green)" />
          <div>
            <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--green)' }}>
              Host Process Safety Shield: ACTIVE
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Host Antigravity 2.0 PID {status?.antigravity_pid ? `(${status.antigravity_pid})` : ''} is protected against accidental termination signals.
            </div>
          </div>
        </div>
      </div>

      {/* System Startup & Desktop Integration Card */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
              System Startup & Desktop Integration
            </div>
            <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
              Configure automatic background startup and minimize-to-tray behavior on system login.
            </div>
          </div>
          <div className={`badge-chip ${startupEnabled ? 'badge-green' : 'badge-tonal'}`} style={{ fontSize: '12px', padding: '6px 14px' }}>
            <Monitor size={14} />
            <span>{startupEnabled ? 'Launch at Startup Active' : 'Manual Launch Only'}</span>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 18px', backgroundColor: 'var(--canvas)', borderRadius: '10px' }}>
          <div>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
              Launch at System Startup (Minimized to Tray)
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Automatically starts the Antigravity companion silently in your system tray when you log into Windows, macOS, or Linux.
            </div>
          </div>

          <ToggleSwitch
            size="md"
            checked={startupEnabled}
            onChange={handleToggleStartup}
          />
        </div>
      </div>

      {/* App Access Password Protection Card */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div
              style={{
                width: '32px',
                height: '32px',
                borderRadius: '8px',
                backgroundColor: isPasswordEnabled ? 'var(--primary-light)' : 'var(--canvas)',
                border: '1px solid var(--border)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: isPasswordEnabled ? 'var(--primary)' : 'var(--text-muted)',
              }}
            >
              <Lock size={18} />
            </div>
            <div>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                App Access Password Protection
              </div>
              <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Require an entry password to unlock and use Antigravity Swiss Knife. Minimum 6 characters (numbers, letters, symbols).
              </div>
            </div>
          </div>

          <span className={`badge-chip ${isPasswordEnabled ? 'badge-green' : 'badge-neutral'}`}>
            {isPasswordEnabled ? 'ENABLED / LOCKED' : 'OPTIONAL / DISABLED'}
          </span>
        </div>

        {passwordFeedback && (
          <div
            style={{
              backgroundColor: passwordFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
              color: passwordFeedback.isError ? 'var(--red)' : 'var(--green)',
              padding: '10px 14px',
              borderRadius: '8px',
              fontSize: '12px',
              marginBottom: '16px',
            }}
          >
            {passwordFeedback.text}
          </div>
        )}

        <div
          style={{
            backgroundColor: 'var(--canvas)',
            border: '1px solid var(--border)',
            borderRadius: '12px',
            padding: '16px',
            display: 'flex',
            flexDirection: 'column',
            gap: '12px',
          }}
        >
          {isPasswordEnabled && (
            <div>
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                <KeyRound size={14} /> Current Password:
              </label>
              <div style={{ position: 'relative', display: 'flex', alignItems: 'center', maxWidth: '400px' }}>
                <input
                  type={showCurrentPassword ? 'text' : 'password'}
                  placeholder="Enter current password to verify"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  style={{ width: '100%', paddingRight: '40px' }}
                />
                <button
                  type="button"
                  onClick={() => setShowCurrentPassword(!showCurrentPassword)}
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
                  title={showCurrentPassword ? 'Hide password' : 'Show password'}
                >
                  {showCurrentPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>
          )}

          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
              <Lock size={14} /> {isPasswordEnabled ? 'New Password (Optional):' : 'Set App Password:'}
            </label>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center', maxWidth: '400px' }}>
              <input
                type={showNewPassword ? 'text' : 'password'}
                placeholder="Min 6 characters (numbers, letters, symbols)"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                style={{ width: '100%', paddingRight: '40px' }}
              />
              <button
                type="button"
                onClick={() => setShowNewPassword(!showNewPassword)}
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
                title={showNewPassword ? 'Hide password' : 'Show password'}
              >
                {showNewPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          <div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
              <Check size={14} /> Confirm Password:
            </label>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center', maxWidth: '400px' }}>
              <input
                type={showConfirmPassword ? 'text' : 'password'}
                placeholder="Re-enter password to confirm"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                style={{ width: '100%', paddingRight: '40px' }}
              />
              <button
                type="button"
                onClick={() => setShowConfirmPassword(!showConfirmPassword)}
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
                title={showConfirmPassword ? 'Hide password' : 'Show password'}
              >
                {showConfirmPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          <div style={{ display: 'flex', gap: '10px', marginTop: '6px' }}>
            <button
              onClick={handleSetPassword}
              disabled={isSubmittingPassword || !newPassword}
              className="btn-pill-primary"
            >
              {isSubmittingPassword ? 'Saving...' : isPasswordEnabled ? 'Update Password' : 'Enable App Password'}
            </button>

            {isPasswordEnabled && (
              <button
                onClick={handleRemovePassword}
                disabled={isSubmittingPassword}
                className="btn-pill-danger"
              >
                Remove Password
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Environment Paths Card */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Runtime Environment & Socket Paths
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          {envPaths.map((item) => (
            <div key={item.key} style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
              <span
                style={{
                  width: '180px',
                  fontSize: '12px',
                  fontWeight: 600,
                  color: 'var(--text-muted)',
                }}
              >
                {item.label}
              </span>

              <input
                type="text"
                readOnly
                value={item.val}
                style={{
                  flex: 1,
                  backgroundColor: 'var(--canvas)',
                  fontFamily: 'monospace',
                  fontSize: '12px',
                  color: 'var(--text)',
                }}
              />

              <button
                onClick={() => copyPath(item.key, item.val)}
                className="btn-pill-tonal"
                style={{ padding: '6px 12px', fontSize: '11px' }}
              >
                {copiedKey === item.key ? <Check size={14} /> : <Copy size={14} />}
                {copiedKey === item.key ? 'Copied' : 'Copy'}
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
