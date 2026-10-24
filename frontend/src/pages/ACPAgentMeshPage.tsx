import React, { useState, useEffect } from 'react'
import {
  Activity,
  CheckCircle2,
  ArrowRight,
  RefreshCw,
  Plus,
  Pencil,
  Trash2,
  Search,
} from 'lucide-react'
import type {
  AcpAgentInput,
  AcpAgentInstance,
  AcpDetectCandidate,
  AcpDetectResponse,
  AcpHandshakeLog,
  AcpHandshakeResult,
  AcpMeshResponse,
  AcpPingResponse,
} from '../types'
import { api } from '../api'
import {
  ACP_CARD_LAYOUT_TOKENS,
  getAcpStatusPresentation,
} from '../utils/acpPresentation'
import { getAcpBrandIcon } from '../components/AcpBrandIcons'
import {
  AcpAgentEditorModal,
  AcpConfirmModal,
  AcpDetectModal,
} from '../components/AcpAgentModals'

export interface ACPAgentMeshPageProps {
  className?: string
  style?: React.CSSProperties
}

type EditorState = {
  open: boolean
  mode: 'create' | 'edit'
  agent: AcpAgentInstance | null
}

type DetectState = {
  agent: AcpAgentInstance | null
  result: AcpDetectResponse | null
  loading: boolean
  error: string | null
}

type ConfirmState = {
  kind: 'delete' | 'setup' | null
  agent: AcpAgentInstance | null
}

const CARD_ACTION_STYLE: React.CSSProperties = {
  height: ACP_CARD_LAYOUT_TOKENS.actionButtonHeight,
  padding: ACP_CARD_LAYOUT_TOKENS.actionButtonPadding,
  fontSize: '11px',
  borderRadius: ACP_CARD_LAYOUT_TOKENS.actionButtonBorderRadius,
  display: 'inline-flex',
  alignItems: 'center',
  gap: '6px',
  whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
  border: '1px solid var(--border)',
  backgroundColor: '#f8f9fa',
  color: 'var(--text)',
}

const ACTIVE_ACP_STATUSES = new Set<AcpAgentInstance['status']>([
  'active_hosting',
  'connected',
  'listening',
])

function acpAgentIsOnline(agent: AcpAgentInstance | undefined): boolean {
  return Boolean(agent && ACTIVE_ACP_STATUSES.has(agent.status))
}

function fallbackHandshakeLogs(agents: AcpAgentInstance[], timestamp: string): AcpHandshakeLog[] {
  return agents.map((agent, index) => {
    const online = acpAgentIsOnline(agent)
    return {
      id: `fallback-${Date.now()}-${index}-${agent.id}`,
      timestamp,
      from_agent: 'Antigravity Swiss Knife',
      to_agent: agent.name,
      action: 'MESH_STATUS_FALLBACK',
      payload_summary: `${agent.port_socket || 'stdio'} reported status ${agent.status}; protocol was not re-negotiated.`,
      status: online ? 'success' : agent.status === 'unreachable' || agent.status === 'not_found' ? 'error' : 'warning',
    }
  })
}

function fallbackHandshakeResults(agents: AcpAgentInstance[]): AcpHandshakeResult[] {
  return agents.map((agent) => ({
    agent_id: agent.id,
    agent_name: agent.name,
    transport: agent.transport || 'socket',
    endpoint: agent.port_socket,
    reachable: acpAgentIsOnline(agent),
    verified: false,
    protocol: 'unknown',
    agent_version: agent.acp_version || undefined,
    tools: agent.supported_tools || [],
    ping_latency_ms: agent.ping_latency_ms || 0,
    summary: `Mesh status reported ${agent.status}; no protocol handshake was performed.`,
  }))
}

async function probeAcpAgents(id?: string): Promise<{
  agents: AcpAgentInstance[]
  results: AcpHandshakeResult[]
  logs: AcpHandshakeLog[]
  usedFallback: boolean
}> {
  let pingResponse: AcpPingResponse | null = null
  if (typeof (api as any).pingAcpAgents === 'function') {
    try {
      pingResponse = await (api as any).pingAcpAgents(id)
    } catch {
      // The older daemon does not have the protocol ping route.
    }
  }

  if (
    pingResponse &&
    Array.isArray((pingResponse as any).results) &&
    Array.isArray((pingResponse as any).agents)
  ) {
    return {
      agents: pingResponse.agents,
      results: pingResponse.results,
      logs: Array.isArray(pingResponse.logs) ? pingResponse.logs : [],
      usedFallback: false,
    }
  }

  const mesh = await api.getAcpMesh()
  const agents = mesh?.agents || []
  const selected = id ? agents.filter((agent) => agent.id === id) : agents
  const timestamp = new Date().toLocaleTimeString()
  return {
    agents,
    results: fallbackHandshakeResults(selected),
    logs: fallbackHandshakeLogs(selected, timestamp),
    usedFallback: true,
  }
}

