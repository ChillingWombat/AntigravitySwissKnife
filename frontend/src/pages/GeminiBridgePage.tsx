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
  Laptop,
  Code2,
  KeyRound,
  Network,
  Play,
  Square,
  Bot,
  Layers,
  FileText,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'
import { api, isAbortError } from '../api'
import { ToggleSwitch } from '../components/ToggleSwitch'

interface BridgeStatusData {
  enabled: boolean
  operating_mode: string
  gateway_url: string
  tunnel_type: string
  tunnel_online: boolean
  auth_token: string
  pairing_pin: string
  surfaces: {
    desktop_app_connected: boolean
    desktop_cdp_port: number
    desktop_active_title?: string
    agy_cli_installed: boolean
    agy_cli_path?: string
    vscode_connected: boolean
    account_resilience: boolean
  }
  tunnel: {
    active: boolean
    type: string
    public_url?: string
    gateway_url: string
    error?: string
    logs?: string[]
  }
  active_lead_session?: {
    conversation_id: string
    title: string
    last_prompt: string
    submission_step: number
    status: string
    last_response?: string
    artifacts?: string[]
    updated_at: string
  }
  recent_attempts?: Array<{
    timestamp: string
    client_ip: string
    method: string
    granted: boolean
    reason: string
  }>
  invocations_count: number
  recent_invocations: Array<{
    id: string
    timestamp: string
    tool: string
    mode?: string
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
  const [copiedUrl, setCopiedUrl] = useState<boolean>(false)
  const [copiedConfig, setCopiedConfig] = useState<boolean>(false)
  const [copiedToken, setCopiedToken] = useState<boolean>(false)
  const [copiedPIN, setCopiedPIN] = useState<boolean>(false)

  // Tunnel controls
  const [tunnelType, setTunnelType] = useState<string>('quick')
  const [customTunnelURL, setCustomTunnelURL] = useState<string>('')
  const [tunnelActionLoading, setTunnelActionLoading] = useState<boolean>(false)
  const [showTunnelLogs, setShowTunnelLogs] = useState<boolean>(false)

  // Mode and testing
  const [modeLoading, setModeLoading] = useState<boolean>(false)
  const [rotatingAuth, setRotatingAuth] = useState<boolean>(false)
  const [testPrompt, setTestPrompt] = useState<string>('List current project status and verify bridge connectivity')
  const [testTool, setTestTool] = useState<string>('antigravity_lead_create_conversation')
  const [dispatching, setDispatching] = useState<boolean>(false)
  const [dispatchResult, setDispatchResult] = useState<string | null>(null)
  const [testingConnection, setTestingConnection] = useState<boolean>(false)

  const loadStatus = async () => {
    try {
      const res = await api.getBridgeStatus()
      setStatus(res)
      if (res?.tunnel?.type) {
        setTunnelType(res.tunnel.type)
      }
    } catch (err) {
      if (isAbortError(err)) return
      console.error('Failed to load bridge status:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadStatus()
    const timer = setInterval(() => {
      loadStatus()
    }, 6000)
    return () => clearInterval(timer)
  }, [])

  const [actionFeedback, setActionFeedback] = useState<{ text: string; isError: boolean } | null>(null)

  const handleTestConnection = async () => {
    try {
      setTestingConnection(true)
      setActionFeedback(null)
      const res = await api.testBridgeConnection()
      if (res && res.status) {
        setStatus(res.status)
      }
      setActionFeedback({ text: res?.message || 'Gemini Bridge connection verified successfully.', isError: false })
    } catch (err: any) {
      setActionFeedback({ text: `Verification failed: ${err?.message || 'Network error'}`, isError: true })
    } finally {
      setTestingConnection(false)
    }
  }

  const handleToggleEnable = async () => {
    try {
      const next = !status?.enabled
      const res = await api.updateBridgeConfig({ enabled: next })
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
      setActionFeedback({
        text: next ? 'Gemini Bridge enabled.' : 'Gemini Bridge disabled.',
        isError: false,
      })
    } catch (err: any) {
      setActionFeedback({ text: `Failed to toggle Gemini Bridge: ${err?.message || 'Unknown error'}`, isError: true })
    }
  }

  const handleStartTunnel = async () => {
    try {
      setTunnelActionLoading(true)
      setActionFeedback(null)
      const res = await api.startBridgeTunnel({
        type: tunnelType,
        custom_url: customTunnelURL,
      })
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
      setActionFeedback({ text: 'Remote access tunnel started successfully.', isError: false })
    } catch (err: any) {
      setActionFeedback({ text: `Tunnel failed to start: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setTunnelActionLoading(false)
    }
  }

  const handleStopTunnel = async () => {
    try {
      setTunnelActionLoading(true)
      setActionFeedback(null)
      const res = await api.stopBridgeTunnel()
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
      setActionFeedback({ text: 'Remote access tunnel stopped.', isError: false })
    } catch (err: any) {
      setActionFeedback({ text: `Failed to stop tunnel: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setTunnelActionLoading(false)
    }
  }

  const handleRotateCredentials = async () => {
    try {
      setRotatingAuth(true)
      setActionFeedback(null)
      const res = await api.rotateBridgeToken()
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
      setActionFeedback({ text: 'Bridge credentials rotated successfully. Update your remote client configuration.', isError: false })
    } catch (err: any) {
      setActionFeedback({ text: `Credential rotation failed: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setRotatingAuth(false)
    }
  }

  const handleSetMode = async (newMode: string) => {
    try {
      setModeLoading(true)
      setActionFeedback(null)
      const res = await api.setBridgeMode(newMode)
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
    } catch (err: any) {
      setActionFeedback({ text: `Failed to set mode: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setModeLoading(false)
    }
  }

  const handleDispatch = async () => {
    if (!testPrompt.trim()) return
    try {
      setDispatching(true)
      setDispatchResult(null)
      const res = await api.dispatchBridgeTask({
        prompt: testPrompt.trim(),
        target: testTool,
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

  const copyToClipboard = (text: string, setter: (val: boolean) => void) => {
    navigator.clipboard.writeText(text)
    setter(true)
    setTimeout(() => setter(false), 2000)
  }

  if (loading && !status) {
    return (
      <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
        Loading Gemini Bridge status...
      </div>
    )
  }

  const isTunnelActive = Boolean(status?.tunnel?.active && status.tunnel.type !== 'loopback')
  const currentMode = status?.operating_mode || 'hybrid'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Overview, Live Status & Enable Toggle */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                Gemini Agent Chat Bridge
              </h2>
              <span className={`badge-chip ${status?.enabled ? 'badge-green' : 'badge-neutral'}`}>
                {status?.enabled ? 'Online' : 'Disabled'}
              </span>
              <span className={`badge-chip ${isTunnelActive ? 'badge-blue' : 'badge-neutral'}`}>
                {isTunnelActive ? (status?.tunnel?.type || 'tunnel').toUpperCase() : 'Loopback'}
              </span>
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Two-way agent orchestration connecting Gemini App / Web to local Antigravity surfaces across account rotations.
            </p>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <button
              onClick={handleTestConnection}
              disabled={testingConnection}
              className="btn-pill-tonal"
            >
              <RefreshCw size={14} className={testingConnection ? 'animate-spin' : ''} />
              <span>Verify Bridge</span>
            </button>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
              <span style={{ whiteSpace: 'nowrap' }}>Enable Bridge</span>
              <ToggleSwitch
                checked={Boolean(status?.enabled)}
                onChange={handleToggleEnable}
              />
            </label>
          </div>
        </div>

        {actionFeedback && (
          <div
            style={{
              padding: '10px 14px',
              borderRadius: '8px',
              border: `1px solid ${actionFeedback.isError ? 'var(--border-danger)' : 'var(--border-success)'}`,
              backgroundColor: actionFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
              fontSize: '12px',
              color: actionFeedback.isError ? 'var(--red)' : 'var(--green)',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              marginBottom: '16px',
            }}
          >
            {actionFeedback.isError ? <AlertCircle size={14} /> : <CheckCircle2 size={14} />}
            <span>{actionFeedback.text}</span>
          </div>
        )}

        {/* Status Metrics Grid (harmonized with Remote, WhatsApp, Telegram, Discord, and BrainCache) */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '12px', marginBottom: '16px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Gateway Status
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: status?.enabled ? 'var(--green)' : 'var(--text-muted)', marginTop: '4px' }}>
              {status?.enabled ? 'Streamable HTTP' : 'Standby'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Port 8765 MCP Server
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Operating Mode
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {currentMode.toUpperCase()}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Direct / Lead / Hybrid
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Remote Internet Pipe
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: isTunnelActive ? 'var(--primary)' : 'var(--text-muted)', marginTop: '4px' }}>
              {isTunnelActive ? (status?.tunnel?.type || 'active').toUpperCase() : 'INACTIVE'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              {isTunnelActive ? (status?.tunnel?.type === 'tailscale' ? 'Tailscale Private VPN' : 'Public HTTPS Tunnel') : 'Remote Access Off'}
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Desktop Surface
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: status?.surfaces?.desktop_app_connected ? 'var(--green)' : 'var(--text-muted)', marginTop: '4px' }}>
              {status?.surfaces?.desktop_app_connected ? 'CDP Connected' : 'Standby'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Port {status?.surfaces?.desktop_cdp_port || 35779}
            </div>
          </div>
        </div>

        {/* Live Gateway URL & Copy */}
        <div
          style={{
            backgroundColor: 'var(--canvas)',
            borderRadius: '8px',
            border: '1px solid var(--border)',
            padding: '12px 14px',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            flexWrap: 'wrap',
            gap: '12px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: '320px', flex: 1 }}>
            <Radio size={16} color="var(--primary)" />
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Active Endpoint:
            </span>
            <code style={{ fontSize: '12px', color: 'var(--primary)', fontWeight: 600, wordBreak: 'break-all' }}>
              {status?.gateway_url || 'http://127.0.0.1:8765/api/bridge/mcp'}
            </code>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <button
              onClick={() => copyToClipboard(status?.gateway_url || '', setCopiedUrl)}
              className="btn-pill-tonal"
              style={{ padding: '6px 12px', fontSize: '12px' }}
            >
              {copiedUrl ? <CheckCircle2 size={13} color="var(--green)" /> : <Copy size={13} />}
              <span>{copiedUrl ? 'Copied' : 'Copy Endpoint'}</span>
            </button>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <ShieldCheck size={16} color="var(--green)" />
              <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--green)', whiteSpace: 'nowrap' }}>
                Account Shield Active
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* 2. Zero-Setup Remote Tunneling */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '12px', marginBottom: '14px' }}>
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '4px' }}>
              Remote Internet Access (Zero-Setup)
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)' }}>
              Connect from Gemini App on mobile phone or web when away from home with zero external account signups.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            {isTunnelActive ? (
              <button
                onClick={handleStopTunnel}
                disabled={tunnelActionLoading}
                className="btn-pill-danger"
              >
                <Square size={13} />
                <span>{tunnelActionLoading ? 'Stopping...' : 'Stop Remote Access'}</span>
              </button>
            ) : (
              <button
                onClick={handleStartTunnel}
                disabled={tunnelActionLoading}
                className="btn-pill-primary"
              >
                <Play size={13} />
                <span>{tunnelActionLoading ? 'Connecting...' : 'Start Remote Access'}</span>
              </button>
            )}
          </div>
        </div>

        {/* Tunnel Selection Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '12px', marginBottom: '12px' }}>
          <div
            onClick={() => setTunnelType('quick')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'quick' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'quick' ? 'var(--canvas)' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <Network size={14} color="var(--primary)" />
                <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                  Quick Tunnel (Recommended)
                </span>
              </div>
              <span className="badge-chip badge-green">
                Zero Signup
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Generates trusted public HTTPS URL automatically via bundled sidecar. No accounts or software installation required.
            </p>
          </div>

          <div
            onClick={() => setTunnelType('tailscale')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'tailscale' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'tailscale' ? 'var(--canvas)' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Tailscale Network
              </span>
              <span className="badge-chip badge-blue">
                Private VPN
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Routes through your personal Tailnet or Tailscale Funnel. Ideal if your phone or client is connected to Tailscale.
            </p>
          </div>

          <div
            onClick={() => setTunnelType('custom')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'custom' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'custom' ? 'var(--canvas)' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Custom HTTPS Tunnel
              </span>
              <span className="badge-chip badge-neutral">
                Reverse Proxy
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Self-hosted Cloudflare Tunnel, ngrok, or reverse proxy endpoint for your custom remote domain.
            </p>
          </div>
        </div>

        {tunnelType === 'custom' && (
          <div style={{ display: 'flex', gap: '8px', marginTop: '10px' }}>
            <input
              type="text"
              placeholder="https://your-domain.com/api/bridge/mcp"
              value={customTunnelURL}
              onChange={(e) => setCustomTunnelURL(e.target.value)}
              style={{
                flex: 1,
                padding: '8px 12px',
                fontSize: '12px',
                borderRadius: '6px',
                border: '1px solid var(--border)',
                backgroundColor: '#ffffff',
              }}
            />
          </div>
        )}

        {/* Tunnel Status & Log Collapse */}
        <div style={{ marginTop: '10px' }}>
          <button
            onClick={() => setShowTunnelLogs(!showTunnelLogs)}
            style={{
              background: 'none',
              border: 'none',
              padding: '4px 0',
              fontSize: '11px',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
              whiteSpace: 'nowrap',
            }}
          >
            {showTunnelLogs ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
            {showTunnelLogs ? 'Hide Connection Logs' : 'View Tunnel Diagnostics & Logs'}
          </button>

          {showTunnelLogs && (
            <pre
              style={{
                marginTop: '8px',
                padding: '12px',
                backgroundColor: 'var(--canvas)',
                borderRadius: '6px',
                border: '1px solid var(--border)',
                fontSize: '11px',
                fontFamily: 'monospace',
                maxHeight: '140px',
                overflowY: 'auto',
                color: 'var(--text)',
              }}
            >
              {status?.tunnel?.logs && status.tunnel.logs.length > 0
                ? status.tunnel.logs.join('\n')
                : 'No tunnel logs yet. Click Start Remote Access to generate an HTTPS endpoint.'}
            </pre>
          )}
        </div>
      </div>

      {/* 3. Authentication & First-Time Setup */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px', flexWrap: 'wrap', gap: '10px' }}>
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '4px' }}>
              Authentication & Pairing Credentials
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)' }}>
              Required when initialising or accessing the MCP on Gemini. Unauthenticated requests are rejected with 401 Unauthorized.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            <button
              onClick={handleRotateCredentials}
              disabled={rotatingAuth}
              className="btn-pill-tonal"
            >
              <RefreshCw size={12} className={rotatingAuth ? 'animate-spin' : ''} />
              <span>Rotate Credentials</span>
            </button>
            <button
              onClick={() => copyToClipboard(mcpConfigSnippet, setCopiedConfig)}
              className="btn-pill-tonal"
            >
              {copiedConfig ? <CheckCircle2 size={13} color="var(--green)" /> : <Copy size={13} />}
              <span>{copiedConfig ? 'Copied' : 'Copy JSON Config'}</span>
            </button>
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '14px' }}>
          {/* Pairing PIN */}
          <div
            style={{
              padding: '14px',
              backgroundColor: 'var(--canvas)',
              borderRadius: '8px',
              border: '1px solid var(--border)',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <KeyRound size={14} color="var(--primary)" />
                <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                  Pairing PIN (Quick Setup)
                </span>
              </div>
              <button
                onClick={() => copyToClipboard(status?.pairing_pin || '', setCopiedPIN)}
                className="btn-pill-tonal"
                style={{ padding: '3px 8px', fontSize: '11px' }}
              >
                {copiedPIN ? <CheckCircle2 size={12} color="var(--green)" /> : <Copy size={12} />}
                <span>{copiedPIN ? 'Copied' : 'Copy PIN'}</span>
              </button>
            </div>
            <div style={{ fontSize: '24px', letterSpacing: '4px', fontWeight: 700, color: 'var(--text)', fontFamily: 'monospace' }}>
              {status?.pairing_pin || '582914'}
            </div>
            <p style={{ margin: '6px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
              Pass as header <code>X-Pairing-PIN</code> or URL query <code>?pin={status?.pairing_pin}</code>.
            </p>
          </div>

          {/* Bearer Token */}
          <div
            style={{
              padding: '14px',
              backgroundColor: 'var(--canvas)',
              borderRadius: '8px',
              border: '1px solid var(--border)',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                Bearer Token (API Key)
              </span>
              <button
                onClick={() => copyToClipboard(status?.auth_token || '', setCopiedToken)}
                className="btn-pill-tonal"
                style={{ padding: '3px 8px', fontSize: '11px' }}
              >
                {copiedToken ? <CheckCircle2 size={12} color="var(--green)" /> : <Copy size={12} />}
                <span>{copiedToken ? 'Copied' : 'Copy Token'}</span>
              </button>
            </div>
            <code style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', wordBreak: 'break-all' }}>
              {status?.auth_token || 'swiss-gemini-auth-token'}
            </code>
            <p style={{ margin: '6px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
              Configured via standard <code>Authorization: Bearer &lt;token&gt;</code> HTTP header.
            </p>
          </div>
        </div>

        {/* JSON Snippet */}
        <div style={{ marginTop: '14px' }}>
          <pre
            style={{
              margin: 0,
              padding: '12px',
              backgroundColor: 'var(--canvas)',
              borderRadius: '6px',
              border: '1px solid var(--border)',
              fontFamily: 'monospace',
              fontSize: '11px',
              color: 'var(--text)',
              overflowX: 'auto',
            }}
          >
            {mcpConfigSnippet}
          </pre>
        </div>
      </div>

      {/* 4. Operational Mode Selection (Direct vs Lead/Messenger) */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '4px' }}>
          Gemini Operating Role & Mode
        </div>
        <p style={{ margin: '0 0 16px', fontSize: '12px', color: 'var(--text-muted)' }}>
          Configure whether Gemini in your chat works directly on the local machine or acts as a lead/messenger dispatching tasks to Antigravity and forwarding responses.
        </p>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '14px' }}>
          {/* Hybrid Mode */}
          <div
            onClick={() => handleSetMode('hybrid')}
            style={{
              padding: '14px',
              borderRadius: '8px',
              border: currentMode === 'hybrid' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: currentMode === 'hybrid' ? 'var(--canvas)' : '#ffffff',
              cursor: modeLoading ? 'not-allowed' : 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
              <Bot size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Hybrid Mode (Recommended)
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
              Gemini has access to both direct tools and lead tools. Gemini dynamically chooses whether to work directly or command Antigravity based on your instruction.
            </p>
          </div>

          {/* Lead / Messenger Mode */}
          <div
            onClick={() => handleSetMode('lead')}
            style={{
              padding: '14px',
              borderRadius: '8px',
              border: currentMode === 'lead' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: currentMode === 'lead' ? 'var(--canvas)' : '#ffffff',
              cursor: modeLoading ? 'not-allowed' : 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
              <Layers size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Lead & Messenger Mode
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
              Gemini acts as your remote team lead. It creates agent conversations in Google Antigravity 2.0, monitors progress, and forwards the completed responses and artifacts back to your chat.
            </p>
          </div>

          {/* Direct Mode */}
          <div
            onClick={() => handleSetMode('direct')}
            style={{
              padding: '14px',
              borderRadius: '8px',
              border: currentMode === 'direct' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: currentMode === 'direct' ? 'var(--canvas)' : '#ffffff',
              cursor: modeLoading ? 'not-allowed' : 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
              <Terminal size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Direct Actuator Mode
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
              Gemini interacts directly with your local files and terminal (executing shell commands, editing files, UI automation) without spawning Antigravity conversations.
            </p>
          </div>
        </div>

        {/* User Interaction Guide */}
        <div
          style={{
            marginTop: '14px',
            backgroundColor: 'var(--canvas)',
            borderRadius: '8px',
            border: '1px solid var(--border)',
            padding: '12px 14px',
            fontSize: '12px',
          }}
        >
          <span style={{ fontWeight: 600, color: 'var(--text)' }}>
            How to direct Gemini in chat:
          </span>
          <ul style={{ margin: '6px 0 0', paddingLeft: '18px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
            <li>
              <strong>To command Antigravity:</strong> <em>"Ask Antigravity to refactor auth and run tests"</em> or <em>"Start a new Antigravity task to inspect bug #12"</em>.
            </li>
            <li>
              <strong>To work on PC directly:</strong> <em>"Work on my PC directly and check git log"</em> or <em>"Directly inspect file /pkg/bridge/bridge.go"</em>.
            </li>
          </ul>
        </div>
      </div>

      {/* 5. Live Antigravity Lead / Messenger Monitor */}
      {status?.active_lead_session && (
        <div className="google-card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px', flexWrap: 'wrap', gap: '8px' }}>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
              Active Antigravity Lead Session
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span
                className={`badge-chip ${status.active_lead_session.status === 'completed' ? 'badge-green' : 'badge-blue'}`}
              >
                {status.active_lead_session.status === 'completed' ? 'Completed & Forwarded' : 'Generating in Antigravity...'}
              </span>
              <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                Conversation: <code>{status.active_lead_session.conversation_id}</code>
              </span>
            </div>
          </div>

          <div style={{ padding: '12px', backgroundColor: 'var(--canvas)', borderRadius: '8px', border: '1px solid var(--border)', marginBottom: '10px' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '2px' }}>
              Last Dispatched Prompt:
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text)', fontWeight: 500 }}>
              {status.active_lead_session.last_prompt}
            </div>
          </div>

          {status.active_lead_session.last_response && (
            <div style={{ padding: '12px', backgroundColor: '#ffffff', borderRadius: '8px', border: '1px solid var(--border)' }}>
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                Forwarded Antigravity Response Preview:
              </div>
              <div style={{ fontSize: '12px', color: 'var(--text)', maxHeight: '120px', overflowY: 'auto', whiteSpace: 'pre-wrap', lineHeight: 1.4 }}>
                {status.active_lead_session.last_response}
              </div>
              {status.active_lead_session.artifacts && status.active_lead_session.artifacts.length > 0 && (
                <div style={{ marginTop: '8px', display: 'flex', gap: '6px', alignItems: 'center', flexWrap: 'wrap' }}>
                  <FileText size={13} color="var(--primary)" />
                  <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>Artifacts:</span>
                  {status.active_lead_session.artifacts.map((art) => (
                    <span
                      key={art}
                      className="badge-chip badge-neutral"
                    >
                      {art}
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* 6. Surface Connectivity Matrix */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
          gap: '16px',
        }}
      >
        {/* Surface 1: Desktop App */}
        <div className="google-card" style={{ padding: '16px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Laptop size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Antigravity 2.0 Desktop
              </span>
            </div>
            <span className={`badge-chip ${status?.surfaces?.desktop_app_connected ? 'badge-green' : 'badge-neutral'}`}>
              {status?.surfaces?.desktop_app_connected ? 'CDP Connected' : 'Standby'}
            </span>
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            Port: <code style={{ color: 'var(--text)' }}>{status?.surfaces?.desktop_cdp_port || '35779'}</code>
            {status?.surfaces?.desktop_active_title && (
              <div style={{ marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                Window: {status.surfaces.desktop_active_title}
              </div>
            )}
          </div>
        </div>

        {/* Surface 2: CLI */}
        <div className="google-card" style={{ padding: '16px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Terminal size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Antigravity CLI (agy)
              </span>
            </div>
            <span className={`badge-chip ${status?.surfaces?.agy_cli_installed ? 'badge-green' : 'badge-neutral'}`}>
              {status?.surfaces?.agy_cli_installed ? 'Ready' : 'Not on PATH'}
            </span>
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            Path: <code style={{ color: 'var(--text)' }}>{status?.surfaces?.agy_cli_path || '~/.local/bin/agy'}</code>
          </div>
        </div>

        {/* Surface 3: VS Code Extension */}
        <div className="google-card" style={{ padding: '16px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Code2 size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                VS Code Extension
              </span>
            </div>
            <span className={`badge-chip ${status?.surfaces?.vscode_connected ? 'badge-green' : 'badge-neutral'}`}>
              {status?.surfaces?.vscode_connected ? 'Detected' : 'Not Connected'}
            </span>
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            {status?.surfaces?.vscode_connected
              ? 'VS Code workspace process detected on host.'
              : 'VS Code process or extension socket not detected.'}
          </div>
        </div>
      </div>

      {/* 7. Test Dispatch Console */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '4px' }}>
          Test Remote Task Dispatch
        </div>
        <p style={{ margin: '0 0 12px', fontSize: '12px', color: 'var(--text-muted)' }}>
          Simulate a remote request from Gemini App to steer local Antigravity.
        </p>

        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '12px' }}>
          <select
            value={testTool}
            onChange={(e) => setTestTool(e.target.value)}
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
            <option value="antigravity_lead_create_conversation">Lead: Create New Conversation</option>
            <option value="antigravity_lead_send_message">Lead: Send Follow-Up Message</option>
            <option value="local_run_command">Direct: Run Local Command</option>
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
            className="btn-pill-primary"
          >
            <Send size={13} />
            <span>{dispatching ? 'Dispatching...' : 'Dispatch Task'}</span>
          </button>
        </div>

        {dispatchResult && (
          <div
            style={{
              padding: '10px 14px',
              backgroundColor: 'var(--canvas)',
              border: '1px solid var(--border)',
              borderRadius: '6px',
              fontSize: '12px',
              fontFamily: 'monospace',
              color: 'var(--text)',
              whiteSpace: 'pre-wrap',
            }}
          >
            {dispatchResult}
          </div>
        )}
      </div>

      {/* 8. Exposed Tools Catalog */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '14px' }}>
          Exposed Model Context Protocol Tools
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '12px' }}>
          {[
            {
              name: 'antigravity_lead_create_conversation',
              type: 'Lead',
              desc: 'Creates a new conversation in local Antigravity 2.0 app and submits initial task instructions.',
            },
            {
              name: 'antigravity_lead_send_message',
              type: 'Lead',
              desc: 'Sends follow-up directions, corrections, or task assignments to Antigravity conversation.',
            },
            {
              name: 'antigravity_lead_await_response',
              type: 'Lead',
              desc: 'Waits for Antigravity response turn and forwards completed response and artifacts to Gemini.',
            },
            {
              name: 'antigravity_lead_get_status',
              type: 'Lead',
              desc: 'Inspects real-time step status, thinking, and completion state of an active Antigravity session.',
            },
            {
              name: 'local_run_command',
              type: 'Direct',
              desc: 'Directly executes a safe shell command on the host machine with timeout protection.',
            },
            {
              name: 'local_read_file',
              type: 'Direct',
              desc: 'Reads file contents from the host filesystem within workspace bounds.',
            },
            {
              name: 'local_write_file',
              type: 'Direct',
              desc: 'Writes or updates file contents on the local computer.',
            },
            {
              name: 'local_computer_use',
              type: 'Direct',
              desc: 'Controls GUI and captures screenshots via display subsystem automation.',
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
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                <code style={{ fontSize: '12px', fontWeight: 700, color: 'var(--primary)' }}>
                  {tool.name}
                </code>
                <span
                  className={`badge-chip ${tool.type === 'Lead' ? 'badge-blue' : 'badge-green'}`}
                  style={{ fontSize: '10px', padding: '1px 6px' }}
                >
                  {tool.type}
                </span>
              </div>
              <p style={{ margin: '4px 0 0', fontSize: '11px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
                {tool.desc}
              </p>
            </div>
          ))}
        </div>
      </div>

      {/* 9. Recent Invocations & Access Audit Logs */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '14px' }}>
          Remote Invocations & Access Audit ({status?.invocations_count || 0})
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                <th style={{ padding: '8px 10px' }}>Timestamp</th>
                <th style={{ padding: '8px 10px' }}>Tool / Event</th>
                <th style={{ padding: '8px 10px' }}>Mode</th>
                <th style={{ padding: '8px 10px' }}>Duration</th>
                <th style={{ padding: '8px 10px' }}>Status</th>
                <th style={{ padding: '8px 10px' }}>Result / Detail</th>
              </tr>
            </thead>
            <tbody>
              {(!status?.recent_invocations || status.recent_invocations.length === 0) ? (
                <tr>
                  <td colSpan={6} style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)' }}>
                    No remote invocations yet. Use the Test Dispatch console above or invoke tools from Gemini App.
                  </td>
                </tr>
              ) : (
                status.recent_invocations.map((inv) => (
                  <tr key={inv.id} style={{ borderBottom: '1px solid var(--border)' }}>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                      {inv.timestamp}
                    </td>
                    <td style={{ padding: '8px 10px', fontWeight: 600, color: 'var(--text)' }}>
                      <code>{inv.tool}</code>
                    </td>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)' }}>
                      {inv.mode || 'hybrid'}
                    </td>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)' }}>
                      {inv.duration_ms}ms
                    </td>
                    <td style={{ padding: '8px 10px' }}>
                      <span className={`badge-chip ${inv.status === 'success' ? 'badge-green' : 'badge-red'}`}>
                        {inv.status}
                      </span>
                    </td>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)', maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
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
