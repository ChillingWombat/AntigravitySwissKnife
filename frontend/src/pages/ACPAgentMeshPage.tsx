import React, { useState, useEffect } from 'react'
import { createPortal } from 'react-dom'
import {
  Activity,
  ArrowRight,
  RefreshCw,
  Plus,
  Pencil,
  Search,
  Trash2,
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
  kind: 'delete' | null
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
  const [promptTargetId, setPromptTargetId] = useState('agent-pi')
  const [promptInput, setPromptInput] = useState('')
  const [isPrompting, setIsPrompting] = useState(false)
  const [isSyncingRegistry, setIsSyncingRegistry] = useState(false)
  const [portalTarget, setPortalTarget] = useState<HTMLElement | null>(null)

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

  useEffect(() => {
    setPortalTarget(document.getElementById('acp-mesh-top-bar-portal'))
  }, [])

  const handlePingAllAcp = async () => {
    setIsPingingAll(true)
    try {
      const probe = await probeAcpAgents()
      setAgentInstances(probe.agents)
      setHandshakeLogs((prev) => [...probe.logs, ...prev])
    } catch (e: any) {
      console.error('ACP Ping failed:', e)
    } finally {
      setIsPingingAll(false)
    }
  }

  const handleSyncRegistry = async () => {
    setIsSyncingRegistry(true)
    try {
      const res = await api.syncAcpRegistry()
      applyMesh(res)
    } catch (e: any) {
      console.error('Failed to sync ACP registry:', e)
    } finally {
      setIsSyncingRegistry(false)
    }
  }

  const handlePingSingleAgent = async (agent: AcpAgentInstance) => {
    setPingingAgentId(agent.id)
    try {
      const probe = await probeAcpAgents(agent.id)
      setAgentInstances(probe.agents)
      if (probe.logs.length > 0) {
        setHandshakeLogs((prev) => [...probe.logs, ...prev])
      }
    } catch (e: any) {
      console.error(`Ping ${agent.name} failed:`, e)
    } finally {
      setPingingAgentId(null)
    }
  }

  const handleSendAcpPrompt = async () => {
    const trimmed = promptInput.trim()
    if (!trimmed || isPrompting) return
    setIsPrompting(true)
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
    } catch (e: any) {
      console.error('Prompt failed:', e)
      const targetName = agentInstances.find((a) => a.id === promptTargetId)?.name || promptTargetId
      const log: AcpHandshakeLog = {
        id: `prompt-err-${Date.now()}-${promptTargetId}`,
        timestamp: new Date().toLocaleTimeString(),
        from_agent: 'Antigravity 2.0',
        to_agent: targetName,
        action: 'ACP_PROMPT / ERROR',
        payload_summary: e?.message || 'Agent error',
        status: 'error',
      }
      setHandshakeLogs((prev) => [log, ...prev])
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
    const isAntigravity = agent.id === 'agent-antigravity-acp' || agent.id === 'agent-antigravity'
    const agentToEdit =
      isAntigravity && agent.name === 'Google Antigravity (ACP)'
        ? { ...agent, name: 'Antigravity ACP Adapter' }
        : agent
    setEditor({ open: true, mode: 'edit', agent: agentToEdit })
  }

  const handleSaveAgent = async (input: AcpAgentInput) => {
    setEditorBusy(true)
    setEditorError(null)
    try {
      const res = await api.saveAcpAgent(input)
      applyMesh(res)
      setEditor({ open: false, mode: 'create', agent: null })
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
    } catch (e: any) {
      console.error('Delete failed:', e)
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
    } catch (e: any) {
      setDetect((prev) => ({ ...prev, loading: false, error: e?.message || 'Failed to save endpoint' }))
    }
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

  const topActions = (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
      <button
        id="btnAcpSyncRegistry"
        type="button"
        className="btn-pill-tonal"
        onClick={handleSyncRegistry}
        disabled={isSyncingRegistry}
        style={{
          padding: '6px 14px',
          fontSize: '12px',
          display: 'inline-flex',
          alignItems: 'center',
          gap: '6px',
          whiteSpace: 'nowrap',
          backgroundColor: 'var(--tonal)',
          border: '1px solid var(--border)',
          borderRadius: '20px',
          color: 'var(--text)',
          cursor: isSyncingRegistry ? 'not-allowed' : 'pointer',
          opacity: isSyncingRegistry ? 0.7 : 1,
        }}
      >
        <RefreshCw size={13} className={isSyncingRegistry ? 'animate-spin' : ''} />
        <span>Sync Registry</span>
      </button>
      <button
        id="btnAcpAddAgent"
        type="button"
        className="btn-pill-primary"
        onClick={openCreateEditor}
        style={{ padding: '6px 14px', fontSize: '12px', display: 'flex', alignItems: 'center', gap: '6px', whiteSpace: 'nowrap' }}
      >
        <Plus size={13} />
        <span>Add Agent</span>
      </button>
    </div>
  )

  const antigravityAgent = agentInstances.find(
    (a) => a.id === 'agent-antigravity-acp' || a.id === 'agent-antigravity'
  )
  const otherAgents = agentInstances.filter(
    (a) => a.id !== 'agent-antigravity-acp' && a.id !== 'agent-antigravity'
  )

  const isAntigravityOnline = acpAgentIsOnline(antigravityAgent)
  const adapterSubSections = antigravityAgent
    ? [
        {
          id: 'sub-antigravity-core',
          name: 'Antigravity 2.0',
          status: isAntigravityOnline ? 'ACTIVE' : null,
          statusBg: isAntigravityOnline ? '#e6f4ea' : undefined,
          statusColor: isAntigravityOnline ? '#137333' : undefined,
          socket: isAntigravityOnline ? 'Pipe connected' : 'stdio (process pipe)',
          integration: isAntigravityOnline ? 'Host Core Active' : 'Native Runtime Host',
          handshake: antigravityAgent.last_handshake || (isAntigravityOnline ? 'Synchronized' : 'Offline'),
        },
        {
          id: 'sub-antigravity-cli',
          name: 'Antigravity CLI',
          status: isAntigravityOnline
            ? 'LINKED'
            : antigravityAgent.status === 'installed_idle'
              ? 'READY'
              : 'STANDBY',
          statusBg: isAntigravityOnline
            ? '#e8f0fe'
            : antigravityAgent.status === 'installed_idle'
              ? '#fef7e0'
              : '#f1f3f4',
          statusColor: isAntigravityOnline
            ? '#1a73e8'
            : antigravityAgent.status === 'installed_idle'
              ? '#b06000'
              : '#5f6368',
          socket: 'Subprocess stdio',
          integration: 'CLI protocol bridge',
          handshake: isAntigravityOnline ? 'Ready' : 'Offline',
        },
        {
          id: 'sub-antigravity-vscode',
          name: 'Antigravity IDE Extension',
          status: isAntigravityOnline
            ? 'REGISTERED'
            : antigravityAgent.status === 'installed_idle'
              ? 'READY'
              : 'STANDBY',
          statusBg: isAntigravityOnline
            ? '#e8f0fe'
            : antigravityAgent.status === 'installed_idle'
              ? '#fef7e0'
              : '#f1f3f4',
          statusColor: isAntigravityOnline
            ? '#1a73e8'
            : antigravityAgent.status === 'installed_idle'
              ? '#b06000'
              : '#5f6368',
          socket: 'Extension IPC pipe',
          integration: 'Sidecar bridge',
          handshake: isAntigravityOnline ? 'Protocol v1.2' : 'Offline',
        },
      ]
    : []

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', ...style }}>
      {portalTarget && createPortal(topActions, portalTarget)}
      {/* Section 1: Antigravity ACP Host Adapter */}
      {antigravityAgent && (
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div
              style={{
                fontSize: '11px',
                fontWeight: 700,
                color: 'var(--text-muted)',
                letterSpacing: '0.8px',
                textTransform: 'uppercase',
              }}
            >
              Antigravity ACP Host Adapter
            </div>
          </div>

          {(() => {
            const statusPres = getAcpStatusPresentation(antigravityAgent.status)
            return (
              <div
                key={antigravityAgent.id}
                data-acp-agent-id={antigravityAgent.id}
                style={{
                  backgroundColor: '#ffffff',
                  border: `${ACP_CARD_LAYOUT_TOKENS.cardBorderWidth} solid var(--border)`,
                  borderRadius: ACP_CARD_LAYOUT_TOKENS.cardBorderRadius,
                  padding: ACP_CARD_LAYOUT_TOKENS.cardPadding,
                  boxShadow: ACP_CARD_LAYOUT_TOKENS.cardShadow,
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '12px',
                }}
              >
                <div>
                  {/* Card Header with Official Brand Avatar & Status */}
                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      marginBottom: ACP_CARD_LAYOUT_TOKENS.headerMarginBottom,
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0 }}>
                      {renderAgentAvatar(antigravityAgent.id)}
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        <h4
                          style={{
                            fontSize: '14px',
                            fontWeight: 700,
                            margin: 0,
                            color: 'var(--text)',
                            whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                          }}
                        >
                          Antigravity ACP Adapter
                        </h4>
                        {antigravityAgent.custom && (
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
                    </div>

                    {statusPres.label && statusPres.label !== 'INSTALLED' && (
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
                    )}
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
                          {antigravityAgent.port_socket || (antigravityAgent.transport === 'stdio' ? 'stdio (process pipe)' : 'stdio (process pipe)')}
                        </span>
                        <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>
                          {antigravityAgent.custom
                            ? 'Custom agent'
                            : antigravityAgent.os_default
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
                        {antigravityAgent.acp_version}
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
                            (antigravityAgent.ping_latency_ms ?? 0) > 0 && (antigravityAgent.ping_latency_ms ?? 0) < 3
                              ? '#137333'
                              : '#b06000',
                          whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                        }}
                      >
                        {(antigravityAgent.ping_latency_ms ?? 0) > 0 ? `${antigravityAgent.ping_latency_ms} ms` : 'Unreachable'}
                      </span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                        Last Handshake:
                      </span>
                      <span style={{ color: 'var(--text)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                        {antigravityAgent.last_handshake}
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
                      Negotiated Tools Sharing ({antigravityAgent.supported_tools.length})
                    </span>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: ACP_CARD_LAYOUT_TOKENS.tagGap }}>
                      {antigravityAgent.supported_tools.length > 0 ? (
                        antigravityAgent.supported_tools.map((t) => (
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

                  {/* 1x3 Sub-sections */}
                  <div
                    style={{
                      display: 'grid',
                      gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
                      gap: '12px',
                      marginTop: '10px',
                    }}
                  >
                    {adapterSubSections.map((sub) => (
                      <div
                        key={sub.id}
                        style={{
                          backgroundColor: 'var(--canvas, #f8f9fa)',
                          border: '1px solid var(--border)',
                          borderRadius: '6px',
                          padding: '10px 12px',
                          display: 'flex',
                          flexDirection: 'column',
                          gap: '8px',
                          minWidth: 0,
                        }}
                      >
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                            gap: '6px',
                          }}
                        >
                          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', minWidth: 0 }}>
                            <span
                              style={{
                                fontSize: '12px',
                                fontWeight: 600,
                                color: 'var(--text)',
                                whiteSpace: 'nowrap',
                                overflow: 'hidden',
                                textOverflow: 'ellipsis',
                              }}
                            >
                              {sub.name}
                            </span>
                          </div>
                          {sub.status && sub.status !== 'INSTALLED' && (
                            <span
                              style={{
                                padding: '2px 6px',
                                borderRadius: '4px',
                                fontSize: '10px',
                                fontWeight: 600,
                                textTransform: 'uppercase',
                                backgroundColor: sub.statusBg,
                                color: sub.statusColor,
                                whiteSpace: 'nowrap',
                              }}
                            >
                              {sub.status}
                            </span>
                          )}
                        </div>

                        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '11px' }}>
                          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '6px' }}>
                            <span style={{ color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>Socket / Pipe:</span>
                            <span style={{ fontFamily: 'monospace', color: 'var(--text)', whiteSpace: 'nowrap' }}>
                              {sub.socket}
                            </span>
                          </div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '6px' }}>
                            <span style={{ color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>Integration:</span>
                            <span style={{ color: 'var(--text)', whiteSpace: 'nowrap' }}>
                              {sub.integration}
                            </span>
                          </div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '6px' }}>
                            <span style={{ color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>Handshake:</span>
                            <span style={{ color: 'var(--text)', whiteSpace: 'nowrap' }}>
                              {sub.handshake}
                            </span>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Card Actions */}
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'flex-start',
                    alignItems: 'center',
                    flexWrap: 'wrap',
                    gap: '6px',
                    borderTop: '1px solid var(--border)',
                    paddingTop: '8px',
                    marginTop: '4px',
                  }}
                >
                  <button
                    className="btn-pill-tonal"
                    onClick={() => handlePingSingleAgent(antigravityAgent)}
                    disabled={pingingAgentId === antigravityAgent.id}
                    style={{ ...CARD_ACTION_STYLE, cursor: pingingAgentId === antigravityAgent.id ? 'not-allowed' : 'pointer' }}
                    aria-label={`Ping ${antigravityAgent.name}`}
                  >
                    <Activity
                      size={12}
                      className={pingingAgentId === antigravityAgent.id ? 'animate-spin' : ''}
                    />
                    <span style={{ whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                      Ping
                    </span>
                  </button>
                  <button
                    className="btn-pill-tonal"
                    onClick={() => openDetect(antigravityAgent)}
                    style={{ ...CARD_ACTION_STYLE, cursor: 'pointer' }}
                    aria-label={`Auto-detect endpoint for ${antigravityAgent.name}`}
                  >
                    <Search size={12} />
                    <span>Auto-detect</span>
                  </button>
                  <button
                    className="btn-pill-tonal"
                    onClick={() => openEditEditor(antigravityAgent)}
                    style={{ ...CARD_ACTION_STYLE, cursor: 'pointer' }}
                    aria-label={`Edit ${antigravityAgent.name}`}
                  >
                    <Pencil size={12} />
                    <span>Edit</span>
                  </button>
                  {(antigravityAgent.custom || antigravityAgent.os_default === false) && (
                    <button
                      className="btn-pill-tonal"
                      onClick={() => setConfirm({ kind: 'delete', agent: antigravityAgent })}
                      style={{ ...CARD_ACTION_STYLE, cursor: 'pointer', color: '#c5221f' }}
                      aria-label={`${antigravityAgent.custom ? 'Delete' : 'Reset'} ${antigravityAgent.name}`}
                    >
                      <Trash2 size={12} />
                      <span>{antigravityAgent.custom ? 'Delete' : 'Reset'}</span>
                    </button>
                  )}
                </div>
              </div>
            )
          })()}
        </div>
      )}

      {/* Section 2: Connected Agent Nodes */}
      <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '8px' }}>
          <div
            style={{
              fontSize: '11px',
              fontWeight: 700,
              color: 'var(--text-muted)',
              letterSpacing: '0.8px',
              textTransform: 'uppercase',
            }}
          >
            Connected Agent Nodes
          </div>
          {!portalTarget && (
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
              {topActions}
            </div>
          )}
        </div>

        {isLoadingMesh ? (
          <div
            style={{
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
            <span>Scanning local ACP Adapter nodes...</span>
          </div>
        ) : otherAgents.length === 0 ? (
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '32px',
              textAlign: 'center',
              color: 'var(--text-muted)',
              fontSize: '13px',
            }}
          >
            {!antigravityAgent && otherAgents.length === 0
              ? 'No active agent daemons or ACP nodes discovered on this system.'
              : 'No connected agent nodes discovered on this system.'}
          </div>
        ) : (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
              gap: '16px',
            }}
          >
            {otherAgents.map((agent) => {
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
                          alignItems: 'center',
                          marginBottom: ACP_CARD_LAYOUT_TOKENS.headerMarginBottom,
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0 }}>
                          {renderAgentAvatar(agent.id)}
                          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <h4
                              style={{
                                fontSize: '14px',
                                fontWeight: 700,
                                margin: 0,
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
                              {agent.port_socket || (agent.transport === 'stdio' ? 'stdio (process pipe)' : 'stdio')}
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
                        justifyContent: 'flex-start',
                        alignItems: 'center',
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
                        aria-label={`Ping ${agent.name}`}
                      >
                        <Activity
                          size={12}
                          className={pingingAgentId === agent.id ? 'animate-spin' : ''}
                        />
                        <span style={{ whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                          Ping
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
              })}
            </div>
          )}
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
            {agentInstances.map((a) => {
              const isAntigravity = a.id === 'agent-antigravity-acp' || a.id === 'agent-antigravity'
              const displayName =
                isAntigravity && (a.name === 'Google Antigravity (ACP)' || a.name === 'Google Antigravity 2.0')
                  ? 'Antigravity ACP Adapter'
                  : a.name
              return (
                <option key={a.id} value={a.id}>
                  {displayName}
                </option>
              )
            })}
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
              No ACP handshakes or delegations.
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

    </div>
  )
}

export default ACPAgentMeshPage
