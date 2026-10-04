import React, { useState, useEffect } from 'react'
import {
  Save,
} from 'lucide-react'
import type { RuleConfig } from '../types'
import { api } from '../api'

interface SwitcherSettingsPageProps {
  initialRules: RuleConfig | null
  onSaved: () => void
}

export const SwitcherSettingsPage: React.FC<SwitcherSettingsPageProps> = ({
  initialRules,
  onSaved,
}) => {
  const [threshold, setThreshold] = useState<number>(initialRules?.auto_switch_threshold ?? 0.05)
  const [pollingInterval, setPollingInterval] = useState<number>(initialRules?.polling_interval_seconds ?? 60)
  const [warmupEnabled, setWarmupEnabled] = useState<boolean>(initialRules?.warmup_enabled ?? true)
  const [warmupLeadTime, setWarmupLeadTime] = useState<number>(initialRules?.warmup_lead_time_seconds ?? 2.0)
  const [isSaving, setIsSaving] = useState<boolean>(false)
  const [feedback, setFeedback] = useState<string | null>(null)

  useEffect(() => {
    if (initialRules) {
      setThreshold(initialRules.auto_switch_threshold)
      setPollingInterval(initialRules.polling_interval_seconds)
      setWarmupEnabled(initialRules.warmup_enabled)
      setWarmupLeadTime(initialRules.warmup_lead_time_seconds)
    }
  }, [initialRules])

  const handleSave = async () => {
    setIsSaving(true)
    setFeedback(null)
    try {
      await api.saveRules({
        auto_switch_threshold: threshold,
        polling_interval_seconds: pollingInterval,
        warmup_enabled: warmupEnabled,
        warmup_lead_time_seconds: warmupLeadTime,
      })
      setFeedback('Configuration saved successfully.')
      onSaved()
    } catch (err: any) {
      setFeedback(`Save error: ${err.message}`)
    } finally {
      setIsSaving(false)
    }
  }

  const thresholdPercent = Math.round(threshold * 100)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Header Info Card */}
      <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Account Switcher & Warmup Settings
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
            Tune auto-rotation rules, anti-thrash hysteresis, and keep-alive triggers.
          </div>
        </div>

        <button onClick={handleSave} disabled={isSaving} className="btn-pill-primary">
          <Save size={15} /> {isSaving ? 'Saving...' : 'Save Configuration'}
        </button>
      </div>

      {feedback && (
        <div
          style={{
            backgroundColor: 'var(--green-bg)',
            color: 'var(--green)',
            padding: '12px 16px',
            borderRadius: '12px',
            fontSize: '13px',
            fontWeight: 500,
          }}
        >
          {feedback}
        </div>
      )}

      {/* Section 1: Auto-Switch Trigger Rules */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Auto-Switch Trigger Rules
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* Threshold Slider */}
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
              <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Exhaustion Threshold Trigger:
              </span>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--primary)' }}>
                {thresholdPercent}% Quota Remaining
              </span>
            </div>
            <input
              type="range"
              min={1}
              max={50}
              value={thresholdPercent}
              onChange={(e) => setThreshold(Number(e.target.value) / 100)}
              style={{
                width: '100%',
                accentColor: 'var(--primary)',
                cursor: 'pointer',
              }}
            />
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
              Triggers proactive rotation to the next highest-quota standby account before reaching zero quota.
            </div>
          </div>

          {/* Polling Interval */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Upstream Quota Polling Interval:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Frequency of fetching cloudcode-pa quota horizons from Google.
              </div>
            </div>
            <select
              value={pollingInterval}
              onChange={(e) => setPollingInterval(Number(e.target.value))}
              style={{ width: '150px' }}
            >
              <option value={30}>30 Seconds</option>
              <option value={60}>60 Seconds</option>
              <option value={120}>2 Minutes</option>
              <option value={300}>5 Minutes</option>
            </select>
          </div>
        </div>
      </div>

      {/* Section 2: Post-Reset Keep-Alive Warmup Engine */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Post-Reset Keep-Alive Warmup Engine
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '10px', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={warmupEnabled}
              onChange={(e) => setWarmupEnabled(e.target.checked)}
              style={{ width: '18px', height: '18px' }}
            />
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Enable Automatic Standby Keep-Alive Warmup
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Dispatches a lightweight probe to newly-reset accounts so their quota pool is immediately warm.
              </div>
            </div>
          </label>

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Warmup Lead Time:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Seconds before scheduled reset time to prepare rotation candidate.
              </div>
            </div>
            <input
              type="number"
              step={0.5}
              min={0.5}
              max={10.0}
              value={warmupLeadTime}
              onChange={(e) => setWarmupLeadTime(Number(e.target.value))}
              style={{ width: '100px', textAlign: 'center' }}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