/**
 * ACP (Agent Client Protocol) Agent Mesh Page
 * Displays discovered local agent daemons, official brand icons, latency metrics, and handshake delegation stream.
 * The registry is user-owned: every endpoint/binary can be edited, auto-detected, or replaced with a custom agent.
 * Adheres strictly to David-Design (4px grid, tokenized borders, single-line action text nowrap, zero decorative emojis).
 */
export const ACPAgentMeshPage: React.FC<ACPAgentMeshPageProps> = ({ style }) => {
  const [isPingingAll, setIsPingingAll] = useState(false)
  const [acpFeedback, setAcpFeedback] = useState<string | null>(null)
  const [acpFeedbackIsError, setAcpFeedbackIsError] = useState(false)
  const [agentInstances, setAgentInstances] = useState<AcpAgentInstance[]>([])
  const [handshakeLogs, setHandshakeLogs] = useState<AcpHandshakeLog[]>([])
  const [isLoadingMesh, setIsLoadingMesh] = useState<boolean>(true)
  const [pingingAgentId, setPingingAgentId] = useState<string | null>(null)

  const [editor, setEditor] = useState<EditorState>({ open: false, mode: 'create', agent: null })
  const [editorBusy, setEditorBusy] = useState(false)
  const [editorError, setEditorError] = useState<string | null>(null)

  const [detect, setDetect] = useState<DetectState>({ agent: null, result: null, loading: false, error: null })
  const [confirm, setConfirm] = useState<ConfirmState>({ kind: null, agent: null })
  const [confirmBusy, setConfirmBusy] = useState(false)
  const [isSettingUp, setIsSettingUp] = useState(false)
  const [promptTargetId, setPromptTargetId] = useState('agent-pi')
  const [promptInput, setPromptInput] = useState('')
  const [isPrompting, setIsPrompting] = useState(false)

  const showFeedback = (message: string, isError = false) => {
    setAcpFeedback(message)
    setAcpFeedbackIsError(isError)
    setTimeout(() => setAcpFeedback(null), 6000)
  }

  const applyMesh = (res: AcpMeshResponse | null) => {
    if (res && res.agents) {
      setAgentInstances(res.agents)
    }
  }

  const loadAcpMesh = async () => {
    setIsLoadingMesh(true)
    try {
      const res = await api.getAcpMesh()
      applyMesh(res)
    } catch (e) {
      console.error('Error fetching ACP mesh:', e)
    } finally {
      setIsLoadingMesh(false)
    }
  }

  useEffect(() => {
    loadAcpMesh()
  }, [])

  const handlePingAllAcp = async () => {
    setIsPingingAll(true)
    setAcpFeedback(null)
    try {
      const probe = await probeAcpAgents()
      setAgentInstances(probe.agents)
      const onlineCount = probe.results.filter((result) => result.reachable).length
      setAcpFeedbackIsError(onlineCount === 0)
      setAcpFeedback(
        `ACP Handshake ping completed. ${onlineCount} active agent daemon${
          onlineCount === 1 ? '' : 's'
        } responded successfully.`
      )
      setHandshakeLogs((prev) => [...probe.logs, ...prev])
    } catch (e: any) {
      setAcpFeedbackIsError(true)
      setAcpFeedback(`ACP Ping failed: ${e?.message || 'Network error'}`)
    } finally {
      setIsPingingAll(false)
      setTimeout(() => setAcpFeedback(null), 4000)
    }
  }

  const handlePingSingleAgent = async (agent: AcpAgentInstance) => {
    setPingingAgentId(agent.id)
    try {
      const probe = await probeAcpAgents(agent.id)
      setAgentInstances(probe.agents)
      const updatedAgent = probe.agents.find((candidate) => candidate.id === agent.id) || agent
      const result = probe.results.find((candidate) => candidate.agent_id === agent.id)
      const isOnline = result?.reachable ?? acpAgentIsOnline(updatedAgent)
      setAcpFeedbackIsError(!isOnline)
      setAcpFeedback(
        `ACP Handshake ping completed for ${agent.name}: ${isOnline ? 'Active & Healthy' : 'Offline'}`
      )
      if (probe.logs.length > 0) {
        setHandshakeLogs((prev) => [...probe.logs, ...prev])
      }
    } catch (e: any) {
      setAcpFeedbackIsError(true)
      setAcpFeedback(`Ping ${agent.name} failed: ${e?.message || 'Network error'}`)
    } finally {
      setPingingAgentId(null)
      setTimeout(() => setAcpFeedback(null), 4000)
    }
  }


  const handleSendAcpPrompt = async () => {
    const trimmed = promptInput.trim()
    if (!trimmed || isPrompting) return
    setIsPrompting(true)
    setAcpFeedback(null)
    try {
      const res = await api.promptAcpAgent(promptTargetId, trimmed)
      const targetName = agentInstances.find((a) => a.id === promptTargetId)?.name || promptTargetId
      const replyText = (res?.response || '').trim() || '(empty response)'
      const log: AcpHandshakeLog = {
        id: `prompt-${Date.now()}-${promptTargetId}`,
        timestamp: new Date().toLocaleTimeString(),
        from_agent: 'Antigravity 2.0',
        to_agent: targetName,
        action: 'ACP_PROMPT / RESPONSE',
        payload_summary: replyText,
        status: 'success',
      }
      setHandshakeLogs((prev) => [log, ...prev])
      setPromptInput('')
      showFeedback(`Received response from ${targetName} (${res?.latency_ms ?? 0} ms).`)
    } catch (e: any) {
      showFeedback(`Prompt failed: ${e?.message || 'Agent error'}`, true)
    } finally {
      setIsPrompting(false)
    }
  }

  const openCreateEditor = () => {
    setEditorError(null)
    setEditor({ open: true, mode: 'create', agent: null })
  }

  const openEditEditor = (agent: AcpAgentInstance) => {
    setEditorError(null)
    setEditor({ open: true, mode: 'edit', agent })
  }

  const handleSaveAgent = async (input: AcpAgentInput) => {
    setEditorBusy(true)
    setEditorError(null)
    try {
      const res = await api.saveAcpAgent(input)
      applyMesh(res)
      setEditor({ open: false, mode: 'create', agent: null })
      showFeedback(`${input.id ? 'Saved' : 'Added'} agent ${input.name}.`)
    } catch (e: any) {
      setEditorError(e?.message || 'Failed to save agent')
    } finally {
      setEditorBusy(false)
    }
  }

  const handleDeleteAgent = async (agent: AcpAgentInstance) => {
    setConfirmBusy(true)
    try {
      const res = await api.deleteAcpAgent(agent.id)
      applyMesh(res)
      showFeedback(
        agent.custom ? `Removed custom agent ${agent.name}.` : `${agent.name} reset to its OS default endpoint.`
      )
    } catch (e: any) {
      showFeedback(`Delete failed: ${e?.message || 'Network error'}`, true)
    } finally {
      setConfirmBusy(false)
      setConfirm({ kind: null, agent: null })
    }
  }

  const openDetect = async (agent: AcpAgentInstance) => {
    setDetect({ agent, result: null, loading: true, error: null })
    try {
      const result = await api.detectAcpAgent(agent.id)
      setDetect({ agent, result, loading: false, error: null })
    } catch (e: any) {
      setDetect({ agent, result: null, loading: false, error: e?.message || 'Detection failed' })
    }
  }

  const applyDetectedEndpoint = async (agent: AcpAgentInstance, candidate: AcpDetectCandidate) => {
    setDetect((prev) => ({ ...prev, loading: true }))
    try {
      const res = await api.saveAcpAgent({
        id: agent.id,
        name: agent.name,
        type: agent.type,
        binary_path: agent.binary_path,
        endpoint: candidate.endpoint,
        acp_version: agent.acp_version,
        tools: agent.supported_tools,
      })
      applyMesh(res)
      setDetect({ agent: null, result: null, loading: false, error: null })
      showFeedback(`${agent.name} now uses ${candidate.endpoint}.`)
    } catch (e: any) {
      setDetect((prev) => ({ ...prev, loading: false, error: e?.message || 'Failed to save endpoint' }))
    }
  }

  const applyOsDefaultsToAll = async () => {
    setConfirm({ kind: null, agent: null })
    setIsSettingUp(true)
    let configured = 0
    let alreadyReachable = 0
    let needsReview = 0
    let unreachable = 0
    const presets = agentInstances.filter((a) => !a.custom)
    for (const agent of presets) {
      try {
        const result = await api.detectAcpAgent(agent.id)
        const reachable = (result.candidates || []).find((c) => c.reachable)
        if (!reachable) {
          unreachable += 1
          continue
        }
        if (reachable.endpoint === agent.port_socket) {
          alreadyReachable += 1
          continue
        }
        // A TCP port opened by a running desktop app may belong to an unrelated
        // service (devtools, LSP, telemetry). Only pipe/socket candidates are
        // applied automatically; ambiguous TCP ports stay manual.
        if (reachable.kind === 'tcp') {
          needsReview += 1
          continue
        }
        const res = await api.saveAcpAgent({
          id: agent.id,
          name: agent.name,
          type: agent.type,
          binary_path: agent.binary_path,
          endpoint: reachable.endpoint,
          acp_version: agent.acp_version,
          tools: agent.supported_tools,
        })
        applyMesh(res)
        configured += 1
      } catch (e) {
        unreachable += 1
      }
    }
    setIsSettingUp(false)
    showFeedback(
      `ACP setup for this OS finished: ${configured} endpoint${
        configured === 1 ? '' : 's'
      } configured, ${alreadyReachable} already answering, ${needsReview} TCP port${
        needsReview === 1 ? '' : 's'
      } left for manual review, ${unreachable} without a reachable endpoint.`,
      configured === 0 && alreadyReachable === 0 && needsReview === 0
    )
  }

  const renderAgentAvatar = (id: string) => {
    return (
      <div
        style={{
          width: ACP_CARD_LAYOUT_TOKENS.avatarSize,
          height: ACP_CARD_LAYOUT_TOKENS.avatarSize,
          borderRadius: ACP_CARD_LAYOUT_TOKENS.avatarBorderRadius,
          backgroundColor: '#f1f3f4',
          color: 'var(--text)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
          border: '1px solid var(--border)',
        }}
      >
        {getAcpBrandIcon(id, 18)}
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', ...style }}>
      <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      {acpFeedback && (
        <div
          style={{
            backgroundColor: acpFeedbackIsError ? '#fce8e6' : '#e6f4ea',
            color: acpFeedbackIsError ? '#c5221f' : '#137333',
            border: `1px solid ${acpFeedbackIsError ? 'var(--border-danger, #f5c6c2)' : 'var(--border-success)'}`,
            borderRadius: '8px',
            padding: '10px 16px',
            fontSize: '13px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
          }}
        >
          <CheckCircle2 size={16} />
          {acpFeedback}
        </div>
      )}

      {/* Discovered Agent Nodes Section Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '12px' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
            Discovered Daemons ({agentInstances.length})
          </h2>
          <p style={{ margin: '2px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
            Agent Client Protocol endpoints on local IPC and sockets. Endpoints are conventional defaults
            until you configure or auto-detect them for this OS.
          </p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
          <button
            id="btnAcpSetupForOS"
            className="btn-pill-tonal"
            onClick={() => setConfirm({ kind: 'setup', agent: null })}
            disabled={isSettingUp || isLoadingMesh}
            style={{ padding: '6px 14px', fontSize: '12px' }}
          >
            <Search size={13} className={isSettingUp ? 'animate-spin' : ''} />
            <span>{isSettingUp ? 'Setting up...' : 'Set up ACP for this OS'}</span>
          </button>
          <button
            id="btnAcpAddAgent"
            className="btn-pill-primary"
            onClick={openCreateEditor}
            style={{ padding: '6px 14px', fontSize: '12px' }}
          >
            <Plus size={13} />
            <span>Add Agent</span>
          </button>
        </div>
      </div>

      {/* Agent Nodes Grid */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
          gap: '16px',
        }}
      >
        {isLoadingMesh ? (
          <div
            style={{
              gridColumn: '1 / -1',
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '32px',
              textAlign: 'center',
              color: 'var(--text-muted)',
              fontSize: '13px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '8px',
            }}
          >
            <RefreshCw size={15} className="animate-spin" />
            <span>Scanning local ACP Agent Mesh nodes...</span>
          </div>
        ) : agentInstances.length === 0 ? (
          <div
            style={{
              gridColumn: '1 / -1',
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '32px',
              textAlign: 'center',
              color: 'var(--text-muted)',
              fontSize: '13px',
            }}
          >
            No active agent daemons or ACP nodes discovered on this system.
          </div>
        ) : (
          agentInstances.map((agent) => {
            const statusPres = getAcpStatusPresentation(agent.status)
            return (
              <div
                key={agent.id}
                data-acp-agent-id={agent.id}
                style={{
                  backgroundColor: '#ffffff',
                  border: `${ACP_CARD_LAYOUT_TOKENS.cardBorderWidth} solid var(--border)`,
                  borderRadius: ACP_CARD_LAYOUT_TOKENS.cardBorderRadius,
                  padding: ACP_CARD_LAYOUT_TOKENS.cardPadding,
                  boxShadow: ACP_CARD_LAYOUT_TOKENS.cardShadow,
                  display: 'flex',
                  flexDirection: 'column',
                  justifyContent: 'space-between',
                  gap: '8px',
                }}
              >
                <div>
                  {/* Card Header with Official Brand Avatar & Status */}
                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'flex-start',
                      marginBottom: ACP_CARD_LAYOUT_TOKENS.headerMarginBottom,
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0 }}>
                      {renderAgentAvatar(agent.id)}
                      <div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                          <h4
                            style={{
                              fontSize: '14px',
                              fontWeight: 700,
                              margin: '0 0 2px 0',
                              color: 'var(--text)',
                              whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                            }}
                          >
                            {agent.name}
                          </h4>
                          {agent.custom && (
                            <span
                              style={{
                                fontSize: '10px',
                                fontWeight: 700,
                                textTransform: 'uppercase',
                                padding: '1px 5px',
                                borderRadius: '4px',
                                backgroundColor: '#e8f0fe',
                                color: '#1967d2',
                              }}
                            >
                              Custom
                            </span>
                          )}
                        </div>
                        <span
                          style={{
                            fontSize: '11px',
                            color: 'var(--text-muted)',
                            whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                          }}
                        >
                          {agent.type} • PID {agent.pid || 'N/A'}
                        </span>
                      </div>
                    </div>

                    <span
                      style={{
                        padding: '3px 8px',
                        borderRadius: '6px',
                        fontSize: '11px',
                        fontWeight: 600,
                        backgroundColor: statusPres.bg,
                        color: statusPres.color,
                        textTransform: 'uppercase',
                        whiteSpace: statusPres.whiteSpace,
                      }}
                    >
                      {statusPres.label}
                    </span>
                  </div>

                  {/* Card Body Metadata */}
                  <div
                    style={{
                      fontSize: '12px',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: ACP_CARD_LAYOUT_TOKENS.bodyGap,
                      marginBottom: '8px',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: '8px' }}>
                      <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                        Socket / Endpoint:
                      </span>
                      <span style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', minWidth: 0 }}>
                        <span
                          style={{
                            fontFamily: 'monospace',
                            fontSize: '11px',
                            color: 'var(--text)',
                            wordBreak: 'break-all',
                            textAlign: 'right',
                          }}
                        >
                          {agent.port_socket}
                        </span>
                        <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>
                          {agent.custom
                            ? 'Custom agent'
                            : agent.os_default
                              ? 'OS default'
                              : 'Custom endpoint'}
                        </span>
                      </span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                        Protocol Version:
                      </span>
                      <span
                        style={{
                          fontWeight: 600,
                          color: 'var(--text)',
                          whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                        }}
                      >
                        {agent.acp_version}
                      </span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                        Ping Latency:
                      </span>
                      <span
                        style={{
                          fontWeight: 600,
                          color:
                            (agent.ping_latency_ms ?? 0) > 0 && (agent.ping_latency_ms ?? 0) < 3
                              ? '#137333'
                              : '#b06000',
                          whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                        }}
                      >
                        {(agent.ping_latency_ms ?? 0) > 0 ? `${agent.ping_latency_ms} ms` : 'Unreachable'}
                      </span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                        Last Handshake:
                      </span>
                      <span style={{ color: 'var(--text)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                        {agent.last_handshake}
                      </span>
                    </div>
                  </div>

                  {/* Shared Tools Tag List */}
                  <div style={{ borderTop: '1px solid var(--border)', paddingTop: '8px' }}>
                    <span
                      style={{
                        fontSize: '11px',
                        fontWeight: 600,
                        color: 'var(--text-muted)',
                        display: 'block',
                        marginBottom: '4px',
                        whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                      }}
                    >
                      Negotiated Tools Sharing ({agent.supported_tools.length})
                    </span>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: ACP_CARD_LAYOUT_TOKENS.tagGap }}>
                      {agent.supported_tools.length > 0 ? (
                        agent.supported_tools.map((t) => (
                          <span
                            key={t}
                            style={{
                              backgroundColor: '#f1f3f4',
                              borderRadius: '4px',
                              padding: '1px 6px',
                              fontSize: '11px',
                              fontFamily: 'monospace',
                              color: '#3c4043',
                              whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                            }}
                          >
                            {t}
                          </span>
                        ))
                      ) : (
                        <span
                          style={{
                            fontSize: '11px',
                            color: 'var(--text-muted)',
                            fontStyle: 'italic',
                            whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                          }}
                        >
                          No tools shared
                        </span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Card Actions */}
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'flex-end',
                    flexWrap: 'wrap',
                    gap: '6px',
                    borderTop: '1px solid var(--border)',
                    paddingTop: '8px',
                    marginTop: '4px',
                  }}
                >
                  <button
                    className="btn-pill-tonal"
                    onClick={() => handlePingSingleAgent(agent)}
                    disabled={pingingAgentId === agent.id}
                    style={{ ...CARD_ACTION_STYLE, cursor: pingingAgentId === agent.id ? 'not-allowed' : 'pointer' }}
                  >
                    <Activity
                      size={12}
                      className={pingingAgentId === agent.id ? 'animate-spin' : ''}
                    />
                    <span style={{ whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                      Ping {agent.name}
                    </span>
                  </button>
                  <button
                    className="btn-pill-tonal"
                    onClick={() => openDetect(agent)}
                    style={{ ...CARD_ACTION_STYLE, cursor: 'pointer' }}
                    aria-label={`Auto-detect endpoint for ${agent.name}`}
                  >
                    <Search size={12} />
                    <span>Auto-detect</span>
                  </button>
                  <button
                    className="btn-pill-tonal"
                    onClick={() => openEditEditor(agent)}
                    style={{ ...CARD_ACTION_STYLE, cursor: 'pointer' }}
                    aria-label={`Edit ${agent.name}`}
                  >
                    <Pencil size={12} />
                    <span>Edit</span>
                  </button>
                  {(agent.custom || agent.os_default === false) && (
                    <button
                      className="btn-pill-tonal"
                      onClick={() => setConfirm({ kind: 'delete', agent })}
                      style={{ ...CARD_ACTION_STYLE, cursor: 'pointer', color: '#c5221f' }}
                      aria-label={`${agent.custom ? 'Delete' : 'Reset'} ${agent.name}`}
                    >
                      <Trash2 size={12} />
                      <span>{agent.custom ? 'Delete' : 'Reset'}</span>
                    </button>
                  )}
                </div>
              </div>
            )
          })
        )}
      </div>
      </div>

      {/* Live Handshake Logs */}
      <div
        style={{
          backgroundColor: 'var(--surface, #ffffff)',
          border: '1px solid var(--border)',
          borderRadius: '8px',
          padding: '20px',
        }}
      >
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: '12px',
          }}
        >
          <div>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Communication & Delegation Stream
            </h2>
            <p style={{ margin: '2px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Real-time capability exchange logs and inter-agent handshake delegations.
            </p>
          </div>
          <button
            onClick={handlePingAllAcp}
            disabled={isPingingAll}
            className="btn-pill-tonal"
            style={{
              padding: '6px 14px',
              fontSize: '12px',
              cursor: isPingingAll ? 'not-allowed' : 'pointer',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              whiteSpace: 'nowrap',
            }}
          >
            <Activity size={13} className={isPingingAll ? 'animate-spin' : ''} />
            <span>{isPingingAll ? 'Pinging Nodes...' : 'Ping All ACP Nodes'}</span>
          </button>
        </div>

        <div
          style={{
            display: 'flex',
            gap: '8px',
            alignItems: 'center',
            flexWrap: 'wrap',
            marginBottom: '12px',
          }}
        >
          <select
            aria-label="Target ACP Agent"
            value={promptTargetId}
            onChange={(e) => setPromptTargetId(e.target.value)}
            style={{ fontSize: '12px', padding: '6px 10px', borderRadius: '6px', minWidth: '180px' }}
          >
            {agentInstances.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </select>
          <input
            type="text"
            aria-label="ACP Prompt Input"
            placeholder="Send prompt over ACP / RPC..."
            value={promptInput}
            onChange={(e) => setPromptInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void handleSendAcpPrompt()
            }}
            style={{ flex: 1, minWidth: '220px', fontSize: '12px', padding: '6px 10px', borderRadius: '6px' }}
          />
          <button
            type="button"
            className="btn-pill-primary"
            disabled={isPrompting || !promptInput.trim()}
            onClick={() => void handleSendAcpPrompt()}
            style={{ padding: '6px 14px', fontSize: '12px' }}
          >
            <span>{isPrompting ? 'Sending...' : 'Send Prompt'}</span>
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          {handshakeLogs.length === 0 ? (
            <div
              style={{
                padding: '24px',
                textAlign: 'center',
                color: 'var(--text-muted)',
                fontSize: '13px',
              }}
            >
              No ACP handshakes or delegations recorded yet. Click &quot;Ping All ACP Nodes&quot; above to initiate a protocol health check across all detected agent processes.
            </div>
          ) : (
            handshakeLogs.map((log) => (
              <div
                key={log.id}
                style={{
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '10px 14px',
                  backgroundColor: '#fafafa',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '4px',
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                  }}
                >
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '6px',
                      fontSize: '12px',
                      fontWeight: 600,
                    }}
                  >
                    <span style={{ color: 'var(--primary)' }}>{log.from_agent}</span>
                    <ArrowRight size={12} color="var(--text-muted)" />
                    <span style={{ color: 'var(--text)' }}>{log.to_agent}</span>
                    <span style={{ color: 'var(--text-muted)', fontWeight: 400, fontSize: '11px' }}>
                      ({log.action})
                    </span>
                  </div>
                  <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                    {log.timestamp}
                  </span>
                </div>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                  {log.payload_summary}
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      <AcpAgentEditorModal
        key={editor.open ? `${editor.mode}:${editor.agent?.id || 'new'}` : 'closed'}
        open={editor.open}
        mode={editor.mode}
        initial={editor.agent}
        busy={editorBusy}
        error={editorError}
        onCancel={() => setEditor({ open: false, mode: 'create', agent: null })}
        onSubmit={handleSaveAgent}
      />

      <AcpDetectModal
        agent={detect.agent}
        result={detect.result}
        loading={detect.loading}
        error={detect.error}
        onCancel={() => setDetect({ agent: null, result: null, loading: false, error: null })}
        onApply={(candidate) => {
          if (detect.agent) {
            void applyDetectedEndpoint(detect.agent, candidate)
          }
        }}
      />

      <AcpConfirmModal
        open={confirm.kind === 'delete'}
        title={`Reset ${confirm.agent?.name || 'agent'}?`}
        message={
          confirm.agent?.custom
            ? 'This removes the custom agent from the registry. The running application is not affected.'
            : 'This clears the stored endpoint and binary override for this preset, so it falls back to the OS default for this machine.'
        }
        confirmLabel={confirm.agent?.custom ? 'Delete Agent' : 'Reset to OS Default'}
        busy={confirmBusy}
        danger
        onCancel={() => setConfirm({ kind: null, agent: null })}
        onConfirm={() => {
          if (confirm.agent) {
            void handleDeleteAgent(confirm.agent)
          }
        }}
      />

      <AcpConfirmModal
        open={confirm.kind === 'setup'}
        title="Set up ACP for this OS"
        message="Each built-in agent will be probed: its configured endpoint, its OS default, TCP ports owned by a running process, and existing socket files. Reachable named pipes and unix sockets are saved as overrides automatically; ambiguous TCP ports are left for manual review. Nothing is changed for agents with no reachable endpoint."
        confirmLabel="Run Setup"
        busy={isSettingUp}
        onCancel={() => setConfirm({ kind: null, agent: null })}
        onConfirm={() => {
          void applyOsDefaultsToAll()
        }}
      />
    </div>
  )
}

export default ACPAgentMeshPage
