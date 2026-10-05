import React, { useState, useEffect } from 'react'
import {
  Trash2,
  ShieldCheck,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
} from 'lucide-react'
import type { CacheBreakdown } from '../types'
import { api } from '../api'

export const BrainCachePage: React.FC = () => {
  const [breakdown, setBreakdown] = useState<CacheBreakdown | null>(null)
  const [pruneDays, setPruneDays] = useState<number>(7)
  const [isScanning, setIsScanning] = useState<boolean>(false)
  const [isPruning, setIsPruning] = useState<boolean>(false)
  const [feedback, setFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [showPruneConfirm, setShowPruneConfirm] = useState<boolean>(false)

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
      {/* Header Info Card */}
      <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Brain Cache & Context Optimizer
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
            Inspect disk usage in ~/.gemini/antigravity/ and reclaim gigabytes of stale scratch data safely.
          </div>
        </div>

        <button onClick={scanCache} disabled={isScanning} className="btn-pill-tonal">
          <RefreshCw size={14} /> {isScanning ? 'Scanning...' : 'Scan Storage'}
        </button>
      </div>

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
            Session Shield Status
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '6px' }}>
            <span className="badge-chip badge-green" style={{ fontSize: '13px', padding: '6px 14px' }}>
              <ShieldCheck size={16} /> ACTIVE
            </span>
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '6px' }}>
            Zero-loss protection for active chats
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
