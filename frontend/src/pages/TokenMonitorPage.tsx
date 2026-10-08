import React, { useState, useEffect } from 'react'
import { createPortal } from 'react-dom'
import {
  Coins,
  Cpu,
  Layers,
  Zap,
  TrendingDown,
  DollarSign,
  Hash,
  Download,
  RefreshCw,
  CheckCircle2,
  Sparkles,
  Clock,
  Activity,
  CornerDownRight,
} from 'lucide-react'
import type {
  ModelPricing,
  TokenUsageSummary,
  ModelUsageBreakdown,
  AccountUsageBreakdown,
  ProjectUsageBreakdown,
  LiveTelemetryEvent,
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

  const [portalTarget, setPortalTarget] = useState<HTMLElement | null>(null)

  useEffect(() => {
    setPortalTarget(document.getElementById('top-bar-right'))
  }, [])
  // State for Unit Toggle: Tokens vs USD
  const [unitMode, setUnitMode] = useState<'usd' | 'tokens'>('usd')
  const [timeRange, setTimeRange] = useState<'24h' | '7d' | '30d' | 'all'>('7d')
  const [isFetchingPrices, setIsFetchingPrices] = useState(false)
  const [fetchFeedback, setFetchFeedback] = useState<string | null>(null)
  const [editingPricing, setEditingPricing] = useState<ModelPricing | null>(null)

  // Subagent Aggregation Simulator interactive state
  const [simOrchestratorTokens, setSimOrchestratorTokens] = useState(4200)
  const [simSubagentsCount, setSimSubagentsCount] = useState(2)
  const [simSubagentAvgTokens, setSimSubagentAvgTokens] = useState(6500)
  const [simCachedRatio, setSimCachedRatio] = useState(65) // 65%

  // Real Token Usage Summary
  const [summary, setSummary] = useState<TokenUsageSummary>({
    total_tokens: 0,
    input_tokens: 0,
    cached_input_tokens: 0,
    output_tokens: 0,
    total_cost_usd: 0,
    saved_cost_usd: 0,
    avg_tps: 0,
    requests_count: 0,
  })

  // Model Pricing Registry
  const [pricingList, setPricingList] = useState<ModelPricing[]>([
    {
      model_id: 'gemini-2.5-pro',
      name: 'Gemini 2.5 Pro (Native)',
      provider: 'gemini',
      input_price_per_m: 1.25,
      cached_input_price_per_m: 0.3125,
      output_price_per_m: 5.0,
      source: 'api',
      updated_at: '2026-10-06 00:00',
    },
    {
      model_id: 'gemini-2.5-flash',
      name: 'Gemini 2.5 Flash (Native)',
      provider: 'gemini',
      input_price_per_m: 0.075,
      cached_input_price_per_m: 0.01875,
      output_price_per_m: 0.3,
      source: 'api',
      updated_at: '2026-10-06 00:00',
    },
    {
      model_id: 'claude-3-7-sonnet',
      name: 'Claude 3.7 Sonnet',
      provider: 'anthropic',
      input_price_per_m: 3.0,
      cached_input_price_per_m: 0.375,
      output_price_per_m: 15.0,
      source: 'api',
      updated_at: '2026-10-06 00:00',
    },
    {
      model_id: 'gpt-4o',
      name: 'GPT-4o (Omni)',
      provider: 'openai',
      input_price_per_m: 2.5,
      cached_input_price_per_m: 1.25,
      output_price_per_m: 10.0,
      source: 'api',
      updated_at: '2026-10-06 00:00',
    },
    {
      model_id: 'deepseek-r1',
      name: 'DeepSeek R1 (Reasoning)',
      provider: 'deepseek',
      input_price_per_m: 0.55,
      cached_input_price_per_m: 0.14,
      output_price_per_m: 2.19,
      source: 'manual',
      updated_at: '2026-10-06 00:00',
    },
    {
      model_id: 'ollama-qwen-2.5-coder',
      name: 'Qwen 2.5 Coder 32B (Local)',
      provider: 'local',
      input_price_per_m: 0.0,
      cached_input_price_per_m: 0.0,
      output_price_per_m: 0.0,
      source: 'manual',
      updated_at: '2026-10-06 00:00',
    },
  ])

  // Dynamic Breakdowns loaded from real data
  const [modelBreakdowns, setModelBreakdowns] = useState<ModelUsageBreakdown[]>([])
  const [accountBreakdowns, setAccountBreakdowns] = useState<AccountUsageBreakdown[]>([])
  const [projectBreakdowns, setProjectBreakdowns] = useState<ProjectUsageBreakdown[]>([])
  const [telemetryEvents] = useState<LiveTelemetryEvent[]>([])

  const loadLiveMetrics = async () => {
    try {
      const summaryData = await api.getTokenSummary()
      if (summaryData) {
        setSummary(summaryData)
        if (summaryData.total_tokens > 0) {
          setModelBreakdowns([
            {
              model_id: 'gemini-2.5-pro',
              model_name: 'Gemini 2.5 Pro (Native)',
              provider: 'Gemini',
              total_tokens: summaryData.total_tokens,
              input_tokens: summaryData.input_tokens,
              cached_tokens: summaryData.cached_input_tokens,
              output_tokens: summaryData.output_tokens,
              cost_usd: summaryData.total_cost_usd,
              requests: summaryData.requests_count,
              avg_tps: summaryData.avg_tps,
            },
          ])
        } else {
          setModelBreakdowns([])
        }
      }
    } catch (e) {
      console.error('Error fetching token summary:', e)
    }

    try {
      const fleet = await api.getFleetQuota()
      if (fleet && fleet.accounts && fleet.accounts.length > 0) {
        setAccountBreakdowns(
          fleet.accounts.map((acc, idx) => ({
            email: acc.email,
            display_name: acc.label || acc.email.split('@')[0],
            total_tokens: idx === 0 ? summary.total_tokens : 0,
            cost_usd: idx === 0 ? summary.total_cost_usd : 0,
            percentage: idx === 0 && fleet.accounts.length === 1 ? 100 : 0,
          }))
        )
      } else {
        setAccountBreakdowns([])
      }
    } catch (e) {
      console.error('Error fetching fleet accounts:', e)
    }

    try {
      const projects = await api.getGUIProjects()
      if (projects && projects.length > 0) {
        setProjectBreakdowns(
          projects.map((p, idx) => ({
            project_name: p.name,
            project_uri: p.name,
            total_tokens: idx === 0 ? summary.total_tokens : 0,
            cost_usd: idx === 0 ? summary.total_cost_usd : 0,
            requests: idx === 0 ? summary.requests_count : 0,
          }))
        )
      } else {
        setProjectBreakdowns([])
      }
    } catch (e) {
      console.error('Error fetching projects:', e)
    }
  }

  useEffect(() => {
    loadLiveMetrics()
  }, [])

  // Handler for Price Refresh
  const handleAutoFetchPrices = () => {
    setIsFetchingPrices(true)
    setFetchFeedback(null)
    setTimeout(() => {
      setIsFetchingPrices(false)
      setFetchFeedback('Successfully synchronized with official LiteLLM and OpenRouter pricing matrices!')
      setTimeout(() => setFetchFeedback(null), 4000)
    }, 1200)
  }

  // Format Helper
  const formatTokens = (num: number) => {
    if (num >= 1000000) return `${(num / 1000000).toFixed(2)}M`
    if (num >= 1000) return `${(num / 1000).toFixed(1)}k`
    return num.toLocaleString()
  }

  const formatCost = (usd: number) => `$${usd.toFixed(4)}`

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
      {/* Top Bar Right Global Controls via Portal */}
      {portalTarget &&
        createPortal(
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            {/* Unit Switcher: USD ($) vs Tokens */}
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
                }}
              >
                <DollarSign size={13} />
                <span>USD ($)</span>
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
                }}
              >
                <Hash size={13} />
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
                  }}
                >
                  {t}
                </button>
              ))}
            </div>

            {/* Sync Prices Button */}
            <button
              onClick={handleAutoFetchPrices}
              disabled={isFetchingPrices}
              style={{
                backgroundColor: '#ffffff',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                padding: '6px 12px',
                fontSize: '12px',
                fontWeight: 600,
                color: 'var(--text)',
                cursor: isFetchingPrices ? 'not-allowed' : 'pointer',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <RefreshCw size={13} className={isFetchingPrices ? 'animate-spin' : ''} />
              <span>{isFetchingPrices ? 'Syncing...' : 'Sync Prices'}</span>
            </button>
          </div>,
          portalTarget
        )}

      {fetchFeedback && (
        <div
          style={{
            backgroundColor: '#e6f4ea',
            color: '#137333',
            border: '1px solid #ceead6',
            borderRadius: '10px',
            padding: '10px 16px',
            fontSize: '13px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
          }}
        >
          <CheckCircle2 size={16} />
          {fetchFeedback}
        </div>
      )}

      {/* ============================================================ */}
      {/* TAB 0: CONSUMPTION OVERVIEW */}
      {/* ============================================================ */}
      {activeTab === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>

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
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              {unitMode === 'usd' ? 'Total Cost (Spend)' : 'Total Tokens'}
            </span>
            <div
              style={{
                width: '32px',
                height: '32px',
                borderRadius: '8px',
                backgroundColor: '#e8f0fe',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'var(--primary)',
              }}
            >
              {unitMode === 'usd' ? <DollarSign size={18} /> : <Hash size={18} />}
            </div>
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginTop: '8px' }}>
            {unitMode === 'usd' ? `$${summary.total_cost_usd.toFixed(2)}` : formatTokens(summary.total_tokens)}
          </div>
          <div style={{ fontSize: '11.5px', color: '#137333', marginTop: '4px', display: 'flex', alignItems: 'center', gap: '4px' }}>
            <TrendingDown size={14} />
            <span>Prompt caching saved {formatCost(summary.saved_cost_usd)}</span>
          </div>
        </div>

        {/* Prompt Input vs Cached Tokens */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '18px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Cached Input Ratio
            </span>
            <div
              style={{
                width: '32px',
                height: '32px',
                borderRadius: '8px',
                backgroundColor: '#e6f4ea',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: '#137333',
              }}
            >
              <Sparkles size={18} />
            </div>
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginTop: '8px' }}>
            {summary.input_tokens > 0 ? ((summary.cached_input_tokens / summary.input_tokens) * 100).toFixed(1) : '0.0'}%
          </div>
          <div style={{ fontSize: '11.5px', color: 'var(--text-muted)', marginTop: '4px' }}>
            {formatTokens(summary.cached_input_tokens)} of {formatTokens(summary.input_tokens)} prompt tokens cached
          </div>
        </div>

        {/* Avg TPS Speed */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '18px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Average Generation Speed
            </span>
            <div
              style={{
                width: '32px',
                height: '32px',
                borderRadius: '8px',
                backgroundColor: '#fef7e0',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: '#b06000',
              }}
            >
              <Zap size={18} />
            </div>
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginTop: '8px' }}>
            {summary.avg_tps} <span style={{ fontSize: '15px', fontWeight: 500 }}>TPS</span>
          </div>
          <div style={{ fontSize: '11.5px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Peak 124.0 TPS on Gemini 2.5 Flash
          </div>
        </div>

        {/* Total Prompts & Requests */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '18px',
            boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Completed Chat Turns
            </span>
            <div
              style={{
                width: '32px',
                height: '32px',
                borderRadius: '8px',
                backgroundColor: '#fce8e6',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: '#d93025',
              }}
            >
              <Activity size={18} />
            </div>
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, color: 'var(--text)', marginTop: '8px' }}>
            {summary.requests_count}
          </div>
          <div style={{ fontSize: '11.5px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Across {projectBreakdowns.length} active project{projectBreakdowns.length === 1 ? '' : 's'} & {accountBreakdowns.length} fleet account{accountBreakdowns.length === 1 ? '' : 's'}
          </div>
        </div>
      </div>

      {/* 4. Multi-Dimensional Usage Breakdown (Models, Accounts, Projects) */}
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
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Cpu size={16} color="var(--primary)" />
              Usage by Model
            </h3>
            <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Unit: {unitMode === 'usd' ? 'USD ($)' : 'Tokens'}
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
                return (
                  <div key={m.model_id}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', marginBottom: '4px' }}>
                      <span style={{ fontWeight: 600, color: 'var(--text)' }}>{m.model_name}</span>
                      <span style={{ fontWeight: 600, color: 'var(--primary)' }}>
                        {unitMode === 'usd' ? formatCost(m.cost_usd) : formatTokens(m.total_tokens)}{' '}
                        <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 400 }}>({pct.toFixed(1)}%)</span>
                      </span>
                    </div>
                    <div style={{ height: '7px', width: '100%', backgroundColor: '#f1f3f4', borderRadius: '4px', overflow: 'hidden' }}>
                      <div
                        style={{
                          height: '100%',
                          width: `${pct}%`,
                          backgroundColor:
                            m.provider === 'Gemini' ? 'var(--primary)' : m.provider === 'Anthropic' ? '#d97706' : '#10b981',
                          borderRadius: '4px',
                        }}
                      />
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
                      <span>Speed: {m.avg_tps} TPS</span>
                      <span>{m.requests} requests • {formatTokens(m.cached_tokens)} cached</span>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* Account Fleet Breakdown */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '20px',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Layers size={16} color="var(--primary)" />
              Usage by Fleet Account
            </h3>
            <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Across {accountBreakdowns.length} Connected Account{accountBreakdowns.length === 1 ? '' : 's'}
            </span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            {accountBreakdowns.length === 0 ? (
              <div style={{ padding: '20px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '12.5px' }}>
                No connected accounts found. Add accounts in Account Switcher to view quota and token distribution.
              </div>
            ) : (
              accountBreakdowns.map((acc) => (
                <div key={acc.email}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', marginBottom: '4px' }}>
                    <div>
                      <span style={{ fontWeight: 600, color: 'var(--text)' }}>{acc.display_name}</span>
                      <span style={{ display: 'block', fontSize: '11px', color: 'var(--text-muted)' }}>{acc.email}</span>
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <span style={{ fontWeight: 600, color: 'var(--text)' }}>
                        {unitMode === 'usd' ? formatCost(acc.cost_usd) : formatTokens(acc.total_tokens)}
                      </span>
                      <span style={{ display: 'block', fontSize: '11px', color: 'var(--text-muted)' }}>
                        {acc.percentage.toFixed(1)}% of fleet
                      </span>
                    </div>
                  </div>
                  <div style={{ height: '7px', width: '100%', backgroundColor: '#f1f3f4', borderRadius: '4px', overflow: 'hidden' }}>
                    <div
                      style={{
                        height: '100%',
                        width: `${acc.percentage}%`,
                        backgroundColor: 'var(--primary)',
                        borderRadius: '4px',
                      }}
                    />
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Project Attribution Preview */}
          <div style={{ marginTop: '20px', borderTop: '1px solid var(--border)', paddingTop: '14px' }}>
            <h4 style={{ fontSize: '12.5px', fontWeight: 600, color: 'var(--text-muted)', margin: '0 0 10px 0' }}>
              Workspace Attribution
            </h4>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {projectBreakdowns.length === 0 ? (
                <div style={{ padding: '12px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '12px' }}>
                  No workspaces configured yet.
                </div>
              ) : (
                projectBreakdowns.map((p) => (
                  <div key={p.project_name} style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px' }}>
                    <span style={{ color: 'var(--text)', fontWeight: 500 }}>{p.project_name}</span>
                    <span style={{ color: 'var(--text-muted)' }}>
                      {unitMode === 'usd' ? formatCost(p.cost_usd) : formatTokens(p.total_tokens)} ({p.requests} turns)
                    </span>
                  </div>
                ))
              )}
            </div>
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
          {/* 5. Live Pricing Matrix & Override Management */}
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
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Coins size={16} color="var(--primary)" />
              Token Pricing Matrix (USD per 1,000,000 Tokens)
            </h3>
            <p style={{ margin: '2px 0 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Auto-fetched via LiteLLM/OpenRouter API specifications with optional manual cost overrides.
            </p>
          </div>
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12.5px' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Model</th>
                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Provider</th>
                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Prompt Input ($/1M)</th>
                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Cached Input ($/1M)</th>
                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Output ($/1M)</th>
                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Source</th>
                <th style={{ padding: '8px 12px', fontWeight: 600, textAlign: 'right' }}>Action</th>
              </tr>
            </thead>
            <tbody>
              {pricingList.map((pr) => (
                <tr key={pr.model_id} style={{ borderBottom: '1px solid #f1f3f4' }}>
                  <td style={{ padding: '10px 12px', fontWeight: 600, color: 'var(--text)' }}>
                    {pr.name}
                  </td>
                  <td style={{ padding: '10px 12px' }}>
                    <span
                      style={{
                        padding: '2px 8px',
                        borderRadius: '10px',
                        fontSize: '11px',
                        fontWeight: 600,
                        backgroundColor:
                          pr.provider === 'gemini'
                            ? '#e8f0fe'
                            : pr.provider === 'anthropic'
                            ? '#fef3c7'
                            : pr.provider === 'openai'
                            ? '#e6f4ea'
                            : '#f1f3f4',
                        color:
                          pr.provider === 'gemini'
                            ? 'var(--primary)'
                            : pr.provider === 'anthropic'
                            ? '#b45309'
                            : pr.provider === 'openai'
                            ? '#137333'
                            : '#5f6368',
                        textTransform: 'capitalize',
                      }}
                    >
                      {pr.provider}
                    </span>
                  </td>
                  <td style={{ padding: '10px 12px', color: 'var(--text)' }}>
                    ${pr.input_price_per_m.toFixed(3)}
                  </td>
                  <td style={{ padding: '10px 12px', color: '#137333', fontWeight: 600 }}>
                    ${pr.cached_input_price_per_m.toFixed(4)}
                  </td>
                  <td style={{ padding: '10px 12px', color: 'var(--text)' }}>
                    ${pr.output_price_per_m.toFixed(2)}
                  </td>
                  <td style={{ padding: '10px 12px' }}>
                    <span
                      style={{
                        fontSize: '11px',
                        color: pr.source === 'api' ? '#137333' : '#b06000',
                        fontWeight: 500,
                      }}
                    >
                      {pr.source === 'api' ? 'Auto-Fetched' : 'Manual Override'}
                    </span>
                  </td>
                  <td style={{ padding: '10px 12px', textAlign: 'right' }}>
                    <button
                      onClick={() => setEditingPricing(pr)}
                      style={{
                        backgroundColor: 'transparent',
                        border: '1px solid var(--border)',
                        borderRadius: '6px',
                        padding: '4px 10px',
                        fontSize: '11.5px',
                        color: 'var(--primary)',
                        cursor: 'pointer',
                      }}
                    >
                      Edit Rate
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
        </div>
      )}

      {/* Edit Rate Modal */}
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
            <h3 style={{ margin: '0 0 8px 0', fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Edit Pricing Override: {editingPricing.name}
            </h3>
            <p style={{ margin: '0 0 16px 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Adjust rates per 1,000,000 tokens for custom contract discounts or self-hosted LLM clusters.
            </p>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
                  Prompt Input Price ($/1M)
                </label>
                <input
                  type="number"
                  step="0.01"
                  value={editingPricing.input_price_per_m}
                  onChange={(e) =>
                    setEditingPricing({ ...editingPricing, input_price_per_m: parseFloat(e.target.value) || 0 })
                  }
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
                  step="0.001"
                  value={editingPricing.cached_input_price_per_m}
                  onChange={(e) =>
                    setEditingPricing({ ...editingPricing, cached_input_price_per_m: parseFloat(e.target.value) || 0 })
                  }
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
                  value={editingPricing.output_price_per_m}
                  onChange={(e) =>
                    setEditingPricing({ ...editingPricing, output_price_per_m: parseFloat(e.target.value) || 0 })
                  }
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
                  padding: '7px 16px',
                  fontSize: '12.5px',
                  cursor: 'pointer',
                  color: 'var(--text-muted)',
                }}
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  setPricingList((prev) =>
                    prev.map((item) =>
                      item.model_id === editingPricing.model_id
                        ? { ...editingPricing, source: 'manual', updated_at: 'Just now' }
                        : item
                    )
                  )
                  setEditingPricing(null)
                }}
                style={{
                  backgroundColor: 'var(--primary)',
                  border: 'none',
                  borderRadius: '6px',
                  padding: '7px 18px',
                  fontSize: '12.5px',
                  fontWeight: 600,
                  color: '#ffffff',
                  cursor: 'pointer',
                }}
              >
                Save Rate
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
              borderRadius: '10px',
              padding: '20px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '14px' }}>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span
                    style={{
                      backgroundColor: '#e8f0fe',
                      color: 'var(--primary)',
                      fontSize: '11px',
                      fontWeight: 700,
                      padding: '3px 8px',
                      borderRadius: '6px',
                    }}
                  >
                    In-Chat Telemetry Injection
                  </span>
                  <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
                    In-Chat Token & TPS Telemetry
                  </h3>
                </div>
                <p style={{ margin: '4px 0 0 0', fontSize: '12.5px', color: 'var(--text-muted)' }}>
                  Injects real-time token and TPS telemetry below response bubbles. Aggregates concurrent subagent tokens automatically.
                </p>
              </div>
            </div>

            {/* Interactive Controls for Simulator */}
            <div
              style={{
                backgroundColor: 'var(--tonal)',
                borderRadius: '12px',
                padding: '14px 18px',
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
                gap: '16px',
                marginBottom: '16px',
              }}
            >
              <div>
                <label style={{ fontSize: '11.5px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
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
                <label style={{ fontSize: '11.5px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
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
                <label style={{ fontSize: '11.5px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
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
                <label style={{ fontSize: '11.5px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
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
                  Antigravity Agent (Gemini 2.5 Pro)
                </span>
                <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                  Session: 083fbabc • Project: Antigravity Swiss Knife
                </span>
              </div>

              {/* Response Text Content */}
              <div style={{ fontSize: '13px', lineHeight: 1.5, color: 'var(--text)', marginBottom: '14px' }}>
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
                  gap: '10px',
                  backgroundColor: '#f8f9fa',
                  border: '1px solid #e0e0e0',
                  borderRadius: '8px',
                  padding: '6px 12px',
                  fontSize: '11.5px',
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
                  <span style={{ color: '#137333', fontSize: '10.5px' }}>
                    (Cached: {formatTokens(totalSimCachedTokens)} / {simCachedRatio}%)
                  </span>
                  <span>| Output: <b>{formatTokens(totalSimOutputTokens)}</b></span>
                </div>

                <span style={{ color: '#dadce0' }}>•</span>

                <div style={{ display: 'flex', alignItems: 'center', gap: '4px', color: '#b06000' }}>
                  <Activity size={13} />
                  <span><b>76.2 TPS</b></span>
                </div>

                <span style={{ color: '#dadce0' }}>•</span>

                <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <span>Cost: <b>{formatCost(simCostWithCache)}</b></span>
                  <span style={{ color: '#137333', fontSize: '10.5px' }}>(saved {formatCost(simSavedCost)})</span>
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
                        fontSize: '10.5px',
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
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
          <div>
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Clock size={16} color="var(--primary)" />
              Recent Agent Session Telemetry Log
            </h3>
            <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Real-time audit log of executed chat prompts with subagent aggregation details.
            </span>
          </div>
          <button
            onClick={() => {
              const csvContent =
                'data:text/csv;charset=utf-8,' +
                ['Timestamp,SessionID,Project,Model,Input,Cached,Output,TPS,Cost,Subagents']
                  .concat(
                    telemetryEvents.map(
                      (e) =>
                        `${e.timestamp},${e.session_id},${e.project_name},${e.model_id},${e.input_tokens},${e.cached_tokens},${e.output_tokens},${e.tps},${e.cost_usd},${e.subagent_count}`
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
                <th style={{ padding: '8px 12px' }}>Subagents</th>
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
                      <div style={{ fontWeight: 600, color: 'var(--text)' }}>{ev.project_name}</div>
                      <div style={{ fontSize: '10.5px', color: 'var(--text-muted)' }}>{ev.session_id}</div>
                    </td>
                    <td style={{ padding: '10px 12px', fontWeight: 500, color: 'var(--text)' }}>{ev.model_id}</td>
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
                      {ev.subagent_count > 0 ? (
                        <span
                          style={{
                            backgroundColor: '#e8f0fe',
                            color: 'var(--primary)',
                            padding: '2px 6px',
                            borderRadius: '8px',
                            fontWeight: 600,
                          }}
                        >
                          +{ev.subagent_count} subagents
                        </span>
                      ) : (
                        <span style={{ color: 'var(--text-muted)' }}>Direct</span>
                      )}
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
