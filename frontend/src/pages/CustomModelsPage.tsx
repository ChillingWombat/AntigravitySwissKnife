import React, { useState, useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import {
  Plus,
  RefreshCw,
  Trash2,
  Edit2,
  CheckCircle2,
  AlertCircle,
  X,
  Cpu,
  Eye,
  EyeOff,
  Zap,
  Info,
  Shield,
  ShieldCheck,
  Loader2,
} from 'lucide-react'
import type {
  CustomModel,
  CustomModelsConfig,
  ModelInfo,
  ProviderType,
  TestResult,
  SecurityAuditReport,
} from '../types'
import { CircularGauge } from '../components/CircularGauge'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { SecurityReportModal } from '../components/SecurityReportModal'
import { auditModelSecurity } from '../utils/securityAudit'
import {
  filterModels,
  computeModelFilterStats,
  type ModelFilterOption,
} from '../utils/modelFilter'
import {
  extractCleanModelId,
  extractContextWindow,
  detectThinkingLevels,
} from '../utils/modelExtraction'
import { getTestConnectionButtonPresentation } from '../utils/testConnectionButton'
import { api } from '../api'

const inferProviderType = (url: string): ProviderType => {
  const lower = (url || '').toLowerCase()
  if (lower.includes('anthropic.com') || lower.endsWith('/messages')) {
    return 'anthropic'
  }
  if (lower.includes('generativelanguage.googleapis.com') || lower.includes(':generatecontent')) {
    return 'gemini'
  }
  if (lower.includes('openai.com') || lower.includes('/chat/completions')) {
    return 'openai'
  }
  return 'custom'
}

export const CustomModelsPage: React.FC = () => {
  const [config, setConfig] = useState<CustomModelsConfig | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [feedback, setFeedback] = useState<string | null>(null)

  // Card quick test states
  const [testingModelId, setTestingModelId] = useState<string | null>(null)
  const [cardTestResults, setCardTestResults] = useState<Record<string, TestResult>>({})

  // Modal State
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)
  const [editingModel, setEditingModel] = useState<CustomModel | null>(null)
  const [deleteConfirmModel, setDeleteConfirmModel] = useState<{ id: string; name: string } | null>(null)
  const [showApiKey, setShowApiKey] = useState<boolean>(false)

  // Modal Form Fields
  const [displayName, setDisplayName] = useState<string>('')
  const [modelName, setModelName] = useState<string>('')
  const [providerType, setProviderType] = useState<ProviderType>('gemini')
  const [baseUrl, setBaseUrl] = useState<string>('')
  const [apiKey, setApiKey] = useState<string>('')
  const [projectMappings, setProjectMappings] = useState<string>('*')
  const [isDefault, setIsDefault] = useState<boolean>(false)
  const [enabled, setEnabled] = useState<boolean>(true)
  const [contextWindow, setContextWindow] = useState<number>(1048576)
  const [notes, setNotes] = useState<string>('')
  const [refreshingQuotas, setRefreshingQuotas] = useState<boolean>(false)

  // Reasoning / Thinking Configuration
  const [thinkingLevels, setThinkingLevels] = useState<string[]>(['low', 'medium', 'high'])
  const [thinkingLevel, setThinkingLevel] = useState<string>('high')

  // Model Fetching States
  const [fetchingModels, setFetchingModels] = useState<boolean>(false)
  const [fetchedModels, setFetchedModels] = useState<ModelInfo[]>([])
  const [fetchFeedback, setFetchFeedback] = useState<string | null>(null)

  // Dropdown Popover States & Refs
  const [showModelDropdown, setShowModelDropdown] = useState<boolean>(false)
  const [showReasoningDropdown, setShowReasoningDropdown] = useState<boolean>(false)
  const modelContainerRef = useRef<HTMLDivElement>(null)
  const reasoningContainerRef = useRef<HTMLDivElement>(null)

  // Modal Test & Save States
  const [modalTesting, setModalTesting] = useState<boolean>(false)
  const [modalTestResult, setModalTestResult] = useState<TestResult | null>(null)
  const [modalSaving, setModalSaving] = useState<boolean>(false)
  const [modalError, setModalError] = useState<string | null>(null)

  // Security Audit States (inspired by toby-bridges/api-relay-audit)
  const [modalAuditResult, setModalAuditResult] = useState<SecurityAuditReport | null>(null)
  const [isReportModalOpen, setIsReportModalOpen] = useState<boolean>(false)
  const [activeReportForView, setActiveReportForView] = useState<SecurityAuditReport | null>(null)

  const loadData = async () => {
    setLoading(true)
    setFeedback(null)
    try {
      const cfg = await api.getCustomModels()
      setConfig(cfg)
    } catch (err: any) {
      setFeedback(`Failed to load custom models: ${err.message}`)
    } finally {
      setLoading(false)
    }
  }

  const [modelFilter, setModelFilter] = useState<ModelFilterOption>('all')
  const [portalTarget, setPortalTarget] = useState<HTMLElement | null>(null)
  const [portalLeftTarget, setPortalLeftTarget] = useState<HTMLElement | null>(null)

  useEffect(() => {
    setPortalTarget(document.getElementById('top-bar-right'))
    setPortalLeftTarget(document.getElementById('top-bar-left'))
  }, [])

  useEffect(() => {
    loadData()
  }, [])

  // Click-outside listener to dismiss popover dropdowns
  useEffect(() => {
    const handleDocumentClick = (e: MouseEvent) => {
      if (modelContainerRef.current && !modelContainerRef.current.contains(e.target as Node)) {
        setShowModelDropdown(false)
      }
      if (reasoningContainerRef.current && !reasoningContainerRef.current.contains(e.target as Node)) {
        setShowReasoningDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleDocumentClick, true)
    document.addEventListener('click', handleDocumentClick, true)
    return () => {
      document.removeEventListener('mousedown', handleDocumentClick, true)
      document.removeEventListener('click', handleDocumentClick, true)
    }
  }, [])

  const openAddModal = () => {
    setEditingModel(null)
    setDisplayName('')
    setModelName('')
    setProviderType('custom')
    setBaseUrl('')
    setApiKey('')
    setProjectMappings('*')
    setIsDefault((config?.models?.length ?? 0) === 0)
    setEnabled(false)
    setContextWindow(1048576)
    setNotes('')
    setThinkingLevels(['low', 'medium', 'high'])
    setThinkingLevel('high')
    setFetchedModels([])
    setFetchFeedback(null)
    setShowModelDropdown(false)
    setShowReasoningDropdown(false)
    setShowApiKey(false)
    setModalTestResult(null)
    setModalAuditResult(null)
    setModalError(null)
    setIsModalOpen(true)
  }

  const openEditModal = (model: CustomModel) => {
    setEditingModel(model)
    setDisplayName(model.display_name)
    setModelName(model.name)
    setProviderType(model.provider_type || inferProviderType(model.base_url))
    setBaseUrl(model.base_url)
    setApiKey(model.api_key || '')
    setProjectMappings(model.project_mappings?.join(', ') || '*')
    setIsDefault(model.is_default)
    setEnabled(model.enabled)
    setContextWindow(model.context_window || 1048576)
    setNotes(model.notes || '')
    const isThinking = !!model.supports_thinking && !!model.thinking_level && model.thinking_level.toLowerCase() !== 'off'
    const rawLevels = (model.thinking_levels || ['low', 'medium', 'high']).filter((l) => l.toLowerCase() !== 'off')
    setThinkingLevels(rawLevels.length > 0 ? rawLevels : ['low', 'medium', 'high'])
    setThinkingLevel(isThinking ? (model.thinking_level || 'high') : '')
    setFetchedModels([])
    setFetchFeedback(null)
    setShowModelDropdown(false)
    setShowReasoningDropdown(false)
    setShowApiKey(false)
    setModalTestResult(null)
    if (model.security_risk_level) {
      setModalAuditResult({
        risk_level: model.security_risk_level,
        risk_score: model.security_audit_score ?? (model.security_risk_level === 'low' ? 5 : model.security_risk_level === 'medium' ? 30 : 65),
        model_id: model.id,
        endpoint: model.base_url,
        provider_type: model.provider_type,
        audited_at: model.last_security_audit || new Date().toISOString(),
        summary: `Previously audited: ${model.security_risk_level.toUpperCase()} Risk.`,
        probes: [],
        recommendations: [],
      })
    } else {
      setModalAuditResult(null)
    }
    setModalError(null)
    setIsModalOpen(true)
  }

  const handleFetchModels = async () => {
    if (!baseUrl.trim()) {
      setModalError('Please enter an Endpoint URL before fetching models.')
      return
    }
    setFetchingModels(true)
    setFetchFeedback(null)
    setModalError(null)
    const inferred = inferProviderType(baseUrl.trim())
    setProviderType(inferred)
    try {
      const res = await api.fetchModels(inferred, baseUrl.trim(), apiKey.trim())
      if (res.success && res.models && res.models.length > 0) {
        setFetchedModels(res.models)
        setShowModelDropdown(true)
        setFetchFeedback(`Discovered ${res.models.length} model(s) via API.`)
      } else {
        const cleanMsg = (res.message || 'No models returned by API.')
          .replace(/<[^>]*>?/gm, ' ')
          .replace(/\s+/g, ' ')
          .trim()
        setFetchFeedback(cleanMsg.length > 180 ? cleanMsg.slice(0, 180) + '...' : cleanMsg)
      }
    } catch (err: any) {
      const cleanMsg = (err.message || 'Unknown fetch error')
        .replace(/<[^>]*>?/gm, ' ')
        .replace(/\s+/g, ' ')
        .trim()
      setFetchFeedback(`Model fetch failed: ${cleanMsg.length > 180 ? cleanMsg.slice(0, 180) + '...' : cleanMsg}`)
    } finally {
      setFetchingModels(false)
    }
  }

  const handleSelectFetchedModel = (selectedId: string) => {
    const found = fetchedModels.find((m) => m.id === selectedId)
    if (!found) return
    const cleanId = extractCleanModelId(found.id || '')
    setModelName(cleanId)
    if (!displayName.trim()) {
      setDisplayName(found.display_name || cleanId)
    }
    const extractedCtx = extractContextWindow(found.id, found)
    if (extractedCtx && extractedCtx > 0) {
      setContextWindow(extractedCtx)
    } else if (found.context_window && found.context_window > 0) {
      setContextWindow(found.context_window)
    } else {
      setContextWindow(1048576)
    }

    const detectedLevels = detectThinkingLevels(cleanId, found)
    if (detectedLevels.length > 0) {
      setThinkingLevels(detectedLevels)
      setThinkingLevel(detectedLevels.includes('high') ? 'high' : detectedLevels[0])
    } else {
      setThinkingLevels([])
      setThinkingLevel('')
    }
  }

  const handleTestInModal = async () => {
    if (!baseUrl.trim()) {
      setModalError('Please enter an Endpoint URL before testing.')
      return
    }
    if (!modelName.trim()) {
      setModalError('Please enter or select a Model Identifier before testing.')
      return
    }

    setModalTesting(true)
    setModalTestResult(null)
    setModalError(null)

    const isThinking = thinkingLevel.trim() !== '' && thinkingLevel.toLowerCase() !== 'off'
    const activeThinkingLvl = isThinking ? thinkingLevel.trim() : ''
    const cleanThinkingLevels = thinkingLevels.filter((l) => l.toLowerCase() !== 'off' && l.trim() !== '')
    const inferred = inferProviderType(baseUrl.trim())
    const draftModel: CustomModel = {
      id: editingModel?.id || 'draft-test',
      name: modelName.trim(),
      display_name: displayName.trim(),
      provider_type: inferred,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      project_mappings: projectMappings.split(',').map((p) => p.trim()).filter(Boolean),
      quota_type: editingModel?.quota_type || 'na',
      balance_value: editingModel?.balance_value,
      quota_value: editingModel?.quota_value,
      prepaid_balance: editingModel?.prepaid_balance || 0,
      total_budget: editingModel?.total_budget || 0,
      quota_fraction: editingModel?.quota_fraction ?? null,
      is_default: isDefault,
      enabled: enabled,
      context_window: Number(contextWindow) || 1048576,
      supports_thinking: isThinking,
      thinking_levels: isThinking ? (cleanThinkingLevels.length > 0 ? cleanThinkingLevels : [activeThinkingLvl]) : undefined,
      thinking_level: isThinking ? activeThinkingLvl : '',
      notes: notes.trim(),
    }

    try {
      const res = await api.testCustomModel(draftModel)
      setModalTestResult(res)
    } catch (err: any) {
      const message = err.message || 'Request failed'
      setModalError(`Test request failed: ${message}`)
      setModalTestResult({
        success: false,
        status_code: 0,
        latency_ms: 0,
        message,
        endpoint: baseUrl.trim(),
      })
    } finally {
      setModalTesting(false)
    }
  }

  const handleSaveModel = async () => {
    if (!modelName.trim()) {
      setModalError('Model identifier name is required.')
      return
    }
    if (!baseUrl.trim()) {
      setModalError('Endpoint URL is required.')
      return
    }

    setModalSaving(true)
    setModalError(null)

    const mappings = projectMappings
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean)

    const isThinking = thinkingLevel.trim() !== '' && thinkingLevel.toLowerCase() !== 'off'
    const activeThinkingLvl = isThinking ? thinkingLevel.trim() : ''
    const updatedThinkingLevels = [...thinkingLevels].filter((l) => l.toLowerCase() !== 'off' && l.trim() !== '')
    if (
      isThinking &&
      activeThinkingLvl &&
      !updatedThinkingLevels.map((l) => l.toLowerCase()).includes(activeThinkingLvl.toLowerCase())
    ) {
      updatedThinkingLevels.push(activeThinkingLvl.toLowerCase())
    }

    const effectiveProvider = providerType || inferProviderType(baseUrl.trim()) || 'custom'

    const modelToSave: CustomModel = {
      id:
        editingModel?.id ||
        `${effectiveProvider}-${modelName.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${Date.now()}`,
      name: modelName.trim(),
      display_name: displayName.trim() || modelName.trim(),
      provider_type: effectiveProvider,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      project_mappings: mappings.length > 0 ? mappings : ['*'],
      quota_type: editingModel?.quota_type || 'na',
      balance_value: editingModel?.balance_value,
      quota_value: editingModel?.quota_value,
      prepaid_balance: editingModel?.prepaid_balance || 0,
      total_budget: editingModel?.total_budget || 0,
      quota_fraction: editingModel?.quota_fraction ?? null,
      is_default: isDefault,
      enabled: enabled,
      context_window: Number(contextWindow) || 1048576,
      supports_thinking: isThinking,
      thinking_levels: isThinking ? (updatedThinkingLevels.length > 0 ? updatedThinkingLevels : [activeThinkingLvl]) : undefined,
      thinking_level: isThinking ? activeThinkingLvl : '',
      notes: notes.trim(),
      security_risk_level: modalAuditResult?.risk_level || editingModel?.security_risk_level,
      security_audit_score: modalAuditResult?.risk_score ?? editingModel?.security_audit_score,
      last_security_audit: modalAuditResult?.audited_at || editingModel?.last_security_audit,
    }

    try {
      await api.saveCustomModel(modelToSave)
      setIsModalOpen(false)
      await loadData()
    } catch (err: any) {
      setModalError(`Failed to save model: ${err.message}`)
    } finally {
      setModalSaving(false)
    }
  }

  const handleDeleteModel = (id: string, name: string) => {
    setDeleteConfirmModel({ id, name })
  }

  const confirmDeleteModel = async () => {
    if (!deleteConfirmModel) return
    const { id } = deleteConfirmModel
    setModalSaving(true)
    try {
      await api.deleteCustomModel(id)
      setDeleteConfirmModel(null)
      if (isModalOpen && editingModel?.id === id) {
        setIsModalOpen(false)
      }
      await loadData()
    } catch (err: any) {
      setFeedback(`Delete failed: ${err.message}`)
      setDeleteConfirmModel(null)
    } finally {
      setModalSaving(false)
    }
  }

  const handleToggleModelEnabled = async (model: CustomModel, newEnabled: boolean) => {
    // Optimistic UI update
    setConfig((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        models: prev.models.map((item) =>
          item.id === model.id ? { ...item, enabled: newEnabled } : item
        ),
      }
    })

    try {
      await api.saveCustomModel({
        ...model,
        enabled: newEnabled,
      })
    } catch (err: any) {
      setFeedback(`Failed to update model status: ${err.message}`)
      await loadData()
    }
  }

  const handleCardTest = async (model: CustomModel) => {
    setTestingModelId(model.id)
    try {
      const res = await api.testCustomModel(model)
      setCardTestResults((prev) => ({ ...prev, [model.id]: res }))
      if (res.quota_result && res.quota_result.quota_type) {
        setConfig((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            models: prev.models.map((item) =>
              item.id === model.id
                ? {
                    ...item,
                    quota_type: res.quota_result!.quota_type,
                    balance_value: res.quota_result!.balance_value,
                    quota_value: res.quota_result!.quota_value,
                    quota_fraction: res.quota_result!.fraction,
                  }
                : item
            ),
          }
        })
      }
    } catch (err: any) {
      setCardTestResults((prev) => ({
        ...prev,
        [model.id]: {
          success: false,
          latency_ms: 0,
          status_code: 500,
          message: err.message || 'Network error',
          endpoint: model.base_url,
        },
      }))
    } finally {
      setTestingModelId(null)
    }
  }

  const handleRefreshAllQuotas = async () => {
    setRefreshingQuotas(true)
    try {
      const updatedCfg = await api.refreshCustomModelQuotas()
      setConfig(updatedCfg)
    } catch (err: any) {
      setFeedback(`Failed to refresh quotas: ${err.message}`)
    } finally {
      setRefreshingQuotas(false)
    }
  }

  const models = config?.models || []
  const filterStats = computeModelFilterStats(models)
  const filteredModels = filterModels(models, modelFilter)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Top Bar Left Filter Pills via Portal */}
      {portalLeftTarget &&
        createPortal(
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              backgroundColor: 'var(--tonal)',
              borderRadius: '20px',
              padding: '3px',
              gap: '2px',
            }}
          >
            {[
              { id: 'all', label: 'All Models', count: filterStats.total },
              { id: 'enabled', label: 'Enabled', count: filterStats.enabled },
              { id: 'disabled', label: 'Disabled', count: filterStats.disabled },
            ].map((tab) => {
              const isActive = modelFilter === tab.id
              return (
                <button
                  key={tab.id}
                  onClick={() => setModelFilter(tab.id as ModelFilterOption)}
                  style={{
                    borderRadius: '16px',
                    padding: '6px 16px',
                    fontSize: '12px',
                    fontWeight: isActive ? 600 : 500,
                    color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                    backgroundColor: isActive ? '#ffffff' : 'transparent',
                    boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                    border: 'none',
                    cursor: 'pointer',
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '6px',
                    whiteSpace: 'nowrap',
                    lineHeight: 1.4,
                    transition: 'all 0.15s ease',
                  }}
                >
                  <span>{tab.label}</span>
                  <span
                    style={{
                      fontSize: '11px',
                      fontWeight: 600,
                      padding: '1px 6px',
                      borderRadius: '10px',
                      backgroundColor: isActive ? 'rgba(26, 115, 232, 0.1)' : 'rgba(0, 0, 0, 0.05)',
                      color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                    }}
                  >
                    {tab.count}
                  </span>
                </button>
              )
            })}
          </div>,
          portalLeftTarget
        )}

      {/* Top Bar Action Buttons via Portal */}
      {portalTarget &&
        createPortal(
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <button onClick={openAddModal} className="btn-pill-primary" style={{ padding: '7px 16px', fontSize: '12px' }}>
              <Plus size={14} /> Add Custom Model
            </button>
            <button
              onClick={handleRefreshAllQuotas}
              disabled={refreshingQuotas || loading}
              className="btn-pill-tonal"
              style={{ padding: '7px 12px', display: 'flex', alignItems: 'center', gap: '6px' }}
              title="Auto-query provider endpoints to refresh balances and rate-limit quotas"
            >
              <RefreshCw size={13} className={refreshingQuotas ? 'spin' : ''} />
              <span style={{ fontSize: '12px' }}>{refreshingQuotas ? 'Refreshing...' : 'Refresh Quotas'}</span>
            </button>
            <button onClick={loadData} disabled={loading} className="btn-pill-tonal" style={{ padding: '7px 12px' }} title="Reload Models">
              <RefreshCw size={14} className={loading && !refreshingQuotas ? 'spin' : ''} />
            </button>
          </div>,
          portalTarget
        )}

      {feedback && (
        <div style={{ padding: '12px 16px', borderRadius: '8px', backgroundColor: '#fce8e6', color: '#b3261e', fontSize: '12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <AlertCircle size={15} />
          <span>{feedback}</span>
        </div>
      )}

      {/* 2. Models Grid */}
      {models.length === 0 && !loading ? (
        <div
          className="google-card"
          style={{
            padding: '48px 24px',
            textAlign: 'center',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '12px',
          }}
        >
          <Cpu size={40} color="var(--text-subtle)" />
          <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text)' }}>
            No Custom Models Configured
          </div>
          <button onClick={openAddModal} className="btn-pill-primary" style={{ marginTop: '8px' }}>
            <Plus size={14} /> Add Custom Model
          </button>
        </div>
      ) : filteredModels.length === 0 && !loading ? (
        <div
          className="google-card"
          style={{
            padding: '48px 24px',
            textAlign: 'center',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '12px',
          }}
        >
          <Cpu size={40} color="var(--text-subtle)" />
          <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text)' }}>
            No {modelFilter === 'enabled' ? 'Enabled' : 'Disabled'} Models
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text-muted)', maxWidth: '400px' }}>
            {modelFilter === 'enabled'
              ? 'None of your custom models are currently enabled. Turn on the toggle switch on a model card to enable it.'
              : 'All configured custom models are currently enabled.'}
          </div>
          <button
            onClick={() => setModelFilter('all')}
            className="btn-pill-tonal"
            style={{ marginTop: '8px', padding: '6px 16px', fontSize: '12px' }}
          >
            Show All Models
          </button>
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: '16px' }}>
          {filteredModels.map((m) => {
            const testResult = cardTestResults[m.id]
            const isTesting = testingModelId === m.id

            // Quota Gauge calculation
            let gaugePct: number | null = null
            let gaugeTitle = 'Untracked'
            let emptyGrey = true

            const qType = (m.quota_type || '').toLowerCase()

            if (qType === 'balance' || qType === 'cost_based') {
              if (m.balance_value && m.balance_value.trim()) {
                gaugeTitle = `Balance: ${m.balance_value}`
              } else if (m.prepaid_balance > 0) {
                gaugeTitle = `Balance: $${m.prepaid_balance.toFixed(2)}`
              } else {
                gaugeTitle = 'Balance'
              }

              if (m.quota_fraction !== null && m.quota_fraction !== undefined) {
                gaugePct = Math.max(0, Math.min(100, Math.round(m.quota_fraction * 100)))
                emptyGrey = false
              } else if (m.total_budget > 0) {
                gaugePct = Math.max(0, Math.min(100, Math.round((m.prepaid_balance / m.total_budget) * 100)))
                emptyGrey = false
              } else {
                emptyGrey = true
                gaugePct = null
              }
            } else if (qType === 'quota' || qType === 'quota_based') {
              if (m.quota_value && m.quota_value.trim()) {
                gaugeTitle = `Quota: ${m.quota_value}`
              } else {
                // If endpoint only returns a percentage value, then show 'Quota'
                gaugeTitle = 'Quota'
              }

              if (m.quota_fraction !== null && m.quota_fraction !== undefined) {
                gaugePct = Math.max(0, Math.min(100, Math.round(m.quota_fraction * 100)))
                emptyGrey = false
              } else {
                emptyGrey = true
                gaugePct = null
              }
            } else {
              emptyGrey = true
              gaugePct = null
              gaugeTitle = 'Untracked'
            }

            return (
              <div
                key={m.id}
                className="google-card"
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  justifyContent: 'space-between',
                  padding: '16px',
                  border: m.is_default ? '2px solid var(--primary)' : '1px solid var(--border)',
                  opacity: m.enabled ? 1 : 0.65,
                  transition: 'opacity 0.2s ease, border-color 0.2s ease',
                }}
              >
                <div>
                  {/* Model Name and Gauge Row */}
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px' }}>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <h3 style={{ fontSize: '15px', fontWeight: 700, color: 'var(--text)', marginBottom: '3px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {m.display_name}
                      </h3>
                      <div style={{ fontFamily: 'monospace', fontSize: '11.5px', color: 'var(--text-muted)', marginBottom: '6px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {m.name}
                      </div>
                      <div
                        style={{
                          fontSize: '11px',
                          color: 'var(--text-subtle)',
                          fontFamily: 'monospace',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                          maxWidth: '100%',
                          marginBottom: '6px',
                        }}
                        title={m.base_url}
                      >
                        {m.base_url}
                      </div>

                      {/* Project Mappings */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap' }}>
                        <span style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)' }}>
                          PROJECTS:
                        </span>
                        {m.project_mappings?.map((p, idx) => (
                          <span
                            key={idx}
                            style={{
                              backgroundColor: 'var(--tonal)',
                              color: 'var(--text)',
                              fontSize: '10px',
                              padding: '2px 6px',
                              borderRadius: '4px',
                              fontFamily: 'monospace',
                            }}
                          >
                            {p === '*' ? '* (All Projects)' : p}
                          </span>
                        ))}
                      </div>

                      {m.notes && (
                        <div
                          style={{
                            fontSize: '11px',
                            color: 'var(--text-muted)',
                            marginTop: '6px',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                            maxWidth: '100%',
                          }}
                          title={m.notes}
                        >
                          Note: {m.notes}
                        </div>
                      )}
                    </div>

                    {/* Circular Quota Gauge */}
                    <div style={{ flexShrink: 0 }}>
                      <CircularGauge
                        percentage={gaugePct}
                        emptyGrey={emptyGrey}
                        title={gaugeTitle}
                        size={88}
                        strokeWidth={8}
                      />
                    </div>
                  </div>

                  {/* Live Test Feedback if run */}
                  {testResult && (
                    <div
                      style={{
                        marginTop: '12px',
                        padding: '8px 12px',
                        borderRadius: '6px',
                        backgroundColor: testResult.success ? 'var(--green-bg)' : '#fce8e6',
                        color: testResult.success ? 'var(--green)' : '#b3261e',
                        fontSize: '11px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '6px',
                      }}
                    >
                      {testResult.success ? <CheckCircle2 size={13} /> : <AlertCircle size={13} />}
                      <span>
                        {testResult.success
                          ? `Connection verified: ${testResult.status_code} OK (${testResult.latency_ms}ms)`
                          : `Test failed: ${testResult.message}`}
                      </span>
                    </div>
                  )}
                </div>

                {/* Card Footer Actions */}
                <div
                  style={{
                    marginTop: '16px',
                    borderTop: '1px solid var(--border-subtle)',
                    paddingTop: '12px',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    flexWrap: 'wrap',
                    gap: '8px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <button
                      onClick={() => handleCardTest(m)}
                      disabled={isTesting}
                      className="btn-pill-tonal"
                      style={{ padding: '5px 12px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '5px' }}
                    >
                      {isTesting ? <Loader2 size={12} className="spin" /> : <Zap size={12} />}
                      {isTesting ? 'Testing...' : 'Test Connection'}
                    </button>

                    {m.security_risk_level ? (
                      <button
                        onClick={async () => {
                          const rep = await auditModelSecurity(m)
                          setActiveReportForView(rep)
                          setIsReportModalOpen(true)
                        }}
                        style={{
                          padding: '3px 8px',
                          borderRadius: '12px',
                          fontSize: '10px',
                          fontWeight: 700,
                          backgroundColor:
                            m.security_risk_level === 'low'
                              ? '#e6f4ea'
                              : m.security_risk_level === 'medium'
                              ? '#fef7e0'
                              : '#fce8e6',
                          color:
                            m.security_risk_level === 'low'
                              ? '#137333'
                              : m.security_risk_level === 'medium'
                              ? '#b06000'
                              : '#c5221f',
                          border: `1px solid ${
                            m.security_risk_level === 'low'
                              ? '#ceead6'
                              : m.security_risk_level === 'medium'
                              ? '#feefc3'
                              : '#fad2cf'
                          }`,
                          cursor: 'pointer',
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '4px',
                        }}
                        title="Click to view Security Audit Report"
                      >
                        <ShieldCheck size={11} />
                        <span>{m.security_risk_level.toUpperCase()} RISK</span>
                      </button>
                    ) : (
                      <button
                        onClick={async () => {
                          const rep = await auditModelSecurity(m)
                          setActiveReportForView(rep)
                          setIsReportModalOpen(true)
                        }}
                        className="btn-pill-tonal"
                        style={{ padding: '4px 8px', fontSize: '10px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
                        title="Run security audit on this relay endpoint"
                      >
                        <Shield size={11} />
                        <span>Audit API</span>
                      </button>
                    )}
                    {/* Edit button moved to right of Audit API button */}
                    <button
                      onClick={() => openEditModal(m)}
                      className="btn-pill-tonal"
                      style={{ padding: '5px 12px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '5px' }}
                    >
                      <Edit2 size={12} /> Edit
                    </button>
                  </div>

                  {/* Toggle button at bottom right */}
                  <div style={{ display: 'flex', alignItems: 'center' }}>
                    <ToggleSwitch
                      size="sm"
                      checked={m.enabled}
                      ariaLabel={m.enabled ? `Disable ${m.display_name}` : `Enable ${m.display_name}`}
                      onChange={(checked) => handleToggleModelEnabled(m, checked)}
                    />
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* 3. Add / Edit Modal Window */}
      {isModalOpen && (
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
          onClick={() => setIsModalOpen(false)}
        >
          <div
            className="google-card"
            style={{
              width: '600px',
              maxWidth: '92vw',
              maxHeight: '90vh',
              overflowY: 'auto',
              padding: '24px',
              boxShadow: 'var(--shadow-md)',
              backgroundColor: '#ffffff',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
              <div>
                <h2 style={{ fontSize: '18px', fontWeight: 700, color: 'var(--text)' }}>
                  {editingModel ? 'Edit Custom Model' : 'Add Custom Model Provider'}
                </h2>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                  Configure provider endpoint, credentials, quota gauge, and project routing.
                </div>
              </div>
              <button
                onClick={() => setIsModalOpen(false)}
                className="btn-pill-tonal"
                style={{ padding: '6px' }}
              >
                <X size={16} />
              </button>
            </div>

            {modalError && (
              <div style={{ padding: '10px 16px', borderRadius: '8px', backgroundColor: '#fce8e6', color: '#b3261e', fontSize: '12px', marginBottom: '16px' }}>
                {modalError}
              </div>
            )}

            {/* Form Fields */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', marginBottom: '20px' }}>
              {/* Display Name & Model Identifier */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                    Display Name:
                  </label>
                  <input
                    type="text"
                    placeholder="e.g. Gemini 4 Argon"
                    value={displayName}
                    onChange={(e) => setDisplayName(e.target.value)}
                    style={{ width: '100%', fontSize: '12px', padding: '8px 10px' }}
                  />
                </div>
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '4px' }}>
                    <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                      Model Identifier:
                    </label>
                    <button
                      type="button"
                      onClick={handleFetchModels}
                      disabled={fetchingModels}
                      className="btn-pill-tonal"
                      style={{ padding: '2px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
                      title="Fetch available models via provider API"
                    >
                      <RefreshCw size={11} className={fetchingModels ? 'spin' : ''} />
                      {fetchingModels ? 'Fetching...' : 'Fetch Models'}
                    </button>
                  </div>
                  <div style={{ position: 'relative' }} ref={modelContainerRef}>
                    <input
                      type="text"
                      placeholder="e.g. gemini-2.5-pro or gpt-4o"
                      value={modelName}
                      onChange={(e) => {
                        const val = e.target.value
                        setModelName(val)
                        if (val.trim()) {
                          setShowModelDropdown(false)
                        } else if (fetchedModels.length > 0) {
                          setShowModelDropdown(true)
                        }
                        const detected = detectThinkingLevels(val)
                        if (detected.length > 0) {
                          setThinkingLevels(detected)
                        }
                      }}
                      onClick={() => {
                        if (!modelName.trim() && fetchedModels.length > 0) {
                          setShowModelDropdown(true)
                        }
                      }}
                      onFocus={() => {
                        if (!modelName.trim() && fetchedModels.length > 0) {
                          setShowModelDropdown(true)
                        }
                      }}
                      style={{ width: '100%', fontSize: '12px', padding: '8px 10px', fontFamily: 'monospace' }}
                    />
                    {showModelDropdown && fetchedModels.length > 0 && (
                      <div
                        style={{
                          position: 'absolute',
                          top: 'calc(100% + 4px)',
                          left: 0,
                          right: 0,
                          backgroundColor: '#ffffff',
                          border: '1px solid var(--border)',
                          borderRadius: '8px',
                          boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
                          zIndex: 1000,
                          maxHeight: '220px',
                          overflowY: 'auto',
                          padding: '4px 0',
                        }}
                      >
                        <div style={{ padding: '6px 12px', fontSize: '11px', fontWeight: 600, color: 'var(--text-subtle)', borderBottom: '1px solid #f1f3f4', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                          <span>Available Models ({fetchedModels.length})</span>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              setShowModelDropdown(false)
                            }}
                            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', padding: '2px' }}
                          >
                            <X size={12} />
                          </button>
                        </div>
                        {fetchedModels.map((m) => {
                          const cleanId = extractCleanModelId(m.id || '')
                          const ctx = extractContextWindow(m.id, m) || m.context_window
                          const levels = detectThinkingLevels(cleanId, m)
                          return (
                            <div
                              key={m.id}
                              onClick={() => {
                                handleSelectFetchedModel(m.id)
                                setShowModelDropdown(false)
                              }}
                              style={{
                                padding: '8px 12px',
                                fontSize: '12px',
                                cursor: 'pointer',
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'space-between',
                                fontFamily: 'monospace',
                                borderBottom: '1px solid #f8f9fa',
                                transition: 'background 0.15s ease',
                              }}
                              onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = '#f1f3f4')}
                              onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                            >
                              <div style={{ display: 'flex', flexDirection: 'column', gap: '2px', minWidth: 0 }}>
                                <span style={{ fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                                  {cleanId}
                                </span>
                                {m.display_name && m.display_name !== cleanId && (
                                  <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>{m.display_name}</span>
                                )}
                              </div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
                                {ctx && (
                                  <span style={{ fontSize: '10px', color: 'var(--text-muted)', backgroundColor: '#f1f3f4', padding: '2px 6px', borderRadius: '4px' }}>
                                    {ctx.toLocaleString()} ctx
                                  </span>
                                )}
                                {levels.length > 0 && (
                                  <span style={{ fontSize: '10px', color: '#137333', backgroundColor: '#e6f4ea', padding: '2px 6px', borderRadius: '4px', fontWeight: 600 }}>
                                    🧠 Thinking
                                  </span>
                                )}
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    )}
                  </div>
                  {fetchFeedback && (
                    <div
                      style={{
                        fontSize: '11px',
                        color: fetchFeedback.toLowerCase().includes('discovered') || fetchFeedback.toLowerCase().includes('success')
                          ? 'var(--green)'
                          : '#b3261e',
                        marginTop: '4px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '4px',
                        fontWeight: 500,
                      }}
                    >
                      {fetchFeedback.toLowerCase().includes('discovered') || fetchFeedback.toLowerCase().includes('success') ? (
                        <CheckCircle2 size={12} />
                      ) : (
                        <AlertCircle size={12} />
                      )}
                      <span>{fetchFeedback}</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Endpoint URL (Full Width) */}
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                  Endpoint URL:
                </label>
                <input
                  type="text"
                  placeholder="e.g. https://api.openai.com/v1/chat/completions or https://generativelanguage.googleapis.com/v1beta/models"
                  value={baseUrl}
                  onChange={(e) => {
                    setBaseUrl(e.target.value)
                    setProviderType(inferProviderType(e.target.value))
                  }}
                  style={{ width: '100%', fontSize: '12px', padding: '8px 10px', fontFamily: 'monospace' }}
                />
              </div>

              {/* Reasoning Level & Context Window */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                    Reasoning Level:
                  </label>
                  <div style={{ position: 'relative' }} ref={reasoningContainerRef}>
                    <input
                      type="text"
                      placeholder="e.g. high (blank for default)"
                      value={thinkingLevel}
                      onChange={(e) => {
                        setThinkingLevel(e.target.value)
                        if (e.target.value.trim()) {
                          setShowReasoningDropdown(false)
                        } else {
                          setShowReasoningDropdown(true)
                        }
                      }}
                      onClick={() => {
                        if (!thinkingLevel.trim()) {
                          setShowReasoningDropdown(true)
                        }
                      }}
                      onFocus={() => {
                        if (!thinkingLevel.trim()) {
                          setShowReasoningDropdown(true)
                        }
                      }}
                      style={{ width: '100%', fontSize: '12px', padding: '8px 10px', borderRadius: '6px', border: '1px solid var(--border)' }}
                    />
                    {showReasoningDropdown && (() => {
                      const presets = thinkingLevels.length > 0
                        ? thinkingLevels
                        : detectThinkingLevels(modelName).length > 0
                          ? detectThinkingLevels(modelName)
                          : ['off', 'low', 'medium', 'high']
                      return presets.length > 0 ? (
                        <div
                          style={{
                            position: 'absolute',
                            top: 'calc(100% + 4px)',
                            left: 0,
                            right: 0,
                            backgroundColor: '#ffffff',
                            border: '1px solid var(--border)',
                            borderRadius: '8px',
                            boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
                            zIndex: 1000,
                            maxHeight: '180px',
                            overflowY: 'auto',
                            padding: '4px 0',
                          }}
                        >
                          <div style={{ padding: '6px 12px', fontSize: '11px', fontWeight: 600, color: 'var(--text-subtle)', borderBottom: '1px solid #f1f3f4' }}>
                            Select Reasoning Level Preset
                          </div>
                          {presets.map((lvl) => (
                            <div
                              key={lvl}
                              onClick={() => {
                                setThinkingLevel(lvl)
                                setShowReasoningDropdown(false)
                              }}
                              style={{
                                padding: '8px 12px',
                                fontSize: '12px',
                                cursor: 'pointer',
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'space-between',
                                transition: 'background 0.15s ease',
                              }}
                              onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = '#f1f3f4')}
                              onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                            >
                              <span style={{ fontWeight: 500, color: 'var(--text)' }}>
                                {lvl.charAt(0).toUpperCase() + lvl.slice(1)}
                              </span>
                              {lvl.toLowerCase() !== 'off' && (
                                <span style={{ fontSize: '10px', color: 'var(--primary)', fontWeight: 600, backgroundColor: 'rgba(26,115,232,0.08)', padding: '2px 6px', borderRadius: '4px' }}>
                                  Reasoning
                                </span>
                              )}
                            </div>
                          ))}
                        </div>
                      ) : null
                    })()}
                  </div>
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                    Context Window (tokens):
                  </label>
                  <input
                    type="number"
                    value={contextWindow}
                    onChange={(e) => setContextWindow(Number(e.target.value))}
                    style={{ width: '100%', fontSize: '12px', padding: '8px 10px' }}
                  />
                </div>
              </div>

              {/* API Key */}
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                  API Key (leave blank for local Ollama / vLLM):
                </label>
                <div style={{ position: 'relative' }}>
                  <input
                    type={showApiKey ? 'text' : 'password'}
                    placeholder="sk-..."
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    style={{ width: '100%', fontSize: '12px', padding: '8px 36px 8px 10px', fontFamily: 'monospace' }}
                  />
                  <button
                    type="button"
                    onClick={() => setShowApiKey(!showApiKey)}
                    style={{
                      position: 'absolute',
                      right: '8px',
                      top: '50%',
                      transform: 'translateY(-50%)',
                      background: 'none',
                      border: 'none',
                      cursor: 'pointer',
                      color: 'var(--text-muted)',
                    }}
                  >
                    {showApiKey ? <EyeOff size={14} /> : <Eye size={14} />}
                  </button>
                </div>
              </div>

              {/* Project Mappings */}
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                  Project Mappings (comma separated, use * for all projects):
                </label>
                <input
                  type="text"
                  placeholder="*, my-web-app, backend-api"
                  value={projectMappings}
                  onChange={(e) => setProjectMappings(e.target.value)}
                  style={{ width: '100%', fontSize: '12px', padding: '8px 10px', fontFamily: 'monospace' }}
                />
              </div>

              {/* Toggles */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '12px', cursor: 'pointer' }}>
                  <ToggleSwitch
                    size="sm"
                    checked={isDefault}
                    onChange={(checked) => setIsDefault(checked)}
                  />
                  <span>Set as default custom model</span>
                </label>
              </div>

              {/* Notes Section: User Notes */}
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                  Notes:
                </label>
                <textarea
                  placeholder="Optional notes or description for this model (e.g. usage guidelines, proxy cluster, billing owner)..."
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  rows={2}
                  style={{
                    width: '100%',
                    fontSize: '12px',
                    padding: '8px 10px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    fontFamily: 'inherit',
                    resize: 'vertical',
                    boxSizing: 'border-box',
                  }}
                />
              </div>

              {/* Configuration Notes Section */}
              <div
                style={{
                  backgroundColor: 'var(--canvas)',
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '12px 16px',
                  fontSize: '11px',
                  color: 'var(--text-muted)',
                  lineHeight: 1.5,
                }}
              >
                <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: '4px', display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <Info size={13} style={{ color: 'var(--primary)' }} />
                  <span>Configuration Notes & Balance Tracking</span>
                </div>
                <ul style={{ margin: 0, paddingLeft: '16px', display: 'flex', flexDirection: 'column', gap: '3px' }}>
                  <li>
                    <strong>Auto-Derived Balance & Quota:</strong> Available balance or rate-limit quotas are automatically queried via API from recognized providers (e.g. OpenRouter, DeepSeek, SiliconFlow, Moonshot/Kimi, Together AI, OneAPI/NewAPI) upon save and testing.
                  </li>
                  <li>
                    <strong>Untracked (N/A):</strong> If the entered Base URL is not in our recognized dictionary or the provider endpoint returns no quota value, tracking defaults to <em>N/A</em> and displays as <em>Untracked</em>.
                  </li>
                  <li>
                    <strong>Manual Activation:</strong> Save the model first. Then toggle the active switch on the model's card in the main list to enable it for Antigravity tasks.
                  </li>
                </ul>
              </div>
            </div>

            {/* Modal Actions Footer: Test Connection on bottom left, Cancel & Save on right */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                borderTop: '1px solid var(--border)',
                paddingTop: '16px',
                marginTop: '8px',
                gap: '12px',
                flexWrap: 'nowrap',
              }}
            >
              {/* Bottom Left: Inline Test Connection Button */}
              {(() => {
                const testBtn = getTestConnectionButtonPresentation({
                  isTesting: modalTesting,
                  testResult: modalTestResult,
                  baseUrl,
                })
                return (
                  <button
                    type="button"
                    onClick={handleTestInModal}
                    disabled={testBtn.disabled}
                    title={testBtn.tooltip}
                    className="btn-pill-tonal"
                    style={{
                      ...testBtn.style,
                      padding: '0 12px',
                      borderRadius: '20px',
                      fontSize: '12px',
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      gap: '6px',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      flexShrink: 0,
                      boxSizing: 'border-box',
                    }}
                  >
                    {testBtn.icon === 'spinner' ? (
                      <Loader2 size={13} className="spin" style={{ flexShrink: 0 }} />
                    ) : testBtn.icon === 'check' ? (
                      <CheckCircle2 size={13} style={{ flexShrink: 0 }} />
                    ) : testBtn.icon === 'alert' ? (
                      <AlertCircle size={13} style={{ flexShrink: 0 }} />
                    ) : (
                      <Zap size={13} style={{ flexShrink: 0 }} />
                    )}
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {testBtn.label}
                    </span>
                  </button>
                )
              })()}

              {/* Bottom Right: Delete (if editing), Cancel & Save Model */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
                {editingModel && (
                  <button
                    type="button"
                    onClick={() => handleDeleteModel(editingModel.id, editingModel.display_name)}
                    disabled={modalSaving}
                    className="btn-pill-danger"
                    style={{ padding: '7px 16px', fontSize: '12px' }}
                  >
                    <Trash2 size={13} /> Delete Model
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="btn-pill-tonal"
                  style={{ padding: '7px 18px', fontSize: '12px' }}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleSaveModel}
                  disabled={modalSaving}
                  className="btn-pill-primary"
                  style={{ padding: '7px 20px', fontSize: '12px' }}
                >
                  {modalSaving ? 'Saving...' : 'Save Model'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation In-App Modal */}
      {deleteConfirmModel && (
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
          onClick={() => setDeleteConfirmModel(null)}
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
                Delete Custom Model
              </h3>
            </div>
            <p style={{ margin: '0 0 20px', fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Are you sure you want to permanently delete custom model <strong>"{deleteConfirmModel.name}"</strong>? This will remove its endpoint configuration and quota tracking.
            </p>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px' }}>
              <button
                type="button"
                onClick={() => setDeleteConfirmModel(null)}
                className="btn-pill-tonal"
                style={{ padding: '7px 16px', fontSize: '12px' }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={confirmDeleteModel}
                disabled={modalSaving}
                className="btn-pill-danger"
                style={{ padding: '7px 18px', fontSize: '12px' }}
              >
                {modalSaving ? 'Deleting...' : 'Delete Model'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Security Report In-App Modal */}
      {isReportModalOpen && activeReportForView && (
        <SecurityReportModal
          report={activeReportForView}
          onClose={() => setIsReportModalOpen(false)}
        />
      )}
    </div>
  )
}
