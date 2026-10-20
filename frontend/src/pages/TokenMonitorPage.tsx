import React, { useState, useEffect, useMemo } from 'react'
import {
  Download,
  RefreshCw,
  CheckCircle2,
  Trash2,
} from 'lucide-react'
import type {
  ModelPricingRecord,
  TokenRange,
  TokenSummaryResponse,
  TokenTrendBucket,
  TokenModelBreakdown,
  TokenProjectBreakdown,
  TokenTelemetryEvent,
} from '../types'
import { api, isAbortError } from '../api'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { buildTrendBars, bucketLabel, OTHERS_COLOR, TREND_COLORS } from '../utils/tokenTrend'

import {
  GEMINI_SELECTOR_ORDER,
  type SortPricingOptions,
  normalizeModelIdentifier,
  getGeminiSelectorRank,
  isModelDefaultNative,
  isModelDefaultCustom,
  sortPricingModels,
} from '../utils/tokenPriceSort'

export {
  GEMINI_SELECTOR_ORDER,
  type SortPricingOptions,
  normalizeModelIdentifier,
  getGeminiSelectorRank,
  isModelDefaultNative,
  isModelDefaultCustom,
  sortPricingModels,
}

// Format Helper exported for external testing and consumers
export const formatTokensHelper = (num: number | null | undefined): string => {
  const n = num ?? 0
  if (n >= 1000000000) return `${(n / 1000000000).toFixed(2)}B`
  if (n >= 1000000) return `${(n / 1000000).toFixed(2)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return n.toLocaleString()
}

// 2-decimal metric formatter shared by the Models / Projects breakdowns exported for external testing
export const formatMetricValueHelper = (
  value: number | null | undefined,
  unitMode: "usd" | "tokens" | "requests"
): string => {
  if (unitMode === "usd" && (value === null || value === undefined)) {
    return "Unpriced (null)"
  }
  const v = value ?? 0
  if (unitMode === "usd") return `$${v.toFixed(2)}`
  if (unitMode === "requests") return `${Math.round(v).toLocaleString()} req`
  return v >= 1000000000
    ? `${(v / 1000000000).toFixed(2)}B`
    : v >= 1000000
    ? `${(v / 1000000).toFixed(2)}M`
    : v >= 1000
    ? `${(v / 1000).toFixed(2)}k`
    : v.toFixed(0)
}

// Chart Y-axis tick formatter exported for external testing
export const formatTrendYTickHelper = (
  v: number,
  unitMode: "usd" | "tokens" | "requests"
): string => {
  if (unitMode === "usd") {
    if (v === 0) return "$0"
    return v >= 10 ? `$${v.toFixed(0)}` : `$${v.toFixed(2)}`
  }
  if (unitMode === "requests") return v === 0 ? "0" : Math.round(v).toString()
  if (v === 0) return "0"
  if (v >= 1000000000) return `${(v / 1000000000).toFixed(1)}B`
  if (v >= 1000000) return `${(v / 1000000).toFixed(1)}M`
  if (v >= 1000) return `${Math.round(v / 1000)}k`
  return Math.round(v).toString()
}

export {
  formatTokensHelper as formatTokens,
  formatMetricValueHelper as formatMetricValue,
  formatTrendYTickHelper as formatTrendYTick,
}

interface TokenMonitorPageProps {
  onRefresh?: () => void
  activeTab?: number
  onTabChange?: (tab: number) => void
}

export const TokenMonitorPage: React.FC<TokenMonitorPageProps> = ({
  onRefresh: _onRefresh,
  activeTab: controlledActiveTab,
  onTabChange: _onTabChange,
}) => {
  const [internalActiveTab] = useState<number>(0)
  const activeTab = controlledActiveTab !== undefined ? controlledActiveTab : internalActiveTab

  // State for Unit Toggle: USD vs Tokens vs Requests (#129)
  const [unitMode, setUnitMode] = useState<'usd' | 'tokens' | 'requests'>('usd')
  // Time range filter (#125): shared by the KPI cards and the histogram so both
  // always describe the same window.
  const [timeRange, setTimeRange] = useState<TokenRange>('7d')
  const [isFetchingPrices, setIsFetchingPrices] = useState(false)
  const [fetchFeedback, setFetchFeedback] = useState<string | null>(null)
  const [editingPricing, setEditingPricing] = useState<ModelPricingRecord | null>(null)
  const [isSavingPricing, setIsSavingPricing] = useState(false)
  const [isIoHovered, setIsIoHovered] = useState(false)

  // In-Chat Telemetry Display settings state
  const [chatTelemetryInputTokens, setChatTelemetryInputTokens] = useState<boolean>(true)
  const [chatTelemetryOutputTokens, setChatTelemetryOutputTokens] = useState<boolean>(true)
  const [chatTelemetryCacheHitRatio, setChatTelemetryCacheHitRatio] = useState<boolean>(true)
  const [chatTelemetryGenerationSpeed, setChatTelemetryGenerationSpeed] = useState<boolean>(true)
  const [chatTelemetryScope, setChatTelemetryScope] = useState<'aggregated' | 'main_only'>('aggregated')
  // Chat navigator (left rail prompt list) display mode
  const [chatNavigatorMode, setChatNavigatorMode] = useState<'panel' | 'line'>('panel')

  // Telemetry Log filter state (All, Main Agent, Sub-Agents)
  const [logFilter, setLogFilter] = useState<'all' | 'main' | 'subagents'>('all')

  // Real Token Usage Summary
  const [summary, setSummary] = useState<TokenSummaryResponse>({
    total_tokens: 0,
    input_tokens: 0,
    cached_input_tokens: 0,
    output_tokens: 0,
    total_cost_usd: 0,
    saved_cost_usd: 0,
    avg_tps: 0,
    requests_count: 0,
    unpriced_models: [],
  })

  // Model Pricing Registry (synced with Antigravity available models + custom models)
  const [pricingList, setPricingList] = useState<ModelPricingRecord[]>([])
  const [defaultGeminiModel, setDefaultGeminiModel] = useState<string>('gemini-3.8-flash-high')
  const [defaultCustomModel, setDefaultCustomModel] = useState<string>('')

  const sortedPricingList = useMemo(() => {
    return sortPricingModels(pricingList, {
      defaultGeminiModel,
      defaultCustomModel,
    })
  }, [pricingList, defaultGeminiModel, defaultCustomModel])

  // Dynamic Breakdowns loaded from real data
  const [modelBreakdowns, setModelBreakdowns] = useState<TokenModelBreakdown[]>([])
  const [projectBreakdowns, setProjectBreakdowns] = useState<TokenProjectBreakdown[]>([])
  const [telemetryEvents, setTelemetryEvents] = useState<TokenTelemetryEvent[]>([])

  // Usage trend gadget state
  const [trendGroup, setTrendGroup] = useState<'model' | 'project'>('model')
  const [trendTopN, setTrendTopN] = useState<number>(5)
  const [hoverBucket, setHoverBucket] = useState<number | null>(null)

  const loadLiveMetrics = async (range: TokenRange = timeRange) => {
    try {
      const [summaryData, rulesData, customModelsData, guiConfigData] = await Promise.all([
        api.getTokenSummary(range).catch(() => null),
        api.getRules().catch(() => null),
        api.getCustomModels().catch(() => null),
        api.getGUIConfig().catch(() => null),
      ])

      if (guiConfigData) {
        if (guiConfigData.chat_telemetry_input_tokens !== undefined) setChatTelemetryInputTokens(guiConfigData.chat_telemetry_input_tokens)
        if (guiConfigData.chat_telemetry_output_tokens !== undefined) setChatTelemetryOutputTokens(guiConfigData.chat_telemetry_output_tokens)
        if (guiConfigData.chat_telemetry_cache_hit_ratio !== undefined) setChatTelemetryCacheHitRatio(guiConfigData.chat_telemetry_cache_hit_ratio)
        if (guiConfigData.chat_telemetry_generation_speed !== undefined) setChatTelemetryGenerationSpeed(guiConfigData.chat_telemetry_generation_speed)
        if (guiConfigData.chat_telemetry_scope !== undefined) setChatTelemetryScope(guiConfigData.chat_telemetry_scope)
        if (guiConfigData.chat_navigator_mode !== undefined) setChatNavigatorMode(guiConfigData.chat_navigator_mode === 'line' ? 'line' : 'panel')
      }

      if (rulesData) {
        if (rulesData.default_gemini_model) {
          setDefaultGeminiModel(rulesData.default_gemini_model)
        }
        if (rulesData.default_custom_model) {
          setDefaultCustomModel(rulesData.default_custom_model)
        }
      }

      if (customModelsData) {
        const defCustom = customModelsData.models?.find((m) => m.is_default)
        if (defCustom) {
          setDefaultCustomModel(defCustom.id || defCustom.name)
        } else if (customModelsData.active_model_id) {
          setDefaultCustomModel(customModelsData.active_model_id)
        }
      }

      if (summaryData) {
        setSummary(summaryData)
        setModelBreakdowns(summaryData.model_breakdowns || [])
        setProjectBreakdowns(summaryData.project_breakdowns || [])
        setTelemetryEvents(summaryData.telemetry_events || [])
        const pl = summaryData.pricing_records || summaryData.models
        if (pl && Array.isArray(pl)) {
          setPricingList(pl)
        }
      }
    } catch (e) {
      if (isAbortError(e)) return
      console.error('Error fetching token summary:', e)
    }
  }

  useEffect(() => {
    loadLiveMetrics(timeRange)
    const interval = setInterval(() => loadLiveMetrics(timeRange), 30_000)
    return () => clearInterval(interval)
  }, [timeRange])

  const handleUpdateTelemetrySetting = async (key: string, value: any) => {
    try {
      const updates: any = {}
      if (key === 'input') {
        setChatTelemetryInputTokens(value)
        updates.chat_telemetry_input_tokens = value
      } else if (key === 'output') {
        setChatTelemetryOutputTokens(value)
        updates.chat_telemetry_output_tokens = value
      } else if (key === 'cache') {
        setChatTelemetryCacheHitRatio(value)
        updates.chat_telemetry_cache_hit_ratio = value
      } else if (key === 'speed') {
        setChatTelemetryGenerationSpeed(value)
        updates.chat_telemetry_generation_speed = value
      } else if (key === 'scope') {
        setChatTelemetryScope(value)
        updates.chat_telemetry_scope = value
      } else if (key === 'navigator') {
        setChatNavigatorMode(value)
        updates.chat_navigator_mode = value
      }
      const updated = await api.updateGUIConfig(updates)
      if (updated) {
        if (updated.chat_telemetry_input_tokens !== undefined) setChatTelemetryInputTokens(updated.chat_telemetry_input_tokens)
        if (updated.chat_telemetry_output_tokens !== undefined) setChatTelemetryOutputTokens(updated.chat_telemetry_output_tokens)
        if (updated.chat_telemetry_cache_hit_ratio !== undefined) setChatTelemetryCacheHitRatio(updated.chat_telemetry_cache_hit_ratio)
        if (updated.chat_telemetry_generation_speed !== undefined) setChatTelemetryGenerationSpeed(updated.chat_telemetry_generation_speed)
        if (updated.chat_telemetry_scope !== undefined) setChatTelemetryScope(updated.chat_telemetry_scope)
        if (updated.chat_navigator_mode !== undefined) setChatNavigatorMode(updated.chat_navigator_mode === 'line' ? 'line' : 'panel')
      }
    } catch (err) {
      console.warn('Failed to update in-chat telemetry setting:', err)
    }
  }

  const filteredTelemetryEvents = useMemo(() => {
    if (logFilter === 'main') {
      return telemetryEvents.filter(e => !e.is_subagent)
    }
    if (logFilter === 'subagents') {
      return telemetryEvents.filter(e => Boolean(e.is_subagent))
    }
    return telemetryEvents
  }, [telemetryEvents, logFilter])

  // Handler for Price Refresh (Provider -> 3rd-Party Backup -> Null)
  const handleAutoFetchPrices = async () => {
    setIsFetchingPrices(true)
    setFetchFeedback(null)
    try {
      const res = await api.getTokenPricing(true)
      const pl = res?.pricing_records || res?.models
      if (pl && Array.isArray(pl)) {
        setPricingList(pl)
      }
      await loadLiveMetrics()
      setFetchFeedback('Synchronized pricing with Antigravity catalog, provider specs, and 3rd-party backup!')
      setTimeout(() => setFetchFeedback(null), 4000)
    } catch (e) {
      console.error('Error syncing prices:', e)
      setFetchFeedback('Failed to sync pricing.')
      setTimeout(() => setFetchFeedback(null), 4000)
    } finally {
      setIsFetchingPrices(false)
    }
  }

  const handleSavePricingModal = async () => {
    if (!editingPricing) return
    setIsSavingPricing(true)
    try {
      await api.updateTokenPricing({
        internal_id: editingPricing.internal_id,
        canonical_id: editingPricing.canonical_id || editingPricing.model_id,
        model_id: editingPricing.model_id || editingPricing.canonical_id,
        input_price_per_m: editingPricing.input_price_per_m,
        cached_input_price_per_m: editingPricing.cached_input_price_per_m,
        output_price_per_m: editingPricing.output_price_per_m,
      })
      setEditingPricing(null)
      await loadLiveMetrics()
    } catch (e) {
      console.error('Error updating pricing:', e)
    } finally {
      setIsSavingPricing(false)
    }
  }

  const handleDeletePricingModel = async (pr: ModelPricingRecord) => {
    try {
      await api.deleteTokenPricingModel({
        internal_id: pr.internal_id,
        canonical_id: pr.canonical_id || pr.model_id,
        model_id: pr.model_id || pr.canonical_id,
      })
      await loadLiveMetrics()
    } catch (e) {
      console.error('Error deleting pricing model:', e)
    }
  }

  // Format Helper
  const formatTokens = (num: number | null | undefined) => formatTokensHelper(num)

  const formatCost = (usd: number | null | undefined) =>
    usd === null || usd === undefined ? '—' : `$${usd.toFixed(4)}`

  // Capitalized classification label ("native" -> "Native", missing -> "Native")
  const formatClassification = (classification: string | null | undefined) => {
    const raw = classification || 'native'
    return raw.charAt(0).toUpperCase() + raw.slice(1)
  }

  // 2-decimal metric formatter shared by the Models / Projects breakdowns
  const formatMetric = (value: number | null | undefined) => {
    if (unitMode === 'usd' && (value === null || value === undefined)) {
      return 'Unpriced (null)'
    }
    const v = value ?? 0
    if (unitMode === 'usd') return `$${v.toFixed(2)}`
    if (unitMode === 'requests') return `${Math.round(v).toLocaleString()} req`
    return v >= 1000000
      ? `${(v / 1000000).toFixed(2)}M`
      : v >= 1000
      ? `${(v / 1000).toFixed(2)}k`
      : v.toFixed(0)
  }

  // The histogram is built exclusively from the backend's real per-bucket
  // aggregates over the selected window (#125). There is no synthetic
  // interpolation: an empty window renders an empty chart.
  const trendBuckets: TokenTrendBucket[] = React.useMemo(
    () => summary.trend?.buckets ?? [],
    [summary]
  )

  const bucketLabelFor = React.useCallback(
    (start: number) => bucketLabel(start, timeRange),
    [timeRange]
  )

  // Rank models/projects over the whole window, then split each bucket into the
  // top N segments plus an aggregated "Others" segment for stacking.
  const { bars: trendBars, entities: trendEntities, hasData: trendHasData } = React.useMemo(
    () => buildTrendBars(trendBuckets, { unit: unitMode, group: trendGroup, topN: trendTopN }),
    [trendBuckets, unitMode, trendGroup, trendTopN]
  )

  // Input vs Output Cost breakdown calculation
  const { totalInputCost, totalOutputCost } = useMemo(() => {
    if (summary.input_cost_usd !== undefined && summary.output_cost_usd !== undefined) {
      return {
        totalInputCost: summary.input_cost_usd,
        totalOutputCost: summary.output_cost_usd,
      }
    }
    let inCost = 0
    let outCost = 0
    let accountedCost = 0
    for (const m of modelBreakdowns) {
      const id = (m.canonical_id || m.model_id || '').toLowerCase()
      const p = pricingList.find(
        (pr) =>
          (pr.canonical_id || '').toLowerCase() === id ||
          (pr.model_id || '').toLowerCase() === id
      )
      if (p && p.input_price_per_m != null && p.output_price_per_m != null) {
        const inRate = p.input_price_per_m
        const cacheRate = p.cached_input_price_per_m ?? inRate * 0.25
        const outRate = p.output_price_per_m
        const cached = m.cached_tokens || 0
        const fresh = Math.max(0, (m.input_tokens || 0) - cached)
        const mIn = (fresh * inRate + cached * cacheRate) / 1_000_000
        const mOut = ((m.output_tokens || 0) * outRate) / 1_000_000
        inCost += mIn
        outCost += mOut
        accountedCost += (m.cost_usd || (mIn + mOut))
      } else if (m.cost_usd != null && m.cost_usd > 0) {
        const mInTok = m.input_tokens || 0
        const mOutTok = m.output_tokens || 0
        const totalTok = mInTok + mOutTok
        if (totalTok > 0) {
          inCost += (m.cost_usd * mInTok) / totalTok
          outCost += (m.cost_usd * mOutTok) / totalTok
        }
        accountedCost += m.cost_usd
      }
    }
    const rem = Math.max(0, (summary.total_cost_usd || 0) - accountedCost)
    if (rem > 0) {
      const totalIn = summary.input_tokens || 0
      const totalOut = summary.output_tokens || 0
      const totalTok = totalIn + totalOut
      if (totalTok > 0) {
        inCost += (rem * totalIn) / totalTok
        outCost += (rem * totalOut) / totalTok
      } else {
        inCost += rem * 0.5
        outCost += rem * 0.5
      }
    }
    return { totalInputCost: inCost, totalOutputCost: outCost }
  }, [summary, modelBreakdowns, pricingList])

  const ioRatio =
    (summary.output_tokens || 0) > 0
      ? `${((summary.input_tokens || 0) / summary.output_tokens).toFixed(1)}x`
      : 'N/A'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* ============================================================ */}
      {/* TAB 0: CONSUMPTION OVERVIEW */}
      {/* ============================================================ */}
      {activeTab === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* Dedicated Controls Gadget Bar */}
          <div
            className="google-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '12px 16px',
              flexWrap: 'wrap',
              gap: '12px',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
              {/* Unit Switcher: USD vs Tokens vs Requests */}
              <div
                style={{
                  display: 'flex',
                  backgroundColor: 'var(--tonal)',
                  borderRadius: '8px',
                  padding: '3px',
                  gap: '2px',
                }}
              >
                <button
                  onClick={() => setUnitMode('usd')}
                  style={{
                    borderRadius: '6px',
                    padding: '5px 12px',
                    fontSize: '12px',
                    fontWeight: unitMode === 'usd' ? 600 : 500,
                    color: unitMode === 'usd' ? 'var(--primary)' : 'var(--text-muted)',
                    backgroundColor: unitMode === 'usd' ? '#ffffff' : 'transparent',
                    boxShadow: unitMode === 'usd' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                    border: 'none',
                    cursor: 'pointer',
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '4px',
                    whiteSpace: 'nowrap',
                  }}
                >
                  <span>USD</span>
                </button>
                <button
                  onClick={() => setUnitMode('tokens')}
                  style={{
                    borderRadius: '6px',
                    padding: '5px 12px',
                    fontSize: '12px',
                    fontWeight: unitMode === 'tokens' ? 600 : 500,
                    color: unitMode === 'tokens' ? 'var(--primary)' : 'var(--text-muted)',
                    backgroundColor: unitMode === 'tokens' ? '#ffffff' : 'transparent',
                    boxShadow: unitMode === 'tokens' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                    border: 'none',
                    cursor: 'pointer',
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '4px',
                    whiteSpace: 'nowrap',
                  }}
                >
                  <span>Tokens</span>
                </button>
                <button
                  onClick={() => setUnitMode('requests')}
                  style={{
                    borderRadius: '6px',
                    padding: '5px 12px',
                    fontSize: '12px',
                    fontWeight: unitMode === 'requests' ? 600 : 500,
                    color: unitMode === 'requests' ? 'var(--primary)' : 'var(--text-muted)',
                    backgroundColor: unitMode === 'requests' ? '#ffffff' : 'transparent',
                    boxShadow: unitMode === 'requests' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                    border: 'none',
                    cursor: 'pointer',
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '4px',
                    whiteSpace: 'nowrap',
                  }}
                >
                  <span>Requests</span>
                </button>
              </div>

              {/* Time Range Selector */}
              <div
                style={{
                  display: 'flex',
                  backgroundColor: 'var(--tonal)',
                  borderRadius: '8px',
                  padding: '3px',
                  gap: '2px',
                }}
              >
                {(['24h', '7d', '30d', 'all'] as const).map((t) => (
                  <button
                    key={t}
                    onClick={() => setTimeRange(t)}
                    style={{
                      borderRadius: '6px',
                      padding: '5px 10px',
                      fontSize: '12px',
                      fontWeight: timeRange === t ? 600 : 500,
                      color: timeRange === t ? 'var(--primary)' : 'var(--text-muted)',
                      backgroundColor: timeRange === t ? '#ffffff' : 'transparent',
                      boxShadow: timeRange === t ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                      border: 'none',
                      cursor: 'pointer',
                      textTransform: 'uppercase',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {t}
                  </button>
                ))}
              </div>
            </div>

            {/* Sync Prices Button */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              {fetchFeedback && (
                <span className="badge-chip badge-green" style={{ fontSize: '11px', padding: '4px 8px' }}>
                  <CheckCircle2 size={12} />
                  <span>{fetchFeedback}</span>
                </span>
              )}
              <button
                onClick={handleAutoFetchPrices}
                disabled={isFetchingPrices}
                className="btn-pill-tonal"
                style={{
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  borderRadius: '6px',
                  padding: '6px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  color: 'var(--text)',
                  cursor: isFetchingPrices ? 'not-allowed' : 'pointer',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                  whiteSpace: 'nowrap',
                }}
              >
                <RefreshCw size={13} className={isFetchingPrices ? 'animate-spin' : ''} />
                <span>{isFetchingPrices ? 'Syncing...' : 'Sync Prices'}</span>
              </button>
            </div>
          </div>

      {/* 2. Key Performance Metrics (KPI Cards) */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(5, minmax(0, 1fr))',
          gap: '10px',
        }}
      >
        {/* Total Cost / Tokens */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 14px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
            minWidth: 0,
            overflow: 'hidden',
          }}
        >
          <div>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
              {unitMode === 'usd' ? 'Total Cost' : unitMode === 'tokens' ? 'Total Tokens' : 'Total Requests'}
            </span>
          </div>
          <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {unitMode === 'usd'
              ? `$${(summary.total_cost_usd ?? 0).toFixed(2)}`
              : unitMode === 'tokens'
              ? formatTokens(summary.total_tokens)
              : (summary.requests_count ?? 0).toLocaleString()}
          </div>
        </div>

        {/* Input/Output Ratio */}
        <div
          onMouseEnter={() => setIsIoHovered(true)}
          onMouseLeave={() => setIsIoHovered(false)}
          title={
            unitMode === 'usd'
              ? `Total Input Cost: $${totalInputCost.toFixed(2)}\nTotal Output Cost: $${totalOutputCost.toFixed(2)}`
              : `Total Input Token: ${(summary.input_tokens || 0).toLocaleString()}\nTotal Output Token: ${(summary.output_tokens || 0).toLocaleString()}`
          }
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 14px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
            position: 'relative',
            cursor: 'default',
            minWidth: 0,
            overflow: 'hidden',
          }}
        >
          {isIoHovered && (
            <div
              style={{
                position: 'absolute',
                bottom: 'calc(100% + 8px)',
                left: '50%',
                transform: 'translateX(-50%)',
                backgroundColor: '#1f2937',
                color: '#ffffff',
                padding: '6px 10px',
                borderRadius: '6px',
                fontSize: '11px',
                lineHeight: '1.4',
                whiteSpace: 'nowrap',
                boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
                zIndex: 50,
                pointerEvents: 'none',
                display: 'flex',
                flexDirection: 'column',
                gap: '2px',
              }}
            >
              {unitMode === 'usd' ? (
                <>
                  <span>Total Input Cost: ${totalInputCost.toFixed(2)}</span>
                  <span>Total Output Cost: ${totalOutputCost.toFixed(2)}</span>
                </>
              ) : (
                <>
                  <span>Total Input Token: ${(summary.input_tokens || 0).toLocaleString()}</span>
                  <span>Total Output Token: ${(summary.output_tokens || 0).toLocaleString()}</span>
                </>
              )}
            </div>
          )}
          <div>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
              Input/Output Ratio
            </span>
          </div>
          <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {ioRatio}
          </div>
        </div>

        {/* Prompt Input vs Cached Tokens */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 14px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
            minWidth: 0,
            overflow: 'hidden',
          }}
        >
          <div>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
              Cached Input Ratio
            </span>
          </div>
          <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {summary.input_tokens > 0 ? (((summary.cached_input_tokens || 0) / summary.input_tokens) * 100).toFixed(1) : '0.0'}%
          </div>
        </div>

        {/* Generation Speed */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 14px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
            minWidth: 0,
            overflow: 'hidden',
          }}
        >
          <div>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
              Generation Speed
            </span>
          </div>
          <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap' }}>
            {summary.avg_tps && summary.avg_tps > 0 ? summary.avg_tps.toFixed(1) : '—'}{' '}
            <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-muted)' }}>TPS</span>
          </div>
        </div>

        {/* Total Prompts & Requests */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 14px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
            minWidth: 0,
            overflow: 'hidden',
          }}
        >
          <div>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
              Completed Chat Turns
            </span>
          </div>
          <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {(summary.requests_count ?? 0).toLocaleString()}
          </div>
        </div>
      </div>

      {/* Usage Histogram Over Time (stacked) */}
      {(() => {
        const n = trendBars.length
        const series = trendEntities.map((e, i) => ({
          name: e.name,
          total: e.total,
          color: TREND_COLORS[i % TREND_COLORS.length],
        }))
        const maxVal = Math.max(...trendBars.map((b) => b.total), 1e-9)
        const hasData = trendHasData
        const W = 1000
        const H = 200
        const py = (v: number) => H - 12 - (v / maxVal) * (H - 24)
        const tickEvery = Math.max(1, Math.ceil(n / 8))
        const slot = n > 0 ? W / n : W
        const barWidth = Math.max(2, Math.min(48, slot * 0.7))

        const formatYTick = (v: number) => {
          if (unitMode === 'usd') {
            if (v === 0) return '$0'
            return v >= 10 ? `$${v.toFixed(0)}` : `$${v.toFixed(2)}`
          }
          if (unitMode === 'requests') return v === 0 ? '0' : Math.round(v).toString()
          if (v === 0) return '0'
          if (v >= 1000000) return `${(v / 1000000).toFixed(1)}M`
          if (v >= 1000) return `${Math.round(v / 1000)}k`
          return Math.round(v).toString()
        }

        return (
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '10px',
              padding: '20px',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '14px', flexWrap: 'wrap' }}>
              <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
                Usage Trend
              </h3>
              <div
                style={{
                  display: 'flex',
                  backgroundColor: 'var(--tonal)',
                  borderRadius: '8px',
                  padding: '3px',
                  gap: '2px',
                }}
              >
                {(['model', 'project'] as const).map((g) => (
                  <button
                    key={g}
                    onClick={() => setTrendGroup(g)}
                    style={{
                      borderRadius: '6px',
                      padding: '4px 12px',
                      fontSize: '12px',
                      fontWeight: trendGroup === g ? 600 : 500,
                      color: trendGroup === g ? 'var(--primary)' : 'var(--text-muted)',
                      backgroundColor: trendGroup === g ? '#ffffff' : 'transparent',
                      boxShadow: trendGroup === g ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                      border: 'none',
                      cursor: 'pointer',
                      textTransform: 'capitalize',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {g}
                  </button>
                ))}
              </div>
              <select
                value={trendTopN}
                onChange={(e) => setTrendTopN(Number(e.target.value))}
                style={{
                  padding: '5px 10px',
                  borderRadius: '8px',
                  border: '1px solid var(--border)',
                  fontSize: '12px',
                  backgroundColor: '#ffffff',
                  color: 'var(--text)',
                  cursor: 'pointer',
                }}
              >
                {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((v) => (
                  <option key={v} value={v}>
                    Top {v}
                  </option>
                ))}
              </select>
              <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                per {timeRange === '24h' ? 'hour' : 'day'} · {timeRange.toUpperCase()} ·{' '}
                {unitMode === 'usd' ? 'USD' : unitMode === 'tokens' ? 'tokens' : 'requests'}
              </span>
            </div>

            {/* Chart with Left Y-Axis Rail */}
            <div style={{ display: 'flex', gap: '8px', alignItems: 'stretch' }}>
              {/* Y-axis rail */}
              <div
                style={{
                  position: 'relative',
                  width: '48px',
                  height: '200px',
                  flexShrink: 0,
                  fontSize: '11px',
                  color: 'var(--text-muted)',
                  userSelect: 'none',
                }}
              >
                {[1.0, 0.75, 0.5, 0.25, 0.0].map((f) => (
                  <div
                    key={f}
                    style={{
                      position: 'absolute',
                      top: `${py(maxVal * f)}px`,
                      right: '6px',
                      transform: 'translateY(-50%)',
                      lineHeight: 1,
                      whiteSpace: 'nowrap',
                      textAlign: 'right',
                    }}
                  >
                    {formatYTick(maxVal * f)}
                  </div>
                ))}
              </div>

              {/* Main SVG Chart & X-Axis */}
              <div style={{ flex: 1, minWidth: 0, position: 'relative' }}>
                <svg
                  viewBox={`0 0 ${W} ${H}`}
                  style={{ width: '100%', height: '200px', display: 'block' }}
                  preserveAspectRatio="none"
                >
                  {/* horizontal grid */}
                  {[1.0, 0.75, 0.5, 0.25, 0.0].map((f) => (
                    <line
                      key={f}
                      x1={0}
                      x2={W}
                      y1={py(maxVal * f)}
                      y2={py(maxVal * f)}
                      stroke={f === 0 ? '#e8eaed' : '#f1f3f4'}
                      strokeWidth={1}
                    />
                  ))}
                  {/* stacked histogram bars: height = real bucket total, one segment per entity */}
                  {trendBars.map((bar, i) => {
                    const x = i * slot + (slot - barWidth) / 2
                    let acc = 0
                    return (
                      <g
                        key={bar.start}
                        onMouseEnter={() => setHoverBucket(i)}
                        onMouseLeave={() => setHoverBucket((cur) => (cur === i ? null : cur))}
                      >
                        {/* invisible hover column */}
                        <rect x={i * slot} y={0} width={slot} height={H} fill="transparent" />
                        {bar.segments
                          .filter((s) => s.value > 0)
                          .map((s) => {
                            const y0 = py(acc + s.value)
                            const y1 = py(acc)
                            acc += s.value
                            return (
                              <rect
                                key={s.name}
                                x={x}
                                y={y0}
                                width={barWidth}
                                height={Math.max(0, y1 - y0)}
                                fill={s.color}
                                opacity={hoverBucket === null || hoverBucket === i ? 1 : 0.55}
                              />
                            )
                          })}
                      </g>
                    )
                  })}
                  {!hasData && (
                    <text
                      x={W / 2}
                      y={H / 2}
                      textAnchor="middle"
                      fill="#9aa0a6"
                      fontSize="14"
                      style={{ fontFamily: 'inherit' }}
                    >
                      No data in this range
                    </text>
                  )}
                </svg>

                {/* hover tooltip: per-segment values + total, respecting the unit */}
                {hoverBucket !== null && trendBars[hoverBucket] && (
                  <div
                    style={{
                      position: 'absolute',
                      top: '8px',
                      left: `${Math.min(70, ((hoverBucket + 0.5) / Math.max(n, 1)) * 100)}%`,
                      transform: 'translateX(-50%)',
                      backgroundColor: '#1f2937',
                      color: '#ffffff',
                      padding: '8px 10px',
                      borderRadius: '6px',
                      fontSize: '11px',
                      lineHeight: '1.5',
                      whiteSpace: 'nowrap',
                      boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
                      zIndex: 50,
                      pointerEvents: 'none',
                    }}
                  >
                    <div style={{ fontWeight: 700, marginBottom: '2px' }}>
                      {bucketLabelFor(trendBars[hoverBucket].start)}
                    </div>
                    {trendBars[hoverBucket].segments
                      .filter((s) => s.value > 0)
                      .map((s) => (
                        <div key={s.name} style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                          <span style={{ width: '8px', height: '8px', borderRadius: '2px', backgroundColor: s.color }} />
                          <span>{s.name}</span>
                          <span style={{ marginLeft: 'auto', fontWeight: 600 }}>{formatMetric(s.value)}</span>
                        </div>
                      ))}
                    <div style={{ marginTop: '4px', borderTop: '1px solid rgba(255,255,255,0.25)', paddingTop: '4px', display: 'flex', gap: '10px' }}>
                      <span>Total: {formatMetric(trendBars[hoverBucket].total)}</span>
                      <span>Requests: {trendBars[hoverBucket].requests.toLocaleString()}</span>
                    </div>
                  </div>
                )}

                {/* x-axis ticks */}
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
                  {trendBars
                    .filter((_, i) => i % tickEvery === 0 || i === n - 1)
                    .map((b) => (
                      <span key={b.start}>{bucketLabelFor(b.start)}</span>
                    ))}
                </div>
              </div>
            </div>

            {/* legend */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginTop: '12px', flexWrap: 'wrap', paddingLeft: '56px' }}>
              {series.length === 0 ? (
                <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                  No {trendGroup} usage recorded in this range yet.
                </span>
              ) : (
                series.slice(0, trendTopN).map((s) => (
                  <span key={s.name} style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text)' }}>
                    <span style={{ width: '14px', height: '10px', borderRadius: '2px', backgroundColor: s.color }} />
                    {s.name} <span style={{ color: 'var(--text-muted)' }}>{formatMetric(s.total)}</span>
                  </span>
                ))
              )}
              {trendEntities.length > trendTopN && (
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text)' }}>
                  <span style={{ width: '14px', height: '10px', borderRadius: '2px', backgroundColor: OTHERS_COLOR }} />
                  Others
                </span>
              )}
            </div>
          </div>
        )
      })()}

      {/* 4. Multi-Dimensional Usage Breakdown (Models, Projects) */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(420px, 1fr))',
          gap: '20px',
        }}
      >
        {/* Model Breakdown */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '20px',
            height: '340px',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px', flexShrink: 0 }}>
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
              Models
            </h3>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', flex: 1, overflowY: 'auto', paddingRight: '4px' }}>
            {modelBreakdowns.length === 0 ? (
              <div style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                No model token usage recorded yet. Start interacting with agent models to see consumption breakdowns.
              </div>
            ) : (
              modelBreakdowns.map((m) => {
                const pct =
                  unitMode === 'usd'
                    ? summary.total_cost_usd > 0
                      ? ((m.cost_usd ?? 0) / summary.total_cost_usd) * 100
                      : 0
                    : unitMode === 'requests'
                    ? (summary.requests_count ?? 0) > 0
                      ? (m.requests / (summary.requests_count as number)) * 100
                      : 0
                    : summary.total_tokens > 0
                    ? (m.total_tokens / summary.total_tokens) * 100
                    : 0
                const provLower = (m.provider || '').toLowerCase()
                return (
                  <div key={m.canonical_id || m.model_id}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '13px', marginBottom: '4px', gap: '8px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap' }}>
                        <span style={{ fontWeight: 600, color: 'var(--text)' }}>{m.model_name || m.name}</span>
                        <span
                          style={{
                            padding: '1px 6px',
                            borderRadius: '6px',
                            fontSize: '10px',
                            fontWeight: 600,
                            backgroundColor: m.classification === 'native' ? '#e8f0fe' : '#fef3c7',
                            color: m.classification === 'native' ? 'var(--primary)' : '#b45309',
                            textTransform: 'uppercase',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {m.classification === 'native' ? 'Native' : m.is_outdated_native ? 'Custom (Outdated Native)' : 'Custom'}
                        </span>
                      </div>
                      <span style={{ fontWeight: 600, color: m.price_missing && unitMode === 'usd' ? '#b06000' : 'var(--primary)', whiteSpace: 'nowrap' }}>
                        {formatMetric(
                          unitMode === 'usd' ? m.cost_usd : unitMode === 'tokens' ? m.total_tokens : m.requests
                        )}{' '}
                        <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 400 }}>({pct.toFixed(2)}%)</span>
                      </span>
                    </div>
                    <div style={{ height: '7px', width: '100%', backgroundColor: '#f1f3f4', borderRadius: '4px', overflow: 'hidden' }}>
                      <div
                        style={{
                          height: '100%',
                          width: `${pct}%`,
                          backgroundColor:
                            provLower.includes('google') || provLower.includes('gemini')
                              ? 'var(--primary)'
                              : provLower.includes('anthropic')
                              ? '#d97706'
                              : '#10b981',
                          borderRadius: '4px',
                        }}
                      />
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
                      <span>Speed: {m.avg_tps && m.avg_tps > 0 ? `${m.avg_tps} TPS` : '— TPS'}</span>
                      <span>
                        {m.requests} requests • {formatTokens(m.cached_tokens)} cached
                        {m.price_missing ? ' • Price null (set in Token Price)' : ''}
                      </span>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* Project Breakdown */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '20px',
            height: '340px',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px', flexShrink: 0 }}>
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
              Projects
            </h3>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', flex: 1, overflowY: 'auto', paddingRight: '4px' }}>
            {projectBreakdowns.length === 0 ? (
              <div style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                No workspaces configured yet.
              </div>
            ) : (
              projectBreakdowns.map((p) => {
                const pct =
                  unitMode === 'usd'
                    ? summary.total_cost_usd > 0
                      ? (p.cost_usd / summary.total_cost_usd) * 100
                      : 0
                    : unitMode === 'requests'
                    ? (summary.requests_count ?? 0) > 0
                      ? (((p.requests ?? 0) / summary.requests_count) * 100)
                      : 0
                    : summary.total_tokens > 0
                    ? (p.total_tokens / summary.total_tokens) * 100
                    : 0
                return (
                  <div key={p.project_name}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', marginBottom: '4px' }}>
                      <span style={{ fontWeight: 600, color: 'var(--text)' }}>{p.project_name}</span>
                      <span style={{ fontWeight: 600, color: 'var(--primary)' }}>
                        {formatMetric(
                          unitMode === 'usd' ? p.cost_usd : unitMode === 'tokens' ? p.total_tokens : (p.requests ?? 0)
                        )}{' '}
                        <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 400 }}>({pct.toFixed(2)}%)</span>
                      </span>
                    </div>
                    <div style={{ height: '7px', width: '100%', backgroundColor: '#f1f3f4', borderRadius: '4px', overflow: 'hidden' }}>
                      <div
                        style={{
                          height: '100%',
                          width: `${pct}%`,
                          backgroundColor: 'var(--primary)',
                          borderRadius: '4px',
                        }}
                      />
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
                      <span>
                        Cache hit:{' '}
                        {(typeof p.cache_hit_ratio === 'number'
                          ? p.cache_hit_ratio
                          : (p.input_tokens || 0) > 0
                          ? (((p.cached_tokens || 0) / (p.input_tokens || 1)) * 100)
                          : 0
                        ).toFixed(1)}%
                      </span>
                      <span>{p.last_active ? `Active ${p.last_active}` : p.workspace_path}</span>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>
      </div>
        </div>
      )}

      {/* ============================================================ */}
      {/* TAB 2: PRICING MATRIX */}
      {/* ============================================================ */}
      {activeTab === 2 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* 5. Live Token Price & Override Management */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '10px',
              padding: '20px',
            }}
          >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px', flexWrap: 'wrap', gap: '10px' }}>
          <div>
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
              Token Price (USD per 1M Tokens)
            </h3>
            <p style={{ margin: '2px 0 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Auto-fetched from model provider first, then 3rd-party backup (OpenRouter/LiteLLM), or null for manual entry. Thinking-level variants share one canonical model entry.
            </p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            {fetchFeedback && (
              <span className="badge-chip badge-green" style={{ fontSize: '11px', padding: '4px 8px', whiteSpace: 'nowrap' }}>
                <CheckCircle2 size={12} />
                <span>{fetchFeedback}</span>
              </span>
            )}
            <button
              onClick={handleAutoFetchPrices}
              disabled={isFetchingPrices}
              className="btn-pill-tonal"
              style={{
                backgroundColor: '#ffffff',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                padding: '6px 14px',
                fontSize: '12px',
                fontWeight: 600,
                color: 'var(--text)',
                cursor: isFetchingPrices ? 'not-allowed' : 'pointer',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '6px',
                whiteSpace: 'nowrap',
              }}
            >
              <RefreshCw size={13} className={isFetchingPrices ? 'animate-spin' : ''} />
              <span>{isFetchingPrices ? 'Syncing...' : 'Sync Prices'}</span>
            </button>
          </div>
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '13px' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)', color: 'var(--text-muted)' }}>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'left' }}>Model</th>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'center' }}>Type</th>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'center' }}>Provider</th>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'center' }}>Prompt Input ($/1M)</th>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'center' }}>Cached Input ($/1M)</th>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'center' }}>Output ($/1M)</th>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'center' }}>Source</th>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'center' }}>Action</th>
              </tr>
            </thead>
            <tbody>
              {sortedPricingList.length === 0 ? (
                <tr>
                  <td colSpan={8} style={{ padding: '28px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                    No models in pricing registry. Click Sync Prices to fetch available Antigravity and Custom models.
                  </td>
                </tr>
              ) : (
                sortedPricingList.map((pr) => {
                  const provLower = (pr.provider || '').toLowerCase()
                  return (
                    <tr key={`${pr.canonical_id || pr.model_id}-${pr.internal_id}`} style={{ borderBottom: '1px solid var(--border)' }}>
                      <td style={{ padding: '10px 12px', fontWeight: 600, color: 'var(--text)', textAlign: 'left' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap' }}>
                          <span>{pr.model_name || pr.name}</span>
                          {pr.is_outdated_native && (
                            <span
                              style={{
                                fontSize: '10px',
                                fontWeight: 600,
                                padding: '1px 6px',
                                borderRadius: '6px',
                                backgroundColor: '#fef3c7',
                                color: '#b45309',
                                whiteSpace: 'nowrap',
                              }}
                              title="Previously fetched from Antigravity; no longer in active catalog. Reclassified as custom so it can be manually deleted."
                            >
                              Outdated Native
                            </span>
                          )}
                        </div>
                      </td>
                      <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                        <span
                          style={{
                            padding: '2px 8px',
                            borderRadius: '10px',
                            fontSize: '11px',
                            fontWeight: 600,
                            backgroundColor: pr.classification === 'native' ? '#e8f0fe' : '#f3e8ff',
                            color: pr.classification === 'native' ? 'var(--primary)' : '#7e22ce',
                            textTransform: 'capitalize',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {pr.classification}
                        </span>
                      </td>
                      <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                        <span
                          style={{
                            padding: '2px 8px',
                            borderRadius: '10px',
                            fontSize: '11px',
                            fontWeight: 600,
                            backgroundColor:
                              provLower.includes('google') || provLower.includes('gemini')
                                ? '#e8f0fe'
                                : provLower.includes('anthropic')
                                ? '#fef3c7'
                                : provLower.includes('openai')
                                ? '#e6f4ea'
                                : '#f1f3f4',
                            color:
                              provLower.includes('google') || provLower.includes('gemini')
                                ? 'var(--primary)'
                                : provLower.includes('anthropic')
                                ? '#b45309'
                                : provLower.includes('openai')
                                ? '#137333'
                                : '#5f6368',
                            textTransform: 'capitalize',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {pr.provider}
                        </span>
                      </td>
                      <td style={{ padding: '10px 12px', color: 'var(--text)', textAlign: 'center' }}>
                        {pr.input_price_per_m === null || pr.input_price_per_m === undefined ? (
                          <span style={{ color: 'var(--text-muted)', fontSize: '12px', fontWeight: 400 }}>—</span>
                        ) : (
                          `$${pr.input_price_per_m.toFixed(3)}`
                        )}
                      </td>
                      <td style={{ padding: '10px 12px', color: '#137333', fontWeight: 600, textAlign: 'center' }}>
                        {pr.cached_input_price_per_m === null || pr.cached_input_price_per_m === undefined ? (
                          <span style={{ color: 'var(--text-muted)', fontSize: '12px', fontWeight: 400 }}>—</span>
                        ) : (
                          `$${pr.cached_input_price_per_m.toFixed(4)}`
                        )}
                      </td>
                      <td style={{ padding: '10px 12px', color: 'var(--text)', textAlign: 'center' }}>
                        {pr.output_price_per_m === null || pr.output_price_per_m === undefined ? (
                          <span style={{ color: 'var(--text-muted)', fontSize: '12px', fontWeight: 400 }}>—</span>
                        ) : (
                          `$${pr.output_price_per_m.toFixed(2)}`
                        )}
                      </td>
                      <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                        {['provider', 'third_party', 'api', 'manual'].includes(pr.source || '') ? (
                          <span
                            style={{
                              fontSize: '11px',
                              color:
                                pr.source === 'provider'
                                  ? '#137333'
                                  : pr.source === 'third_party' || pr.source === 'api'
                                  ? 'var(--primary)'
                                  : '#7e22ce',
                              fontWeight: 600,
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {pr.source === 'provider'
                              ? 'Provider Official'
                              : pr.source === 'third_party' || pr.source === 'api'
                              ? '3rd-Party Backup'
                              : 'Manual Override'}
                          </span>
                        ) : (
                          <span style={{ color: 'var(--text-muted)', fontSize: '12px', fontWeight: 400 }}>—</span>
                        )}
                      </td>
                      <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                        <div style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '6px' }}>
                          <button
                            onClick={() => setEditingPricing({ ...pr })}
                            style={{
                              backgroundColor: 'transparent',
                              border: '1px solid var(--border)',
                              borderRadius: '6px',
                              padding: '4px 10px',
                              fontSize: '12px',
                              color: 'var(--primary)',
                              cursor: 'pointer',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            Edit Rate
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
        </div>
      )}

      {/* Edit Rate Modal (Price-only editing; shared with Custom Model Setup) */}
      {editingPricing && (
        <div
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            backgroundColor: 'rgba(0,0,0,0.45)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
        >
          <div
            style={{
              backgroundColor: '#ffffff',
              borderRadius: '10px',
              padding: '24px',
              width: '420px',
              maxHeight: '85vh',
              overflowY: 'auto',
              boxShadow: '0 8px 30px rgba(0,0,0,0.18)',
            }}
          >
            <h3 style={{ margin: '0 0 6px 0', fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Edit Token Price: {editingPricing.model_name || editingPricing.name}
            </h3>
            <p style={{ margin: '0 0 14px 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Only token price fields can be modified here. Leave blank to set as null. Changes sync directly with Custom Models.
            </p>

            <div
              style={{
                backgroundColor: 'var(--tonal)',
                borderRadius: '8px',
                padding: '8px 12px',
                marginBottom: '14px',
                fontSize: '12px',
                display: 'flex',
                justifyContent: 'space-between',
                color: 'var(--text-muted)',
              }}
            >
              <span>Provider: <b style={{ color: 'var(--text)' }}>{editingPricing.provider}</b></span>
              <span>Type: <b style={{ color: 'var(--text)', textTransform: 'capitalize' }}>{editingPricing.classification}</b></span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Prompt Input Price ($/1M)
                </label>
                <input
                  type="number"
                  step="0.001"
                  placeholder="null (unconfigured)"
                  value={editingPricing.input_price_per_m === null || editingPricing.input_price_per_m === undefined ? '' : editingPricing.input_price_per_m}
                  onChange={(e) => {
                    const raw = e.target.value.trim()
                    const parsed = raw === '' ? null : parseFloat(raw)
                    setEditingPricing({
                      ...editingPricing,
                      input_price_per_m: parsed !== null && !Number.isNaN(parsed) ? parsed : null,
                    })
                  }}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    fontSize: '13px',
                    boxSizing: 'border-box',
                  }}
                />
              </div>

              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Cached Input Price ($/1M)
                </label>
                <input
                  type="number"
                  step="0.0001"
                  placeholder="null (unconfigured)"
                  value={editingPricing.cached_input_price_per_m === null || editingPricing.cached_input_price_per_m === undefined ? '' : editingPricing.cached_input_price_per_m}
                  onChange={(e) => {
                    const raw = e.target.value.trim()
                    const parsed = raw === '' ? null : parseFloat(raw)
                    setEditingPricing({
                      ...editingPricing,
                      cached_input_price_per_m: parsed !== null && !Number.isNaN(parsed) ? parsed : null,
                    })
                  }}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    fontSize: '13px',
                    boxSizing: 'border-box',
                  }}
                />
              </div>

              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Output Generation Price ($/1M)
                </label>
                <input
                  type="number"
                  step="0.01"
                  placeholder="null (unconfigured)"
                  value={editingPricing.output_price_per_m === null || editingPricing.output_price_per_m === undefined ? '' : editingPricing.output_price_per_m}
                  onChange={(e) => {
                    const raw = e.target.value.trim()
                    const parsed = raw === '' ? null : parseFloat(raw)
                    setEditingPricing({
                      ...editingPricing,
                      output_price_per_m: parsed !== null && !Number.isNaN(parsed) ? parsed : null,
                    })
                  }}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    fontSize: '13px',
                    boxSizing: 'border-box',
                  }}
                />
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '20px' }}>
              <div>
                {editingPricing.classification === 'custom' && (
                  <button
                    onClick={async () => {
                      await handleDeletePricingModel(editingPricing)
                      setEditingPricing(null)
                    }}
                    title="Delete custom/outdated model price and usage history"
                    style={{
                      backgroundColor: 'transparent',
                      border: '1px solid var(--border-danger)',
                      borderRadius: '6px',
                      padding: '8px 14px',
                      fontSize: '13px',
                      color: 'var(--red)',
                      cursor: 'pointer',
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '6px',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    <Trash2 size={13} />
                    <span>Delete</span>
                  </button>
                )}
              </div>
              <div style={{ display: 'flex', gap: '10px' }}>
                <button
                  onClick={() => setEditingPricing(null)}
                  style={{
                    backgroundColor: 'transparent',
                    border: '1px solid var(--border)',
                    borderRadius: '6px',
                    padding: '8px 16px',
                    fontSize: '13px',
                    cursor: 'pointer',
                    color: 'var(--text-muted)',
                    whiteSpace: 'nowrap',
                  }}
                >
                  Cancel
                </button>
                <button
                  onClick={handleSavePricingModal}
                  disabled={isSavingPricing}
                  style={{
                    backgroundColor: 'var(--primary)',
                    border: 'none',
                    borderRadius: '6px',
                    padding: '8px 16px',
                    fontSize: '13px',
                    fontWeight: 600,
                    color: '#ffffff',
                    cursor: isSavingPricing ? 'not-allowed' : 'pointer',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {isSavingPricing ? 'Saving...' : 'Save Rate'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ============================================================ */}
      {/* TAB 1: TELEMETRY & LOGS */}
      {/* ============================================================ */}
      {activeTab === 1 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* In-Chat Telemetry Display Settings Gadget */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '20px',
            }}
          >
            <div style={{ marginBottom: '16px' }}>
              <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
                In-Chat Telemetry Display
              </h3>
              <p style={{ margin: '4px 0 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
                Configure real-time per-chat metrics shown at the bottom action row of each assistant response alongside timestamp and copy controls.
              </p>
            </div>

            {/* Scope Switcher: Aggregated vs Main Agent Only (Placed ABOVE the 2x2 grid) */}
            <div
              style={{
                padding: '14px 16px',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                backgroundColor: '#ffffff',
                marginBottom: '16px',
              }}
            >
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Main Conversation Metrics Scope
                </div>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                  Choose whether metrics in the main chat reflect the main agent only or aggregate usage from concurrent subagents.
                </div>
              </div>
              <div style={{ marginTop: '10px' }}>
                <div
                  style={{
                    display: 'inline-flex',
                    backgroundColor: 'var(--tonal)',
                    borderRadius: '8px',
                    padding: '3px',
                    gap: '2px',
                  }}
                >
                  <button
                    type="button"
                    onClick={() => handleUpdateTelemetrySetting('scope', 'aggregated')}
                    style={{
                      borderRadius: '6px',
                      padding: '6px 14px',
                      fontSize: '12px',
                      fontWeight: chatTelemetryScope === 'aggregated' ? 600 : 500,
                      color: chatTelemetryScope === 'aggregated' ? 'var(--primary)' : 'var(--text-muted)',
                      backgroundColor: chatTelemetryScope === 'aggregated' ? '#ffffff' : 'transparent',
                      boxShadow: chatTelemetryScope === 'aggregated' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                      border: 'none',
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    Aggregated (Main + Sub-agents)
                  </button>
                  <button
                    type="button"
                    onClick={() => handleUpdateTelemetrySetting('scope', 'main_only')}
                    style={{
                      borderRadius: '6px',
                      padding: '6px 14px',
                      fontSize: '12px',
                      fontWeight: chatTelemetryScope === 'main_only' ? 600 : 500,
                      color: chatTelemetryScope === 'main_only' ? 'var(--primary)' : 'var(--text-muted)',
                      backgroundColor: chatTelemetryScope === 'main_only' ? '#ffffff' : 'transparent',
                      boxShadow: chatTelemetryScope === 'main_only' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                      border: 'none',
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    Main Agent Only
                  </button>
                </div>
              </div>
            </div>

            {/* 2x2 Layout of Metric Toggle Subsections */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(2, 1fr)',
                gap: '12px',
              }}
            >
              {/* 1. Input Tokens */}
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '12px 14px',
                  border: '1px solid var(--border)',
                  borderRadius: '6px',
                  backgroundColor: '#ffffff',
                }}
              >
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Input Tokens
                </div>
                <ToggleSwitch
                  checked={chatTelemetryInputTokens}
                  onChange={(val: boolean) => handleUpdateTelemetrySetting('input', val)}
                />
              </div>

              {/* 2. Output Tokens */}
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '12px 14px',
                  border: '1px solid var(--border)',
                  borderRadius: '6px',
                  backgroundColor: '#ffffff',
                }}
              >
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Output Tokens
                </div>
                <ToggleSwitch
                  checked={chatTelemetryOutputTokens}
                  onChange={(val: boolean) => handleUpdateTelemetrySetting('output', val)}
                />
              </div>

              {/* 3. Cache Hit Ratio */}
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '12px 14px',
                  border: '1px solid var(--border)',
                  borderRadius: '6px',
                  backgroundColor: '#ffffff',
                }}
              >
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Cache Hit Ratio
                </div>
                <ToggleSwitch
                  checked={chatTelemetryCacheHitRatio}
                  onChange={(val: boolean) => handleUpdateTelemetrySetting('cache', val)}
                />
              </div>

              {/* 4. Generation Speed */}
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '12px 14px',
                  border: '1px solid var(--border)',
                  borderRadius: '6px',
                  backgroundColor: '#ffffff',
                }}
              >
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Generation Speed
                </div>
                <ToggleSwitch
                  checked={chatTelemetryGenerationSpeed}
                  onChange={(val: boolean) => handleUpdateTelemetrySetting('speed', val)}
                />
              </div>
            </div>

            {/* Chat Navigator (left rail prompt list) display mode */}
            <div
              style={{
                marginTop: '16px',
                paddingTop: '16px',
                borderTop: '1px solid var(--border)',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  gap: '12px',
                  padding: '12px 14px',
                  border: '1px solid var(--border)',
                  borderRadius: '6px',
                  backgroundColor: '#ffffff',
                }}
              >
                <div>
                  <label
                    htmlFor="chat-navigator-mode"
                    style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}
                  >
                    Chat navigator
                  </label>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    {chatNavigatorMode === 'line'
                      ? 'Show the rail as plain dashes.'
                      : 'Hover the left rail to expand the prompt list.'}
                  </div>
                </div>
                <select
                  id="chat-navigator-mode"
                  aria-label="Chat navigator"
                  value={chatNavigatorMode}
                  onChange={(e) =>
                    handleUpdateTelemetrySetting('navigator', e.target.value === 'line' ? 'line' : 'panel')
                  }
                  style={{
                    padding: '6px 10px',
                    fontSize: '12px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: '#ffffff',
                    color: 'var(--text)',
                    cursor: 'pointer',
                  }}
                >
                  <option value="panel">Panel (default)</option>
                  <option value="line">Single line</option>
                </select>
              </div>
            </div>
          </div>

          {/* 6. Live Telemetry Stream Log */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '20px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px', flexWrap: 'wrap', gap: '12px' }}>
              <div>
                <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
                  Recent Agent Session Telemetry Log
                </h3>
                <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                  Real-time audit log of executed chat turns extracted from local session transcripts.
                </span>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                {/* Switch button filter: All, Main Agent, Sub-Agents */}
                <div style={{ display: 'inline-flex', borderRadius: '6px', border: '1px solid var(--border)', overflow: 'hidden' }}>
                  <button
                    type="button"
                    onClick={() => setLogFilter('all')}
                    style={{
                      padding: '5px 12px',
                      fontSize: '12px',
                      fontWeight: logFilter === 'all' ? 700 : 500,
                      backgroundColor: logFilter === 'all' ? 'var(--primary)' : '#ffffff',
                      color: logFilter === 'all' ? '#ffffff' : 'var(--text-muted)',
                      border: 'none',
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    All
                  </button>
                  <button
                    type="button"
                    onClick={() => setLogFilter('main')}
                    style={{
                      padding: '5px 12px',
                      fontSize: '12px',
                      fontWeight: logFilter === 'main' ? 700 : 500,
                      backgroundColor: logFilter === 'main' ? 'var(--primary)' : '#ffffff',
                      color: logFilter === 'main' ? '#ffffff' : 'var(--text-muted)',
                      border: 'none',
                      borderLeft: '1px solid var(--border)',
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    Main Agent
                  </button>
                  <button
                    type="button"
                    onClick={() => setLogFilter('subagents')}
                    style={{
                      padding: '5px 12px',
                      fontSize: '12px',
                      fontWeight: logFilter === 'subagents' ? 700 : 500,
                      backgroundColor: logFilter === 'subagents' ? 'var(--primary)' : '#ffffff',
                      color: logFilter === 'subagents' ? '#ffffff' : 'var(--text-muted)',
                      border: 'none',
                      borderLeft: '1px solid var(--border)',
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    Sub-Agents
                  </button>
                </div>

                <button
                  onClick={() => {
                    const csvContent =
                      'data:text/csv;charset=utf-8,' +
                      ['Timestamp,TurnID,Project,Model,Type,Input,Cached,Output,TPS,Cost']
                        .concat(
                          filteredTelemetryEvents.map(
                            (e) =>
                              `${e.timestamp},${e.id},${e.project},${e.model},${e.classification},${e.input_tokens},${e.cached_tokens},${e.output_tokens},${e.tps},${e.cost_usd ?? 'null'}`
                          )
                        )
                        .join('\n')
                    const encodedUri = encodeURI(csvContent)
                    const link = document.createElement('a')
                    link.setAttribute('href', encodedUri)
                    link.setAttribute('download', `antigravity_token_audit_${Date.now()}.csv`)
                    document.body.appendChild(link)
                    link.click()
                    document.body.removeChild(link)
                  }}
                  style={{
                    backgroundColor: '#ffffff',
                    border: '1px solid var(--border)',
                    borderRadius: '6px',
                    padding: '6px 14px',
                    fontSize: '12px',
                    fontWeight: 600,
                    color: 'var(--text)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    whiteSpace: 'nowrap',
                  }}
                >
                  <Download size={14} />
                  Export CSV
                </button>
              </div>
            </div>

            <div style={{ maxHeight: '420px', overflowY: 'auto', overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                    <th style={{ padding: '8px 12px' }}>Time</th>
                    <th style={{ padding: '8px 12px' }}>Session / Project</th>
                    <th style={{ padding: '8px 12px' }}>Agent</th>
                    <th style={{ padding: '8px 12px' }}>Model</th>
                    <th style={{ padding: '8px 12px' }}>Tokens (In / Cache / Out)</th>
                    <th style={{ padding: '8px 12px' }}>Speed</th>
                    <th style={{ padding: '8px 12px' }}>Cost</th>
                    <th style={{ padding: '8px 12px' }}>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredTelemetryEvents.length === 0 ? (
                    <tr>
                      <td colSpan={8} style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                        No live telemetry sessions recorded for the selected filter.
                      </td>
                    </tr>
                  ) : (
                    filteredTelemetryEvents.map((ev) => (
                    <tr key={ev.id} style={{ borderBottom: '1px solid var(--border)' }}>
                      <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{ev.timestamp}</td>
                    <td style={{ padding: '10px 12px' }}>
                      <div style={{ fontWeight: 600, color: 'var(--text)' }}>{ev.project}</div>
                    </td>
                    <td style={{ padding: '10px 12px' }}>
                      <span
                        title={ev.id}
                        style={{ fontWeight: 500, color: 'var(--text)', cursor: 'help' }}
                      >
                        {ev.agent_role || (ev.is_subagent ? 'Sub-Agent' : 'Main Agent')}
                      </span>
                    </td>
                    <td style={{ padding: '10px 12px', fontWeight: 500, color: 'var(--text)' }}>
                      <span
                        title={`${ev.model || ev.canonical_id || ev.model_id || 'Model'} · ${formatClassification(ev.classification)}`}
                        style={{ cursor: 'help' }}
                      >
                        {ev.model || ev.canonical_id || ev.model_id || 'Model'}
                      </span>
                    </td>
                    <td style={{ padding: '10px 12px' }}>
                      <span>{formatTokens(ev.input_tokens)} in</span> •{' '}
                      <span style={{ color: '#137333' }}>{formatTokens(ev.cached_tokens)} cache</span> •{' '}
                      <span>{formatTokens(ev.output_tokens)} out</span>
                    </td>
                    <td style={{ padding: '10px 12px', fontWeight: 600, color: '#b06000' }}>
                      {ev.tps && ev.tps > 0 ? `${ev.tps} TPS` : '— TPS'}
                    </td>
                    <td style={{ padding: '10px 12px', fontWeight: 600, color: 'var(--text)' }}>
                      {formatCost(ev.cost_usd)}
                    </td>
                    <td style={{ padding: '10px 12px' }}>
                      <span
                        style={{
                          backgroundColor: '#e6f4ea',
                          color: '#137333',
                          padding: '2px 8px',
                          borderRadius: '10px',
                          fontSize: '11px',
                          fontWeight: 600,
                        }}
                      >
                        Completed
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
        </div>
      )}
    </div>
  )
}

export default TokenMonitorPage
