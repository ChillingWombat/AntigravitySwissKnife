import React, { useState, useEffect } from 'react'
import {
  Trash2,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  Info,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import type { CacheBreakdown, VaultStatus } from '../types'
import { api } from '../api'

const PRUNE_AGE_LABELS: Record<number, string> = {
  3: '3 Days',
  7: '7 Days',
  14: '14 Days',
  30: '30 Days',
  90: '3 Months',
  180: '6 Months',
  365: '1 Year',
  0: 'Unlimited',
}

export const BrainCachePage: React.FC = () => {
  const [breakdown, setBreakdown] = useState<CacheBreakdown | null>(null)
  const [pruneDays, setPruneDays] = useState<number>(0)
  const [maxSizeGB, setMaxSizeGB] = useState<number>(0)
  const [autoPrune, setAutoPrune] = useState<boolean>(false)
  const [isTogglingAutoPrune, setIsTogglingAutoPrune] = useState<boolean>(false)
  const [isScanning, setIsScanning] = useState<boolean>(false)
  const [isPruning, setIsPruning] = useState<boolean>(false)
  const [feedback, setFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [showPruneConfirm, setShowPruneConfirm] = useState<boolean>(false)

  // Vault state
  const [vaultStatus, setVaultStatus] = useState<VaultStatus | null>(null)
  const [isSyncingVault, setIsSyncingVault] = useState<boolean>(false)
  const [isTogglingVault, setIsTogglingVault] = useState<boolean>(false)
  const [vaultFeedback, setVaultFeedback] = useState<{ text: string; isError: boolean } | null>(null)

  const scanCache = async (daysOverride?: number, sizeOverride?: number) => {
    const targetDays = daysOverride !== undefined ? daysOverride : pruneDays
    const targetSize = sizeOverride !== undefined ? sizeOverride : maxSizeGB
    setIsScanning(true)
    setFeedback(null)
    try {
      const data = await api.scanCache(targetDays, targetSize)
      setBreakdown(data)
    } catch (err: any) {
      setFeedback({ text: `Scan error: ${err.message}`, isError: true })
    } finally {
      setIsScanning(false)
    }
  }

  const loadCacheConfigAndScan = async () => {
    let initialDays = 0
    let initialSize = 0
    try {
      const cfg = await api.getCacheConfig()
      if (cfg) {
        if (typeof cfg.auto_prune_enabled === 'boolean') {
          setAutoPrune(cfg.auto_prune_enabled)
        }
        if (typeof cfg.prune_days === 'number' && cfg.prune_days >= 0) {
          initialDays = cfg.prune_days
          setPruneDays(cfg.prune_days)
        }
        if (typeof cfg.max_size_gb === 'number' && cfg.max_size_gb >= 0) {
          initialSize = cfg.max_size_gb
          setMaxSizeGB(cfg.max_size_gb)
        }
      }
    } catch {
      // Fallback to defaults (0 days, 0 GB, auto-prune off)
    }
    await scanCache(initialDays, initialSize)
  }

  const handleToggleAutoPrune = async (enabled: boolean) => {
    setIsTogglingAutoPrune(true)
    setFeedback(null)
    try {
      const res = await api.saveCacheConfig({
        auto_prune_enabled: enabled,
        prune_days: pruneDays,
        max_size_gb: maxSizeGB,
      })
      setAutoPrune(res.auto_prune_enabled)
      setFeedback({
        text: res.auto_prune_enabled ? 'Auto-Prune enabled.' : 'Auto-Prune disabled.',
        isError: false,
      })
    } catch (err: any) {
      setFeedback({ text: `Failed to update Auto-Prune: ${err.message}`, isError: true })
    } finally {
      setIsTogglingAutoPrune(false)
    }
  }

  const handlePruneDaysChange = async (days: number) => {
    setPruneDays(days)
    try {
      await api.saveCacheConfig({ prune_days: days })
    } catch {}
    await scanCache(days, maxSizeGB)
  }

  const handleMaxSizeGBChange = async (val: number) => {
    const safeVal = Number.isFinite(val) && val >= 0 ? val : 0
    setMaxSizeGB(safeVal)
    try {
      await api.saveCacheConfig({ max_size_gb: safeVal })
    } catch {}
    await scanCache(pruneDays, safeVal)
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
      const res = await api.pruneCache(pruneDays, maxSizeGB)
      const mb = (res.freed_bytes / (1024 * 1024)).toFixed(1)
      setFeedback({ text: `Reclaimed ${mb} MB across ${res.deleted_files} files safely.`, isError: false })
      await scanCache(pruneDays, maxSizeGB)
    } catch (err: any) {
      setFeedback({ text: `Prune error: ${err.message}`, isError: true })
    } finally {
      setIsPruning(false)
    }
  }

  useEffect(() => {
    loadCacheConfigAndScan()
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
      {/* Main Gadget 1: Conversation Vault */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
              Conversation Vault
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Protects conversations beyond Antigravity's 500-session limit using filesystem hardlinks.
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
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

        {/* Vault Little Metric Gadgets */}
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
            Antigravity deletes SQLite databases once chat history exceeds 500 sessions. The vault keeps filesystem hardlinks on Linux, macOS, and Windows. Pruned sessions restore automatically upon access; manual deletes inside Antigravity remove files from the vault.
          </div>
        </div>

        {/* Vault Setting & Action Gadget */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            backgroundColor: 'var(--canvas)',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 16px',
            gap: '16px',
            flexWrap: 'wrap',
          }}
        >
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
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
            className="btn-pill-tonal"
            style={{ padding: '8px 16px', fontSize: '12px', whiteSpace: 'nowrap' }}
          >
            <RefreshCw size={14} className={isSyncingVault ? 'animate-spin' : ''} />
            <span>{isSyncingVault ? 'Scanning...' : 'Scan Conversation'}</span>
          </button>
        </div>
      </div>

      {/* Main Gadget 2: Cache Manager */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
              Cache Manager
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Reclaims disk space from stale scratch files, step logs, and task outputs without touching active sessions.
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
              <span>Auto-Prune</span>
              <ToggleSwitch
                checked={autoPrune}
                disabled={isTogglingAutoPrune}
                onChange={(checked) => handleToggleAutoPrune(checked)}
              />
            </label>
          </div>
        </div>

        {feedback && (
          <div
            style={{
              backgroundColor: feedback.isError ? '#fce8e6' : 'var(--green-bg)',
              color: feedback.isError ? '#b3261e' : 'var(--green)',
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
            {feedback.isError ? <AlertCircle size={14} /> : <CheckCircle2 size={14} />}
            <span>{feedback.text}</span>
          </div>
        )}

        {/* Cache Little Metric Gadgets */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '16px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Total Antigravity Data
            </div>
            <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {formatBytes(totalBytes)}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              {breakdown?.total_files ?? 1420} indexed files
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Reclaimable Stale Cache
            </div>
            <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--primary)', marginTop: '4px' }}>
              {formatBytes(reclaimableBytes)}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--green)', marginTop: '2px', fontWeight: 500 }}>
              Safe for one-click prune
            </div>
          </div>
        </div>

        {/* Cache Setting & Action Gadget */}
        <div
          style={{
            display: 'flex',
            gap: '16px',
            alignItems: 'center',
            flexWrap: 'wrap',
            backgroundColor: 'var(--canvas)',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 14px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text)', whiteSpace: 'nowrap' }}>
              Prune Stale Artifacts Older Than:
            </span>
            <select
              value={pruneDays}
              onChange={(e) => handlePruneDaysChange(Number(e.target.value))}
              style={{ width: '130px' }}
            >
              <option value={3}>3 Days</option>
              <option value={7}>7 Days</option>
              <option value={14}>14 Days</option>
              <option value={30}>30 Days</option>
              <option value={90}>3 Months</option>
              <option value={180}>6 Months</option>
              <option value={365}>1 Year</option>
              <option value={0}>Unlimited</option>
            </select>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text)', whiteSpace: 'nowrap' }}>
              Size Limit (GB):
            </span>
            <input
              type="number"
              min={0}
              step={0.5}
              value={maxSizeGB}
              onChange={(e) => handleMaxSizeGBChange(Number(e.target.value))}
              title={maxSizeGB === 0 ? '0 (Unlimited)' : `${maxSizeGB} GB`}
              style={{
                width: '88px',
                color: maxSizeGB === 0 ? 'var(--text-subtle)' : 'var(--text)',
                fontWeight: maxSizeGB === 0 ? 500 : 600,
              }}
            />
            {maxSizeGB === 0 && (
              <span
                style={{
                  fontSize: '12px',
                  color: 'var(--text-subtle)',
                  fontWeight: 500,
                  whiteSpace: 'nowrap',
                }}
              >
                (Unlimited)
              </span>
            )}
          </div>

          <div style={{ flex: 1 }} />

          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <button
              onClick={() => scanCache()}
              disabled={isScanning}
              className="btn-pill-tonal"
              style={{ padding: '8px 16px', fontSize: '12px', whiteSpace: 'nowrap' }}
            >
              <RefreshCw size={14} className={isScanning ? 'animate-spin' : ''} />
              <span>{isScanning ? 'Scanning...' : 'Scan Cache'}</span>
            </button>

            <button
              onClick={handlePrune}
              disabled={isPruning}
              className="btn-pill-primary"
              style={{ padding: '8px 16px', fontSize: '12px', whiteSpace: 'nowrap' }}
            >
              <Trash2 size={14} />
              <span>{isPruning ? 'Pruning...' : 'Prune Cache Safely'}</span>
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
              {pruneDays === 0 && maxSizeGB === 0 ? (
                <>
                  Both age retention and size limit are set to <strong>Unlimited</strong>. Active sessions, project directories, and the Conversation Vault will remain protected.
                </>
              ) : pruneDays === 0 ? (
                <>
                  Are you sure you want to prune cache files exceeding the <strong>{maxSizeGB} GB</strong> size limit (with <strong>Unlimited</strong> age retention)? Active sessions, project directories, and the Conversation Vault will remain protected.
                </>
              ) : maxSizeGB === 0 ? (
                <>
                  Are you sure you want to prune cache files older than <strong>{PRUNE_AGE_LABELS[pruneDays] || `${pruneDays} Days`}</strong> (with <strong>Unlimited</strong> size limit)? Active sessions, project directories, and the Conversation Vault will remain protected.
                </>
              ) : (
                <>
                  Are you sure you want to prune cache files older than <strong>{PRUNE_AGE_LABELS[pruneDays] || `${pruneDays} Days`}</strong> or exceeding the <strong>{maxSizeGB} GB</strong> size limit? Active sessions, project directories, and the Conversation Vault will remain protected.
                </>
              )}
            </p>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px' }}>
              <button
                type="button"
                onClick={() => setShowPruneConfirm(false)}
                className="btn-pill-tonal"
                style={{ padding: '8px 16px', fontSize: '12px', whiteSpace: 'nowrap' }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={confirmPrune}
                disabled={isPruning}
                className="btn-pill-danger"
                style={{ padding: '8px 16px', fontSize: '12px', whiteSpace: 'nowrap' }}
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
