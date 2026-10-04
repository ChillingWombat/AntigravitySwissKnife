import React, { useState, useEffect } from 'react'
import {
  ShieldCheck,
  Copy,
  Check,
  RefreshCw,
  Monitor,
  Code,
} from 'lucide-react'
import type { SystemStatus, SystemInstallations } from '../types'
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

  const loadInstallations = async () => {
    try {
      const data = await api.getInstallations()
      setInstallations(data)
    } catch (err: any) {
      console.error('Failed to load installations:', err)
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
              Host IDE PID {status?.antigravity_pid ? `(${status.antigravity_pid})` : ''} is protected against accidental termination signals.
            </div>
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
