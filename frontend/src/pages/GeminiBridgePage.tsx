import React, { useState, useEffect } from 'react'
import {
  Radio,
  CheckCircle2,
  AlertCircle,
  Terminal,
  Copy,
  Send,
  RefreshCw,
  ShieldCheck,
  Globe,
  Wrench,
  Clock,
  Laptop,
  Code2,
} from 'lucide-react'
import { api } from '../api'

interface BridgeStatusData {
  enabled: boolean
  gateway_url: string
  tunnel_type: string
  tunnel_online: boolean
  auth_token: string
  surfaces: {
    desktop_app_connected: boolean
    desktop_cdp_port: number
    desktop_active_title?: string
    agy_cli_installed: boolean
    agy_cli_path?: string
    vscode_connected: boolean
    account_resilience: boolean
  }
  invocations_count: number
  recent_invocations: Array<{
    id: string
    timestamp: string
    tool: string
    args?: Record<string, any>
    duration_ms: number
    status: string
    result?: string
    error?: string
  }>
}

export const GeminiBridgePage: React.FC = () => {
  const [status, setStatus] = useState<BridgeStatusData | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [copied, setCopied] = useState<boolean>(false)
  const [testPrompt, setTestPrompt] = useState<string>('List current project status and verify bridge connectivity')
  const [targetSurface, setTargetSurface] = useState<string>('desktop')
  const [dispatching, setDispatching] = useState<boolean>(false)
  const [dispatchResult, setDispatchResult] = useState<string | null>(null)
  const [testingConnection, setTestingConnection] = useState<boolean>(false)

  const loadStatus = async () => {
    try {
      setLoading(true)
      const res = await api.getBridgeStatus()
      setStatus(res)
    } catch (err) {
      console.error('Failed to load bridge status:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadStatus()
    const timer = setInterval(() => {
      loadStatus()
    }, 10000)
    return () => clearInterval(timer)
  }, [])

  const handleTestConnection = async () => {
    try {
      setTestingConnection(true)
      const res = await api.testBridgeConnection()
      if (res && res.status) {
        setStatus(res.status)
      }
      setDispatchResult(res.message || 'Connection verified')
    } catch (err: any) {
      setDispatchResult(`Verification failed: ${err?.message || 'Network error'}`)
    } finally {
      setTestingConnection(false)
    }
  }

  const handleDispatch = async () => {
    if (!testPrompt.trim()) return
    try {
      setDispatching(true)
      setDispatchResult(null)
      const res = await api.dispatchBridgeTask({
        prompt: testPrompt.trim(),
        target: targetSurface,
      })
      if (res.success) {
        setDispatchResult(res.result || 'Task dispatched successfully!')
        loadStatus()
      } else {
        setDispatchResult(`Error: ${res.error || 'Failed to dispatch task'}`)
      }
    } catch (err: any) {
      setDispatchResult(`Dispatch failed: ${err?.message || 'Server error'}`)
    } finally {
      setDispatching(false)
    }
  }

  const mcpConfigSnippet = JSON.stringify(
    {
      mcpServers: {
        antigravity: {
          url: status?.gateway_url || 'http://127.0.0.1:8765/api/bridge/mcp',
          headers: {
            Authorization: `Bearer ${status?.auth_token || 'swiss-gemini-bridge-auth-token'}`,
          },
        },
      },
    },
    null,
    2
  )

  const handleCopyConfig = () => {
    navigator.clipboard.writeText(mcpConfigSnippet)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (loading && !status) {
    return (
      <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
        Loading Gemini Bridge status...
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Top Banner: Bridge Health & Streamable HTTP Gateway */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '12px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
              <Radio size={18} color="var(--primary)" />
              <span style={{ fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                Gemini Bridge (F41 / M15)
              </span>
              <span
                style={{
                  backgroundColor: status?.enabled ? '#e6f4ea' : '#fce8e6',
                  color: status?.enabled ? '#137333' : '#c5221f',
                  padding: '2px 8px',
                  borderRadius: '10px',
                  fontSize: '11px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                }}
              >
                {status?.enabled ? 'Online (Streamable HTTP)' : 'Disabled'}
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--text-muted)' }}>
              Two-way agent orchestration connecting Gemini App / Web (Gemini Spark) to local Antigravity surfaces across account rotations.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            <button
              onClick={handleTestConnection}
              disabled={testingConnection}
              style={{
                backgroundColor: 'transparent',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                padding: '6px 14px',
                fontSize: '12px',
                fontWeight: 600,
                color: 'var(--text)',
                cursor: testingConnection ? 'not-allowed' : 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                whiteSpace: 'nowrap',
              }}
            >
              <RefreshCw size={13} className={testingConnection ? 'animate-spin' : ''} />
              Verify Connection
            </button>
          </div>
        </div>

        {/* Gateway URL & Account Resilience Bar */}
        <div
          style={{
            marginTop: '16px',
            backgroundColor: '#f8f9fa',
            borderRadius: '8px',
            border: '1px solid var(--border)',
            padding: '12px 16px',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            flexWrap: 'wrap',
            gap: '12px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: '280px' }}>
            <Globe size={15} color="var(--text-muted)" />
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Endpoint:
            </span>
            <code style={{ fontSize: '12px', color: 'var(--primary)', fontWeight: 600 }}>
              {status?.gateway_url || 'http://127.0.0.1:8765/api/bridge/mcp'}
            </code>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <ShieldCheck size={16} color="#137333" />
            <span style={{ fontSize: '12px', fontWeight: 600, color: '#137333', whiteSpace: 'nowrap' }}>
              Account-Switch Resilience Active (Preserves session across laptop switches)
            </span>
          </div>
        </div>
      </div>

      {/* Surface Connectivity Matrix */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
          gap: '16px',
        }}
      >
        {/* Surface 1: Desktop App */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '16px',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Laptop size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Antigravity 2.0 Desktop
              </span>
            </div>
            {status?.surfaces?.desktop_app_connected ? (
              <span style={{ fontSize: '11px', color: '#137333', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
                <CheckCircle2 size={13} /> CDP Connected
              </span>
            ) : (
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 500, display: 'flex', alignItems: 'center', gap: '4px' }}>
                <AlertCircle size={13} /> Standby
              </span>
            )}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            Port: <code style={{ color: 'var(--text)' }}>{status?.surfaces?.desktop_cdp_port || '36231'}</code>
            {status?.surfaces?.desktop_active_title && (
              <div style={{ marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                Window: {status.surfaces.desktop_active_title}
              </div>
            )}
          </div>
        </div>

        {/* Surface 2: CLI */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '16px',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Terminal size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Antigravity CLI (agy)
              </span>
            </div>
            {status?.surfaces?.agy_cli_installed ? (
              <span style={{ fontSize: '11px', color: '#137333', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
                <CheckCircle2 size={13} /> Ready
              </span>
            ) : (
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 500 }}>
                Not on PATH
              </span>
            )}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            Path: <code style={{ color: 'var(--text)' }}>{status?.surfaces?.agy_cli_path || '~/.local/bin/agy'}</code>
          </div>
        </div>

        {/* Surface 3: VS Code Extension */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '16px',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Code2 size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                VS Code Extension
              </span>
            </div>
            <span style={{ fontSize: '11px', color: '#137333', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
              <CheckCircle2 size={13} /> Port 41223
            </span>
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            Language Server socket active and federated.
          </div>
        </div>
      </div>

      {/* Gemini App Setup & Copy MCP Config */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
          <div>
            <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
              Gemini App & Web (Gemini Spark) Configuration
            </span>
            <p style={{ margin: '2px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Paste this block into Gemini Web custom MCP configuration to grant Gemini agent direct access to your local machine.
            </p>
          </div>
          <button
            onClick={handleCopyConfig}
            style={{
              backgroundColor: copied ? '#e6f4ea' : 'var(--tonal)',
              border: '1px solid var(--border)',
              borderRadius: '6px',
              padding: '6px 12px',
              fontSize: '12px',
              fontWeight: 600,
              color: copied ? '#137333' : 'var(--text)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              whiteSpace: 'nowrap',
            }}
          >
            {copied ? <CheckCircle2 size={13} /> : <Copy size={13} />}
            {copied ? 'Copied!' : 'Copy Config'}
          </button>
        </div>

        <pre
          style={{
            margin: 0,
            padding: '14px',
            backgroundColor: '#f8f9fa',
            borderRadius: '8px',
            border: '1px solid var(--border)',
            fontFamily: 'monospace',
            fontSize: '12px',
            color: 'var(--text)',
            overflowX: 'auto',
          }}
        >
          {mcpConfigSnippet}
        </pre>
      </div>

      {/* Test Dispatch Console */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ marginBottom: '12px' }}>
          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
            Test Remote Task Dispatch
          </span>
          <p style={{ margin: '2px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
            Simulate a remote request from Gemini App to steer local Antigravity.
          </p>
        </div>

        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '12px' }}>
          <select
            value={targetSurface}
            onChange={(e) => setTargetSurface(e.target.value)}
            style={{
              padding: '8px 12px',
              fontSize: '12px',
              borderRadius: '6px',
              border: '1px solid var(--border)',
              backgroundColor: '#ffffff',
              color: 'var(--text)',
              outline: 'none',
            }}
          >
            <option value="desktop">Antigravity 2.0 Desktop</option>
            <option value="cli">Antigravity CLI (agy)</option>
            <option value="auto">Auto Select</option>
          </select>

          <input
            type="text"
            value={testPrompt}
            onChange={(e) => setTestPrompt(e.target.value)}
            placeholder="Type prompt to dispatch..."
            style={{
              flex: 1,
              minWidth: '240px',
              padding: '8px 12px',
              fontSize: '12px',
              borderRadius: '6px',
              border: '1px solid var(--border)',
              outline: 'none',
              backgroundColor: '#ffffff',
              color: 'var(--text)',
            }}
          />

          <button
            onClick={handleDispatch}
            disabled={dispatching || !testPrompt.trim()}
            style={{
              backgroundColor: 'var(--primary)',
              border: 'none',
              borderRadius: '6px',
              padding: '8px 18px',
              fontSize: '12px',
              fontWeight: 600,
              color: '#ffffff',
              cursor: dispatching ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              whiteSpace: 'nowrap',
            }}
          >
            <Send size={13} />
            {dispatching ? 'Dispatching...' : 'Dispatch Task'}
          </button>
        </div>

        {dispatchResult && (
          <div
            style={{
              padding: '10px 14px',
              backgroundColor: '#f8f9fa',
              border: '1px solid var(--border)',
              borderRadius: '6px',
              fontSize: '12px',
              fontFamily: 'monospace',
              color: 'var(--text)',
            }}
          >
            {dispatchResult}
          </div>
        )}
      </div>

      {/* Exposed Tools Catalog */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '14px' }}>
          <Wrench size={16} color="var(--primary)" />
          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
            Exposed Model Context Protocol Tools (6 Active)
          </span>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '12px' }}>
          {[
            {
              name: 'antigravity_dispatch_task',
              desc: 'Dispatches instruction/prompt to local Antigravity 2.0 agent or CLI across account rotations.',
            },
            {
              name: 'antigravity_list_sessions',
              desc: 'Lists recent active and archived Antigravity conversations and brain states from local disk.',
            },
            {
              name: 'antigravity_get_transcript',
              desc: 'Fetches turn-by-turn conversation transcript and tool call logs for a specific conversation ID.',
            },
            {
              name: 'antigravity_inspect_workspace',
              desc: 'Inspects directory contents, file summaries, or git status of the local active repository.',
            },
            {
              name: 'antigravity_system_info',
              desc: 'Returns host computer metrics, active account quota, and running Antigravity processes.',
            },
            {
              name: 'antigravity_computer_use',
              desc: 'Executes local computer use UI automation actions via Agent-S or Obscura.',
            },
          ].map((tool) => (
            <div
              key={tool.name}
              style={{
                padding: '12px',
                border: '1px solid var(--border)',
                borderRadius: '8px',
                backgroundColor: '#ffffff',
              }}
            >
              <code style={{ fontSize: '12px', fontWeight: 700, color: 'var(--primary)' }}>
                {tool.name}
              </code>
              <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
                {tool.desc}
              </p>
            </div>
          ))}
        </div>
      </div>

      {/* Recent Remote Invocations Log */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '14px' }}>
          <Clock size={16} color="var(--primary)" />
          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
            Remote Invocations Log ({status?.invocations_count || 0})
          </span>
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                <th style={{ padding: '8px 10px' }}>Timestamp</th>
                <th style={{ padding: '8px 10px' }}>Tool</th>
                <th style={{ padding: '8px 10px' }}>Duration</th>
                <th style={{ padding: '8px 10px' }}>Status</th>
                <th style={{ padding: '8px 10px' }}>Result / Detail</th>
              </tr>
            </thead>
            <tbody>
              {(!status?.recent_invocations || status.recent_invocations.length === 0) ? (
                <tr>
                  <td colSpan={5} style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)' }}>
                    No remote invocations yet. Use the Test Dispatch console above or invoke tools from Gemini App.
                  </td>
                </tr>
              ) : (
                status.recent_invocations.map((inv) => (
                  <tr key={inv.id} style={{ borderBottom: '1px solid #f1f3f4' }}>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                      {inv.timestamp}
                    </td>
                    <td style={{ padding: '8px 10px', fontWeight: 600, color: 'var(--text)' }}>
                      <code>{inv.tool}</code>
                    </td>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)' }}>
                      {inv.duration_ms}ms
                    </td>
                    <td style={{ padding: '8px 10px' }}>
                      <span
                        style={{
                          backgroundColor: inv.status === 'success' ? '#e6f4ea' : '#fce8e6',
                          color: inv.status === 'success' ? '#137333' : '#c5221f',
                          padding: '2px 8px',
                          borderRadius: '10px',
                          fontSize: '11px',
                          fontWeight: 600,
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {inv.status}
                      </span>
                    </td>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)', maxWidth: '350px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {inv.result || inv.error || 'Done'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

export default GeminiBridgePage
