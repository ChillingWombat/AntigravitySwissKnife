import React, { useState, useEffect } from 'react'
import {
  Activity,
  CheckCircle2,
  ArrowRight,
  RefreshCw,
} from 'lucide-react'
import type { AcpAgentInstance, AcpHandshakeLog } from '../types'
import { api } from '../api'
import {
  ACP_CARD_LAYOUT_TOKENS,
  getAcpStatusPresentation,
} from '../utils/acpPresentation'
import { getAcpBrandIcon } from '../components/AcpBrandIcons'

export interface ACPAgentMeshPageProps {
  className?: string
  style?: React.CSSProperties
}

/**
 * ACP (Agent Client Protocol) Agent Mesh Page
 * Displays discovered local agent daemons, official brand icons, latency metrics, and handshake delegation stream.
 * Adheres strictly to David-Design (4px grid, tokenized borders, single-line action text nowrap, zero decorative emojis).
 */
export const ACPAgentMeshPage: React.FC<ACPAgentMeshPageProps> = ({ style }) => {
  const [isPingingAll, setIsPingingAll] = useState(false)
  const [acpFeedback, setAcpFeedback] = useState<string | null>(null)
  const [agentInstances, setAgentInstances] = useState<AcpAgentInstance[]>([])
  const [handshakeLogs, setHandshakeLogs] = useState<AcpHandshakeLog[]>([])
  const [isLoadingMesh, setIsLoadingMesh] = useState<boolean>(true)
  const [pingingAgentId, setPingingAgentId] = useState<string | null>(null)

  const loadAcpMesh = async () => {
    setIsLoadingMesh(true)
    try {
      const res = await api.getAcpMesh()
      if (res && res.agents) {
        setAgentInstances(res.agents)
      }
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
      const res = await api.getAcpMesh()
      if (res && res.agents) {
        setAgentInstances(res.agents)
        const online = res.agents.filter((a: any) => a.status !== 'unreachable')
        setAcpFeedback(
          `ACP Handshake ping completed. ${online.length} active agent daemon${
            online.length === 1 ? '' : 's'
          } responded successfully.`
        )
        const newLogs: AcpHandshakeLog[] = online.map((a: any) => ({
          id: `log-${Date.now()}-${a.id}`,
          timestamp: new Date().toLocaleTimeString(),
          from_agent: 'Antigravity 2.0',
          to_agent: a.name,
          action: 'ACP_HELLO / CAPABILITY_EXCHANGE',
          payload_summary: `Negotiated ${a.supported_tools?.length || 0} tools (${(
            a.supported_tools || []
          )
            .slice(0, 3)
            .join(', ')}...) over socket ${a.port_socket}`,
          status: 'success',
        }))
        setHandshakeLogs((prev) => [...newLogs, ...prev])
      }
    } catch (e: any) {
      setAcpFeedback(`ACP Ping failed: ${e?.message || 'Network error'}`)
    } finally {
      setIsPingingAll(false)
      setTimeout(() => setAcpFeedback(null), 4000)
    }
  }

  const handlePingSingleAgent = async (agent: AcpAgentInstance) => {
    setPingingAgentId(agent.id)
    try {
      const res = await api.getAcpMesh()
      if (res && res.agents) {
        setAgentInstances(res.agents)
        const updatedAgent = res.agents.find((a: any) => a.id === agent.id) || agent
        const isOnline = updatedAgent.status !== 'unreachable'
        setAcpFeedback(
          `ACP Handshake ping completed for ${agent.name}: ${isOnline ? 'Active & Healthy' : 'Offline'}`
        )
        const log: AcpHandshakeLog = {
          id: `log-${Date.now()}-${agent.id}`,
          timestamp: new Date().toLocaleTimeString(),
          from_agent: 'Antigravity 2.0',
          to_agent: agent.name,
          action: 'ACP_HELLO / DIRECT_PING',
          payload_summary: isOnline
            ? `Pinged ${agent.name} over socket ${agent.port_socket} (latency: ${updatedAgent.ping_latency_ms} ms)`
            : `Attempted ping to ${agent.name} over socket ${agent.port_socket} - daemon unreachable`,
          status: isOnline ? 'success' : 'warning',
        }
        setHandshakeLogs((prev) => [log, ...prev])
      }
    } catch (e: any) {
      setAcpFeedback(`Ping ${agent.name} failed: ${e?.message || 'Network error'}`)
    } finally {
      setPingingAgentId(null)
      setTimeout(() => setAcpFeedback(null), 4000)
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

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', ...style }}>
      {acpFeedback && (
        <div
          style={{
            backgroundColor: '#e6f4ea',
            color: '#137333',
            border: '1px solid #ceead6',
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
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      {renderAgentAvatar(agent.id)}
                      <div>
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
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                        Socket / Endpoint:
                      </span>
                      <span
                        style={{
                          fontFamily: 'monospace',
                          fontSize: '11px',
                          color: 'var(--text)',
                          whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                        }}
                      >
                        {agent.port_socket}
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
                            agent.ping_latency_ms > 0 && agent.ping_latency_ms < 3
                              ? '#137333'
                              : '#b06000',
                          whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                        }}
                      >
                        {agent.ping_latency_ms > 0 ? `${agent.ping_latency_ms} ms` : 'Unreachable'}
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
                  <div style={{ borderTop: '1px solid #f1f3f4', paddingTop: '8px' }}>
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

                {/* Card Action Button: Ping {agent.name} */}
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'flex-end',
                    borderTop: '1px solid #f1f3f4',
                    paddingTop: '8px',
                    marginTop: '4px',
                  }}
                >
                  <button
                    className="btn-pill-tonal"
                    onClick={() => handlePingSingleAgent(agent)}
                    disabled={pingingAgentId === agent.id}
                    style={{
                      height: ACP_CARD_LAYOUT_TOKENS.actionButtonHeight,
                      padding: ACP_CARD_LAYOUT_TOKENS.actionButtonPadding,
                      fontSize: '11px',
                      borderRadius: ACP_CARD_LAYOUT_TOKENS.actionButtonBorderRadius,
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '6px',
                      whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                      cursor: pingingAgentId === agent.id ? 'not-allowed' : 'pointer',
                      border: '1px solid var(--border)',
                      backgroundColor: '#f8f9fa',
                      color: 'var(--text)',
                    }}
                  >
                    <Activity
                      size={12}
                      className={pingingAgentId === agent.id ? 'animate-spin' : ''}
                    />
                    <span style={{ whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                      Ping {agent.name}
                    </span>
                  </button>
                </div>
              </div>
            )
          })
        )}
      </div>

      {/* Live Handshake Logs */}
      <div
        style={{
          backgroundColor: '#ffffff',
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
          <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
            ACP Communication & Delegation Stream
          </h3>
          <button
            onClick={handlePingAllAcp}
            disabled={isPingingAll}
            style={{
              backgroundColor: 'var(--primary)',
              color: '#ffffff',
              border: 'none',
              borderRadius: '6px',
              padding: '6px 14px',
              fontSize: '12px',
              fontWeight: 600,
              cursor: isPingingAll ? 'not-allowed' : 'pointer',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              whiteSpace: 'nowrap',
              boxShadow: '0 1px 2px rgba(0,0,0,0.08)',
            }}
          >
            <Activity size={13} className={isPingingAll ? 'animate-spin' : ''} />
            <span>{isPingingAll ? 'Pinging Nodes...' : 'Ping All ACP Nodes'}</span>
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
                  border: '1px solid #f1f3f4',
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
    </div>
  )
}

export default ACPAgentMeshPage
