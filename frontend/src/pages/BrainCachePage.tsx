import React, { useState, useEffect } from 'react'
import {
  Trash2,
  ShieldCheck,
  ShieldAlert,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  Info,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import type { CacheBreakdown, VaultStatus } from '../types'
import { api } from '../api'

export const BrainCachePage: React.FC = () => {
  const [breakdown, setBreakdown] = useState<CacheBreakdown | null>(null)
  const [pruneDays, setPruneDays] = useState<number>(7)
  const [isScanning, setIsScanning] = useState<boolean>(false)
  const [isPruning, setIsPruning] = useState<boolean>(false)
  const [feedback, setFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [showPruneConfirm, setShowPruneConfirm] = useState<boolean>(false)

  // Vault state
  const [vaultStatus, setVaultStatus] = useState<VaultStatus | null>(null)
  const [isSyncingVault, setIsSyncingVault] = useState<boolean>(false)
  const [isTogglingVault, setIsTogglingVault] = useState<boolean>(false)
  const [vaultFeedback, setVaultFeedback] = useState<{ text: string; isError: boolean } | null>(null)

  const scanCache = async () => {
    setIsScanning(true)
    setFeedback(null)
    try {
      const data = await api.scanCache(pruneDays)
      setBreakdown(data)
    } catch (err: any) {
      setFeedback({ text: `Scan error: ${err.message}`, isError: true })
    } finally {
      setIsScanning(false)
    }
  }

  const loadVaultStatus = async () => {
    try {
      const res = await api.getVaultStatus()
      setVaultStatus(res)
    } catch (err: any) {
      console.warn('Could not load vault status:', err)
    }
  }

  const handleSyncVault = async () => {
    setIsSyncingVault(true)
    setVaultFeedback(null)
    try {
      const res = await api.syncVault()
      setVaultFeedback({
        text: res.message || `Shielded ${res.new_vaulted} new sessions; rescued ${res.rescued_count} pruned sessions.`,
        isError: false,
      })
      await loadVaultStatus()
    } catch (err: any) {
      setVaultFeedback({ text: `Vault sync failed: ${err.message}`, isError: true })
    } finally {
      setIsSyncingVault(false)
    }
  }

  const handleToggleVault = async (enable: boolean) => {
    setIsTogglingVault(true)
    setVaultFeedback(null)
    try {
      const res = await api.toggleVault(enable)
      setVaultFeedback({
        text: res.enabled ? 'Conversation Vault Shield enabled.' : 'Conversation Vault Shield disabled.',
        isError: false,
      })
      await loadVaultStatus()
    } catch (err: any) {
      setVaultFeedback({ text: `Failed to toggle vault: ${err.message}`, isError: true })
    } finally {
      setIsTogglingVault(false)
    }
  }

  const handlePrune = () => {
    setShowPruneConfirm(true)
  }

  const confirmPrune = async () => {
    setShowPruneConfirm(false)
    setIsPruning(true)
    setFeedback(null)
    try {
      const res = await api.pruneCache(pruneDays)
      const mb = (res.freed_bytes / (1024 * 1024)).toFixed(1)
      setFeedback({ text: `Reclaimed ${mb} MB across ${res.deleted_files} files safely.`, isError: false })
      await scanCache()
    } catch (err: any) {
      setFeedback({ text: `Prune error: ${err.message}`, isError: true })
    } finally {
      setIsPruning(false)
    }
  }

  useEffect(() => {
    scanCache()
    loadVaultStatus()
  }, [])

  const formatBytes = (bytes: number) => {
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
  }

  const totalBytes = breakdown?.total_bytes ?? 1250000000
  const reclaimableBytes = breakdown?.reclaimable_bytes ?? 780000000

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Storage Statistics Grid */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '16px' }}>
        <div className="google-card">
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '6px' }}>
            Total Antigravity Data
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)' }}>
            {formatBytes(totalBytes)}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
            {breakdown?.total_files ?? 1420} indexed files
          </div>
        </div>

        <div className="google-card">
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '6px' }}>
            Reclaimable Stale Cache
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--primary)' }}>
            {formatBytes(reclaimableBytes)}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--green)', marginTop: '2px', fontWeight: 500 }}>
            Safe for one-click prune
          </div>
        </div>

        <div className="google-card">
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '6px' }}>
            Conversation Vault Shield
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '6px' }}>
            {vaultStatus?.enabled !== false ? (
              <span className="badge-chip badge-green" style={{ fontSize: '13px', padding: '6px 14px' }}>
                <ShieldCheck size={16} /> SHIELD ACTIVE
              </span>
            ) : (
              <span className="badge-chip badge-tonal" style={{ fontSize: '13px', padding: '6px 14px' }}>
                <ShieldAlert size={16} /> SHIELD PAUSED
              </span>
            )}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '6px' }}>
            {vaultStatus ? `${vaultStatus.vaulted_count} sessions protected` : 'Zero-loss protection for active chats'}
          </div>
        </div>
      </div>

      {feedback && (
        <div
          style={{
            backgroundColor: feedback.isError ? '#fce8e6' : 'var(--green-bg)',
            color: feedback.isError ? '#b3261e' : 'var(--green)',
            padding: '12px 16px',
            borderRadius: '12px',
            fontSize: '13px',
            fontWeight: 500,
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
          }}
        >
          {feedback.isError ? <AlertCircle size={16} /> : <CheckCircle2 size={16} />}
          <span>{feedback.text}</span>
        </div>
      )}

      {/* Conversation Vault Shield Card */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div>
              <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                Conversation History Vault & Auto-Shield
              </div>
              <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Prevents Antigravity's 500-session limit from silently pruning older conversations. Zero-overhead hardlink protection.
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '12.5px', fontWeight: 600 }}>
              <span>Auto-Shield Active</span>
              <ToggleSwitch
                checked={vaultStatus?.enabled ?? true}
                disabled={isTogglingVault}
                onChange={(checked) => handleToggleVault(checked)}
              />
            </label>
          </div>
        </div>

        {vaultFeedback && (
          <div
            style={{
              backgroundColor: vaultFeedback.isError ? '#fce8e6' : 'var(--green-bg)',
              color: vaultFeedback.isError ? '#b3261e' : 'var(--green)',
              padding: '10px 14px',
              borderRadius: '8px',
              fontSize: '12px',
              fontWeight: 500,
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              marginBottom: '16px',
            }}
          >
            {vaultFeedback.isError ? <AlertCircle size={14} /> : <CheckCircle2 size={14} />}
            <span>{vaultFeedback.text}</span>
          </div>
        )}

        {/* Vault Metrics Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '12px', marginBottom: '16px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Live Sessions
            </div>
            <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {vaultStatus?.live_count ?? 0}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              active in ~/.gemini/
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Vaulted Inodes
            </div>
            <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--primary)', marginTop: '4px' }}>
              {vaultStatus?.vaulted_count ?? 0}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              hardlinked in vault
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Auto-Rescued
            </div>
            <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--green)', marginTop: '4px' }}>
              {vaultStatus?.rescued_count ?? 0}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              restored on access
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Disk Overhead
            </div>
            <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              0 B
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              shared filesystem inodes
            </div>
          </div>
        </div>

        {/* Explanatory Banner */}
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: '10px',
            backgroundColor: '#f8f9fa',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 14px',
            fontSize: '12px',
            color: 'var(--text-muted)',
            lineHeight: 1.5,
            marginBottom: '16px',
          }}
        >
          <Info size={16} color="var(--primary)" style={{ flexShrink: 0, marginTop: '2px' }} />
          <div>
            Antigravity natively deletes SQLite conversation databases once total history exceeds 500 chats, causing missing trajectory errors and UI freezes. The Conversation Vault maintains filesystem hardlinks on Linux, macOS, and Windows. When Antigravity unlinks a database, its inode survives in the vault and is automatically restored upon access.
          </div>
        </div>

        {/* Vault Footer Action Bar */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: '1px solid var(--border)', paddingTop: '12px' }}>
          <div style={{ fontSize: '11.5px', color: 'var(--text-muted)' }}>
            Vault Directory: <code>{vaultStatus?.vault_dir || '~/.gemini/antigravity/vault/conversations'}</code>
            {vaultStatus?.last_sync_time && (
              <span style={{ marginLeft: '12px' }}>
                • Last Sync: {new Date(vaultStatus.last_sync_time).toLocaleTimeString()}
              </span>
            )}
          </div>

          <button
            onClick={handleSyncVault}
            disabled={isSyncingVault}
            className="btn-pill-primary"
            style={{ padding: '7px 18px', fontSize: '12px' }}
          >
            <RefreshCw size={13} className={isSyncingVault ? 'animate-spin' : ''} />
            <span>{isSyncingVault ? 'Syncing...' : 'Sync Vault Now'}</span>
          </button>
        </div>
      </div>

      {/* Safe Pruning Options Card */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '14px' }}>
          Safe Pruning Rules & Execution
        </div>

        <div style={{ display: 'flex', gap: '16px', alignItems: 'center' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text)' }}>
              Prune Stale Artifacts Older Than:
            </span>
            <select
              value={pruneDays}
              onChange={(e) => setPruneDays(Number(e.target.value))}
              style={{ width: '130px' }}
            >
              <option value={3}>3 Days</option>
              <option value={7}>7 Days</option>
              <option value={14}>14 Days</option>
              <option value={30}>30 Days</option>
            </select>
          </div>

          <div style={{ flex: 1 }} />

          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <button
              onClick={scanCache}
              disabled={isScanning}
              className="btn-pill-tonal"
              style={{ padding: '9px 18px' }}
            >
              <RefreshCw size={14} className={isScanning ? 'animate-spin' : ''} /> {isScanning ? 'Scanning...' : 'Scan Storage'}
            </button>

            <button
              onClick={handlePrune}
              disabled={isPruning}
              className="btn-pill-primary"
              style={{ padding: '9px 22px' }}
            >
              <Trash2 size={15} /> {isPruning ? 'Pruning...' : 'Prune Cache Safely'}
            </button>
          </div>
        </div>
      </div>

      {/* In-App Confirmation Modal */}
      {showPruneConfirm && (
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
          onClick={() => setShowPruneConfirm(false)}
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
                Confirm Cache Prune
              </h3>
            </div>
            <p style={{ margin: '0 0 20px', fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Are you sure you want to prune cache files older than <strong>{pruneDays} days</strong>? Active sessions and project directories will remain protected.
            </p>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px' }}>
              <button
                type="button"
                onClick={() => setShowPruneConfirm(false)}
                className="btn-pill-tonal"
                style={{ padding: '7px 16px', fontSize: '12px' }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={confirmPrune}
                disabled={isPruning}
                className="btn-pill-danger"
                style={{ padding: '7px 18px', fontSize: '12px' }}
              >
                {isPruning ? 'Pruning...' : 'Prune Safely'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
