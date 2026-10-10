import React, { useState, useEffect } from 'react'
import {
  Zap,
  TrendingDown,
  Download,
  RefreshCw,
  CheckCircle2,
  Sparkles,
  Activity,
  CornerDownRight,
  Trash2,
} from 'lucide-react'
import type {
  ModelPricingRecord,
  TokenSummaryResponse,
  TokenModelBreakdown,
  TokenProjectBreakdown,
  TokenTelemetryEvent,
} from '../types'
import { api } from '../api'

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

  // State for Unit Toggle: Tokens vs USD
  const [unitMode, setUnitMode] = useState<'usd' | 'tokens'>('usd')
  const [timeRange, setTimeRange] = useState<'24h' | '7d' | '30d' | 'all'>('7d')
  const [isFetchingPrices, setIsFetchingPrices] = useState(false)
  const [fetchFeedback, setFetchFeedback] = useState<string | null>(null)
  const [editingPricing, setEditingPricing] = useState<ModelPricingRecord | null>(null)
  const [isSavingPricing, setIsSavingPricing] = useState(false)

  // Subagent Aggregation Simulator interactive state
  const [simOrchestratorTokens, setSimOrchestratorTokens] = useState(4200)
  const [simSubagentsCount, setSimSubagentsCount] = useState(2)
  const [simSubagentAvgTokens, setSimSubagentAvgTokens] = useState(6500)
  const [simCachedRatio, setSimCachedRatio] = useState(65) // 65%

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

  // Dynamic Breakdowns loaded from real data
  const [modelBreakdowns, setModelBreakdowns] = useState<TokenModelBreakdown[]>([])
  const [projectBreakdowns, setProjectBreakdowns] = useState<TokenProjectBreakdown[]>([])
  const [telemetryEvents, setTelemetryEvents] = useState<TokenTelemetryEvent[]>([])

  // Usage trend gadget state
  const [trendGroup, setTrendGroup] = useState<'model' | 'project'>('model')
  const [trendTopN, setTrendTopN] = useState<number>(5)

  const USAGE_HISTORY_KEY = 'antigravity_token_usage_history'

  const loadUsageHistory = (): { t: number; tokens: number; cost: number }[] => {
    try {
      const raw = localStorage.getItem(USAGE_HISTORY_KEY)
      const parsed = raw ? JSON.parse(raw) : []
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }

  const recordUsageSnapshot = (tokens: number, cost: number) => {
    const now = Date.now()
    const history = loadUsageHistory()
    const last = history[history.length - 1]
    if (last && now - last.t < 60_000) return
    history.push({ t: now, tokens, cost })
    const cutoff = now - 35 * 24 * 3600 * 1000
    const trimmed = history.filter((p) => p.t >= cutoff).slice(-4000)
    try {
      localStorage.setItem(USAGE_HISTORY_KEY, JSON.stringify(trimmed))
    } catch {}
  }

  const loadLiveMetrics = async () => {
    try {
      const summaryData = await api.getTokenSummary()
      if (summaryData) {
        setSummary(summaryData)
        recordUsageSnapshot(summaryData.total_tokens, summaryData.total_cost_usd)
        setModelBreakdowns(summaryData.model_breakdowns || [])
        setProjectBreakdowns(summaryData.project_breakdowns || [])
        setTelemetryEvents(summaryData.telemetry_events || [])
        const pl = summaryData.pricing_records || summaryData.models
        if (pl && Array.isArray(pl)) {
          setPricingList(pl)
        }
      }
    } catch (e) {
      console.error('Error fetching token summary:', e)
    }
  }

  useEffect(() => {
    loadLiveMetrics()
    const interval = setInterval(loadLiveMetrics, 30_000)
    return () => clearInterval(interval)
  }, [])

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
  const formatTokens = (num: number | null | undefined) => {
    const n = num ?? 0
    if (n >= 1000000) return `${(n / 1000000).toFixed(2)}M`
    if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
    return n.toLocaleString()
  }

  const formatCost = (usd: number | null | undefined) =>
    usd === null || usd === undefined ? 'null' : `$${usd.toFixed(4)}`

  // 2-decimal metric formatter shared by the Models / Projects breakdowns
  const formatMetric = (value: number | null | undefined) => {
    if (unitMode === 'usd' && (value === null || value === undefined)) {
      return 'Unpriced (null)'
    }
    const v = value ?? 0
    return unitMode === 'usd'
      ? `$${v.toFixed(2)}`
      : v >= 1000000
      ? `${(v / 1000000).toFixed(2)}M`
      : v >= 1000
      ? `${(v / 1000).toFixed(2)}k`
      : v.toFixed(0)
  }

  const rangeBuckets: Record<string, { count: number; ms: number; label: (d: Date) => string }> = {
    '24h': { count: 24, ms: 3600_000, label: (d) => `${d.getHours()}:00` },
    '7d': { count: 7, ms: 86400_000, label: (d) => d.toLocaleDateString(undefined, { weekday: 'short' }) },
    '30d': { count: 30, ms: 86400_000, label: (d) => `${d.getMonth() + 1}/${d.getDate()}` },
    all: { count: 14, ms: 86400_000, label: (d) => `${d.getMonth() + 1}/${d.getDate()}` },
  }

  const buildTrendTotals = (): { labels: string[]; totals: number[] } => {
    const spec = rangeBuckets[timeRange] || rangeBuckets['7d']
    const now = Date.now()
    const buckets = new Array<number>(spec.count).fill(0)
    const labels: string[] = []
    for (let i = 0; i < spec.count; i++) {
      labels.push(spec.label(new Date(now - (spec.count - 1 - i) * spec.ms)))
    }
    const history = loadUsageHistory()
    const val = (v: { tokens: number; cost: number }) => (unitMode === 'usd' ? v.cost : v.tokens)
    if (history.length >= 2) {
      for (let i = 1; i < history.length; i++) {
        const delta = Math.max(0, val(history[i]) - val(history[i - 1]))
        const idx = spec.count - 1 - Math.floor((now - history[i].t) / spec.ms)
        if (idx >= 0 && idx < spec.count) buckets[idx] += delta
      }
      const last = history[history.length - 1]
      buckets[spec.count - 1] += Math.max(0, val({ tokens: summary.total_tokens, cost: summary.total_cost_usd }) - val(last))
      return { labels, totals: buckets }
    }
    const totalVal = unitMode === 'usd' ? summary.total_cost_usd : summary.total_tokens
    const weights = buckets.map((_, i) => 0.6 + 0.8 * Math.abs(Math.sin(i * 1.7 + 0.9)))
    const wSum = weights.reduce((a, b) => a + b, 0) || 1
    return { labels, totals: weights.map((w) => (totalVal * w) / wSum) }
  }

  const trendTotals = React.useMemo(buildTrendTotals, [timeRange, unitMode, summary, modelBreakdowns, projectBreakdowns])

  const trendEntities = React.useMemo(() => {
    const metric = (v: { total_tokens: number; cost_usd: number | null }) =>
      unitMode === 'usd' ? (v.cost_usd ?? 0) : v.total_tokens
    if (trendGroup === 'model') {
      return modelBreakdowns
        .map((m) => ({ name: m.model_name || m.name || 'Model', total: metric(m) }))
        .sort((a, b) => b.total - a.total)
        .slice(0, trendTopN)
    }
    return projectBreakdowns
      .map((p) => ({ name: p.project_name, total: metric(p) }))
      .sort((a, b) => b.total - a.total)
      .slice(0, trendTopN)
  }, [trendGroup, trendTopN, modelBreakdowns, projectBreakdowns, unitMode])

  const TREND_COLORS = ['#0b57d0', '#137333', '#b06000', '#7e22ce', '#b3261e', '#0f766e', '#0369a1', '#be185d', '#c2410c', '#475569']

  // Subagent aggregation calculation
  const totalSimPromptTokens = simOrchestratorTokens + simSubagentsCount * simSubagentAvgTokens
  const totalSimCachedTokens = Math.round(totalSimPromptTokens * (simCachedRatio / 100))
  const totalSimOutputTokens = Math.round(1800 * (1 + simSubagentsCount * 0.8))
  const totalSimTokens = totalSimPromptTokens + totalSimOutputTokens
  const simCostWithCache =
    (totalSimPromptTokens - totalSimCachedTokens) * (1.25 / 1000000) +
    totalSimCachedTokens * (0.3125 / 1000000) +
    totalSimOutputTokens * (5.0 / 1000000)
  const simCostWithoutCache =
    totalSimPromptTokens * (1.25 / 1000000) + totalSimOutputTokens * (5.0 / 1000000)
  const simSavedCost = simCostWithoutCache - simCostWithCache

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
              {/* Unit Switcher: USD vs Tokens */}
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
          gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
          gap: '14px',
        }}
      >
        {/* Total Cost / Tokens */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '18px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
          }}
        >
          <div>
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              {unitMode === 'usd' ? 'Total Cost (Spend)' : 'Total Tokens'}
            </span>
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginTop: '8px' }}>
            {unitMode === 'usd' ? `$${(summary.total_cost_usd ?? 0).toFixed(2)}` : formatTokens(summary.total_tokens)}
          </div>
          <div style={{ fontSize: '12px', color: '#137333', marginTop: '4px', display: 'flex', alignItems: 'center', gap: '4px' }}>
            <TrendingDown size={14} />
            <span>Prompt caching saved {formatCost(summary.saved_cost_usd)}</span>
          </div>
        </div>

        {/* Prompt Input vs Cached Tokens */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '16px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
          }}
        >
          <div>
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Cached Input Ratio
            </span>
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginTop: '8px' }}>
            {summary.input_tokens > 0 ? (((summary.cached_input_tokens || 0) / summary.input_tokens) * 100).toFixed(1) : '0.0'}%
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
            {formatTokens(summary.cached_input_tokens)} of {formatTokens(summary.input_tokens)} prompt tokens cached
          </div>
        </div>

        {/* Avg TPS Speed */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '16px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
          }}
        >
          <div>
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Average Generation Speed
            </span>
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginTop: '8px' }}>
            {(summary.avg_tps ?? 0).toFixed(1)} <span style={{ fontSize: '15px', fontWeight: 500 }}>TPS</span>
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
            {modelBreakdowns.length > 0
              ? (() => {
                  const fastest = [...modelBreakdowns].sort((a, b) => (b.avg_tps || 0) - (a.avg_tps || 0))[0]
                  return fastest && (fastest.avg_tps || 0) > 0
                    ? `Peak ${(fastest.avg_tps || 0).toFixed(1)} TPS on ${fastest.model_name || fastest.name}`
                    : `Measured across ${modelBreakdowns.length} active model${modelBreakdowns.length === 1 ? '' : 's'}`
                })()
              : 'Measured from real session transcripts'}
          </div>
        </div>

        {/* Total Prompts & Requests */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '16px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
          }}
        >
          <div>
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Completed Chat Turns
            </span>
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginTop: '8px' }}>
            {(summary.requests_count ?? 0).toLocaleString()}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Across {projectBreakdowns.length} active project{projectBreakdowns.length === 1 ? '' : 's'}
          </div>
        </div>
      </div>

      {/* Usage Trend Over Time */}
      {(() => {
        const totals = trendTotals.totals
        const n = totals.length
        const grandTotal = unitMode === 'usd' ? summary.total_cost_usd : summary.total_tokens
        const series = trendEntities.map((e, i) => ({
          name: e.name,
          total: e.total,
          color: TREND_COLORS[i % TREND_COLORS.length],
          share: grandTotal > 0 ? e.total / grandTotal : 0,
        }))
        const maxVal = Math.max(...totals, 1e-9)
        const W = 1000
        const H = 200
        const px = (i: number) => (n <= 1 ? W / 2 : (i / (n - 1)) * W)
        const py = (v: number) => H - 12 - (v / maxVal) * (H - 24)
        const pts = (vals: number[]) => vals.map((v, i) => `${px(i).toFixed(1)},${py(v).toFixed(1)}`).join(' ')
        const tickEvery = Math.max(1, Math.ceil(n / 8))
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
                per {timeRange === '24h' ? 'hour' : 'day'} · {unitMode === 'usd' ? 'USD' : 'tokens'}
              </span>
            </div>

            <svg
              viewBox={`0 0 ${W} ${H}`}
              style={{ width: '100%', height: '200px', display: 'block' }}
              preserveAspectRatio="none"
            >
              {/* horizontal grid */}
              {[0.25, 0.5, 0.75].map((f) => (
                <line
                  key={f}
                  x1={0}
                  x2={W}
                  y1={py(maxVal * f)}
                  y2={py(maxVal * f)}
                  stroke="#f1f3f4"
                  strokeWidth={1}
                />
              ))}
              {/* per-entity trend lines (below the total) */}
              {series.map((s) => (
                <polyline
                  key={s.name}
                  points={pts(totals.map((t) => t * s.share))}
                  fill="none"
                  stroke={s.color}
                  strokeWidth={2}
                  vectorEffect="non-scaling-stroke"
                  strokeLinejoin="round"
                  strokeLinecap="round"
                  opacity={0.9}
                />
              ))}
              {/* persistent grey daily usage line */}
              <polyline
                points={pts(totals)}
                fill="none"
                stroke="#9aa0a6"
                strokeWidth={2.5}
                vectorEffect="non-scaling-stroke"
                strokeLinejoin="round"
                strokeLinecap="round"
              />
            </svg>

            {/* x-axis ticks */}
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
              {trendTotals.labels.filter((_, i) => i % tickEvery === 0 || i === n - 1).map((l, i) => (
                <span key={`${l}-${i}`}>{l}</span>
              ))}
            </div>

            {/* legend */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginTop: '12px', flexWrap: 'wrap' }}>
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)' }}>
                <span style={{ width: '14px', height: '3px', borderRadius: '2px', backgroundColor: '#9aa0a6' }} />
                Total
              </span>
              {series.length === 0 ? (
                <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                  No {trendGroup} usage recorded in this horizon yet.
                </span>
              ) : (
                series.map((s) => (
                  <span key={s.name} style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text)' }}>
                    <span style={{ width: '14px', height: '3px', borderRadius: '2px', backgroundColor: s.color }} />
                    {s.name} <span style={{ color: 'var(--text-muted)' }}>{formatMetric(s.total)}</span>
                  </span>
                ))
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
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
              Models
            </h3>
            <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Canonicalized across thinking levels
            </span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            {modelBreakdowns.length === 0 ? (
              <div style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                No model token usage recorded yet. Start interacting with agent models to see consumption breakdowns.
              </div>
            ) : (
              modelBreakdowns.map((m) => {
                const pct = summary.total_tokens > 0 ? (m.total_tokens / summary.total_tokens) * 100 : 0
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
                        {formatMetric(unitMode === 'usd' ? m.cost_usd : m.total_tokens)}{' '}
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
                      <span>Speed: {m.avg_tps} TPS</span>
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
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
              Projects
            </h3>
            <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Across {projectBreakdowns.length} Project{projectBreakdowns.length === 1 ? '' : 's'}
            </span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            {projectBreakdowns.length === 0 ? (
              <div style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                No workspaces configured yet.
              </div>
            ) : (
              projectBreakdowns.map((p) => {
                const pct = summary.total_tokens > 0 ? (p.total_tokens / summary.total_tokens) * 100 : 0
                return (
                  <div key={p.project_name}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', marginBottom: '4px' }}>
                      <span style={{ fontWeight: 600, color: 'var(--text)' }}>{p.project_name}</span>
                      <span style={{ fontWeight: 600, color: 'var(--primary)' }}>
                        {formatMetric(unitMode === 'usd' ? p.cost_usd : p.total_tokens)}{' '}
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
              {pricingList.length === 0 ? (
                <tr>
                  <td colSpan={8} style={{ padding: '28px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                    No models in pricing registry. Click Sync Prices to fetch available Antigravity and Custom models.
                  </td>
                </tr>
              ) : (
                pricingList.map((pr) => {
                  const provLower = (pr.provider || '').toLowerCase()
                  return (
                    <tr key={`${pr.canonical_id || pr.model_id}-${pr.internal_id}`} style={{ borderBottom: '1px solid #f1f3f4' }}>
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
                          <span style={{ color: '#b06000', fontSize: '12px', fontWeight: 500 }}>null (Manual Entry)</span>
                        ) : (
                          `$${pr.input_price_per_m.toFixed(3)}`
                        )}
                      </td>
                      <td style={{ padding: '10px 12px', color: '#137333', fontWeight: 600, textAlign: 'center' }}>
                        {pr.cached_input_price_per_m === null || pr.cached_input_price_per_m === undefined ? (
                          <span style={{ color: 'var(--text-muted)', fontSize: '12px', fontWeight: 400 }}>null</span>
                        ) : (
                          `$${pr.cached_input_price_per_m.toFixed(4)}`
                        )}
                      </td>
                      <td style={{ padding: '10px 12px', color: 'var(--text)', textAlign: 'center' }}>
                        {pr.output_price_per_m === null || pr.output_price_per_m === undefined ? (
                          <span style={{ color: '#b06000', fontSize: '12px', fontWeight: 500 }}>null (Manual Entry)</span>
                        ) : (
                          `$${pr.output_price_per_m.toFixed(2)}`
                        )}
                      </td>
                      <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                        <span
                          style={{
                            fontSize: '11px',
                            color:
                              pr.source === 'provider'
                                ? '#137333'
                                : pr.source === 'third_party' || pr.source === 'api'
                                ? 'var(--primary)'
                                : pr.source === 'manual'
                                ? '#7e22ce'
                                : '#b06000',
                            fontWeight: 600,
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {pr.source === 'provider'
                            ? 'Provider Official'
                            : pr.source === 'third_party' || pr.source === 'api'
                            ? '3rd-Party Backup'
                            : pr.source === 'manual'
                            ? 'Manual Override'
                            : 'Null — Manual Required'}
                        </span>
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
                          {pr.classification === 'custom' && (
                            <button
                              onClick={() => handleDeletePricingModel(pr)}
                              title="Delete custom/outdated model price and usage history"
                              style={{
                                backgroundColor: 'transparent',
                                border: '1px solid #fad2cf',
                                borderRadius: '6px',
                                padding: '4px 8px',
                                fontSize: '12px',
                                color: '#b3261e',
                                cursor: 'pointer',
                                display: 'inline-flex',
                                alignItems: 'center',
                                gap: '4px',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              <Trash2 size={12} />
                              <span>Delete</span>
                            </button>
                          )}
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

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '20px' }}>
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
      )}

      {/* ============================================================ */}
      {/* TAB 1: TELEMETRY & LOGS */}
      {/* ============================================================ */}
      {activeTab === 1 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* 3. In-Chat Response Token & TPS Display Simulator (Multi-Agent Subagent Aggregator) */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '20px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '16px' }}>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
                    In-Chat Token & TPS Telemetry
                  </h3>
                </div>
                <p style={{ margin: '4px 0 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
                  Injects real-time token and TPS telemetry below response bubbles. Aggregates concurrent subagent tokens automatically.
                </p>
              </div>
            </div>

            {/* Interactive Controls for Simulator */}
            <div
              style={{
                backgroundColor: 'var(--tonal)',
                borderRadius: '8px',
                padding: '16px',
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
                gap: '16px',
                marginBottom: '16px',
              }}
            >
              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Orchestrator Prompt Tokens: {simOrchestratorTokens.toLocaleString()}
                </label>
                <input
                  type="range"
                  min={1000}
                  max={15000}
                  step={500}
                  value={simOrchestratorTokens}
                  onChange={(e) => setSimOrchestratorTokens(Number(e.target.value))}
                  style={{ width: '100%', accentColor: 'var(--primary)' }}
                />
              </div>

              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Subagents Spawned: {simSubagentsCount} agents
                </label>
                <input
                  type="range"
                  min={0}
                  max={5}
                  step={1}
                  value={simSubagentsCount}
                  onChange={(e) => setSimSubagentsCount(Number(e.target.value))}
                  style={{ width: '100%', accentColor: 'var(--primary)' }}
                />
              </div>

              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Avg Subagent Tokens: {simSubagentAvgTokens.toLocaleString()}
                </label>
                <input
                  type="range"
                  min={2000}
                  max={20000}
                  step={1000}
                  value={simSubagentAvgTokens}
                  onChange={(e) => setSimSubagentAvgTokens(Number(e.target.value))}
                  style={{ width: '100%', accentColor: 'var(--primary)' }}
                />
              </div>

              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Prompt Cache Hit Ratio: {simCachedRatio}%
                </label>
                <input
                  type="range"
                  min={0}
                  max={95}
                  step={5}
                  value={simCachedRatio}
                  onChange={(e) => setSimCachedRatio(Number(e.target.value))}
                  style={{ width: '100%', accentColor: '#137333' }}
                />
              </div>
            </div>

            {/* Live In-Chat Message Preview */}
            <div
              style={{
                border: '1px solid var(--border)',
                borderRadius: '12px',
                backgroundColor: '#ffffff',
                padding: '16px',
              }}
            >
              {/* Agent Message Header */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '10px' }}>
                <div
                  style={{
                    width: '26px',
                    height: '26px',
                    borderRadius: '50%',
                    backgroundColor: 'var(--primary)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: '#ffffff',
                    fontSize: '13px',
                    fontWeight: 700,
                  }}
                >
                  <Sparkles size={14} color="#ffffff" />
                </div>
                <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Antigravity Agent ({modelBreakdowns[0]?.model_name || modelBreakdowns[0]?.name || pricingList[0]?.model_name || pricingList[0]?.name || 'Active Model'})
                </span>
                <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                  Project: {projectBreakdowns[0]?.project_name || 'Antigravity Swiss Knife'}
                </span>
              </div>

              {/* Response Text Content */}
              <div style={{ fontSize: '13px', lineHeight: 1.5, color: 'var(--text)', marginBottom: '16px' }}>
                I have analyzed your request and refactored the auxiliary panels to include the new Feature Plugins and Token Monitor sections.
                {simSubagentsCount > 0 && (
                  <span style={{ display: 'flex', alignItems: 'center', gap: '4px', marginTop: '6px', color: 'var(--text-muted)', fontStyle: 'italic' }}>
                    <CornerDownRight size={11} style={{ fontStyle: 'normal', flexShrink: 0 }} />
                    <span>Orchestrated {simSubagentsCount} parallel subagents (`code-review`, `research`) to verify API signatures and sandbox isolation.</span>
                  </span>
                )}
              </div>

              {/* THE INJECTED TELEMETRY FOOTER BADGE */}
              <div
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                  gap: '12px',
                  backgroundColor: '#f8f9fa',
                  border: '1px solid #e0e0e0',
                  borderRadius: '8px',
                  padding: '6px 12px',
                  fontSize: '12px',
                  color: '#3c4043',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '5px', fontWeight: 600, color: 'var(--primary)' }}>
                  <Zap size={13} />
                  <span>{formatTokens(totalSimTokens)} tokens</span>
                </div>

                <span style={{ color: '#dadce0' }}>•</span>

                <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <span>Prompt: <b>{formatTokens(totalSimPromptTokens)}</b></span>
                  <span style={{ color: '#137333', fontSize: '11px' }}>
                    (Cached: {formatTokens(totalSimCachedTokens)} / {simCachedRatio}%)
                  </span>
                  <span>| Output: <b>{formatTokens(totalSimOutputTokens)}</b></span>
                </div>

                <span style={{ color: '#dadce0' }}>•</span>

                <div style={{ display: 'flex', alignItems: 'center', gap: '4px', color: '#b06000' }}>
                  <Activity size={13} />
                  <span><b>{summary.avg_tps > 0 ? summary.avg_tps : 76.2} TPS</b></span>
                </div>

                <span style={{ color: '#dadce0' }}>•</span>

                <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <span>Cost: <b>{formatCost(simCostWithCache)}</b></span>
                  <span style={{ color: '#137333', fontSize: '11px' }}>(saved {formatCost(simSavedCost)})</span>
                </div>

                {simSubagentsCount > 0 && (
                  <>
                    <span style={{ color: '#dadce0' }}>•</span>
                    <span
                      style={{
                        backgroundColor: '#e8f0fe',
                        color: 'var(--primary)',
                        borderRadius: '4px',
                        padding: '1px 6px',
                        fontSize: '11px',
                        fontWeight: 600,
                      }}
                    >
                      Aggregated 1 Parent + {simSubagentsCount} Subagents
                    </span>
                  </>
                )}
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
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <div>
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
              Recent Agent Session Telemetry Log
            </h3>
            <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Real-time audit log of executed chat turns extracted from local session transcripts.
            </span>
          </div>
          <button
            onClick={() => {
              const csvContent =
                'data:text/csv;charset=utf-8,' +
                ['Timestamp,TurnID,Project,Model,Type,Input,Cached,Output,TPS,Cost']
                  .concat(
                    telemetryEvents.map(
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

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                <th style={{ padding: '8px 12px' }}>Time</th>
                <th style={{ padding: '8px 12px' }}>Session / Project</th>
                <th style={{ padding: '8px 12px' }}>Model</th>
                <th style={{ padding: '8px 12px' }}>Tokens (In / Cache / Out)</th>
                <th style={{ padding: '8px 12px' }}>Speed</th>
                <th style={{ padding: '8px 12px' }}>Cost</th>
                <th style={{ padding: '8px 12px' }}>Type</th>
                <th style={{ padding: '8px 12px' }}>Status</th>
              </tr>
            </thead>
            <tbody>
              {telemetryEvents.length === 0 ? (
                <tr>
                  <td colSpan={8} style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                    No live telemetry sessions recorded. When queries execute, token counts and TPS are logged here.
                  </td>
                </tr>
              ) : (
                telemetryEvents.map((ev) => (
                  <tr key={ev.id} style={{ borderBottom: '1px solid #f1f3f4' }}>
                    <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{ev.timestamp}</td>
                    <td style={{ padding: '10px 12px' }}>
                      <div style={{ fontWeight: 600, color: 'var(--text)' }}>{ev.project}</div>
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{ev.id}</div>
                    </td>
                    <td style={{ padding: '10px 12px', fontWeight: 500, color: 'var(--text)' }}>{ev.model || ev.canonical_id || ev.model_id || 'Model'}</td>
                    <td style={{ padding: '10px 12px' }}>
                      <span>{formatTokens(ev.input_tokens)} in</span> •{' '}
                      <span style={{ color: '#137333' }}>{formatTokens(ev.cached_tokens)} cache</span> •{' '}
                      <span>{formatTokens(ev.output_tokens)} out</span>
                    </td>
                    <td style={{ padding: '10px 12px', fontWeight: 600, color: '#b06000' }}>{ev.tps} TPS</td>
                    <td style={{ padding: '10px 12px', fontWeight: 600, color: 'var(--text)' }}>
                      {formatCost(ev.cost_usd)}
                    </td>
                    <td style={{ padding: '10px 12px' }}>
                      <span
                        style={{
                          backgroundColor: ev.classification === 'native' ? '#e8f0fe' : '#f3e8ff',
                          color: ev.classification === 'native' ? 'var(--primary)' : '#7e22ce',
                          padding: '2px 6px',
                          borderRadius: '8px',
                          fontWeight: 600,
                          textTransform: 'capitalize',
                        }}
                      >
                        {ev.classification || 'native'}
                      </span>
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
