import React, { useState, useEffect, useRef } from 'react'
import {
  ArrowUp,
  ArrowDown,
  Layers,
  GripVertical,
  RefreshCw,
  Download,
  Upload,
  Copy,
  Check,
  FileText,
  AlertCircle,
  CheckCircle2,
  X,
  Code,
  Scale,
  Sigma,
  Infinity as InfinityIcon,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import type { RuleConfig, AvailableModelItem, SwitchMode } from '../types'
import { api } from '../api'
import { resolveEffectiveCustomModel } from '../utils/modelFilter'
import {
  MODEL_SOURCE_LABELS,
  normalizeModelSourceHierarchy,
  type ModelSourceKey,
} from '../utils/modelSourceHierarchy'

interface SwitcherSettingsPageProps {
  initialRules: RuleConfig | null
  onSaved: () => void
}

export const SwitcherSettingsPage: React.FC<SwitcherSettingsPageProps> = ({
  initialRules,
  onSaved,
}) => {
  const [threshold, setThreshold] = useState<number>(initialRules?.auto_switch_threshold ?? 0.05)
  const [thresholdText, setThresholdText] = useState<string>(
    String(Math.round((initialRules?.auto_switch_threshold ?? 0.05) * 100))
  )
  const [weeklyThreshold, setWeeklyThreshold] = useState<number>(initialRules?.auto_switch_weekly_threshold ?? 0.05)
  const [weeklyThresholdText, setWeeklyThresholdText] = useState<string>(
    String(Math.round((initialRules?.auto_switch_weekly_threshold ?? 0.05) * 100))
  )
  const [switchMode, setSwitchMode] = useState<SwitchMode>(initialRules?.switch_mode || 'balanced')
  const [pollingInterval, setPollingInterval] = useState<number>(initialRules?.polling_interval_seconds ?? 60)
  const [activePollingInterval, setActivePollingInterval] = useState<number>(initialRules?.active_polling_interval_seconds ?? 120)
  const [standbyPollingInterval, setStandbyPollingInterval] = useState<number>(initialRules?.standby_polling_interval_seconds ?? 900)
  const [standbyRandomJitter, setStandbyRandomJitter] = useState<number>(initialRules?.standby_random_jitter_seconds ?? 30)
  const [warmupEnabled, setWarmupEnabled] = useState<boolean>(initialRules?.warmup_enabled ?? true)
  const [warmupLeadTime, setWarmupLeadTime] = useState<number>(initialRules?.warmup_lead_time_seconds ?? 2.0)

  // New Model Source Hierarchy & Model Defaults
  const [allowAICredits, setAllowAICredits] = useState<boolean>(initialRules?.allow_ai_credits_usage ?? false)
  const [allowNonGemini, setAllowNonGemini] = useState<boolean>(initialRules?.allow_non_gemini_native_models ?? false)
  const [hierarchy, setHierarchy] = useState<ModelSourceKey[]>(
    normalizeModelSourceHierarchy(initialRules?.model_source_hierarchy)
  )
  const [defaultGemini, setDefaultGemini] = useState<string>(initialRules?.default_gemini_model || 'gemini-3.8-flash-high')
  const [defaultCustom, setDefaultCustom] = useState<string>(initialRules?.default_custom_model || '')
  const [defaultNonGemini, setDefaultNonGemini] = useState<string>(initialRules?.default_non_gemini_model || 'claude-opus-4-6')
  const [geminiReasoningLevel, setGeminiReasoningLevel] = useState<string>(initialRules?.default_gemini_reasoning_level || 'high')

  // Dynamic available model lists (fetched automatically from running IDE / CloudCode)
  const [geminiModelOptions, setGeminiModelOptions] = useState<AvailableModelItem[]>([])
  const [nonGeminiModelOptions, setNonGeminiModelOptions] = useState<AvailableModelItem[]>([])
  const [isFetchingModels, setIsFetchingModels] = useState<boolean>(false)
  const [autoImportActive, setAutoImportActive] = useState<boolean>(initialRules?.auto_import_active_account ?? false)
  const [customModelOptions, setCustomModelOptions] = useState<{ id: string; name: string }[]>([])
  const [draggedIdx, setDraggedIdx] = useState<number | null>(null)
  const [dragOverIdx, setDragOverIdx] = useState<number | null>(null)

  const [feedback, setFeedback] = useState<string | null>(null)
  const hydratedRef = useRef(false)
  const lastPersistedPayloadRef = useRef<string | null>(null)

  // Account Export & Import States
  const [isExporting, setIsExporting] = useState<boolean>(false)
  const [copiedExport, setCopiedExport] = useState<boolean>(false)
  const [showExportPreview, setShowExportPreview] = useState<boolean>(false)
  const [exportPreviewText, setExportPreviewText] = useState<string>('')
  const [isImporting, setIsImporting] = useState<boolean>(false)
  const [importStatus, setImportStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
  const [showPasteModal, setShowPasteModal] = useState<boolean>(false)
  const [pasteText, setPasteText] = useState<string>('')

  const handleExportDownload = async () => {
    setIsExporting(true)
    setImportStatus(null)
    try {
      const data = await api.exportAccounts()
      const jsonStr = JSON.stringify(data, null, 2)
      setExportPreviewText(jsonStr)
      const blob = new Blob([jsonStr], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      const dateStr = new Date().toISOString().slice(0, 10)
      a.download = `antigravity_accounts_${dateStr}.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      setImportStatus({
        type: 'success',
        message: `Successfully exported ${data.length} accounts to JSON file.`,
      })
    } catch (err: any) {
      setImportStatus({
        type: 'error',
        message: `Export failed: ${err.message}`,
      })
    } finally {
      setIsExporting(false)
    }
  }

  const handleExportCopy = async () => {
    setIsExporting(true)
    setImportStatus(null)
    try {
      const data = await api.exportAccounts()
      const jsonStr = JSON.stringify(data, null, 2)
      setExportPreviewText(jsonStr)
      await navigator.clipboard.writeText(jsonStr)
      setCopiedExport(true)
      setTimeout(() => setCopiedExport(false), 2000)
      setImportStatus({
        type: 'success',
        message: `Copied ${data.length} accounts JSON to clipboard.`,
      })
    } catch (err: any) {
      setImportStatus({
        type: 'error',
        message: `Copy failed: ${err.message}`,
      })
    } finally {
      setIsExporting(false)
    }
  }

  const handleTogglePreview = async () => {
    if (!showExportPreview && !exportPreviewText) {
      setIsExporting(true)
      try {
        const data = await api.exportAccounts()
        setExportPreviewText(JSON.stringify(data, null, 2))
      } catch (err: any) {
        setImportStatus({ type: 'error', message: `Failed to load preview: ${err.message}` })
      } finally {
        setIsExporting(false)
      }
    }
    setShowExportPreview(!showExportPreview)
  }

  const processImportJson = async (jsonText: string) => {
    if (!jsonText.trim()) {
      setImportStatus({ type: 'error', message: 'JSON content is empty.' })
      return
    }
    setIsImporting(true)
    setImportStatus(null)
    try {
      let parsed: any
      try {
        parsed = JSON.parse(jsonText)
      } catch (parseErr: any) {
        throw new Error(`Invalid JSON syntax: ${parseErr.message}`)
      }
      const res = await api.batchImportAccounts(parsed)
      setImportStatus({
        type: 'success',
        message: res.message || `Successfully imported ${res.imported} accounts.`,
      })
      setShowPasteModal(false)
      setPasteText('')
      onSaved()
    } catch (err: any) {
      setImportStatus({
        type: 'error',
        message: err.message || 'Import failed.',
      })
    } finally {
      setIsImporting(false)
    }
  }

  const handleImportFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = async (evt) => {
      try {
        const content = evt.target?.result as string
        await processImportJson(content)
      } catch (err: any) {
        setImportStatus({ type: 'error', message: `File read error: ${err.message}` })
      }
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  const sampleJson = `[
  {
    "id": "work.engineer@gmail.com",
    "password": "AccountPassword123!",
    "mfa": "JBSWY3DPEHPK3PXP",
    "oath_token": "1//06xyzSampleOAuthRefreshToken...",
    "label": "Work Account",
    "plan_tier": "Google AI Pro",
    "priority": "High"
  }
]`

  useEffect(() => {
    if (!initialRules || hydratedRef.current) return
    hydratedRef.current = true
    setThreshold(initialRules.auto_switch_threshold)
    setThresholdText(String(Math.round(initialRules.auto_switch_threshold * 100)))
    if (initialRules.auto_switch_weekly_threshold !== undefined) {
      setWeeklyThreshold(initialRules.auto_switch_weekly_threshold)
      setWeeklyThresholdText(String(Math.round(initialRules.auto_switch_weekly_threshold * 100)))
    }
    if (initialRules.switch_mode) {
      setSwitchMode(initialRules.switch_mode)
    }
    setPollingInterval(initialRules.polling_interval_seconds)
    if (initialRules.active_polling_interval_seconds) {
      setActivePollingInterval(initialRules.active_polling_interval_seconds)
    }
    if (initialRules.standby_polling_interval_seconds) {
      setStandbyPollingInterval(initialRules.standby_polling_interval_seconds)
    }
    if (initialRules.standby_random_jitter_seconds) {
      setStandbyRandomJitter(initialRules.standby_random_jitter_seconds)
    }
    setWarmupEnabled(initialRules.warmup_enabled)
    setWarmupLeadTime(initialRules.warmup_lead_time_seconds)
    if (initialRules.allow_ai_credits_usage !== undefined) {
      setAllowAICredits(initialRules.allow_ai_credits_usage)
    }
    if (initialRules.allow_non_gemini_native_models !== undefined) {
      setAllowNonGemini(initialRules.allow_non_gemini_native_models)
    }
    setHierarchy(normalizeModelSourceHierarchy(initialRules.model_source_hierarchy))
    if (initialRules.default_gemini_model) {
      setDefaultGemini(initialRules.default_gemini_model)
    }
    if (initialRules.default_custom_model) {
      setDefaultCustom(initialRules.default_custom_model)
    }
    if (initialRules.default_non_gemini_model) {
      setDefaultNonGemini(initialRules.default_non_gemini_model)
    }
    if (initialRules.default_gemini_reasoning_level) {
      setGeminiReasoningLevel(initialRules.default_gemini_reasoning_level)
    }
    if (initialRules.auto_import_active_account !== undefined) {
      setAutoImportActive(initialRules.auto_import_active_account)
    }
  }, [initialRules])

  const fetchAvailableModels = async (force: boolean = false) => {
    setIsFetchingModels(true)
    try {
      const res = await api.getAvailableModels(force)
      if (res && res.success) {
        if (res.gemini_models && res.gemini_models.length > 0) {
          setGeminiModelOptions(res.gemini_models)
          setDefaultGemini((prev) => {
            if (prev && res.gemini_models.some((opt) => opt.id === prev)) {
              return prev
            }
            if (res.default_gemini && res.gemini_models.some((opt) => opt.id === res.default_gemini)) {
              return res.default_gemini
            }
            return res.gemini_models[0].id
          })
        }
        if (res.non_gemini_models && res.non_gemini_models.length > 0) {
          setNonGeminiModelOptions(res.non_gemini_models)
          setDefaultNonGemini((prev) => {
            if (prev && res.non_gemini_models.some((opt) => opt.id === prev)) {
              return prev
            }
            if (res.default_non_gemini && res.non_gemini_models.some((opt) => opt.id === res.default_non_gemini)) {
              return res.default_non_gemini
            }
            return res.non_gemini_models[0].id
          })
        }
      }
    } catch (_) {
      // Keep baseline models active
    } finally {
      setIsFetchingModels(false)
    }

    try {
      const cmRes = await api.getCustomModels()
      if (cmRes && cmRes.models) {
        const enabled = cmRes.models.filter((m) => m.enabled)
        setCustomModelOptions(enabled.map((m) => ({ id: m.id, name: m.display_name || m.name || m.id })))
        if (enabled.length > 0) {
          setDefaultCustom((prev) => {
            const exists = enabled.some((m) => m.id === prev)
            return exists && prev !== '' ? prev : enabled[0].id
          })
        } else {
          setDefaultCustom('')
        }
      } else {
        setCustomModelOptions([])
        setDefaultCustom('')
      }
    } catch (_) {
      // Keep existing custom models
    }
  }

  useEffect(() => {
    fetchAvailableModels()
  }, [])

  useEffect(() => {
    api.getCustomModels()
      .then((res) => {
        if (res && res.models) {
          const enabled = res.models.filter((m) => m.enabled)
          setCustomModelOptions(enabled.map((m) => ({ id: m.id, name: m.display_name || m.name || m.id })))
          if (enabled.length > 0) {
            setDefaultCustom((prev) => {
              const exists = enabled.some((m) => m.id === prev)
              return exists && prev !== '' ? prev : enabled[0].id
            })
          } else {
            setDefaultCustom('')
          }
        } else {
          setCustomModelOptions([])
          setDefaultCustom('')
        }
      })
      .catch(() => {
        setCustomModelOptions([])
        setDefaultCustom('')
      })
  }, [])

  const effectiveCustomModel = resolveEffectiveCustomModel(customModelOptions, defaultCustom)

  const moveHierarchyItem = (index: number, direction: 'up' | 'down') => {
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= hierarchy.length) return
    const updated = [...hierarchy]
    const [moved] = updated.splice(index, 1)
    updated.splice(targetIndex, 0, moved)
    setHierarchy(updated)
  }

  useEffect(() => {
    if (!hydratedRef.current) return
    const payload = {
      auto_switch_threshold: threshold,
      auto_switch_weekly_threshold: weeklyThreshold,
      switch_mode: switchMode,
      polling_interval_seconds: pollingInterval,
      active_polling_interval_seconds: activePollingInterval,
      standby_polling_interval_seconds: standbyPollingInterval,
      standby_random_jitter_seconds: standbyRandomJitter,
      warmup_enabled: warmupEnabled,
      warmup_lead_time_seconds: warmupLeadTime,
      allow_ai_credits_usage: allowAICredits,
      allow_non_gemini_native_models: allowNonGemini,
      model_source_hierarchy: hierarchy,
      default_gemini_model: defaultGemini,
      default_custom_model: effectiveCustomModel,
      default_non_gemini_model: defaultNonGemini,
      default_gemini_reasoning_level: geminiReasoningLevel,
      auto_import_active_account: autoImportActive,
    }
    const serialized = JSON.stringify(payload)
    if (lastPersistedPayloadRef.current === null) {
      lastPersistedPayloadRef.current = serialized
      return
    }
    if (serialized === lastPersistedPayloadRef.current) return
    const timer = setTimeout(() => {
      api.saveRules(payload)
        .then(() => {
          lastPersistedPayloadRef.current = serialized
          setFeedback(null)
          onSaved()
        })
        .catch((err: any) => {
          setFeedback(`Save error: ${err.message}`)
        })
    }, 600)
    return () => clearTimeout(timer)
  }, [
    threshold,
    weeklyThreshold,
    switchMode,
    pollingInterval,
    activePollingInterval,
    standbyPollingInterval,
    standbyRandomJitter,
    warmupEnabled,
    warmupLeadTime,
    allowAICredits,
    allowNonGemini,
    hierarchy,
    defaultGemini,
    effectiveCustomModel,
    defaultNonGemini,
    geminiReasoningLevel,
    autoImportActive,
    onSaved,
  ])

  const commitThresholdPercent = (
    text: string,
    currentFraction: number,
    setFraction: (n: number) => void,
    setText: (s: string) => void
  ) => {
    const trimmed = text.trim()
    const parsed = trimmed === '' ? NaN : Number(trimmed)
    const pct = Number.isFinite(parsed)
      ? Math.min(50, Math.max(1, Math.round(parsed)))
      : Math.round(currentFraction * 100)
    setFraction(pct / 100)
    setText(String(pct))
  }

  const handleThresholdTextChange = (
    text: string,
    setFraction: (n: number) => void,
    setText: (s: string) => void
  ) => {
    setText(text)
    const parsed = Number(text)
    if (Number.isInteger(parsed) && parsed >= 1 && parsed <= 50) {
      setFraction(parsed / 100)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {feedback && (
        <div
          style={{
            backgroundColor: 'var(--red-bg)',
            color: 'var(--red)',
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
          {/* Switch Mode Strategy Selector */}
          <div>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
              Account Switch Mode:
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '12px' }}>
              Configure how the rotation engine prioritizes candidate accounts and evaluates switch triggers.
            </div>

            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                gap: '12px',
              }}
            >
              {/* Option 1: Balanced (Default) */}
              <div
                onClick={() => setSwitchMode('balanced')}
                style={{
                  border: switchMode === 'balanced' ? '2px solid var(--primary)' : '1px solid var(--border)',
                  backgroundColor: switchMode === 'balanced' ? 'var(--primary-light, rgba(11, 87, 208, 0.04))' : 'var(--surface)',
                  borderRadius: '8px',
                  padding: '14px',
                  cursor: 'pointer',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '8px',
                  transition: 'all 0.15s ease',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <Scale size={15} color={switchMode === 'balanced' ? 'var(--primary)' : 'var(--text-muted)'} />
                    <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                      Balanced
                    </span>
                  </div>
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
                  Rotates only when active quota drops to exhaustion threshold. Standby accounts ranked by balanced composite score across 5h and weekly quota.
                </div>
              </div>

              {/* Option 2: Max Total Tokens */}
              <div
                onClick={() => setSwitchMode('max_tokens')}
                style={{
                  border: switchMode === 'max_tokens' ? '2px solid var(--primary)' : '1px solid var(--border)',
                  backgroundColor: switchMode === 'max_tokens' ? 'var(--primary-light, rgba(11, 87, 208, 0.04))' : 'var(--surface)',
                  borderRadius: '8px',
                  padding: '14px',
                  cursor: 'pointer',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '8px',
                  transition: 'all 0.15s ease',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <Sigma size={15} color={switchMode === 'max_tokens' ? 'var(--primary)' : 'var(--text-muted)'} />
                    <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                      Max Total Tokens
                    </span>
                  </div>
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
                  Maximizes aggregate tokens across staggered windows. Switches at threshold or proactively after ≥10m of use if an idle 100% reset clock can be ignited.
                </div>
              </div>

              {/* Option 3: Max Continuous Usage */}
              <div
                onClick={() => setSwitchMode('max_continuous')}
                style={{
                  border: switchMode === 'max_continuous' ? '2px solid var(--primary)' : '1px solid var(--border)',
                  backgroundColor: switchMode === 'max_continuous' ? 'var(--primary-light, rgba(11, 87, 208, 0.04))' : 'var(--surface)',
                  borderRadius: '8px',
                  padding: '14px',
                  cursor: 'pointer',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '8px',
                  transition: 'all 0.15s ease',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <InfinityIcon size={15} color={switchMode === 'max_continuous' ? 'var(--primary)' : 'var(--text-muted)'} />
                    <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                      Max Continuous Usage
                    </span>
                  </div>
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
                  Maximizes continuous working time without switching. Prioritizes highest available 5h quota (Ultra 20X before Pro). Tie-breaks with reset countdowns.
                </div>
              </div>
            </div>
          </div>

          {/* 5-Hour Threshold */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                5-Hour Quota Threshold Trigger:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Triggers proactive rotation when the active account 5-hour quota drops to or below this threshold.
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
              <input
                type="number"
                min={1}
                max={50}
                step={1}
                value={thresholdText}
                onChange={(e) => handleThresholdTextChange(e.target.value, setThreshold, setThresholdText)}
                onBlur={() => commitThresholdPercent(thresholdText, threshold, setThreshold, setThresholdText)}
                style={{ width: '80px', textAlign: 'center' }}
                aria-label="5-Hour Quota Threshold Percent"
              />
              <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-muted)' }}>%</span>
            </div>
          </div>

          {/* 7-Day (Weekly) Quota Threshold */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                7-Day (Weekly) Quota Threshold Trigger:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Triggers auto-rotation when the active account 7-day rolling quota drops to or below this threshold, preventing lockouts when 5-hour quota remains high.
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
              <input
                type="number"
                min={1}
                max={50}
                step={1}
                value={weeklyThresholdText}
                onChange={(e) => handleThresholdTextChange(e.target.value, setWeeklyThreshold, setWeeklyThresholdText)}
                onBlur={() => commitThresholdPercent(weeklyThresholdText, weeklyThreshold, setWeeklyThreshold, setWeeklyThresholdText)}
                style={{ width: '80px', textAlign: 'center' }}
                aria-label="7-Day Quota Threshold Percent"
              />
              <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-muted)' }}>%</span>
            </div>
          </div>

          {/* Active Account Polling Interval */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Active Account Quota Refresh Interval:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                The active account quota is refreshed frequently (e.g. every 2m) to guarantee prompt rotation triggers.
              </div>
            </div>
            <select
              value={activePollingInterval}
              onChange={(e) => setActivePollingInterval(Number(e.target.value))}
              style={{ width: '160px' }}
            >
              <option value={30}>30 Seconds</option>
              <option value={60}>1 Minute</option>
              <option value={120}>2 Minutes</option>
              <option value={180}>3 Minutes</option>
              <option value={300}>5 Minutes</option>
            </select>
          </div>

          {/* Standby Accounts Polling Interval */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Standby Accounts Quota Refresh Interval:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Standby accounts are refreshed much less frequently. Quotas are polled in randomized order with random time gaps.
              </div>
            </div>
            <select
              value={standbyPollingInterval}
              onChange={(e) => setStandbyPollingInterval(Number(e.target.value))}
              style={{ width: '160px' }}
            >
              <option value={300}>5 Minutes</option>
              <option value={600}>10 Minutes</option>
              <option value={900}>15 Minutes</option>
              <option value={1800}>30 Minutes</option>
              <option value={3600}>1 Hour</option>
            </select>
          </div>

          {/* Standby Account Staggered Jitter Gap */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Standby Account Staggered Jitter Gap:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Random delay time gap between refreshing individual standby accounts to avoid spike load.
              </div>
            </div>
            <select
              value={standbyRandomJitter}
              onChange={(e) => setStandbyRandomJitter(Number(e.target.value))}
              style={{ width: '160px' }}
            >
              <option value={10}>5–10 Seconds Gap</option>
              <option value={20}>5–20 Seconds Gap</option>
              <option value={30}>5–30 Seconds Gap</option>
              <option value={45}>5–45 Seconds Gap</option>
              <option value={60}>5–60 Seconds Gap</option>
            </select>
          </div>
        </div>
      </div>

      {/* Section 2: Model Source Hierarchy & Failover Priority */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '14px' }}>
          <Layers size={16} style={{ color: 'var(--primary)' }} />
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Model Source Hierarchy & Priority Order
          </div>
        </div>
        <p style={{ margin: '0 0 16px', fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
          When quotas deplete or requests require fallback, Antigravity attempts model sources in this exact sequential order. Drag items or use arrows to adjust priority.
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          {hierarchy.map((sourceKey, idx) => {
            const isDragging = draggedIdx === idx
            const isOver = dragOverIdx === idx

            return (
              <div
                key={sourceKey}
                draggable={true}
                onDragStart={(e) => {
                  setDraggedIdx(idx)
                  e.dataTransfer.effectAllowed = 'move'
                  e.dataTransfer.setData('text/plain', String(idx))
                }}
                onDragOver={(e) => {
                  e.preventDefault()
                  e.dataTransfer.dropEffect = 'move'
                  if (dragOverIdx !== idx) setDragOverIdx(idx)
                }}
                onDragLeave={() => {
                  if (dragOverIdx === idx) setDragOverIdx(null)
                }}
                onDrop={(e) => {
                  e.preventDefault()
                  if (draggedIdx === null || draggedIdx === idx) {
                    setDraggedIdx(null)
                    setDragOverIdx(null)
                    return
                  }
                  const updated = [...hierarchy]
                  const [moved] = updated.splice(draggedIdx, 1)
                  updated.splice(idx, 0, moved)
                  setHierarchy(updated)
                  setDraggedIdx(null)
                  setDragOverIdx(null)
                }}
                onDragEnd={() => {
                  setDraggedIdx(null)
                  setDragOverIdx(null)
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '12px 16px',
                  borderRadius: '10px',
                  border: isOver ? '2px dashed var(--primary)' : '1.5px solid var(--border)',
                  background: isDragging ? 'var(--tonal)' : 'var(--canvas)',
                  opacity: isDragging ? 0.5 : 1,
                  cursor: 'grab',
                  userSelect: 'none',
                  transition: 'border-color 0.15s, background-color 0.15s, opacity 0.15s',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                  <div style={{ color: 'var(--text-muted)', display: 'flex', alignItems: 'center', cursor: 'grab' }} title="Drag to reorder priority">
                    <GripVertical size={16} />
                  </div>
                  <div
                    style={{
                      width: '28px',
                      height: '28px',
                      borderRadius: '8px',
                      background: 'var(--tonal)',
                      color: 'var(--text-muted)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      fontSize: '12px',
                      fontWeight: 700,
                    }}
                  >
                    {idx + 1}
                  </div>
                  <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                    {MODEL_SOURCE_LABELS[sourceKey]}
                  </span>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }} onClick={(e) => e.stopPropagation()}>
                  <button
                    type="button"
                    onClick={() => moveHierarchyItem(idx, 'up')}
                    disabled={idx === 0}
                    style={{
                      padding: '5px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      background: idx === 0 ? 'transparent' : '#ffffff',
                      color: idx === 0 ? 'var(--text-muted)' : 'var(--text)',
                      cursor: idx === 0 ? 'not-allowed' : 'pointer',
                      opacity: idx === 0 ? 0.4 : 1,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                    title="Move up in priority"
                  >
                    <ArrowUp size={14} />
                  </button>
                  <button
                    type="button"
                    onClick={() => moveHierarchyItem(idx, 'down')}
                    disabled={idx === hierarchy.length - 1}
                    style={{
                      padding: '5px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      background: idx === hierarchy.length - 1 ? 'transparent' : '#ffffff',
                      color: idx === hierarchy.length - 1 ? 'var(--text-muted)' : 'var(--text)',
                      cursor: idx === hierarchy.length - 1 ? 'not-allowed' : 'pointer',
                      opacity: idx === hierarchy.length - 1 ? 0.4 : 1,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                    title="Move down in priority"
                  >
                    <ArrowDown size={14} />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Section 3: AI Credits & Non-Gemini Feature Toggles */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Credit & External Model Policies
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {/* Allow AI Credits Usage */}
          <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '14px', cursor: 'pointer' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Allow AI Credits Usage
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                When standard quota limits are reached, permits the switcher to burn available AI account credits before switching accounts.
              </div>
            </div>
            <ToggleSwitch
              checked={allowAICredits}
              onChange={(checked) => setAllowAICredits(checked)}
              style={{ flexShrink: 0 }}
            />
          </label>

          {/* Allow Non-Gemini Native Models */}
          <div style={{ borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
            <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '14px', cursor: 'pointer' }}>
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Allow Non-Gemini Native Models
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                  Permit Antigravity to route requests to supported non-Gemini models (e.g. Anthropic Claude, OpenAI GPT) when enabled.
                </div>
              </div>
              <ToggleSwitch
                checked={allowNonGemini}
                onChange={(checked) => setAllowNonGemini(checked)}
                style={{ flexShrink: 0 }}
              />
            </label>
          </div>
        </div>
      </div>

      {/* Section 4: Default Models Configuration */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Default Models Configuration
          </div>
          <button
            type="button"
            className="secondary-btn"
            onClick={() => fetchAvailableModels(true)}
            disabled={isFetchingModels}
            style={{ fontSize: '11px', padding: '4px 10px', height: '26px', gap: '5px', display: 'flex', alignItems: 'center', cursor: isFetchingModels ? 'not-allowed' : 'pointer' }}
            title="Refresh active models from Google CloudCode and local configurations"
          >
            <RefreshCw size={12} className={isFetchingModels ? 'spinning' : ''} />
            {isFetchingModels ? 'Refreshing...' : 'Refresh Models'}
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {/* Default Gemini Model */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Default Gemini Model:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                The preferred Google Gemini model assigned for new conversations and default execution.
              </div>
            </div>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <select
                value={defaultGemini}
                onChange={(e) => {
                  const val = e.target.value
                  setDefaultGemini(val)
                  if (val.endsWith('-high') || val.includes('high')) {
                    setGeminiReasoningLevel('high')
                  } else if (val.endsWith('-medium') || val.includes('medium')) {
                    setGeminiReasoningLevel('medium')
                  } else if (val.endsWith('-low') || val.includes('low')) {
                    setGeminiReasoningLevel('low')
                  } else if (val.endsWith('-off') || val.includes('off')) {
                    setGeminiReasoningLevel('off')
                  }
                }}
                style={{ width: '220px' }}
                aria-label="Default Gemini Model"
              >
                {geminiModelOptions.length === 0 ? (
                  <option value={defaultGemini}>
                    {isFetchingModels ? 'Fetching live models...' : (defaultGemini || 'No Gemini models found')}
                  </option>
                ) : (
                  geminiModelOptions.map((opt) => (
                    <option key={opt.id} value={opt.id}>
                      {opt.display_name}
                    </option>
                  ))
                )}
              </select>

              <select
                value={geminiReasoningLevel}
                onChange={(e) => {
                  const lvl = e.target.value
                  setGeminiReasoningLevel(lvl)
                  const baseModel = defaultGemini.replace(/-(high|medium|low|off)$/, '')
                  const candidate = `${baseModel}-${lvl}`
                  if (geminiModelOptions.some((opt) => opt.id === candidate)) {
                    setDefaultGemini(candidate)
                  }
                }}
                style={{ width: '100px' }}
                aria-label="Default Gemini Reasoning Level"
                title="Reasoning Level for Gemini models"
              >
                <option value="high">High</option>
                <option value="medium">Medium</option>
                <option value="low">Low</option>
                <option value="off">Off</option>
              </select>
            </div>
          </div>

          {/* Default Custom Model */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px', borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Default Custom Model:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Selected BYOK model to route to when custom model source is triggered.
              </div>
            </div>
            <select
              value={effectiveCustomModel}
              onChange={(e) => setDefaultCustom(e.target.value)}
              disabled={customModelOptions.length === 0}
              style={{
                width: '240px',
                cursor: customModelOptions.length === 0 ? 'not-allowed' : 'pointer',
                opacity: customModelOptions.length === 0 ? 0.6 : 1,
                backgroundColor: customModelOptions.length === 0 ? 'var(--canvas)' : undefined,
                color: customModelOptions.length === 0 ? 'var(--text-muted)' : undefined,
                borderColor: customModelOptions.length === 0 ? 'var(--border)' : undefined,
              }}
              aria-label="Default Custom Model"
            >
              {customModelOptions.length === 0 ? (
                <option value=""></option>
              ) : (
                customModelOptions.map((cm) => (
                  <option key={cm.id} value={cm.id}>
                    {cm.name || cm.id}
                  </option>
                ))
              )}
            </select>
          </div>

          {/* Default Non-Gemini Native Model */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px', borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Default Non-Gemini Native Model:
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                The fallback third-party native model to utilize when non-Gemini routing is enabled.
              </div>
            </div>
            <select
              value={defaultNonGemini}
              onChange={(e) => setDefaultNonGemini(e.target.value)}
              style={{ width: '240px' }}
              aria-label="Default Non-Gemini Native Model"
            >
              {nonGeminiModelOptions.length === 0 ? (
                <option value={defaultNonGemini}>
                  {isFetchingModels ? 'Fetching live models...' : (defaultNonGemini || 'No Non-Gemini models found')}
                </option>
              ) : (
                nonGeminiModelOptions.map((opt) => (
                  <option key={opt.id} value={opt.id}>
                    {opt.display_name}
                  </option>
                ))
              )}
            </select>
          </div>
        </div>
      </div>

      {/* Section 5: Post-Reset Keep-Alive Warmup Engine */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Post-Reset Keep-Alive Warmup Engine
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', cursor: 'pointer' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Enable Automatic Standby Keep-Alive Warmup
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Dispatches a lightweight probe to newly-reset accounts so their quota pool is immediately warm.
              </div>
            </div>
            <ToggleSwitch
              checked={warmupEnabled}
              onChange={(checked) => setWarmupEnabled(checked)}
              style={{ flexShrink: 0 }}
            />
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

      {/* Auto-Import */}
      <div className="google-card">
        <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', cursor: 'pointer' }}>
          <div>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
              Auto Import &amp; Sync Antigravity Account
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '3px', lineHeight: 1.5 }}>
              Imports the account signed in to Antigravity when Swiss Knife doesn't have it yet and makes it active. Antigravity 2.0 takes priority; the CLI and VS Code extension follow it.
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
              When off, an account that isn't imported leaves Swiss Knife with no active account.
            </div>
          </div>
          <ToggleSwitch
            checked={autoImportActive}
            onChange={(checked) => setAutoImportActive(checked)}
            style={{ flexShrink: 0 }}
          />
        </label>
      </div>

      {/* 6. Account Data Export & Import (JSON) */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Account Data Export &amp; Import (JSON)
        </div>

        {/* Status notification banner */}
        {importStatus && (
          <div
            style={{
              padding: '10px 14px',
              borderRadius: '8px',
              backgroundColor: importStatus.type === 'success' ? '#e6f4ea' : '#fce8e6',
              border: `1px solid ${importStatus.type === 'success' ? '#ceead6' : '#fad2cf'}`,
              color: importStatus.type === 'success' ? '#137333' : '#c5221f',
              fontSize: '12px',
              fontWeight: 500,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '8px',
              marginBottom: '16px',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              {importStatus.type === 'success' ? <CheckCircle2 size={16} /> : <AlertCircle size={16} />}
              <span>{importStatus.message}</span>
            </div>
            <button
              type="button"
              onClick={() => setImportStatus(null)}
              style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '2px', color: 'inherit' }}
            >
              <X size={14} />
            </button>
          </div>
        )}

        {/* Two-card layout: Left = Export, Right = Import */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '16px' }}>
          {/* Export Card */}
          <div
            style={{
              padding: '16px',
              borderRadius: '12px',
              backgroundColor: 'var(--canvas)',
              border: '1px solid var(--border)',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
            }}
          >
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                  Export Accounts
                </span>
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '12px', lineHeight: 1.4 }}>
                Download all saved accounts or copy JSON directly. Includes ID, password, MFA, and OAuth tokens.
              </div>
            </div>

            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', alignItems: 'center' }}>
              <button
                type="button"
                onClick={handleExportDownload}
                disabled={isExporting}
                className="btn-pill-primary"
                style={{
                  padding: '7px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                }}
              >
                <Download size={14} />
                {isExporting ? 'Exporting...' : 'Export JSON File'}
              </button>

              <button
                type="button"
                onClick={handleExportCopy}
                disabled={isExporting}
                className="btn-pill-outlined"
                style={{
                  padding: '7px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                }}
              >
                {copiedExport ? <Check size={14} color="#137333" /> : <Copy size={14} />}
                {copiedExport ? 'Copied' : 'Copy JSON'}
              </button>

              <button
                type="button"
                onClick={handleTogglePreview}
                disabled={isExporting}
                className="btn-pill-outlined"
                style={{
                  padding: '7px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                }}
              >
                <FileText size={14} />
                {showExportPreview ? 'Hide Preview' : 'Preview'}
              </button>
            </div>
          </div>

          {/* Import Card */}
          <div
            style={{
              padding: '16px',
              borderRadius: '12px',
              backgroundColor: 'var(--canvas)',
              border: '1px solid var(--border)',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
            }}
          >
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                  Import Accounts
                </span>
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '12px', lineHeight: 1.4 }}>
                Upload a JSON file or paste JSON text. Existing accounts are updated; new accounts are safely registered.
              </div>
            </div>

            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', alignItems: 'center' }}>
              <label
                className="btn-pill-primary"
                style={{
                  padding: '7px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                  cursor: isImporting ? 'not-allowed' : 'pointer',
                  opacity: isImporting ? 0.7 : 1,
                }}
              >
                <Upload size={14} />
                {isImporting ? 'Importing...' : 'Upload JSON File'}
                <input
                  type="file"
                  accept=".json,application/json"
                  onChange={handleImportFileUpload}
                  disabled={isImporting}
                  style={{ display: 'none' }}
                />
              </label>

              <button
                type="button"
                onClick={() => setShowPasteModal(true)}
                disabled={isImporting}
                className="btn-pill-outlined"
                style={{
                  padding: '7px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                }}
              >
                <FileText size={14} />
                Paste JSON Text
              </button>
            </div>
          </div>
        </div>

        {/* Collapsible Export Preview */}
        {showExportPreview && (
          <div
            style={{
              marginTop: '16px',
              padding: '12px 16px',
              borderRadius: '8px',
              backgroundColor: 'var(--canvas)',
              border: '1px solid var(--border)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
              <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Export JSON Preview
              </span>
              <button
                type="button"
                onClick={handleExportCopy}
                className="btn-pill-tonal"
                style={{ padding: '3px 8px', fontSize: '11px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
              >
                {copiedExport ? <Check size={12} color="#137333" /> : <Copy size={12} />}
                {copiedExport ? 'Copied' : 'Copy Preview'}
              </button>
            </div>
            <pre
              style={{
                margin: 0,
                maxHeight: '260px',
                overflowY: 'auto',
                fontSize: '11px',
                fontFamily: 'monospace',
                color: 'var(--text)',
                backgroundColor: 'var(--card)',
                padding: '10px',
                borderRadius: '6px',
                border: '1px solid var(--border)',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
              }}
            >
              {exportPreviewText || 'Loading export preview...'}
            </pre>
          </div>
        )}
      </div>

      {/* Paste JSON Import Modal */}
      {showPasteModal && (
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
          onClick={() => setShowPasteModal(false)}
        >
          <div
            className="google-card"
            style={{
              width: '640px',
              maxWidth: '92vw',
              maxHeight: '90vh',
              overflowY: 'auto',
              padding: '24px',
              display: 'flex',
              flexDirection: 'column',
              gap: '16px',
              backgroundColor: '#ffffff',
              boxShadow: 'var(--shadow-lg)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '12px' }}>
              <div>
                <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                  Import Accounts (JSON)
                </h3>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
                  Paste account credentials in JSON format. Supported attributes: <code>id</code> / <code>email</code>, <code>password</code>, <code>mfa</code> / <code>totp_secret</code>, and <code>oath_token</code> / <code>refresh_token</code>.
                </div>
              </div>
              <button
                type="button"
                onClick={() => setShowPasteModal(false)}
                className="btn-pill-tonal"
                style={{ padding: '6px' }}
                title="Close"
              >
                <X size={16} />
              </button>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                JSON Payload
              </span>
              <button
                type="button"
                onClick={() => setPasteText(sampleJson)}
                className="btn-pill-tonal"
                style={{ padding: '4px 8px', fontSize: '11px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
              >
                <Code size={12} />
                Insert Sample Template
              </button>
            </div>

            <textarea
              value={pasteText}
              onChange={(e) => setPasteText(e.target.value)}
              placeholder={`[\n  {\n    "id": "user@gmail.com",\n    "password": "Password123!",\n    "mfa": "JBSWY3DPEHPK3PXP",\n    "oath_token": "1//06xyz..."\n  }\n]`}
              style={{
                width: '100%',
                height: '240px',
                padding: '12px',
                fontFamily: 'monospace',
                fontSize: '12px',
                lineHeight: 1.5,
                borderRadius: '8px',
                border: '1px solid var(--border)',
                backgroundColor: 'var(--canvas)',
                color: 'var(--text)',
                resize: 'vertical',
                outline: 'none',
                boxSizing: 'border-box',
              }}
            />

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
              <button
                type="button"
                onClick={() => setShowPasteModal(false)}
                className="btn-pill-outlined"
                style={{ padding: '7px 16px', fontSize: '12px', fontWeight: 600 }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => processImportJson(pasteText)}
                disabled={isImporting || !pasteText.trim()}
                className="btn-pill-primary"
                style={{ padding: '7px 18px', fontSize: '12px', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: '6px' }}
              >
                <Upload size={14} />
                {isImporting ? 'Importing...' : 'Import Accounts'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
