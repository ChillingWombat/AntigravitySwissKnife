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
  Loader2,
  Shield,
  ShieldCheck,
  ShieldAlert,
  ChevronDown,
  ChevronRight,
  Coins,
} from 'lucide-react'
import type {
  CustomModel,
  CustomModelsConfig,
  ModelInfo,
  ProviderType,
  QuotaType,
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
import {
  getTestConnectionButtonPresentation,
  sanitizeConnectionErrorMessage,
} from '../utils/testConnectionButton'
import { MODEL_CARD_MIN_WIDTH, MODEL_CARD_GRID_GAP } from '../utils/layoutTokens'
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

  // Card quick test & audit states
  const [testingModelId, setTestingModelId] = useState<string | null>(null)
  const [auditingModelId, setAuditingModelId] = useState<string | null>(null)
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

  // Foldable Quota & Price Section States
  const [isQuotaPriceOpen, setIsQuotaPriceOpen] = useState<boolean>(true)
  const [quotaType, setQuotaType] = useState<QuotaType>('na')
  const [quotaManualOverride, setQuotaManualOverride] = useState<boolean>(false)
  const [balanceValue, setBalanceValue] = useState<string>('')
  const [quotaValue, setQuotaValue] = useState<string>('')
  const [quotaFraction, setQuotaFraction] = useState<number | null>(null)
  const [inputPricePerM, setInputPricePerM] = useState<number | null>(null)
  const [cachedInputPricePerM, setCachedInputPricePerM] = useState<number | null>(null)
  const [outputPricePerM, setOutputPricePerM] = useState<number | null>(null)
  const [priceSource, setPriceSource] = useState<string>('unconfigured')
  const [isFetchingQuotaPrice, setIsFetchingQuotaPrice] = useState<boolean>(false)
  const [budgetCapType, setBudgetCapType] = useState<'none' | 'dollar' | 'percentage' | 'tokens'>('none')
  const [budgetCapValue, setBudgetCapValue] = useState<number | null>(null)

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
    setIsQuotaPriceOpen(true)
    setQuotaType('na')
    setQuotaManualOverride(false)
    setBalanceValue('')
    setQuotaValue('')
    setQuotaFraction(null)
    setInputPricePerM(null)
    setCachedInputPricePerM(null)
    setOutputPricePerM(null)
    setPriceSource('unconfigured')
    setBudgetCapType('none')
    setBudgetCapValue(null)
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
    setIsQuotaPriceOpen(true)
    setQuotaType(model.quota_type || 'na')
    setQuotaManualOverride(!!model.quota_manual_override)
    setBalanceValue(model.balance_value || '')
    setQuotaValue(model.quota_value || '')
    setQuotaFraction(model.quota_fraction ?? null)
    setInputPricePerM(model.input_price_per_m !== undefined ? model.input_price_per_m : null)
    setCachedInputPricePerM(model.cached_input_price_per_m !== undefined ? model.cached_input_price_per_m : null)
    setOutputPricePerM(model.output_price_per_m !== undefined ? model.output_price_per_m : null)
    setPriceSource(model.price_source || 'unconfigured')
    setBudgetCapType(model.budget_cap_type || 'none')
    setBudgetCapValue(model.budget_cap_value !== undefined ? model.budget_cap_value : null)
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
        model_id: model.name?.trim() || model.id,
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
      if (res.quota_result) {
        if (!quotaManualOverride) {
          if (res.quota_result.quota_type) setQuotaType(res.quota_result.quota_type)
          if (res.quota_result.balance_value) setBalanceValue(res.quota_result.balance_value)
          if (res.quota_result.quota_value) setQuotaValue(res.quota_result.quota_value)
          if (res.quota_result.fraction !== null && res.quota_result.fraction !== undefined) {
            setQuotaFraction(res.quota_result.fraction)
          }
        }
        if (res.quota_result.input_price_per_m !== undefined) {
          setInputPricePerM(res.quota_result.input_price_per_m)
          setCachedInputPricePerM(res.quota_result.cached_input_price_per_m ?? null)
          setOutputPricePerM(res.quota_result.output_price_per_m ?? null)
          setPriceSource(res.quota_result.price_source || (res.quota_result.input_price_per_m !== null ? 'provider' : 'unconfigured'))
        }
      }
    } catch (err: any) {
      const message = err.message || 'Request failed'
      setModalTestResult({
        success: false,
        status_code: 0,
        latency_ms: 0,
        message,
        endpoint: draftModel.base_url,
      })
      setModalError(`Test request failed: ${sanitizeConnectionErrorMessage(message)}`)
    } finally {
      setModalTesting(false)
    }
  }

  const handleAutoFetchQuotaPrice = async () => {
    if (!baseUrl.trim() && !modelName.trim()) {
      setModalError('Please enter an Endpoint URL or Model Identifier before fetching quota & price.')
      return
    }
    setIsFetchingQuotaPrice(true)
    setModalError(null)
    const effectiveProvider = providerType || (baseUrl.trim() ? inferProviderType(baseUrl.trim()) : 'custom')
    const draftModel: CustomModel = {
      id: editingModel?.id || 'draft-quota',
      name: modelName.trim() || 'custom-model',
      display_name: displayName.trim() || modelName.trim() || 'Custom Model',
      provider_type: effectiveProvider,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      project_mappings: ['*'],
      quota_type: quotaType,
      prepaid_balance: 0,
      total_budget: 0,
      quota_fraction: null,
      is_default: isDefault,
      enabled: enabled,
    }
    try {
      const res = await api.fetchCustomModelQuota(draftModel)
      if (res) {
        if (!quotaManualOverride) {
          if (res.quota_type) setQuotaType(res.quota_type)
          if (res.balance_value) setBalanceValue(res.balance_value)
          if (res.quota_value) setQuotaValue(res.quota_value)
          if (res.fraction !== null && res.fraction !== undefined) {
            setQuotaFraction(res.fraction)
          }
        }
        if (res.input_price_per_m !== undefined) {
          setInputPricePerM(res.input_price_per_m)
          setCachedInputPricePerM(res.cached_input_price_per_m ?? null)
          setOutputPricePerM(res.output_price_per_m ?? null)
          setPriceSource(res.price_source || (res.input_price_per_m !== null ? 'provider' : 'unconfigured'))
        }
      }
    } catch (err: any) {
      setModalError(`Failed to auto-fetch quota & price: ${err.message || 'Network error'}`)
    } finally {
      setIsFetchingQuotaPrice(false)
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
    if (budgetCapType !== 'none') {
      if (budgetCapValue === null || budgetCapValue === undefined || isNaN(budgetCapValue) || budgetCapValue <= 0) {
        setModalError('Please enter a positive value for the budget/usage cap.')
        return
      }
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
      internal_id: editingModel?.internal_id,
      name: modelName.trim(),
      display_name: displayName.trim() || modelName.trim(),
      provider_type: effectiveProvider,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      project_mappings: mappings.length > 0 ? mappings : ['*'],
      quota_type: quotaType,
      quota_manual_override: quotaManualOverride,
      balance_value: balanceValue.trim() || undefined,
      quota_value: quotaValue.trim() || undefined,
      prepaid_balance: editingModel?.prepaid_balance || 0,
      total_budget: editingModel?.total_budget || 0,
      quota_fraction: quotaFraction,
      budget_cap_type: budgetCapType,
      budget_cap_value: budgetCapValue,
      input_price_per_m: inputPricePerM,
      cached_input_price_per_m: cachedInputPricePerM,
      output_price_per_m: outputPricePerM,
      price_source: priceSource,
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

  const handleCardAudit = async (model: CustomModel) => {
    setAuditingModelId(model.id)
    try {
      const rep = await auditModelSecurity(model)
      setConfig((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          models: prev.models.map((item) =>
            item.id === model.id
              ? {
                  ...item,
                  security_risk_level: rep.risk_level,
                  security_audit_score: rep.risk_score,
                  last_security_audit: rep.audited_at,
                }
              : item
          ),
        }
      })
      setActiveReportForView(rep)
      setIsReportModalOpen(true)
    } finally {
      setAuditingModelId(null)
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
              borderRadius: '8px',
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
                    borderRadius: '6px',
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
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: `repeat(auto-fill, minmax(${MODEL_CARD_MIN_WIDTH}px, 1fr))`,
            gap: `${MODEL_CARD_GRID_GAP}px`,
          }}
        >
          {filteredModels.map((m) => {
            const testResult = cardTestResults[m.id]
            const isTesting = testingModelId === m.id
            const isAuditing = auditingModelId === m.id

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

                      {/* Budget Cap & Token Price Badges */}
                      {((m.budget_cap_type && m.budget_cap_type !== 'none' && m.budget_cap_value !== null && m.budget_cap_value !== undefined) || (m.input_price_per_m !== undefined)) && (
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap', marginTop: '6px' }}>
                          {m.budget_cap_type && m.budget_cap_type !== 'none' && m.budget_cap_value !== null && m.budget_cap_value !== undefined && (
                            <span
                              style={{
                                backgroundColor: '#fef3c7',
                                color: '#b45309',
                                fontSize: '10px',
                                fontWeight: 600,
                                padding: '2px 6px',
                                borderRadius: '4px',
                                display: 'inline-flex',
                                alignItems: 'center',
                                gap: '3px',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              <span>
                                Cap:{' '}
                                {m.budget_cap_type === 'dollar'
                                  ? `$${m.budget_cap_value}`
                                  : m.budget_cap_type === 'percentage'
                                  ? `${m.budget_cap_value}%`
                                  : `${
                                      m.budget_cap_value >= 1_000_000
                                        ? `${(m.budget_cap_value / 1_000_000).toFixed(m.budget_cap_value % 1_000_000 === 0 ? 0 : 1)}M`
                                        : m.budget_cap_value >= 1_000
                                        ? `${(m.budget_cap_value / 1_000).toFixed(m.budget_cap_value % 1_000 === 0 ? 0 : 1)}k`
                                        : m.budget_cap_value
                                    } tokens`}
                              </span>
                            </span>
                          )}
                          {m.input_price_per_m !== null && m.input_price_per_m !== undefined ? (
                            <span
                              style={{
                                backgroundColor: '#f1f3f4',
                                color: 'var(--text-muted)',
                                fontSize: '10px',
                                fontWeight: 500,
                                padding: '2px 6px',
                                borderRadius: '4px',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              ${m.input_price_per_m.toFixed(2)} / ${(m.output_price_per_m ?? 0).toFixed(2)} per 1M
                            </span>
                          ) : m.price_source === 'unconfigured' || m.input_price_per_m === null ? (
                            <span
                              style={{
                                backgroundColor: '#fef3c7',
                                color: '#b45309',
                                fontSize: '10px',
                                fontWeight: 500,
                                padding: '2px 6px',
                                borderRadius: '4px',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              Price null
                            </span>
                          ) : null}
                        </div>
                      )}

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
                          ? `Connection verified: OK (${testResult.latency_ms}ms)`
                          : `Test failed: ${sanitizeConnectionErrorMessage(testResult.message)}`}
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
                    flexWrap: 'nowrap',
                    gap: '16px',
                    width: '100%',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'nowrap', flexShrink: 0 }}>
                    <button
                      onClick={() => handleCardTest(m)}
                      disabled={isTesting}
                      className="btn-pill-tonal"
                      style={{
                        padding: '5px 12px',
                        fontSize: '11px',
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '5px',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {isTesting ? <Loader2 size={12} className="spin" /> : <Zap size={12} />}
                      <span>{isTesting ? 'Testing...' : 'Test Connection'}</span>
                    </button>

                    {m.security_risk_level ? (
                      <button
                        onClick={() => handleCardAudit(m)}
                        disabled={isAuditing}
                        style={{
                          padding: '5px 12px',
                          borderRadius: '20px',
                          fontSize: '11px',
                          fontWeight: 600,
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
                          cursor: isAuditing ? 'default' : 'pointer',
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '5px',
                          whiteSpace: 'nowrap',
                        }}
                        title="Click to view Security Audit Report"
                      >
                        {isAuditing ? (
                          <Loader2 size={12} className="spin" />
                        ) : m.security_risk_level === 'low' ? (
                          <ShieldCheck size={12} />
                        ) : (
                          <ShieldAlert size={12} />
                        )}
                        <span>{isAuditing ? 'Auditing...' : `${m.security_risk_level.toUpperCase()} RISK`}</span>
                      </button>
                    ) : (
                      <button
                        onClick={() => handleCardAudit(m)}
                        disabled={isAuditing}
                        className="btn-pill-tonal"
                        style={{
                          padding: '5px 12px',
                          fontSize: '11px',
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '5px',
                          whiteSpace: 'nowrap',
                        }}
                        title="Run security audit on this relay endpoint"
                      >
                        {isAuditing ? <Loader2 size={12} className="spin" /> : <Shield size={12} />}
                        <span>{isAuditing ? 'Auditing...' : 'Audit API'}</span>
                      </button>
                    )}
                    {/* Edit button moved to right of Audit API button */}
                    <button
                      onClick={() => openEditModal(m)}
                      className="btn-pill-tonal"
                      style={{
                        padding: '5px 12px',
                        fontSize: '11px',
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '5px',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      <Edit2 size={12} />
                      <span>Edit</span>
                    </button>
                  </div>

                  {/* Toggle button at right end */}
                  <div style={{ display: 'flex', alignItems: 'center', marginLeft: 'auto', flexShrink: 0 }}>
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
                      placeholder="e.g. gemini-4-argon"
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
                                    Thinking
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
                  placeholder="e.g. https://generativelanguage.googleapis.com/v1beta/models"
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
                          : ['low', 'medium', 'high']
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
                  <span>Set as default custom model</span>
                  <ToggleSwitch
                    size="sm"
                    checked={isDefault}
                    onChange={(checked) => setIsDefault(checked)}
                  />
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

              {/* Foldable Quota & Price Section */}
              <div
                style={{
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  backgroundColor: '#ffffff',
                  overflow: 'hidden',
                  marginTop: '4px',
                }}
              >
                {/* Foldable Header */}
                <div
                  onClick={() => setIsQuotaPriceOpen(!isQuotaPriceOpen)}
                  style={{
                    padding: '10px 14px',
                    backgroundColor: 'var(--canvas)',
                    borderBottom: isQuotaPriceOpen ? '1px solid var(--border)' : 'none',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    cursor: 'pointer',
                    userSelect: 'none',
                    gap: '10px',
                    flexWrap: 'wrap',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    {isQuotaPriceOpen ? <ChevronDown size={15} color="var(--text-muted)" /> : <ChevronRight size={15} color="var(--text-muted)" />}
                    <Coins size={15} color="var(--primary)" />
                    <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                      Quota & Price
                    </span>
                    <span
                      style={{
                        fontSize: '10px',
                        padding: '1px 6px',
                        borderRadius: '4px',
                        backgroundColor: quotaType === 'balance' ? '#e6f4ea' : quotaType === 'quota' ? '#e8f0fe' : '#f1f3f4',
                        color: quotaType === 'balance' ? '#137333' : quotaType === 'quota' ? 'var(--primary)' : 'var(--text-muted)',
                        fontWeight: 600,
                        textTransform: 'capitalize',
                      }}
                    >
                      {quotaType === 'balance' ? 'Balance' : quotaType === 'quota' ? 'Quota' : 'Untracked'}
                    </span>
                    {budgetCapType !== 'none' && budgetCapValue !== null && (
                      <span
                        style={{
                          fontSize: '10px',
                          padding: '1px 6px',
                          borderRadius: '4px',
                          backgroundColor: '#fef3c7',
                          color: '#b45309',
                          fontWeight: 600,
                        }}
                      >
                        Cap: {budgetCapType === 'dollar' ? `$${budgetCapValue}` : budgetCapType === 'percentage' ? `${budgetCapValue}%` : `${budgetCapValue} tok`}
                      </span>
                    )}
                  </div>

                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation()
                      handleAutoFetchQuotaPrice()
                    }}
                    disabled={isFetchingQuotaPrice}
                    className="btn-pill-tonal"
                    style={{
                      padding: '4px 10px',
                      fontSize: '11px',
                      fontWeight: 600,
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '5px',
                      backgroundColor: '#ffffff',
                      border: '1px solid var(--border)',
                      cursor: isFetchingQuotaPrice ? 'not-allowed' : 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    <RefreshCw size={12} className={isFetchingQuotaPrice ? 'spin' : ''} />
                    <span>{isFetchingQuotaPrice ? 'Fetching...' : 'Auto-Fetch Quota & Price'}</span>
                  </button>
                </div>

                {/* Foldable Content Body */}
                {isQuotaPriceOpen && (
                  <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
                    {/* A. Fee Charging Mode */}
                    <div>
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px', flexWrap: 'wrap', gap: '8px' }}>
                        <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                          Fee Charging Mode:
                        </label>
                        <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '11px', cursor: 'pointer', color: 'var(--text)' }}>
                          <span>Manual Override</span>
                          <ToggleSwitch
                            size="sm"
                            checked={quotaManualOverride}
                            onChange={(checked) => setQuotaManualOverride(checked)}
                          />
                        </label>
                      </div>

                      <div style={{ display: 'flex', gap: '6px', marginBottom: '10px' }}>
                        {[
                          { id: 'balance' as QuotaType, label: 'Balance Mode' },
                          { id: 'quota' as QuotaType, label: 'Quota Mode' },
                          { id: 'na' as QuotaType, label: 'Untracked / N/A' },
                        ].map((m) => (
                          <button
                            key={m.id}
                            type="button"
                            onClick={() => setQuotaType(m.id)}
                            style={{
                              flex: 1,
                              padding: '6px 8px',
                              borderRadius: '6px',
                              fontSize: '11.5px',
                              fontWeight: quotaType === m.id ? 600 : 500,
                              color: quotaType === m.id ? 'var(--primary)' : 'var(--text-muted)',
                              backgroundColor: quotaType === m.id ? '#e8f0fe' : '#ffffff',
                              border: quotaType === m.id ? '1px solid var(--primary)' : '1px solid var(--border)',
                              cursor: 'pointer',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {m.label}
                          </button>
                        ))}
                      </div>

                      {quotaType === 'balance' && (
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', backgroundColor: 'var(--canvas)', padding: '10px', borderRadius: '6px' }}>
                          <div>
                            <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '3px' }}>
                              Current Balance:
                            </label>
                            <input
                              type="text"
                              placeholder="e.g. $12.50 or ¥50.00"
                              value={balanceValue}
                              onChange={(e) => setBalanceValue(e.target.value)}
                              style={{ width: '100%', fontSize: '12px', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--border)' }}
                            />
                          </div>
                          <div>
                            <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '3px' }}>
                              Remaining Fraction (0-100%):
                            </label>
                            <input
                              type="number"
                              min="0"
                              max="100"
                              placeholder="e.g. 75"
                              value={quotaFraction !== null && quotaFraction !== undefined ? Math.round(quotaFraction * 100) : ''}
                              onChange={(e) => {
                                const v = e.target.value.trim()
                                setQuotaFraction(v === '' ? null : Math.max(0, Math.min(100, Number(v))) / 100)
                              }}
                              style={{ width: '100%', fontSize: '12px', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--border)' }}
                            />
                          </div>
                        </div>
                      )}

                      {quotaType === 'quota' && (
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', backgroundColor: 'var(--canvas)', padding: '10px', borderRadius: '6px' }}>
                          <div>
                            <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '3px' }}>
                              Quota Level / Tier:
                            </label>
                            <input
                              type="text"
                              placeholder="e.g. Tier 1 (500 RPM)"
                              value={quotaValue}
                              onChange={(e) => setQuotaValue(e.target.value)}
                              style={{ width: '100%', fontSize: '12px', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--border)' }}
                            />
                          </div>
                          <div>
                            <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '3px' }}>
                              Remaining Fraction (0-100%):
                            </label>
                            <input
                              type="number"
                              min="0"
                              max="100"
                              placeholder="e.g. 80"
                              value={quotaFraction !== null && quotaFraction !== undefined ? Math.round(quotaFraction * 100) : ''}
                              onChange={(e) => {
                                const v = e.target.value.trim()
                                setQuotaFraction(v === '' ? null : Math.max(0, Math.min(100, Number(v))) / 100)
                              }}
                              style={{ width: '100%', fontSize: '12px', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--border)' }}
                            />
                          </div>
                        </div>
                      )}

                      {quotaType === 'na' && (
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', padding: '6px 8px', backgroundColor: 'var(--canvas)', borderRadius: '6px' }}>
                          Untracked: No automated quota or prepaid balance queries will be executed for this model.
                        </div>
                      )}
                    </div>

                    {/* B. Token Pricing (Shared with Token Monitor) */}
                    <div>
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px', flexWrap: 'wrap', gap: '8px' }}>
                        <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                          Token Pricing ($/1M Tokens)
                        </label>
                        <span
                          style={{
                            fontSize: '11px',
                            fontWeight: 600,
                            padding: '2px 8px',
                            borderRadius: '10px',
                            backgroundColor:
                              priceSource === 'provider'
                                ? '#e6f4ea'
                                : priceSource === 'third_party' || priceSource === 'api'
                                ? '#e8f0fe'
                                : priceSource === 'manual'
                                ? '#f3e8ff'
                                : '#fef3c7',
                            color:
                              priceSource === 'provider'
                                ? '#137333'
                                : priceSource === 'third_party' || priceSource === 'api'
                                ? 'var(--primary)'
                                : priceSource === 'manual'
                                ? '#7e22ce'
                                : '#b45309',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {priceSource === 'provider'
                            ? 'Provider Official'
                            : priceSource === 'third_party' || priceSource === 'api'
                            ? '3rd-Party Backup'
                            : priceSource === 'manual'
                            ? 'Manual Override'
                            : 'Null — Manual Entry Required'}
                        </span>
                      </div>

                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '10px' }}>
                        <div>
                          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '3px' }}>
                            Prompt Input ($/1M):
                          </label>
                          <input
                            type="number"
                            step="0.001"
                            placeholder="null"
                            value={inputPricePerM === null || inputPricePerM === undefined ? '' : inputPricePerM}
                            onChange={(e) => {
                              const raw = e.target.value.trim()
                              const v = raw === '' ? null : parseFloat(raw)
                              setInputPricePerM(v !== null && !Number.isNaN(v) ? v : null)
                              setPriceSource('manual')
                            }}
                            style={{ width: '100%', fontSize: '12px', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--border)' }}
                          />
                        </div>
                        <div>
                          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '3px' }}>
                            Cached Input ($/1M):
                          </label>
                          <input
                            type="number"
                            step="0.0001"
                            placeholder="null"
                            value={cachedInputPricePerM === null || cachedInputPricePerM === undefined ? '' : cachedInputPricePerM}
                            onChange={(e) => {
                              const raw = e.target.value.trim()
                              const v = raw === '' ? null : parseFloat(raw)
                              setCachedInputPricePerM(v !== null && !Number.isNaN(v) ? v : null)
                              setPriceSource('manual')
                            }}
                            style={{ width: '100%', fontSize: '12px', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--border)' }}
                          />
                        </div>
                        <div>
                          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '3px' }}>
                            Output ($/1M):
                          </label>
                          <input
                            type="number"
                            step="0.01"
                            placeholder="null"
                            value={outputPricePerM === null || outputPricePerM === undefined ? '' : outputPricePerM}
                            onChange={(e) => {
                              const raw = e.target.value.trim()
                              const v = raw === '' ? null : parseFloat(raw)
                              setOutputPricePerM(v !== null && !Number.isNaN(v) ? v : null)
                              setPriceSource('manual')
                            }}
                            style={{ width: '100%', fontSize: '12px', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--border)' }}
                          />
                        </div>
                      </div>
                    </div>

                    {/* C. Model Budget / Usage Cap */}
                    <div>
                      <div style={{ marginBottom: '8px' }}>
                        <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                          Model Budget / Usage Cap:
                        </label>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                          Set a consumption cap for this model. Options adapt to the selected fee mode and price configuration.
                        </div>
                      </div>

                      <div style={{ display: 'flex', gap: '6px', marginBottom: '10px', flexWrap: 'wrap' }}>
                        {[
                          { id: 'none' as const, label: 'No Cap', available: true },
                          {
                            id: 'dollar' as const,
                            label: 'Dollar Spend Cap ($)',
                            available: quotaType === 'balance' || inputPricePerM !== null || outputPricePerM !== null,
                          },
                          {
                            id: 'percentage' as const,
                            label: 'Usage Cap (%)',
                            available: quotaType === 'quota' || quotaType === 'balance',
                          },
                          { id: 'tokens' as const, label: 'Token Count Cap', available: true },
                        ].map((c) => (
                          <button
                            key={c.id}
                            type="button"
                            disabled={!c.available}
                            onClick={() => {
                              setBudgetCapType(c.id)
                              if (c.id === 'none') setBudgetCapValue(null)
                            }}
                            style={{
                              padding: '5px 10px',
                              borderRadius: '6px',
                              fontSize: '11px',
                              fontWeight: budgetCapType === c.id ? 600 : 500,
                              color: !c.available
                                ? 'var(--text-subtle)'
                                : budgetCapType === c.id
                                ? 'var(--primary)'
                                : 'var(--text)',
                              backgroundColor: budgetCapType === c.id ? '#e8f0fe' : '#ffffff',
                              border: budgetCapType === c.id ? '1px solid var(--primary)' : '1px solid var(--border)',
                              cursor: c.available ? 'pointer' : 'not-allowed',
                              opacity: c.available ? 1 : 0.45,
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {c.label}
                          </button>
                        ))}
                      </div>

                      {budgetCapType !== 'none' && (
                        <div style={{ backgroundColor: 'var(--canvas)', padding: '10px', borderRadius: '6px' }}>
                          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '3px' }}>
                            Cap Value {budgetCapType === 'dollar' ? '($ USD)' : budgetCapType === 'percentage' ? '(%)' : '(Tokens)'}:
                          </label>
                          <input
                            type="number"
                            min="0"
                            step={budgetCapType === 'dollar' ? '1' : budgetCapType === 'percentage' ? '1' : '10000'}
                            placeholder={
                              budgetCapType === 'dollar'
                                ? 'e.g. 25.00'
                                : budgetCapType === 'percentage'
                                ? 'e.g. 85'
                                : 'e.g. 1000000'
                            }
                            value={budgetCapValue !== null && budgetCapValue !== undefined ? budgetCapValue : ''}
                            onChange={(e) => {
                              const raw = e.target.value.trim()
                              const v = raw === '' ? null : parseFloat(raw)
                              setBudgetCapValue(v !== null && !Number.isNaN(v) ? v : null)
                            }}
                            style={{ width: '100%', fontSize: '12px', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--border)' }}
                          />
                        </div>
                      )}
                    </div>
                  </div>
                )}
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
                      borderRadius: '6px',
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
