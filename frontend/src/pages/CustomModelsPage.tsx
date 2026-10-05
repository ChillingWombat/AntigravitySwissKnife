import React, { useState, useEffect } from 'react'
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
} from 'lucide-react'
import type {
  CustomModel,
  CustomModelsConfig,
  ModelInfo,
  ProviderType,
  QuotaType,
  TestResult,
} from '../types'
import { CircularGauge } from '../components/CircularGauge'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'

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
  const [providerType, setProviderType] = useState<ProviderType>('openai')
  const [baseUrl, setBaseUrl] = useState<string>('')
  const [apiKey, setApiKey] = useState<string>('')
  const [quotaType, setQuotaType] = useState<QuotaType>('none')
  const [prepaidBalance, setPrepaidBalance] = useState<number>(10)
  const [totalBudget, setTotalBudget] = useState<number>(100)
  const [quotaFraction, setQuotaFraction] = useState<number>(1.0)
  const [projectMappings, setProjectMappings] = useState<string>('*')
  const [isDefault, setIsDefault] = useState<boolean>(false)
  const [enabled, setEnabled] = useState<boolean>(true)
  const [contextWindow, setContextWindow] = useState<number>(1000000)

  // Reasoning / Thinking Configuration
  const [supportsThinking, setSupportsThinking] = useState<boolean>(false)
  const [thinkingLevels, setThinkingLevels] = useState<string[]>(['off', 'low', 'medium', 'high'])
  const [thinkingLevel, setThinkingLevel] = useState<string>('high')

  // Model Fetching States
  const [fetchingModels, setFetchingModels] = useState<boolean>(false)
  const [fetchedModels, setFetchedModels] = useState<ModelInfo[]>([])
  const [fetchFeedback, setFetchFeedback] = useState<string | null>(null)
  const [manualModelEntry, setManualModelEntry] = useState<boolean>(false)

  // Modal Test & Save States
  const [modalTesting, setModalTesting] = useState<boolean>(false)
  const [modalTestResult, setModalTestResult] = useState<TestResult | null>(null)
  const [modalSaving, setModalSaving] = useState<boolean>(false)
  const [modalError, setModalError] = useState<string | null>(null)

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

  useEffect(() => {
    loadData()
  }, [])

  const openAddModal = () => {
    setEditingModel(null)
    setDisplayName('')
    setModelName('')
    setProviderType('openai')
    setBaseUrl('')
    setApiKey('')
    setQuotaType('none')
    setPrepaidBalance(25)
    setTotalBudget(100)
    setQuotaFraction(1.0)
    setProjectMappings('*')
    setIsDefault((config?.models?.length ?? 0) === 0)
    setEnabled(true)
    setContextWindow(1000000)
    setSupportsThinking(false)
    setThinkingLevels(['off', 'low', 'medium', 'high'])
    setThinkingLevel('off')
    setFetchedModels([])
    setFetchFeedback(null)
    setManualModelEntry(false)
    setShowApiKey(false)
    setModalTestResult(null)
    setModalError(null)
    setIsModalOpen(true)
  }

  const openEditModal = (model: CustomModel) => {
    setEditingModel(model)
    setDisplayName(model.display_name)
    setModelName(model.name)
    setProviderType(model.provider_type)
    setBaseUrl(model.base_url)
    setApiKey(model.api_key || '')
    setQuotaType(model.quota_type)
    setPrepaidBalance(model.prepaid_balance || 0)
    setTotalBudget(model.total_budget || 0)
    setQuotaFraction(model.quota_fraction ?? 1.0)
    setProjectMappings(model.project_mappings?.join(', ') || '*')
    setIsDefault(model.is_default)
    setEnabled(model.enabled)
    setContextWindow(model.context_window || 1000000)
    const isThinking = !!model.supports_thinking && model.thinking_level?.toLowerCase() !== 'off'
    setSupportsThinking(isThinking)
    setThinkingLevels(
      model.thinking_levels && model.thinking_levels.length > 0
        ? model.thinking_levels
        : ['off', 'low', 'medium', 'high']
    )
    setThinkingLevel(isThinking ? (model.thinking_level || 'high') : 'off')
    setFetchedModels([])
    setFetchFeedback(null)
    setManualModelEntry(false)
    setShowApiKey(false)
    setModalTestResult(null)
    setModalError(null)
    setIsModalOpen(true)
  }

  const resolveEndpointPreview = (type: ProviderType, url: string, model: string) => {
    const raw = (url || '').trim()
    if (!raw) return '(enter base URL above)'
    const cleanU = raw.replace(/\/+$/, '')
    if (type === 'custom') {
      if (
        cleanU.endsWith('/chat/completions') ||
        cleanU.endsWith('/chat/completion') ||
        cleanU.endsWith('/completions') ||
        cleanU.includes('/chat') ||
        cleanU.includes(':generateContent') ||
        cleanU.endsWith('/messages')
      ) {
        return raw
      }
      if (cleanU.endsWith('/v1') || cleanU.includes('/v1/') || cleanU.includes('/v2/')) {
        return `${cleanU}/chat/completions`
      }
      return `${cleanU}/v1/chat/completions`
    }
    if (type === 'anthropic') {
      if (cleanU.endsWith('/messages')) return cleanU
      if (cleanU.endsWith('/v1') || cleanU.includes('/v1/')) return `${cleanU}/messages`
      return `${cleanU}/v1/messages`
    }
    if (type === 'gemini') {
      if (cleanU.includes(':generateContent')) return cleanU
      const m = model.trim() || '{model}'
      if (cleanU.endsWith('/v1beta') || cleanU.includes('/v1beta/')) return `${cleanU}/models/${m}:generateContent`
      return `${cleanU}/v1beta/models/${m}:generateContent`
    }
    // OpenAI Compatible
    if (cleanU.endsWith('/chat/completions') || cleanU.endsWith('/chat/completion')) return cleanU
    if (cleanU.endsWith('/v1') || cleanU.includes('/v1/') || cleanU.includes('/v2/')) return `${cleanU}/chat/completions`
    return `${cleanU}/v1/chat/completions`
  }

  const handleFetchModels = async () => {
    if (!baseUrl.trim() && providerType !== 'anthropic' && providerType !== 'gemini') {
      setModalError('Please enter a Base URL before fetching models.')
      return
    }
    setFetchingModels(true)
    setFetchFeedback(null)
    setModalError(null)
    try {
      const res = await api.fetchModels(providerType, baseUrl.trim(), apiKey.trim())
      if (res.success && res.models && res.models.length > 0) {
        setFetchedModels(res.models)
        setManualModelEntry(false)
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
    setModelName(found.id)
    if (!displayName.trim()) {
      setDisplayName(found.display_name || found.id)
    }
    if (found.context_window && found.context_window > 0) {
      setContextWindow(found.context_window)
    } else {
      setContextWindow(1000000)
    }
    if (found.supports_thinking) {
      setSupportsThinking(true)
      setThinkingLevels(
        found.thinking_levels && found.thinking_levels.length > 0
          ? found.thinking_levels
          : ['off', 'low', 'medium', 'high']
      )
      setThinkingLevel('high')
    } else {
      setSupportsThinking(false)
      setThinkingLevel('off')
    }
  }

  const handleTestInModal = async () => {
    if (!baseUrl.trim() && providerType !== 'anthropic' && providerType !== 'gemini') {
      setModalError('Please enter a Base URL before testing.')
      return
    }
    if (!modelName.trim()) {
      setModalError('Please enter or select a Model Identifier before testing.')
      return
    }

    setModalTesting(true)
    setModalTestResult(null)
    setModalError(null)

    const isThinking = supportsThinking && thinkingLevel.toLowerCase() !== 'off'
    const activeThinkingLvl = isThinking ? (thinkingLevel.trim() || 'high') : 'off'
    const draftModel: CustomModel = {
      id: editingModel?.id || 'draft-test',
      name: modelName.trim(),
      display_name: displayName.trim(),
      provider_type: providerType,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      project_mappings: projectMappings.split(',').map((p) => p.trim()).filter(Boolean),
      quota_type: quotaType,
      prepaid_balance: Number(prepaidBalance),
      total_budget: Number(totalBudget),
      quota_fraction: quotaType === 'quota_based' ? Number(quotaFraction) : null,
      is_default: isDefault,
      enabled: enabled,
      context_window: Number(contextWindow) || 1000000,
      supports_thinking: isThinking,
      thinking_levels: isThinking ? thinkingLevels : undefined,
      thinking_level: isThinking ? activeThinkingLvl : 'off',
    }

    try {
      const res = await api.testCustomModel(draftModel)
      setModalTestResult(res)
    } catch (err: any) {
      setModalError(`Test request failed: ${err.message}`)
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
      setModalError('Base URL is required.')
      return
    }

    setModalSaving(true)
    setModalError(null)

    const mappings = projectMappings
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean)

    const isThinking = supportsThinking && thinkingLevel.toLowerCase() !== 'off'
    const activeThinkingLvl = isThinking ? (thinkingLevel.trim() || 'high') : 'off'
    const updatedThinkingLevels = [...thinkingLevels]
    if (
      isThinking &&
      activeThinkingLvl &&
      !updatedThinkingLevels.map((l) => l.toLowerCase()).includes(activeThinkingLvl.toLowerCase())
    ) {
      updatedThinkingLevels.push(activeThinkingLvl.toLowerCase())
    }

    const modelToSave: CustomModel = {
      id:
        editingModel?.id ||
        `${providerType}-${modelName.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${Date.now()}`,
      name: modelName.trim(),
      display_name: displayName.trim() || modelName.trim(),
      provider_type: providerType,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      project_mappings: mappings.length > 0 ? mappings : ['*'],
      quota_type: quotaType,
      prepaid_balance: Number(prepaidBalance) || 0,
      total_budget: Number(totalBudget) || 0,
      quota_fraction: quotaType === 'quota_based' ? Number(quotaFraction) : null,
      is_default: isDefault,
      enabled: enabled,
      context_window: Number(contextWindow) || 1000000,
      supports_thinking: isThinking,
      thinking_levels: isThinking ? updatedThinkingLevels : undefined,
      thinking_level: isThinking ? activeThinkingLvl : 'off',
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

  const models = config?.models || []

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Top Header Card */}
      <div className="google-card" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Custom Model Providers & Endpoints
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
            Configure third-party LLMs (OpenAI, Anthropic Claude, Gemini, DeepSeek, OpenCode, Ollama) with thinking level, quota tracking, and project routing.
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <button onClick={openAddModal} className="btn-pill-primary" style={{ padding: '7px 16px', fontSize: '12px' }}>
            <Plus size={14} /> Add Custom Model
          </button>
          <button onClick={loadData} disabled={loading} className="btn-pill-tonal" style={{ padding: '7px 12px' }} title="Refresh Custom Models">
            <RefreshCw size={14} className={loading ? 'spin' : ''} />
          </button>
        </div>
      </div>

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
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(420px, 1fr))', gap: '16px' }}>
          {models.map((m) => {
            const testResult = cardTestResults[m.id]
            const isTesting = testingModelId === m.id

            // Quota Gauge calculation
            let gaugePct: number | null = null
            let gaugeTitle = 'Untracked Quota'
            let gaugeSub: string | undefined = 'No budget/limits'
            let emptyGrey = true

            if (m.quota_type === 'cost_based') {
              if (m.total_budget > 0) {
                gaugePct = Math.max(0, Math.min(100, (m.prepaid_balance / m.total_budget) * 100))
                emptyGrey = false
                gaugeSub = `$${m.prepaid_balance.toFixed(2)} / $${m.total_budget.toFixed(2)}`
              } else {
                emptyGrey = true
                gaugePct = null
                gaugeSub = 'Untracked deposit balance'
              }
              gaugeTitle = 'Deposit Balance'
            } else if (m.quota_type === 'quota_based') {
              if (m.quota_fraction !== null && m.quota_fraction !== undefined) {
                gaugePct = Math.max(0, Math.min(100, m.quota_fraction * 100))
                emptyGrey = false
              }
              gaugeTitle = 'Quota Remaining'
              gaugeSub = 'Rate-limit tracked'
            } else {
              emptyGrey = true
              gaugePct = null
              gaugeTitle = 'Untracked Quota'
              gaugeSub = 'No limits configured'
            }

            return (
              <div
                key={m.id}
                className="google-card"
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  justifyContent: 'space-between',
                  padding: '20px',
                  border: m.is_default ? '2px solid var(--primary)' : '1px solid var(--border)',
                  opacity: m.enabled ? 1 : 0.65,
                  transition: 'opacity 0.2s ease, border-color 0.2s ease',
                }}
              >
                <div>
                  {/* Top Row: Switch Button to Enable/Disable (Tags removed) */}
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', marginBottom: '8px' }}>
                    <ToggleSwitch
                      size="sm"
                      checked={m.enabled}
                      ariaLabel={m.enabled ? `Disable ${m.display_name}` : `Enable ${m.display_name}`}
                      onChange={(checked) => handleToggleModelEnabled(m, checked)}
                    />
                  </div>

                  {/* Model Name and Gauge Row */}
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px' }}>
                    <div style={{ flex: 1 }}>
                      <h3 style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginBottom: '4px' }}>
                        {m.display_name}
                      </h3>
                      <div style={{ fontFamily: 'monospace', fontSize: '12px', color: 'var(--text-muted)', marginBottom: '8px' }}>
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
                          maxWidth: '220px',
                          marginBottom: '8px',
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
                    </div>

                    {/* Circular Quota Gauge */}
                    <div style={{ flexShrink: 0 }}>
                      <CircularGauge
                        percentage={gaugePct}
                        emptyGrey={emptyGrey}
                        title={gaugeTitle}
                        subtitle={gaugeSub}
                        size={100}
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
                  }}
                >
                  <button
                    onClick={() => handleCardTest(m)}
                    disabled={isTesting}
                    className="btn-pill-tonal"
                    style={{ padding: '5px 12px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '5px' }}
                  >
                    <Zap size={12} />
                    {isTesting ? 'Testing...' : 'Test Connection'}
                  </button>

                  <button
                    onClick={() => openEditModal(m)}
                    className="btn-pill-tonal"
                    style={{ padding: '5px 14px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '5px' }}
                  >
                    <Edit2 size={12} /> Edit
                  </button>
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
              <div style={{ padding: '10px 14px', borderRadius: '8px', backgroundColor: '#fce8e6', color: '#b3261e', fontSize: '12px', marginBottom: '16px' }}>
                {modalError}
              </div>
            )}

            {/* Form Fields */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', marginBottom: '20px' }}>
              {/* Display Name & Model Identifier */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                    Display Name:
                  </label>
                  <input
                    type="text"
                    placeholder="e.g. GPT-4o Production"
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
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      {fetchedModels.length > 0 && (
                        <button
                          type="button"
                          onClick={() => setManualModelEntry(!manualModelEntry)}
                          className="btn-pill-tonal"
                          style={{ padding: '2px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
                          title={manualModelEntry ? 'Choose from fetched models dropdown' : 'Type custom model identifier manually'}
                        >
                          {manualModelEntry ? '📋 Choose from List' : '✏️ Type Manually'}
                        </button>
                      )}
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
                  </div>
                  {fetchedModels.length > 0 && !manualModelEntry ? (
                    <select
                      value={fetchedModels.some((m) => m.id === modelName) ? modelName : (modelName ? '__custom__' : '')}
                      onChange={(e) => {
                        if (e.target.value === '__manual__') {
                          setManualModelEntry(true)
                        } else if (e.target.value !== '__custom__') {
                          handleSelectFetchedModel(e.target.value)
                        }
                      }}
                      style={{
                        width: '100%',
                        fontSize: '12px',
                        padding: '8px 10px',
                        fontFamily: 'monospace',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                      }}
                    >
                      <option value="">-- Choose a model ({fetchedModels.length} available) --</option>
                      {modelName && !fetchedModels.some((m) => m.id === modelName) && (
                        <option value="__custom__">Current: {modelName}</option>
                      )}
                      {fetchedModels.map((m) => (
                        <option key={m.id} value={m.id}>
                          {m.id} {m.context_window ? `(${m.context_window.toLocaleString()} ctx)` : ''} {m.supports_thinking ? '🧠 [Thinking]' : ''}
                        </option>
                      ))}
                      <option value="__manual__">✏️ Custom / Type manually...</option>
                    </select>
                  ) : (
                    <input
                      type="text"
                      placeholder="e.g. gpt-4o, claude-3-7-sonnet"
                      value={modelName}
                      onChange={(e) => setModelName(e.target.value)}
                      style={{ width: '100%', fontSize: '12px', padding: '8px 10px', fontFamily: 'monospace' }}
                    />
                  )}
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

              {/* Provider Type & Context Window */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                    Provider Protocol:
                  </label>
                  <select
                    value={providerType === 'local' ? 'openai' : providerType}
                    onChange={(e) => setProviderType(e.target.value as ProviderType)}
                    style={{ width: '100%', fontSize: '12px', padding: '8px 10px' }}
                  >
                    <option value="openai">OpenAI Compatible (/v1/chat/completions)</option>
                    <option value="anthropic">Anthropic Claude (/v1/messages)</option>
                    <option value="gemini">Google Gemini API (:generateContent)</option>
                    <option value="custom">Custom (Direct / Raw Endpoint)</option>
                  </select>
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

              {/* Base URL */}
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                  {providerType === 'custom' ? 'Custom Endpoint URL:' : 'Base URL:'}
                </label>
                <input
                  type="text"
                  placeholder={
                    providerType === 'custom'
                      ? 'e.g. https://my-custom-proxy.internal/v1/chat/completions'
                      : 'e.g. https://api.openai.com/v1 or https://opencode.ai/zen/go/v1'
                  }
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  style={{ width: '100%', fontSize: '12px', padding: '8px 10px', fontFamily: 'monospace' }}
                />
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px', fontFamily: 'monospace' }}>
                  Resolved Test Endpoint:{' '}
                  <span style={{ color: 'var(--primary)', fontWeight: 600 }}>
                    {resolveEndpointPreview(providerType, baseUrl, modelName)}
                  </span>
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

              {/* Thinking / Reasoning Level */}
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                  Thinking / Reasoning Level:
                </label>
                <select
                  value={supportsThinking && thinkingLevel.toLowerCase() !== 'off' ? thinkingLevel.toLowerCase() : 'off'}
                  onChange={(e) => {
                    const val = e.target.value
                    if (val === 'off') {
                      setSupportsThinking(false)
                      setThinkingLevel('off')
                    } else {
                      setSupportsThinking(true)
                      setThinkingLevel(val)
                    }
                  }}
                  style={{ width: '100%', fontSize: '12px', padding: '8px 10px' }}
                >
                  <option value="off">Off (Disabled / Standard Generation)</option>
                  <option value="low">Low</option>
                  <option value="medium">Medium</option>
                  <option value="high">High</option>
                  {!['off', 'low', 'medium', 'high'].includes(thinkingLevel.toLowerCase()) && thinkingLevel && (
                    <option value={thinkingLevel.toLowerCase()}>
                      {thinkingLevel.charAt(0).toUpperCase() + thinkingLevel.slice(1)} (Custom)
                    </option>
                  )}
                </select>
              </div>

              {/* Quota Type Selection */}
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                  Quota Tracking Mode:
                </label>
                <select
                  value={quotaType}
                  onChange={(e) => setQuotaType(e.target.value as QuotaType)}
                  style={{ width: '100%', fontSize: '12px', padding: '8px 10px' }}
                >
                  <option value="none">Untracked / None (Always renders grey N/A ring gauge)</option>
                  <option value="cost_based">Cost-Based / Prepaid Balance (Deposit remaining fraction)</option>
                  <option value="quota_based">Quota-Based / Rate Limits (Remaining fraction percentage)</option>
                </select>
              </div>

              {/* Conditional Quota Inputs */}
              {quotaType === 'cost_based' && (
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', backgroundColor: 'var(--canvas)', padding: '12px', borderRadius: '8px' }}>
                  <div>
                    <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                      Prepaid Balance ($):
                    </label>
                    <input
                      type="number"
                      step="0.01"
                      value={prepaidBalance}
                      onChange={(e) => setPrepaidBalance(parseFloat(e.target.value) || 0)}
                      style={{ width: '100%', fontSize: '12px', padding: '6px 8px' }}
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                      Total Budget ($):
                    </label>
                    <input
                      type="number"
                      step="0.01"
                      value={totalBudget}
                      onChange={(e) => setTotalBudget(parseFloat(e.target.value) || 0)}
                      style={{ width: '100%', fontSize: '12px', padding: '6px 8px' }}
                    />
                  </div>
                </div>
              )}

              {quotaType === 'quota_based' && (
                <div style={{ backgroundColor: 'var(--canvas)', padding: '12px', borderRadius: '8px' }}>
                  <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                    Quota Fraction Remaining (0.0 to 1.0):
                  </label>
                  <input
                    type="number"
                    min="0"
                    max="1"
                    step="0.05"
                    value={quotaFraction}
                    onChange={(e) => setQuotaFraction(parseFloat(e.target.value) || 0)}
                    style={{ width: '100%', fontSize: '12px', padding: '6px 8px' }}
                  />
                </div>
              )}

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
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '12px', cursor: 'pointer' }}>
                  <ToggleSwitch
                    size="sm"
                    checked={enabled}
                    onChange={(checked) => setEnabled(checked)}
                  />
                  <span>Enabled</span>
                </label>
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
                flexWrap: 'wrap',
              }}
            >
              {/* Bottom Left: Test Connection Button & Result Feedback */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
                <button
                  type="button"
                  onClick={handleTestInModal}
                  disabled={modalTesting || !baseUrl.trim()}
                  className="btn-pill-tonal"
                  style={{ padding: '7px 16px', fontSize: '12px', display: 'flex', alignItems: 'center', gap: '6px' }}
                >
                  <Zap size={13} />
                  {modalTesting ? 'Testing Endpoint...' : 'Test Connection'}
                </button>

                {modalTestResult && (() => {
                  const isSuccess = modalTestResult.success && modalTestResult.status_code >= 200 && modalTestResult.status_code < 300
                  return (
                    <div
                      style={{
                        fontSize: '11px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '5px',
                        color: isSuccess ? 'var(--green)' : '#b3261e',
                        fontWeight: 600,
                        backgroundColor: isSuccess ? 'var(--green-bg)' : '#fce8e6',
                        padding: '4px 10px',
                        borderRadius: '9999px',
                      }}
                    >
                      {isSuccess ? <CheckCircle2 size={13} /> : <AlertCircle size={13} />}
                      <span>
                        {isSuccess
                          ? `${modalTestResult.status_code} OK (${modalTestResult.latency_ms}ms)`
                          : `Failed: ${modalTestResult.message}`}
                      </span>
                    </div>
                  )
                })()}
              </div>

              {/* Bottom Right: Delete (if editing), Cancel & Save Model */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                {editingModel && (
                  <button
                    type="button"
                    onClick={() => handleDeleteModel(editingModel.id, editingModel.display_name)}
                    disabled={modalSaving}
                    className="btn-pill-danger"
                    style={{ padding: '7px 14px', fontSize: '12px' }}
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
    </div>
  )
}
